package cmd

import (
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/cjbarker/rufus/internal/db"
	"github.com/cjbarker/rufus/internal/faces"
	"github.com/cjbarker/rufus/internal/ui"
	"github.com/spf13/cobra"
)

// detectResult carries the outcome of a single image's face detection back to
// the main goroutine for DB writes.
type detectResult struct {
	img        db.ImageRecord
	detections []faces.Detection
	err        error
}

// faceMatchThreshold is the maximum Euclidean descriptor distance used to
// auto-assign a detected face to an already-labeled person.
const faceMatchThreshold = 0.6

var facesTolerance float64

var facesCmd = &cobra.Command{
	Use:   "faces",
	Short: "Detect and manage face labels",
	Long: `Detect faces in indexed images, label them with names,
and find images by person.`,
}

var facesForce bool

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

var facesUnlabeledCmd = &cobra.Command{
	Use:   "unlabeled",
	Short: "List detected faces that have not been assigned a name",
	RunE:  runFacesUnlabeled,
}

func init() {
	facesCmd.PersistentFlags().Float64Var(&facesTolerance, "tolerance", 0.6, "face match tolerance")
	facesDetectCmd.Flags().BoolVarP(&facesForce, "force", "f", false, "re-scan all images, ignoring the face-scan cache")
	facesCmd.AddCommand(facesDetectCmd)
	facesCmd.AddCommand(facesLabelCmd)
	facesCmd.AddCommand(facesFindCmd)
	facesCmd.AddCommand(facesListCmd)
	facesCmd.AddCommand(facesUnlabeledCmd)
	rootCmd.AddCommand(facesCmd)
}

