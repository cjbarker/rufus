package cmd

import (
	"fmt"

	"github.com/cjbarker/rufus/internal/db"
	"github.com/cjbarker/rufus/internal/faces"
	"github.com/cjbarker/rufus/internal/ui"
	"github.com/spf13/cobra"
)

var facesTolerance float64

var facesCmd = &cobra.Command{
	Use:   "faces",
	Short: "Detect and manage face labels",
	Long: `Detect faces in indexed images, label them with names,
and find images by person.`,
}

var facesDetectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Detect faces in indexed images",
	Long: `Run face detection on all indexed images that haven't been processed yet.
Requires dlib models to be installed.`,
	RunE: runFacesDetect,
}

var facesLabelCmd = &cobra.Command{
	Use:   "label <face-id> <name>",
	Short: "Label a detected face with a person's name",
	Args:  cobra.ExactArgs(2),
	RunE:  runFacesLabel,
}

var facesFindCmd = &cobra.Command{
	Use:   "find <name>",
	Short: "Find all images containing a named person",
	Args:  cobra.ExactArgs(1),
	RunE:  runFacesFind,
}

var facesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all known people",
	RunE:  runFacesList,
}

func init() {
	facesCmd.PersistentFlags().Float64Var(&facesTolerance, "tolerance", 0.6, "face match tolerance")
	facesCmd.AddCommand(facesDetectCmd)
	facesCmd.AddCommand(facesLabelCmd)
	facesCmd.AddCommand(facesFindCmd)
	facesCmd.AddCommand(facesListCmd)
	rootCmd.AddCommand(facesCmd)
}

func runFacesDetect(cmd *cobra.Command, args []string) error {
	ui.WarningMessage("Face detection requires dlib models.")
	ui.InfoMessage("This feature will be fully functional when dlib is installed.")
	ui.InfoMessage("See: https://github.com/Kagami/go-face for setup instructions.")
	return nil
}

func runFacesLabel(cmd *cobra.Command, args []string) error {
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer store.Close()

	var faceID int64
	if _, err := fmt.Sscanf(args[0], "%d", &faceID); err != nil {
		return fmt.Errorf("invalid face ID %q: %w", args[0], err)
	}
	name := args[1]

	detector := faces.NewDetector(store, facesTolerance)
	if err := detector.LabelFace(faceID, name); err != nil {
		return fmt.Errorf("labeling face: %w", err)
	}

	ui.SuccessMessage(fmt.Sprintf("Labeled face %s as %s",
		ui.Highlight.Render(fmt.Sprintf("%d", faceID)),
		ui.Bold.Render(fmt.Sprintf("%q", name))))
	return nil
}

func runFacesFind(cmd *cobra.Command, args []string) error {
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer store.Close()

	detector := faces.NewDetector(store, facesTolerance)
	images, err := detector.FindByPerson(args[0])
	if err != nil {
		return fmt.Errorf("finding images: %w", err)
	}

	if len(images) == 0 {
		ui.WarningMessage(fmt.Sprintf("No images found for %q", args[0]))
		return nil
	}

	fmt.Println()
	ui.SectionHeader(fmt.Sprintf("Images of %s", args[0]))
	fmt.Println()

	tbl := ui.NewTable("PATH", "SIZE", "RESOLUTION")
	for _, img := range images {
		tbl.AddRow(
			ui.FileLink(img.FilePath),
			ui.SizeStyle.Render(formatSize(img.FileSize)),
			fmt.Sprintf("%dx%d", img.Width, img.Height),
		)
	}
	tbl.Render()
	fmt.Println()
	return nil
}

func runFacesList(cmd *cobra.Command, args []string) error {
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer store.Close()

	people, err := store.GetAllPeople()
	if err != nil {
		return fmt.Errorf("listing people: %w", err)
	}

	if len(people) == 0 {
		ui.InfoMessage("No people labeled yet. Use 'rufus faces label' to label faces.")
		return nil
	}

	fmt.Println()
	ui.SectionHeader("Known People")
	fmt.Println()

	tbl := ui.NewTable("ID", "NAME", "CREATED")
	for _, p := range people {
		tbl.AddRow(
			ui.Dim.Render(fmt.Sprintf("%d", p.ID)),
			ui.Bold.Render(p.Name),
			ui.Dim.Render(p.CreatedAt.Format("2006-01-02")),
		)
	}
	tbl.Render()
	fmt.Println()
	return nil
}
