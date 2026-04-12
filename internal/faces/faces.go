package faces

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/cjbarker/rufus/internal/db"
)

// Detector handles face detection and recognition.
// In a full implementation this wraps go-face (dlib).
// This initial version provides the interface and descriptor utilities.
type Detector struct {
	store     *db.Store
	tolerance float64
}

// NewDetector creates a new face detector.
func NewDetector(store *db.Store, tolerance float64) *Detector {
	return &Detector{
		store:     store,
		tolerance: tolerance,
	}
}

// EncodeDescriptor converts a 128-dimensional float64 slice to bytes for storage.
func EncodeDescriptor(desc []float64) []byte {
	buf := make([]byte, len(desc)*8)
	for i, v := range desc {
		binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(v))
	}
	return buf
}

// DecodeDescriptor converts stored bytes back to a 128-dimensional float64 slice.
func DecodeDescriptor(data []byte) ([]float64, error) {
	if len(data)%8 != 0 {
		return nil, fmt.Errorf("invalid descriptor length: %d bytes", len(data))
	}
	desc := make([]float64, len(data)/8)
	for i := range desc {
		desc[i] = math.Float64frombits(binary.LittleEndian.Uint64(data[i*8:]))
	}
	return desc, nil
}

// EuclideanDistance computes the Euclidean distance between two face descriptors.
func EuclideanDistance(a, b []float64) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("descriptor length mismatch: %d vs %d", len(a), len(b))
	}
	var sum float64
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return math.Sqrt(sum), nil
}

// MatchResult represents a face match result.
type MatchResult struct {
	PersonID   int64
	PersonName string
	Distance   float64
	FaceID     int64
}

// LabelFace assigns a person name to a detected face.
func (d *Detector) LabelFace(faceID int64, personName string) error {
	person, err := d.store.GetPersonByName(personName)
	if err != nil {
		return fmt.Errorf("looking up person: %w", err)
	}

	var personID int64
	if person == nil {
		personID, err = d.store.InsertPerson(personName)
		if err != nil {
			return fmt.Errorf("creating person: %w", err)
		}
	} else {
		personID = person.ID
	}

	return d.store.UpdateFacePerson(faceID, personID)
}

// FindByPerson finds all images containing a named person.
func (d *Detector) FindByPerson(personName string) ([]db.ImageRecord, error) {
	person, err := d.store.GetPersonByName(personName)
	if err != nil {
		return nil, fmt.Errorf("looking up person: %w", err)
	}
	if person == nil {
		return nil, fmt.Errorf("person %q not found", personName)
	}
	return d.store.GetImagesByPerson(person.ID)
}
