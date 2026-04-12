package hasher

import (
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestHashFile(t *testing.T) {
	path := createTestImage(t, 100, 100)

	result, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile failed: %v", err)
	}

	if result.FilePath != path {
		t.Errorf("FilePath = %q, want %q", result.FilePath, path)
	}
	if result.FileHash == "" {
		t.Error("FileHash is empty")
	}
	if result.Width != 100 || result.Height != 100 {
		t.Errorf("dimensions = %dx%d, want 100x100", result.Width, result.Height)
	}
	if result.Format != "jpeg" {
		t.Errorf("Format = %q, want jpeg", result.Format)
	}
	if result.AHash == 0 && result.DHash == 0 && result.PHash == 0 {
		t.Error("all hashes are zero")
	}
}

func TestHashFileDeterministic(t *testing.T) {
	path := createTestImage(t, 50, 50)

	r1, err := HashFile(path)
	if err != nil {
		t.Fatalf("first hash: %v", err)
	}
	r2, err := HashFile(path)
	if err != nil {
		t.Fatalf("second hash: %v", err)
	}

	if r1.FileHash != r2.FileHash {
		t.Error("FileHash is not deterministic")
	}
	if r1.AHash != r2.AHash || r1.DHash != r2.DHash || r1.PHash != r2.PHash {
		t.Error("perceptual hashes are not deterministic")
	}
}

func TestHammingDistance(t *testing.T) {
	tests := []struct {
		a, b uint64
		want int
	}{
		{0, 0, 0},
		{0xFF, 0xFF, 0},
		{0xFF, 0x00, 8},
		{0b1010, 0b0101, 4},
		{0xFFFFFFFFFFFFFFFF, 0, 64},
	}

	for _, tt := range tests {
		got := HammingDistance(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("HammingDistance(%#x, %#x) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestHashFileNotFound(t *testing.T) {
	_, err := HashFile("/nonexistent/file.jpg")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func createTestImage(t *testing.T, w, h int) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jpg")

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, image.White)
		}
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
	return path
}
