package faces

import (
	"math"
	"testing"
)

func TestEncodeDecodeDescriptor(t *testing.T) {
	original := make([]float64, 128)
	for i := range original {
		original[i] = float64(i) * 0.01
	}

	encoded := EncodeDescriptor(original)
	if len(encoded) != 128*8 {
		t.Errorf("encoded length = %d, want %d", len(encoded), 128*8)
	}

	decoded, err := DecodeDescriptor(encoded)
	if err != nil {
		t.Fatalf("DecodeDescriptor failed: %v", err)
	}

	if len(decoded) != len(original) {
		t.Fatalf("decoded length = %d, want %d", len(decoded), len(original))
	}

	for i := range original {
		if decoded[i] != original[i] {
			t.Errorf("decoded[%d] = %f, want %f", i, decoded[i], original[i])
		}
	}
}

func TestDecodeDescriptorInvalid(t *testing.T) {
	_, err := DecodeDescriptor([]byte{1, 2, 3}) // not divisible by 8
	if err == nil {
		t.Error("expected error for invalid descriptor length")
	}
}

func TestEuclideanDistance(t *testing.T) {
	a := []float64{0, 0, 0}
	b := []float64{3, 4, 0}

	dist, err := EuclideanDistance(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(dist-5.0) > 0.0001 {
		t.Errorf("distance = %f, want 5.0", dist)
	}
}

func TestEuclideanDistanceIdentical(t *testing.T) {
	a := []float64{1, 2, 3}

	dist, err := EuclideanDistance(a, a)
	if err != nil {
		t.Fatal(err)
	}
	if dist != 0 {
		t.Errorf("distance between identical = %f, want 0", dist)
	}
}

func TestEuclideanDistanceMismatch(t *testing.T) {
	a := []float64{1, 2}
	b := []float64{1, 2, 3}

	_, err := EuclideanDistance(a, b)
	if err == nil {
		t.Error("expected error for mismatched lengths")
	}
}
