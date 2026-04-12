//go:build dlib

package faces

import (
	"fmt"

	"github.com/Kagami/go-face"
)

// Recognizer wraps go-face's recognizer so callers don't import go-face directly.
type Recognizer struct {
	rec *face.Recognizer
}

// NewRecognizer initialises the dlib face recognizer from modelsDir.
// The caller must call Close() when done.
func NewRecognizer(modelsDir string) (*Recognizer, error) {
	rec, err := face.NewRecognizer(modelsDir)
	if err != nil {
		return nil, fmt.Errorf("initializing face recognizer: %w", err)
	}
	return &Recognizer{rec: rec}, nil
}

// Close releases resources held by the recognizer.
func (r *Recognizer) Close() { r.rec.Close() }

// RunDetection detects faces in imagePath using the provided recognizer.
func RunDetection(rec *Recognizer, imagePath string) ([]Detection, error) {
	detected, err := rec.rec.RecognizeFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("detecting faces: %w", err)
	}

	results := make([]Detection, len(detected))
	for i, f := range detected {
		desc := make([]float64, len(f.Descriptor))
		for j, v := range f.Descriptor {
			desc[j] = float64(v)
		}
		results[i] = Detection{
			Left:       f.Rectangle.Min.X,
			Top:        f.Rectangle.Min.Y,
			Right:      f.Rectangle.Max.X,
			Bottom:     f.Rectangle.Max.Y,
			Descriptor: desc,
		}
	}
	return results, nil
}

// DlibAvailable reports whether this binary was compiled with CGO/dlib support.
func DlibAvailable() bool { return true }
