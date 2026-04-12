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
	"github.com/cjbarker/rufus/internal/ui"
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

		ui.SectionHeader("Scanning")
		ui.StatusLine("Path", absRoot)
		ui.StatusLine("Workers", fmt.Sprintf("%d", cfg.Workers))
		if scanUpdate {
			ui.StatusLine("Mode", "incremental (update only)")
		}
		fmt.Println()

		// Phase 1: Discover images with spinner
		spinner := ui.NewSpinner(fmt.Sprintf("Discovering images in %s...", ui.PathStyle.Render(absRoot)))
		spinner.Start()

		results := crawler.Crawl(absRoot, scanRecursive)

		// Collect all results first to know total for progress bar
		type crawlResult struct {
			path    string
			size    int64
			modTime time.Time
			err     error
		}
		var crawlResults []crawlResult
		for result := range results {
			if result.Err != nil {
				errors.Add(1)
				if cfg.Verbose {
					log.Printf("crawl error: %v", result.Err)
				}
				crawlResults = append(crawlResults, crawlResult{err: result.Err})
				continue
			}
			filesFound.Add(1)
			crawlResults = append(crawlResults, crawlResult{path: result.Path, size: result.Size, modTime: result.ModTime})
			spinner.UpdateMessage(fmt.Sprintf("Discovering images... %s found", ui.Highlight.Render(fmt.Sprintf("%d", filesFound.Load()))))
		}

		found := filesFound.Load()
		spinner.StopWithSuccess(fmt.Sprintf("Discovered %s images", ui.Highlight.Render(fmt.Sprintf("%d", found))))

		if found == 0 {
			ui.WarningMessage("No images found in the specified path.")
			continue
		}

		// Phase 2: Hash and index with progress bar
		progress := ui.NewProgress(
			ui.InfoStyle.Render("Indexing"),
			found,
		)
		progress.Start()

		// Fan out to worker pool
		type hashJob struct {
			path    string
			size    int64
			modTime time.Time
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
						ModTime:  job.modTime,
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
					progress.Increment()
					continue
				}
				if scanUpdate {
					existing, err := store.GetImageByPath(res.rec.FilePath)
					if err == nil && existing != nil &&
						existing.FileSize == res.rec.FileSize &&
						existing.ModTime.Equal(res.rec.ModTime) {
						filesSkipped.Add(1)
						progress.Increment()
						continue
					}
				}
				if _, err := store.InsertImage(res.rec); err != nil {
					errors.Add(1)
					if cfg.Verbose {
						log.Printf("db error: %v", err)
					}
					progress.Increment()
					continue
				}
				filesIndexed.Add(1)
				progress.Increment()
				if cfg.Verbose {
					fmt.Printf("\r\033[K  %s %s (%dx%d %s)\n",
						ui.SuccessStyle.Render("✔"),
						ui.FileLink(res.rec.FilePath),
						res.rec.Width, res.rec.Height,
						ui.FormatStyle.Render(res.rec.Format))
				}
			}
		}()

		// Feed collected results to workers
		for _, cr := range crawlResults {
			if cr.err != nil {
				continue
			}
			jobs <- hashJob{path: cr.path, size: cr.size, modTime: cr.modTime}
		}
		close(jobs)
		wg.Wait()
		close(indexed)
		dbWg.Wait()
		progress.Stop()
	}

	elapsed := time.Since(start)
	fmt.Println()
	ui.SectionHeader("Scan Complete")
	fmt.Println()
	ui.StatusLine("Duration", elapsed.Round(time.Millisecond).String())
	ui.StatusLine("Found", fmt.Sprintf("%d images", filesFound.Load()))

	indexedCount := filesIndexed.Load()
	if indexedCount > 0 {
		fmt.Printf("  %s %s\n",
			ui.Dim.Render("Indexed:"),
			ui.SuccessStyle.Render(fmt.Sprintf("%d images", indexedCount)))
	}

	skippedCount := filesSkipped.Load()
	if skippedCount > 0 {
		fmt.Printf("  %s %s\n",
			ui.Dim.Render("Skipped:"),
			ui.WarningStyle.Render(fmt.Sprintf("%d images", skippedCount)))
	}

	errCount := errors.Load()
	if errCount > 0 {
		fmt.Printf("  %s %s\n",
			ui.Dim.Render("Errors:"),
			ui.ErrorStyle.Render(fmt.Sprintf("%d", errCount)))
	}
	fmt.Println()

	return nil
}
