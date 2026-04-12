package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/cjbarker/rufus/internal/db"
	"github.com/cjbarker/rufus/internal/duplicates"
	"github.com/spf13/cobra"
)

var (
	dupesThreshold int
	dupesHash      string
	dupesFormat    string
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
	rootCmd.AddCommand(dupesCmd)
}

func runDupes(cmd *cobra.Command, args []string) error {
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer store.Close()

	images, err := store.GetAllImages()
	if err != nil {
		return fmt.Errorf("loading images: %w", err)
	}

	if len(images) == 0 {
		fmt.Println("No images indexed. Run 'rufus scan' first.")
		return nil
	}

	hashType := duplicates.HashType(dupesHash)
	groups := duplicates.FindDuplicates(images, hashType, dupesThreshold)

	if len(groups) == 0 {
		fmt.Println("No duplicates found.")
		return nil
	}

	switch dupesFormat {
	case "json":
		return outputDupesJSON(groups)
	case "csv":
		return outputDupesCSV(groups)
	default:
		outputDupesTable(groups)
		return nil
	}
}

func outputDupesTable(groups []duplicates.Group) {
	fmt.Printf("Found %d duplicate groups:\n\n", len(groups))

	for i, group := range groups {
		ranked := duplicates.RankForKeeping(group)
		fmt.Printf("Group %d (%d images, max distance: %d, hash: %s)\n",
			i+1, len(group.Images), group.MaxDistance, group.HashType)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  KEEP\tPATH\tSIZE\tRESOLUTION\tFORMAT\n")
		for j, img := range ranked {
			keep := " "
			if j == 0 {
				keep = "*"
			}
			fmt.Fprintf(w, "  %s\t%s\t%s\t%dx%d\t%s\n",
				keep, img.FilePath, formatSize(img.FileSize),
				img.Width, img.Height, img.Format)
		}
		w.Flush()
		fmt.Println()
	}
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
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Format     string `json:"format"`
	Recommended bool  `json:"recommended_keep"`
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
