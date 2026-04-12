package cmd

import (
	"fmt"
	"os"

	"github.com/cjbarker/rufus/internal/db"
	"github.com/cjbarker/rufus/internal/ui"
	"github.com/spf13/cobra"
)

var cleanDryRun bool

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove index entries for files no longer on disk",
	Long: `Scan the index for image records whose files no longer exist on disk
and remove them. Associated face and tag records are removed automatically.
Use --dry-run to preview what would be removed without making any changes.`,
	RunE: runClean,
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanDryRun, "dry-run", false, "preview removals without deleting anything")
	rootCmd.AddCommand(cleanCmd)
}

func runClean(cmd *cobra.Command, args []string) error {
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer store.Close()

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

	// Find stale records.
	var stale []db.ImageRecord
	for _, img := range images {
		if _, err := os.Stat(img.FilePath); os.IsNotExist(err) {
			stale = append(stale, img)
		}
	}

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
	return nil
}
