package cmd

import (
	"fmt"

	"github.com/cjbarker/rufus/internal/db"
	"github.com/cjbarker/rufus/internal/ui"
	"github.com/cjbarker/rufus/internal/util"
	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show database statistics",
	Long:  `Display aggregate counts of indexed images, detected faces, named people, and tags, along with database file size.`,
	RunE:  runStats,
}

func init() {
	rootCmd.AddCommand(statsCmd)
}

func runStats(cmd *cobra.Command, args []string) error {
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = store.Close() }()

	spinner := ui.NewSpinner("Gathering statistics...")
	spinner.Start()

	stats, err := store.GetStats(cfg.DBPath)
	if err != nil {
		spinner.StopWithError("Failed to read stats")
		return fmt.Errorf("reading stats: %w", err)
	}
	spinner.Stop()

	fmt.Println()
	ui.SectionHeader("Database Statistics")
	fmt.Println()
	ui.StatusLine("Database", cfg.DBPath)
	if stats.DBSizeBytes > 0 {
		ui.StatusLine("Size", util.FormatSize(stats.DBSizeBytes))
	}
	fmt.Println()
	ui.StatusLine("Images", fmt.Sprintf("%d", stats.Images))
	ui.StatusLine("Faces", fmt.Sprintf("%d", stats.Faces))
	ui.StatusLine("People", fmt.Sprintf("%d", stats.People))
	ui.StatusLine("Tags", fmt.Sprintf("%d", stats.Tags))
	fmt.Println()

	return nil
}
