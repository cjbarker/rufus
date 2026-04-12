package cmd

import (
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cjbarker/rufus/internal/crawler"
	"github.com/cjbarker/rufus/internal/db"
	"github.com/cjbarker/rufus/internal/hasher"
	"github.com/spf13/cobra"
)

var (
	scanRecursive bool
	scanUpdate    bool
)

var scanCmd = &cobra.Command{
	Use:   "scan <path> [paths...]",
	Short: "Crawl directories and index images",
	Long: `Scan recursively walks the given directories, discovers image files,
computes perceptual hashes (aHash, dHash, pHash) and SHA-256 file hashes,
and stores the results in the index database.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runScan,
}

func init() {
	scanCmd.Flags().BoolVarP(&scanRecursive, "recursive", "r", true, "recurse into subdirectories")
	scanCmd.Flags().BoolVar(&scanUpdate, "update", false, "only process new/modified files")
	rootCmd.AddCommand(scanCmd)
}

func runScan(cmd *cobra.Command, args []string) error {
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer store.Close()

	start := time.Now()
	var filesFound, filesIndexed, filesSkipped, errors atomic.Int64

	for _, root := range args {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			return fmt.Errorf("resolving path %q: %w", root, err)
		}

		if cfg.Verbose {
			fmt.Printf("Scanning %s...\n", absRoot)
		}

		results := crawler.Crawl(absRoot, scanRecursive)

		// Fan out to worker pool
		type hashJob struct {
			path string
			size int64
		}
		jobs := make(chan hashJob, 256)
		var wg sync.WaitGroup

		// Collect results for batched DB writes
		type indexResult struct {
			rec *db.ImageRecord
			err error
		}
		indexed := make(chan indexResult, 256)

		// Worker pool: hash images
		for i := 0; i < cfg.Workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for job := range jobs {
					hr, err := hasher.HashFile(job.path)
					if err != nil {
						indexed <- indexResult{err: fmt.Errorf("%s: %w", job.path, err)}
						continue
					}
					indexed <- indexResult{rec: &db.ImageRecord{
						FilePath: hr.FilePath,
						FileSize: job.size,
						FileHash: hr.FileHash,
						Width:    hr.Width,
						Height:   hr.Height,
						Format:   hr.Format,
						ModTime:  time.Now(), // Would use file mod time in production
						AHash:    hr.AHash,
						DHash:    hr.DHash,
						PHash:    hr.PHash,
					}}
				}
			}()
		}

		// DB writer goroutine
		var dbWg sync.WaitGroup
		dbWg.Add(1)
		go func() {
			defer dbWg.Done()
			for res := range indexed {
				if res.err != nil {
					errors.Add(1)
					if cfg.Verbose {
						log.Printf("error: %v", res.err)
					}
					continue
				}
				if scanUpdate {
					existing, err := store.GetImageByPath(res.rec.FilePath)
					if err == nil && existing != nil {
						filesSkipped.Add(1)
						continue
					}
				}
				if _, err := store.InsertImage(res.rec); err != nil {
					errors.Add(1)
					if cfg.Verbose {
						log.Printf("db error: %v", err)
					}
					continue
				}
				filesIndexed.Add(1)
				if cfg.Verbose {
					fmt.Printf("  indexed: %s (%dx%d %s)\n", res.rec.FilePath, res.rec.Width, res.rec.Height, res.rec.Format)
				}
			}
		}()

		// Feed crawler results to workers
		for result := range results {
			if result.Err != nil {
				errors.Add(1)
				if cfg.Verbose {
					log.Printf("crawl error: %v", result.Err)
				}
				continue
			}
			filesFound.Add(1)
			jobs <- hashJob{path: result.Path, size: result.Size}
		}
		close(jobs)
		wg.Wait()
		close(indexed)
		dbWg.Wait()
	}

	elapsed := time.Since(start)
	fmt.Printf("\nScan complete in %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("  Found:   %d images\n", filesFound.Load())
	fmt.Printf("  Indexed: %d images\n", filesIndexed.Load())
	fmt.Printf("  Skipped: %d images\n", filesSkipped.Load())
	fmt.Printf("  Errors:  %d\n", errors.Load())

	return nil
}