func runFacesDetect(cmd *cobra.Command, args []string) error {
	// Open the database.
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = store.Close() }()

	// Load images to process: unscanned only, or all images when --force is set.
	var images []db.ImageRecord
	if facesForce {
		images, err = store.GetAllImages()
		if err != nil {
			return fmt.Errorf("loading images: %w", err)
		}
	} else {
		images, err = store.GetUnscannedImages()
		if err != nil {
			return fmt.Errorf("loading images: %w", err)
		}
	}

	// Load existing labeled faces for auto-matching and the rematch pass.
	knownFaces, err := store.GetAllFacesWithPerson()
	if err != nil {
		return fmt.Errorf("loading known faces: %w", err)
	}

	knownLabeledCount := 0
	for _, kf := range knownFaces {
		if kf.PersonName != "" {
			knownLabeledCount++
		}
	}

	// Early exit only when there's nothing new to detect and no labels to propagate.
	if len(images) == 0 && knownLabeledCount == 0 {
		ui.SuccessMessage("All images have already been scanned for faces. Use --force to re-scan.")
		return nil
	}

	var (
		detected           int
		labeled            int
		rematched          int
		errs               int
		remainingUnlabeled int
	)

	if len(images) > 0 {
		// Gate: binary must be compiled with the dlib build tag.
		if !faces.DlibAvailable() {
			ui.ErrorMessage("This binary was compiled without dlib support.")
			ui.InfoMessage("Install dlib, then rebuild with: make build-faces")
			return nil
		}

		spinner := ui.NewSpinner("Checking dlib installation...")
		spinner.Start()

		if !faces.DlibInstalled() {
			spinner.UpdateMessage("dlib not found — installing...")
			if err := faces.EnsureDlib(func(msg string) { spinner.UpdateMessage(msg) }); err != nil {
				spinner.StopWithError("dlib installation failed")
				ui.ErrorMessage(err.Error())
				ui.InfoMessage("See https://github.com/Kagami/go-face for manual setup instructions.")
				return nil
			}
			spinner.StopWithSuccess("dlib installed")
		} else {
			spinner.StopWithSuccess("dlib ready")
		}

		// Ensure model files are present.
		modelsDir := faces.ModelsDir()
		spinner2 := ui.NewSpinner("Checking face recognition models...")
		spinner2.Start()
		if err := faces.EnsureModels(modelsDir, func(msg string) { spinner2.UpdateMessage(msg) }); err != nil {
			spinner2.StopWithError("Failed to download models")
			return fmt.Errorf("ensuring models: %w", err)
		}
		spinner2.StopWithSuccess("Models ready")

		// Initialise the recognizer once — reused across all images.
		rec, err := faces.NewRecognizer(modelsDir)
		if err != nil {
			return fmt.Errorf("initializing face recognizer: %w", err)
		}
		defer rec.Close()

		fmt.Println()
		ui.SectionHeader(fmt.Sprintf("Scanning %d images for faces", len(images)))
		fmt.Println()

		// When forcing a re-scan, purge existing face records upfront so workers
		// never race with old data.
		if facesForce {
			for _, img := range images {
				if err := store.DeleteFacesByImage(img.ID); err != nil {
					return fmt.Errorf("clearing faces for %s: %w", img.FilePath, err)
				}
			}
		}

		// Worker pool: workers parallelise file I/O and image resizing;
		// CNN detection serialises inside dlib's C layer (thread-safe but not parallel).
		numWorkers := cfg.Workers
		if numWorkers > len(images) {
			numWorkers = len(images)
		}

		jobs := make(chan db.ImageRecord, numWorkers*2)
		results := make(chan detectResult, numWorkers*2)

		var wg sync.WaitGroup
		for range numWorkers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for img := range jobs {
					detections, err := faces.RunDetection(rec, img.FilePath)
					results <- detectResult{img: img, detections: detections, err: err}
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

		var scanSpinner *ui.Spinner
		if !cfg.Verbose {
			scanSpinner = ui.NewSpinner(fmt.Sprintf("[0/%d] Starting...", len(images)))
			scanSpinner.Start()
		}

		var completed atomic.Int32
		for result := range results {
			n := int(completed.Add(1))
			img := result.img
			prefix := fmt.Sprintf("[%d/%d]", n, len(images))

			if cfg.Verbose {
				fmt.Printf("  %s %s\n", ui.Dim.Render(prefix), ui.FileLink(img.FilePath))
			} else {
				scanSpinner.UpdateMessage(fmt.Sprintf("%s %s", prefix, filepath.Base(img.FilePath)))
			}

			if result.err != nil {
				if cfg.Verbose {
					ui.ErrorMessage(fmt.Sprintf("    error: %v", result.err))
				}
				errs++
				// Mark as scanned so a bad file doesn't block every future run.
				_ = store.MarkImageFaceScanned(img.ID)
				continue
			}

			if cfg.Verbose {
				if len(result.detections) == 0 {
					fmt.Printf("    %s\n", ui.Dim.Render("no faces detected"))
				} else {
					fmt.Printf("    %s\n", ui.InfoStyle.Render(fmt.Sprintf("%d face(s) detected", len(result.detections))))
				}
			}

			for _, d := range result.detections {
				faceRec := &db.FaceRecord{
					ImageID:    img.ID,
					BoundsX:   d.Left,
					BoundsY:   d.Top,
					BoundsW:   d.Right - d.Left,
					BoundsH:   d.Bottom - d.Top,
					Descriptor: faces.EncodeDescriptor(d.Descriptor),
				}

				if personID, name := matchKnownFace(d.Descriptor, knownFaces); personID > 0 {
					faceRec.PersonID = &personID
					if cfg.Verbose {
						fmt.Printf("    %s\n", ui.InfoStyle.Render(fmt.Sprintf("→ matched %q", name)))
					}
					labeled++
				}

				if _, err := store.InsertFace(faceRec); err != nil {
					ui.ErrorMessage(fmt.Sprintf("storing face: %v", err))
					errs++
					continue
				}
				detected++
			}

			if err := store.MarkImageFaceScanned(img.ID); err != nil {
				ui.ErrorMessage(fmt.Sprintf("marking image as scanned: %v", err))
			}
		}

		if !cfg.Verbose {
			scanSpinner.StopWithSuccess(fmt.Sprintf("Scanned %d images", len(images)))
		}
	}

	// Rematch pass: apply known labels to any unlabeled faces already in the DB.
	// This runs regardless of whether new images were scanned, so newly added labels
	// are propagated to faces detected in previous runs.
	if knownLabeledCount > 0 {
		unlabeled, err := store.GetUnlabeledFacesWithDescriptors()
		if err != nil {
			return fmt.Errorf("loading unlabeled faces for rematch: %w", err)
		}
		if len(images) == 0 && len(unlabeled) == 0 {
			ui.SuccessMessage("No unlabeled faces to re-match. Use --force to re-scan all images.")
			return nil
		}
		if len(unlabeled) > 0 {
			fmt.Println()
			ui.SectionHeader(fmt.Sprintf("Re-matching %d unlabeled face(s) against %d known label(s)", len(unlabeled), knownLabeledCount))
			fmt.Println()
			for _, f := range unlabeled {
				desc, err := faces.DecodeDescriptor(f.Descriptor)
				if err != nil {
					continue
				}
				personID, name := matchKnownFace(desc, knownFaces)
				if personID == 0 {
					continue
				}
				if err := store.UpdateFacePerson(f.ID, personID); err != nil {
					ui.ErrorMessage(fmt.Sprintf("updating face %d: %v", f.ID, err))
					continue
				}
				if cfg.Verbose {
					fmt.Printf("  face %d → %q\n", f.ID, name)
				}
				rematched++
			}
		}
		remainingUnlabeled = len(unlabeled) - rematched
	} else {
		remainingUnlabeled = detected - labeled
	}

	fmt.Println()
	if len(images) > 0 {
		ui.StatusLine("Images scanned", fmt.Sprintf("%d", len(images)))
		ui.StatusLine("Faces found", fmt.Sprintf("%d", detected))
		ui.StatusLine("Auto-labeled", fmt.Sprintf("%d", labeled))
	}
	if rematched > 0 {
		ui.StatusLine("Re-matched", fmt.Sprintf("%d", rematched))
	}
	if errs > 0 {
		ui.StatusLine("Errors", fmt.Sprintf("%d", errs))
	}
	fmt.Println()

	if remainingUnlabeled > 0 {
		ui.InfoMessage(fmt.Sprintf(
			"%d unlabeled face(s) — use 'rufus faces label <face-id> <name>' to assign names.",
			remainingUnlabeled,
		))
	}
	return nil
}

