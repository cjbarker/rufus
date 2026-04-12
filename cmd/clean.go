package cmd

import (
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/cjbarker/rufus/internal/db"
	"github.com/cjbarker/rufus/internal/ui"
	"github.com/spf13/cobra"
)

var (
	cleanDryRun bool
	cleanVacuum bool
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove index entries for files no longer on disk",
	Long: `Scan the index for image records whose files no longer exist on disk
and remove them. Associated face and tag records are removed automatically.
Use --dry-run to preview what would be removed without making any changes.
Use --vacuum to reclaim disk space after removals.`,
	RunE: runClean,
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanDryRun, "dry-run", false, "preview removals without deleting anything")
	cleanCmd.Flags().BoolVar(&cleanVacuum, "vacuum", false, "reclaim disk space from the database after removing stale entries")
	rootCmd.AddCommand(cleanCmd)
}

func runClean(cmd *cobra.Command, args []string) error {
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = store.Close() }()

	spinner := ui.NewSpinner("Loading index...")
	spinner.Start()

	images, err := store.GetAllImages()
	if err != nil {
		spinner.StopWithError("Failed to load index")
		return fmt.Errorf("loading images: %w", err)
	}
	spinner.StopWithSuccess(fmt.Sprintf("Loaded %d indexed images", len(images)))

	if len(images) == 0 {
		ui.InfoMessage("Index is empty.")
		return nil
	}

	// Find stale records by checking each path concurrently.
	numWorkers := cfg.Workers
	if numWorkers > len(images) {
		numWorkers = len(images)
	}

	type statResult struct {
		img   db.ImageRecord
		stale bool
	}

	jobs := make(chan db.ImageRecord, numWorkers*2)
	results := make(chan statResult, numWorkers*2)

	var wg sync.WaitGroup
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for img := range jobs {
				_, statErr := os.Stat(img.FilePath)
				results <- statResult{img: img, stale: os.IsNotExist(statErr)}
			}
		}()
	}

	go func() {
		for _, img := range images {
			jobs <- img
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var stale []db.ImageRecord
	for r := range results {
		if r.stale {
			stale = append(stale, r.img)
		}
	}
	// Sort for consistent output order regardless of goroutine scheduling.
	sort.Slice(stale, func(i, j int) bool {
		return stale[i].FilePath < stale[j].FilePath
	})

	if len(stale) == 0 {
		fmt.Println()
		ui.SuccessMessage("Index is clean — all indexed files are present on disk.")
		return nil
	}

	fmt.Println()
	ui.SectionHeader(fmt.Sprintf("%d stale index entry/entries", len(stale)))
	fmt.Println()

	tbl := ui.NewTable("PATH")
	for _, img := range stale {
		tbl.AddRow(ui.Dim.Render(img.FilePath))
	}
	tbl.Render()
	fmt.Println()

	if cleanDryRun {
		ui.InfoMessage(fmt.Sprintf("Dry run: %d record(s) would be removed. Re-run without --dry-run to apply.", len(stale)))
		return nil
	}

	removed := 0
	for _, img := range stale {
		if err := store.DeleteImage(img.ID); err != nil {
			ui.ErrorMessage(fmt.Sprintf("Failed to remove %s: %v", img.FilePath, err))
			continue
		}
		removed++
	}

	ui.SuccessMessage(fmt.Sprintf("Removed %d stale record(s) from the index.", removed))

	if cleanVacuum && removed > 0 {
		spinner := ui.NewSpinner("Vacuuming database...")
		spinner.Start()
		if err := store.Vacuum(); err != nil {
			spinner.StopWithError("Vacuum failed")
			return fmt.Errorf("vacuuming database: %w", err)
		}
		spinner.StopWithSuccess("Database vacuumed")
	}

	return nil
}
