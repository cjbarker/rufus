package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cjbarker/rufus/internal/db"
	"github.com/cjbarker/rufus/internal/duplicates"
	"github.com/cjbarker/rufus/internal/ui"
	"github.com/spf13/cobra"
)

// exactBadge is the styled label shown in group headers for SHA-256 exact matches.
var exactBadge = ui.ErrorStyle.Render("EXACT DUPLICATE")

var (
	dupesThreshold int
	dupesHash      string
	dupesFormat    string
	dupesYes       bool
)

var dupesCmd = &cobra.Command{
	Use:   "dupes",
	Short: "Find and report duplicate images",
	Long: `Analyze indexed images to find duplicates using perceptual hashing.
Images are grouped by visual similarity based on the configured
hash algorithm and Hamming distance threshold.`,
	RunE: runDupes,
}

func init() {
	dupesCmd.Flags().IntVar(&dupesThreshold, "threshold", 10, "Hamming distance threshold for similarity")
	dupesCmd.Flags().StringVar(&dupesHash, "hash", "dhash", "hash algorithm: ahash, dhash, phash")
	dupesCmd.Flags().StringVar(&dupesFormat, "format", "table", "output format: table, json, csv")
	dupesCmd.Flags().BoolVarP(&dupesYes, "yes", "y", false, "auto-confirm deletion of exact duplicate files without prompting")
	rootCmd.AddCommand(dupesCmd)
}

func runDupes(cmd *cobra.Command, args []string) error {
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = store.Close() }()

	spinner := ui.NewSpinner("Loading indexed images...")
	spinner.Start()

	images, err := store.GetAllImages()
	if err != nil {
		spinner.StopWithError("Failed to load images")
		return fmt.Errorf("loading images: %w", err)
	}

	if len(images) == 0 {
		spinner.StopWithMessage("")
		ui.WarningMessage("No images indexed. Run 'rufus scan' first.")
		return nil
	}

	spinner.UpdateMessage(fmt.Sprintf("Analyzing %s images for duplicates...",
		ui.Highlight.Render(fmt.Sprintf("%d", len(images)))))

	hashType := duplicates.HashType(dupesHash)
	groups := duplicates.FindDuplicates(images, hashType, dupesThreshold)

	if len(groups) == 0 {
		spinner.StopWithSuccess("Analysis complete")
		fmt.Println()
		ui.SuccessMessage("No duplicates found. Your library is clean!")
		return nil
	}

	spinner.StopWithSuccess(fmt.Sprintf("Found %s duplicate groups",
		ui.WarningStyle.Render(fmt.Sprintf("%d", len(groups)))))

	switch dupesFormat {
	case "json":
		return outputDupesJSON(groups)
	case "csv":
		return outputDupesCSV(groups)
	default:
		return outputDupesTable(groups, store, dupesYes)
	}
}

