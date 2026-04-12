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
	scanExcludes  []string
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
	scanCmd.Flags().StringArrayVar(&scanExcludes, "exclude", nil, "directory names or paths to exclude (repeatable)")
	rootCmd.AddCommand(scanCmd)
}

func runScan(cmd *cobra.Command, args []string) error {
	release, err := db.AcquireLock(cfg.DBPath)
	if err != nil {
		return err
	}
	defer release()

	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = store.Close() }()

	start := time.Now()
	var filesFound, filesIndexed, filesSkipped, errors atomic.Int64

	// For --update mode, preload all existing image metadata once so workers
	// can skip unchanged files before performing any hashing (O(1) map lookup
	// instead of one DB query per file).
	var existingByPath map[string]db.ImageRecord
	if scanUpdate {
		spinner := ui.NewSpinner("Loading existing index...")
		spinner.Start()
		existing, loadErr := store.GetAllImages()
		if loadErr != nil {
			spinner.StopWithError("Failed to load index")
			return fmt.Errorf("loading existing images: %w", loadErr)
		}
		existingByPath = make(map[string]db.ImageRecord, len(existing))
		for _, img := range existing {
			existingByPath[img.FilePath] = img
		}
		spinner.StopWithSuccess(fmt.Sprintf("Loaded %d existing records", len(existing)))
	}

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

		results := crawler.Crawl(absRoot, scanRecursive, scanExcludes)

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

		type indexResult struct {
			rec  *db.ImageRecord
			err  error
			skip bool // true when --update determines file is unchanged
		}
		indexed := make(chan indexResult, 256)

		// Worker pool: check skip, then hash images.
		for i := 0; i < cfg.Workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for job := range jobs {
					// Skip unchanged files before the expensive hash operation.
					if scanUpdate {
						if ex, ok := existingByPath[job.path]; ok &&
							ex.FileSize == job.size &&
							ex.ModTime.Equal(job.modTime) {
							indexed <- indexResult{skip: true}
							continue
						}
					}
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

		// DB writer goroutine — collects records into batches of scanBatchSize
		// and flushes them in a single transaction for significantly lower I/O.
		const scanBatchSize = 100
		var dbWg sync.WaitGroup
		dbWg.Add(1)
		go func() {
			defer dbWg.Done()
			var batch []*db.ImageRecord

			flush := func() {
				if len(batch) == 0 {
					return
				}
				if err := store.InsertImageBatch(batch); err != nil {
					errors.Add(int64(len(batch)))
					if cfg.Verbose {
						log.Printf("batch db error: %v", err)
					}
				} else {
					filesIndexed.Add(int64(len(batch)))
				}
				batch = batch[:0]
			}

			for res := range indexed {
				progress.Increment()
				switch {
				case res.skip:
					filesSkipped.Add(1)
				case res.err != nil:
					errors.Add(1)
					if cfg.Verbose {
						log.Printf("error: %v", res.err)
					}
				default:
					if cfg.Verbose {
						fmt.Printf("\r\033[K  %s %s (%dx%d %s)\n",
							ui.SuccessStyle.Render("✔"),
							ui.FileLink(res.rec.FilePath),
							res.rec.Width, res.rec.Height,
							ui.FormatStyle.Render(res.rec.Format))
					}
					batch = append(batch, res.rec)
					if len(batch) >= scanBatchSize {
						flush()
					}
				}
			}
			flush() // commit any remaining records
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