// matchKnownFace returns the personID and name of the closest labeled face
// within faceMatchThreshold, or (0, "") if no match is found.
func matchKnownFace(descriptor []float64, known []db.FaceWithPerson) (int64, string) {
	best := faceMatchThreshold
	var bestID int64
	var bestName string

	for _, kf := range known {
		if kf.PersonName == "" || kf.Face.PersonID == nil {
			continue
		}
		kDesc, err := faces.DecodeDescriptor(kf.Face.Descriptor)
		if err != nil {
			continue
		}
		dist, err := faces.EuclideanDistance(descriptor, kDesc)
		if err != nil {
			continue
		}
		if dist < best {
			best = dist
			bestID = *kf.Face.PersonID
			bestName = kf.PersonName
		}
	}
	return bestID, bestName
}

func runFacesLabel(cmd *cobra.Command, args []string) error {
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = store.Close() }()

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
	defer func() { _ = store.Close() }()

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
	defer func() { _ = store.Close() }()

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

func runFacesUnlabeled(cmd *cobra.Command, args []string) error {
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = store.Close() }()

	unlabeled, err := store.GetUnlabeledFaces()
	if err != nil {
		return fmt.Errorf("listing unlabeled faces: %w", err)
	}

	if len(unlabeled) == 0 {
		ui.SuccessMessage("No unlabeled faces — all detected faces have been assigned a name.")
		return nil
	}

	fmt.Println()
	ui.SectionHeader(fmt.Sprintf("%d Unlabeled Face(s)", len(unlabeled)))
	fmt.Println()

	tbl := ui.NewTable("FACE ID", "IMAGE", "BOUNDS")
	for _, f := range unlabeled {
		bounds := fmt.Sprintf("%d,%d  %dx%d", f.BoundsX, f.BoundsY, f.BoundsW, f.BoundsH)
		tbl.AddRow(
			ui.Highlight.Render(fmt.Sprintf("%d", f.FaceID)),
			ui.FileLink(f.FilePath),
			ui.Dim.Render(bounds),
		)
	}
	tbl.Render()
	fmt.Println()
	ui.InfoMessage("Use 'rufus faces label <face-id> <name>' to assign a name.")
	fmt.Println()
	return nil
}
