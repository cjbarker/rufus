package cmd

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/cjbarker/rufus/internal/db"
	"github.com/cjbarker/rufus/internal/ui"
	"github.com/spf13/cobra"
)

var (
	showOutput  string
	showPadding int
	showNoOpen  bool
)

var facesShowCmd = &cobra.Command{
	Use:   "show <face-id>",
	Short: "Crop and display a detected face from its source image",
	Long: `Extract the bounding box for a detected face, crop it from the source
image (with optional padding), save it as a PNG, and open it in your default
image viewer. Use --output to choose a specific save path. Use --no-open to
skip the viewer and just print the saved path.`,
	Args: cobra.ExactArgs(1),
	RunE: runFacesShow,
}

func init() {
	facesShowCmd.Flags().StringVarP(&showOutput, "output", "o", "", "save crop to this path (default: temp file)")
	facesShowCmd.Flags().IntVar(&showPadding, "padding", 40, "pixels to add around the face bounds")
	facesShowCmd.Flags().BoolVar(&showNoOpen, "no-open", false, "save the file but skip opening the viewer")
	facesCmd.AddCommand(facesShowCmd)
}

func runFacesShow(_ *cobra.Command, args []string) error {
	var faceID int64
	if _, err := fmt.Sscanf(args[0], "%d", &faceID); err != nil {
		return fmt.Errorf("invalid face ID %q: %w", args[0], err)
	}

	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = store.Close() }()

	fw, err := store.GetFaceByID(faceID)
	if err != nil {
		return fmt.Errorf("querying face: %w", err)
	}
	if fw == nil {
		return fmt.Errorf("face %d not found — run 'rufus faces detect' first", faceID)
	}

	cropped, err := cropFaceRegion(fw.FilePath, fw.Face.BoundsX, fw.Face.BoundsY, fw.Face.BoundsW, fw.Face.BoundsH, showPadding)
	if err != nil {
		return fmt.Errorf("cropping face from %s: %w", fw.FilePath, err)
	}

	outPath := showOutput
	if outPath == "" {
		outPath = filepath.Join(os.TempDir(), fmt.Sprintf("rufus-face-%d.png", faceID))
	}

	if err := saveFacePNG(cropped, outPath); err != nil {
		return fmt.Errorf("saving crop: %w", err)
	}

	fmt.Println()
	ui.StatusLine("Face ID", fmt.Sprintf("%d", faceID))
	if fw.PersonName != "" {
		ui.StatusLine("Person", ui.Highlight.Render(fw.PersonName))
	} else {
		ui.StatusLine("Person", ui.Dim.Render("(unlabeled)"))
	}
	bounds := fmt.Sprintf("(%d,%d)–(%d,%d)",
		fw.Face.BoundsX, fw.Face.BoundsY,
		fw.Face.BoundsX+fw.Face.BoundsW, fw.Face.BoundsY+fw.Face.BoundsH)
	ui.StatusLine("Bounds", bounds)
	ui.StatusLine("Source", ui.FileLink(fw.FilePath))
	ui.StatusLine("Saved", ui.FileLink(outPath))
	fmt.Println()

	if !showNoOpen {
		if err := openWithSystemViewer(outPath); err != nil {
			ui.WarningMessage(fmt.Sprintf("Could not open viewer: %v", err))
		}
	}
	return nil
}

// cropFaceRegion decodes the image at imgPath, applies padding, clamps to
// image bounds, and returns the cropped sub-image.
func cropFaceRegion(imgPath string, x, y, w, h, padding int) (image.Image, error) {
	f, err := os.Open(imgPath)
	if err != nil {
		return nil, fmt.Errorf("opening image: %w", err)
	}
	defer func() { _ = f.Close() }()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}

	b := img.Bounds()
	x1 := max(b.Min.X, x-padding)
	y1 := max(b.Min.Y, y-padding)
	x2 := min(b.Max.X, x+w+padding)
	y2 := min(b.Max.Y, y+h+padding)

	if x2 <= x1 || y2 <= y1 {
		return nil, fmt.Errorf("face bounds out of image range")
	}

	rect := image.Rect(x1, y1, x2, y2)
	si, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	})
	if !ok {
		return nil, fmt.Errorf("image type %T does not support cropping", img)
	}
	return si.SubImage(rect), nil
}

func saveFacePNG(img image.Image, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("encoding PNG: %w", err)
	}
	return nil
}

func openWithSystemViewer(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}
