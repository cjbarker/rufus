//go:build !dlib

package faces

import "fmt"

// Recognizer is a stub type used when the binary is compiled without dlib.
type Recognizer struct{}

// NewRecognizer is a stub — returns an error directing the user to rebuild.
func NewRecognizer(_ string) (*Recognizer, error) {
	return nil, fmt.Errorf(
		"face detection requires CGO: install dlib then rebuild with 'make build-faces'",
	)
}

// Close is a no-op stub.
func (r *Recognizer) Close() {}

// RunDetection is a stub.
func RunDetection(_ *Recognizer, _ string) ([]Detection, error) {
	return nil, fmt.Errorf(
		"face detection requires CGO: install dlib then rebuild with 'make build-faces'",
	)
}

// DlibAvailable reports whether this binary was compiled with CGO/dlib support.
func DlibAvailable() bool { return false }
