package hasher

import (
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkHashFile measures the cost of hashing a single JPEG image including
// SHA-256 computation and all three perceptual hashes.
func BenchmarkHashFile(b *testing.B) {
	path := makeBenchImage(b, 256, 256)
	b.ResetTimer()
	for range b.N {
		if _, err := HashFile(path); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkHashFileLarge measures hashing on a larger image (simulates a real photo).
func BenchmarkHashFileLarge(b *testing.B) {
	path := makeBenchImage(b, 3840, 2160) // 4K
	b.ResetTimer()
	for range b.N {
		if _, err := HashFile(path); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkHammingDistance measures the bitwise XOR + popcount operation used
// for perceptual hash comparison.
func BenchmarkHammingDistance(b *testing.B) {
	a := uint64(0xFF00FF00FF00FF00)
	c := uint64(0x00FF00FF00FF00FF)
	b.ResetTimer()
	for range b.N {
		_ = HammingDistance(a, c)
	}
}

// makeBenchImage creates a solid-colour JPEG at the given dimensions and
// returns its path. The file is placed in b.TempDir() which is cleaned up
// automatically after the benchmark run.
func makeBenchImage(b *testing.B, w, h int) string {
	b.Helper()
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.jpg")

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Paint a simple gradient to avoid a fully uniform image (which compresses
	// very differently from real photos and can skew hash timings).
	for y := range h {
		for x := range w {
			img.Pix[(y*img.Stride+x*4)+0] = uint8(x % 256)
			img.Pix[(y*img.Stride+x*4)+1] = uint8(y % 256)
			img.Pix[(y*img.Stride+x*4)+2] = uint8((x + y) % 256)
			img.Pix[(y*img.Stride+x*4)+3] = 255
		}
	}

	f, err := os.Create(path)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 85}); err != nil {
		b.Fatal(err)
	}
	return path
}
