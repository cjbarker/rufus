package faces

import (
	"compress/bzip2"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// Detection represents a single detected face within an image.
type Detection struct {
	Left, Top, Right, Bottom int
	Descriptor               []float64 // 128-dimensional face embedding
}

// ModelsDir returns the directory where dlib model files are stored (~/.rufus/models).
func ModelsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".rufus", "models")
}

var dlibModels = []struct {
	file string
	url  string
}{
	{
		"mmod_human_face_detector.dat",
		"https://dlib.net/files/mmod_human_face_detector.dat.bz2",
	},
	{
		"shape_predictor_5_face_landmarks.dat",
		"https://dlib.net/files/shape_predictor_5_face_landmarks.dat.bz2",
	},
	{
		"dlib_face_recognition_resnet_model_v1.dat",
		"https://dlib.net/files/dlib_face_recognition_resnet_model_v1.dat.bz2",
	},
}

// EnsureModels downloads any missing dlib model files into modelsDir.
// progress is called with status messages suitable for display in a spinner.
func EnsureModels(modelsDir string, progress func(string)) error {
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		return fmt.Errorf("creating models directory: %w", err)
	}

	for _, m := range dlibModels {
		dst := filepath.Join(modelsDir, m.file)
		if _, err := os.Stat(dst); err == nil {
			continue // already present
		}
		progress(fmt.Sprintf("Downloading %s...", m.file))
		if err := downloadBz2(m.url, dst); err != nil {
			return fmt.Errorf("downloading %s: %w", m.file, err)
		}
	}
	return nil
}

func downloadBz2(url, dst string) error {
	resp, err := http.Get(url) //nolint:noctx // URL is a compile-time HTTPS constant; no context needed for one-shot download
	if err != nil {
		return fmt.Errorf("fetching %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, url)
	}

	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating %s: %w", dst, err)
	}
	defer func() { _ = f.Close() }()

	if _, err = io.Copy(f, bzip2.NewReader(resp.Body)); err != nil {
		// Remove partial file on failure
		_ = os.Remove(dst)
		return fmt.Errorf("decompressing: %w", err)
	}
	return nil
}
