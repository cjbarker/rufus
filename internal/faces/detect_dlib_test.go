//go:build dlib

package faces

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// testFaceImageURL is a group photo with 10 clearly visible frontal faces,
// used by the go-face test suite to verify face detection.
const testFaceImageURL = "https://github.com/Kagami/go-face-testdata/raw/master/images/pristin.jpg"

// skipIfModelsAbsent skips the test if the dlib model files have not been
// downloaded yet. Run 'rufus faces detect' once to download them.
func skipIfModelsAbsent(t *testing.T) {
	t.Helper()
	required := []string{
		"mmod_human_face_detector.dat",
		"shape_predictor_5_face_landmarks.dat",
		"dlib_face_recognition_resnet_model_v1.dat",
	}
	modelsDir := ModelsDir()
	for _, f := range required {
		if _, err := os.Stat(filepath.Join(modelsDir, f)); os.IsNotExist(err) {
			t.Skipf("dlib model %q not present in %s; run 'rufus faces detect' to download", f, modelsDir)
		}
	}
}

// fetchTestImage downloads url into a temp file and returns its path.
// Skips the test if the download fails (e.g. no network).
func fetchTestImage(t *testing.T, url string) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "test_face.jpg")
	resp, err := http.Get(url) //nolint:gosec // URL is a test constant
	if err != nil {
		t.Skipf("could not download test image (no network?): %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("could not download test image: HTTP %d from %s", resp.StatusCode, url)
	}
	f, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		t.Fatal(err)
	}
	return dst
}

// TestRunDetection_DetectsFaces verifies that RunDetection finds faces in a
// known group photo. This exercises the full CNN detection pipeline.
func TestRunDetection_DetectsFaces(t *testing.T) {
	skipIfModelsAbsent(t)
	imgPath := fetchTestImage(t, testFaceImageURL)

	rec, err := NewRecognizer(ModelsDir())
	if err != nil {
		t.Fatalf("NewRecognizer: %v", err)
	}
	defer rec.Close()

	detections, err := RunDetection(rec, imgPath)
	if err != nil {
		t.Fatalf("RunDetection: %v", err)
	}

	if len(detections) == 0 {
		t.Fatal("expected faces to be detected in group photo, got 0 — check CNN model and image format")
	}
	t.Logf("detected %d face(s)", len(detections))

	for i, d := range detections {
		if len(d.Descriptor) != 128 {
			t.Errorf("face[%d]: descriptor length %d, want 128", i, len(d.Descriptor))
		}
		if d.Right <= d.Left {
			t.Errorf("face[%d]: invalid bounding box width (left=%d, right=%d)", i, d.Left, d.Right)
		}
		if d.Bottom <= d.Top {
			t.Errorf("face[%d]: invalid bounding box height (top=%d, bottom=%d)", i, d.Top, d.Bottom)
		}
	}
}

// TestRunDetection_NoFacesInBlankImage verifies that a solid-color image
// returns zero detections without error.
func TestRunDetection_NoFacesInBlankImage(t *testing.T) {
	skipIfModelsAbsent(t)

	imgPath := filepath.Join("..", "..", "testdata", "red_100x100.jpg")

	rec, err := NewRecognizer(ModelsDir())
	if err != nil {
		t.Fatalf("NewRecognizer: %v", err)
	}
	defer rec.Close()

	detections, err := RunDetection(rec, imgPath)
	if err != nil {
		t.Fatalf("RunDetection: %v", err)
	}

	if len(detections) != 0 {
		t.Errorf("expected 0 faces in solid-color image, got %d", len(detections))
	}
}

// TestRunDetectionWithEnvImage tests detection on a caller-supplied image.
// This is useful for diagnosing why specific photos return 0 detections.
//
// Usage:
//
//	TEST_FACE_IMAGE=/path/to/photo.jpg go test -tags dlib ./internal/faces/ -run TestRunDetectionWithEnvImage -v
func TestRunDetectionWithEnvImage(t *testing.T) {
	skipIfModelsAbsent(t)

	imgPath := os.Getenv("TEST_FACE_IMAGE")
	if imgPath == "" {
		t.Skip("TEST_FACE_IMAGE env var not set")
	}

	rec, err := NewRecognizer(ModelsDir())
	if err != nil {
		t.Fatalf("NewRecognizer: %v", err)
	}
	defer rec.Close()

	detections, err := RunDetection(rec, imgPath)
	if err != nil {
		t.Fatalf("RunDetection: %v", err)
	}

	t.Logf("image: %s → %d face(s) detected", imgPath, len(detections))
	for i, d := range detections {
		t.Logf("  face[%d]: bounds=(%d,%d)-(%d,%d) descriptor[0]=%.4f",
			i, d.Left, d.Top, d.Right, d.Bottom, d.Descriptor[0])
	}

	if len(detections) == 0 {
		t.Error("expected at least 1 face — image may be unsupported format, too small, or faces too angled")
	}
}
