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

// moveCandidate pairs a stale record with the live record it was moved to.
type moveCandidate struct {
	old db.ImageRecord
	new db.ImageRecord
}

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

	// Detect moves: for each stale record, check if another DB record with the
	// same SHA-256 hash exists and its file is present on disk.
	var moved []moveCandidate
	var trulyStale []db.ImageRecord

	for _, img := range stale {
		candidates, err := store.GetImagesByHash(img.FileHash)
		if err != nil {
			// Non-fatal — treat as stale if hash lookup fails.
			trulyStale = append(trulyStale, img)
			continue
		}

		var liveMatch *db.ImageRecord
		for i := range candidates {
			if candidates[i].ID == img.ID {
				continue // skip the stale record itself
			}
			if _, statErr := os.Stat(candidates[i].FilePath); statErr == nil {
				liveMatch = &candidates[i]
				break
			}
		}

		if liveMatch != nil {
			moved = append(moved, moveCandidate{old: img, new: *liveMatch})
		} else {
			trulyStale = append(trulyStale, img)
		}
	}

	fmt.Println()

	// Display moved files.
	if len(moved) > 0 {
		ui.SectionHeader(fmt.Sprintf("%d moved — metadata will be preserved", len(moved)))
		fmt.Println()
		tbl := ui.NewTable("OLD PATH", "NEW PATH")
		for _, m := range moved {
			tbl.AddRow(ui.Dim.Render(m.old.FilePath), ui.FileLink(m.new.FilePath))
		}
		tbl.Render()
		fmt.Println()
	}

	// Display truly stale files.
	if len(trulyStale) > 0 {
		ui.SectionHeader(fmt.Sprintf("%d stale — file missing from disk", len(trulyStale)))
		fmt.Println()
		tbl := ui.NewTable("PATH")
		for _, img := range trulyStale {
			tbl.AddRow(ui.Dim.Render(img.FilePath))
		}
		tbl.Render()
		fmt.Println()
	}

	if cleanDryRun {
		parts := []string{}
		if len(moved) > 0 {
			parts = append(parts, fmt.Sprintf("%d moved (metadata migration)", len(moved)))
		}
		if len(trulyStale) > 0 {
			parts = append(parts, fmt.Sprintf("%d removed", len(trulyStale)))
		}
		msg := "Dry run"
		for i, p := range parts {
			if i == 0 {
				msg += ": " + p
			} else {
				msg += ", " + p
			}
		}
		msg += ". Re-run without --dry-run to apply."
		ui.InfoMessage(msg)
		return nil
	}

	// Apply: migrate moved records, delete stale records.
	migrated := 0
	for _, m := range moved {
		if err := store.MigrateImageMetadata(m.old.ID, m.new.ID); err != nil {
			ui.ErrorMessage(fmt.Sprintf("Failed to migrate %s: %v", m.old.FilePath, err))
			continue
		}
		migrated++
	}

	removed := 0
	for _, img := range trulyStale {
		if err := store.DeleteImage(img.ID); err != nil {
			ui.ErrorMessage(fmt.Sprintf("Failed to remove %s: %v", img.FilePath, err))
			continue
		}
		removed++
	}

	if migrated > 0 {
		ui.SuccessMessage(fmt.Sprintf("Migrated metadata for %d moved file(s).", migrated))
	}
	if removed > 0 {
		ui.SuccessMessage(fmt.Sprintf("Removed %d stale record(s) from the index.", removed))
	}

	total := migrated + removed
	if cleanVacuum && total > 0 {
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