func outputDupesTable(groups []duplicates.Group, store *db.Store, autoYes bool) error {
	totalDupes := 0
	for _, g := range groups {
		totalDupes += len(g.Images) - 1 // subtract 1 for the "keep" image
	}

	fmt.Println()
	ui.SectionHeader("Duplicate Report")
	fmt.Println()
	ui.StatusLine("Groups", fmt.Sprintf("%d", len(groups)))
	ui.StatusLine("Duplicates", fmt.Sprintf("%d images", totalDupes))
	fmt.Println()

	divider := ui.SeparatorStyle.Render("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for i, group := range groups {
		ranked := duplicates.RankForKeeping(group)
		isExact := group.HashType == duplicates.Exact

		// Group header
		fmt.Println(divider)
		var hashLabel string
		if isExact {
			hashLabel = exactBadge
		} else {
			hashLabel = ui.FormatStyle.Render(string(group.HashType)) +
				"   " + ui.Dim.Render(fmt.Sprintf("distance: %d", group.MaxDistance))
		}
		fmt.Printf("  %s   %s   %s\n",
			ui.Highlight.Render(fmt.Sprintf("Group %d", i+1)),
			ui.Bold.Render(fmt.Sprintf("%d images", len(group.Images))),
			hashLabel,
		)
		fmt.Println(divider)
		fmt.Println()

		// Image cards
		for j, img := range ranked {
			badge := ui.RemoveBadge
			if j == 0 {
				badge = ui.KeepBadge
			}
			meta := fmt.Sprintf("%s   %s   %s",
				ui.SizeStyle.Render(formatSize(img.FileSize)),
				ui.Dim.Render(fmt.Sprintf("%dx%d", img.Width, img.Height)),
				ui.FormatStyle.Render(img.Format),
			)
			fmt.Printf("  %s  %s\n", badge, ui.FileLink(img.FilePath))
			fmt.Printf("          %s\n\n", meta)
		}

		// Prompt to delete exact duplicates (all images except the recommended keep)
		if isExact {
			toDelete := ranked[1:]
			noun := "duplicate"
			if len(toDelete) > 1 {
				noun = "duplicates"
			}
			if autoYes || ui.Confirm(fmt.Sprintf("Delete %d exact %s?", len(toDelete), noun), false) {
				for _, img := range toDelete {
					// Remove from the index first. If this fails the file is
					// untouched and the state remains consistent.
					if err := store.DeleteImage(img.ID); err != nil {
						ui.ErrorMessage(fmt.Sprintf("Failed to update index, skipping deletion: %v", err))
						continue
					}
					// Index entry is gone; now remove the file. If this fails
					// the file survives on disk and will be re-indexed on the
					// next scan, which is safe.
					if err := os.Remove(img.FilePath); err != nil {
						ui.ErrorMessage(fmt.Sprintf("Removed from index but could not delete file: %v", err))
						continue
					}
					ui.SuccessMessage(fmt.Sprintf("Deleted %s", img.FilePath))
				}
			}
			fmt.Println()
		}
	}
	return nil
}

type dupesJSONOutput struct {
	Groups []dupesJSONGroup `json:"groups"`
}

type dupesJSONGroup struct {
	GroupID     int              `json:"group_id"`
	HashType   string           `json:"hash_type"`
	MaxDistance int              `json:"max_distance"`
	Images     []dupesJSONImage `json:"images"`
}

type dupesJSONImage struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Format      string `json:"format"`
	Recommended bool   `json:"recommended_keep"`
}

func outputDupesJSON(groups []duplicates.Group) error {
	output := dupesJSONOutput{Groups: make([]dupesJSONGroup, len(groups))}

	for i, group := range groups {
		ranked := duplicates.RankForKeeping(group)
		jg := dupesJSONGroup{
			GroupID:     i + 1,
			HashType:   string(group.HashType),
			MaxDistance: group.MaxDistance,
			Images:     make([]dupesJSONImage, len(ranked)),
		}
		for j, img := range ranked {
			jg.Images[j] = dupesJSONImage{
				Path:        img.FilePath,
				Size:        img.FileSize,
				Width:       img.Width,
				Height:      img.Height,
				Format:      img.Format,
				Recommended: j == 0,
			}
		}
		output.Groups[i] = jg
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func outputDupesCSV(groups []duplicates.Group) error {
	w := csv.NewWriter(os.Stdout)
	defer w.Flush()

	if err := w.Write([]string{"group", "path", "size", "width", "height", "format", "recommended_keep"}); err != nil {
		return err
	}

	for i, group := range groups {
		ranked := duplicates.RankForKeeping(group)
		for j, img := range ranked {
			keep := "no"
			if j == 0 {
				keep = "yes"
			}
			if err := w.Write([]string{
				fmt.Sprintf("%d", i+1),
				img.FilePath,
				fmt.Sprintf("%d", img.FileSize),
				fmt.Sprintf("%d", img.Width),
				fmt.Sprintf("%d", img.Height),
				img.Format,
				keep,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1fGB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
