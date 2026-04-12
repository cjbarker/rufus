package cmd

import (
	"fmt"
	"strings"

	"github.com/cjbarker/rufus/internal/db"
	"github.com/cjbarker/rufus/internal/ui"
	"github.com/cjbarker/rufus/internal/util"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info <image-path>",
	Short: "Show everything Rufus knows about an indexed image",
	Long: `Display all stored metadata for a single indexed image: file details,
perceptual hashes, tags, and detected faces with person assignments.
The image must be indexed first with "rufus scan".`,
	Args: cobra.ExactArgs(1),
	RunE: runInfo,
}

func init() {
	rootCmd.AddCommand(infoCmd)
}

func runInfo(_ *cobra.Command, args []string) error {
	imagePath := args[0]

	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = store.Close() }()

	img, err := store.GetImageByPath(imagePath)
	if err != nil {
		return fmt.Errorf("querying image: %w", err)
	}
	if img == nil {
		return fmt.Errorf("%q is not in the index — run 'rufus scan' first", imagePath)
	}

	tags, err := store.GetTagsForImage(img.ID)
	if err != nil {
		return fmt.Errorf("loading tags: %w", err)
	}

	faces, err := store.GetFacesWithPersonByImage(img.ID)
	if err != nil {
		return fmt.Errorf("loading faces: %w", err)
	}

	// ── Image metadata ────────────────────────────────────────────────────────
	fmt.Println()
	ui.SectionHeader(ui.FileLink(imagePath))
	fmt.Println()

	ui.StatusLine("File Size", util.FormatSize(img.FileSize))
	ui.StatusLine("Resolution", fmt.Sprintf("%d × %d", img.Width, img.Height))
	ui.StatusLine("Format", img.Format)

	shortHash := img.FileHash
	if len(shortHash) > 16 {
		shortHash = shortHash[:16] + "…"
	}
	ui.StatusLine("SHA-256", shortHash)
	ui.StatusLine("AHash", fmt.Sprintf("0x%016x", img.AHash))
	ui.StatusLine("DHash", fmt.Sprintf("0x%016x", img.DHash))
	ui.StatusLine("PHash", fmt.Sprintf("0x%016x", img.PHash))

	ui.StatusLine("Modified", img.ModTime.Format("2006-01-02 15:04:05"))
	ui.StatusLine("Scanned", img.ScannedAt.Format("2006-01-02 15:04:05"))
	if img.FaceScannedAt != nil {
		ui.StatusLine("Face Scan", img.FaceScannedAt.Format("2006-01-02 15:04:05"))
	} else {
		ui.StatusLine("Face Scan", ui.Dim.Render("not yet"))
	}

	// ── Tags ──────────────────────────────────────────────────────────────────
	fmt.Println()
	ui.SectionHeader("Tags")
	fmt.Println()

	if len(tags) == 0 {
		ui.InfoMessage("No tags — use 'rufus tag add' to add some.")
	} else {
		rendered := make([]string, len(tags))
		for i, t := range tags {
			rendered[i] = ui.Highlight.Render(t)
		}
		fmt.Println("  " + strings.Join(rendered, "  "))
	}

	// ── Faces ─────────────────────────────────────────────────────────────────
	fmt.Println()
	ui.SectionHeader("Faces")
	fmt.Println()

	switch {
	case img.FaceScannedAt == nil:
		ui.InfoMessage("Face detection has not run on this image yet — use 'rufus faces detect'.")
	case len(faces) == 0:
		ui.InfoMessage("No faces detected.")
	default:
		tbl := ui.NewTable("ID", "PERSON", "BOUNDS")
		for _, fw := range faces {
			person := fw.PersonName
			if person == "" {
				person = ui.Dim.Render("(unlabeled)")
			} else {
				person = ui.Highlight.Render(person)
			}
			bounds := fmt.Sprintf("(%d,%d)–(%d,%d)",
				fw.Face.BoundsX, fw.Face.BoundsY,
				fw.Face.BoundsX+fw.Face.BoundsW, fw.Face.BoundsY+fw.Face.BoundsH)
			tbl.AddRow(fmt.Sprintf("%d", fw.Face.ID), person, bounds)
		}
		tbl.Render()
	}

	fmt.Println()
	return nil
}
