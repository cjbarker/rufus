package duplicates

import (
	"fmt"
	"testing"

	"github.com/cjbarker/rufus/internal/db"
)

// makeBenchImages returns n ImageRecords with distinct dhash values spread
// across the uint64 space to approximate a realistic library.
func makeBenchImages(n int) []db.ImageRecord {
	imgs := make([]db.ImageRecord, n)
	for i := range n {
		imgs[i] = db.ImageRecord{
			ID:       int64(i + 1),
			FilePath: fmt.Sprintf("/photos/image%05d.jpg", i),
			FileHash: fmt.Sprintf("hash%d", i),
			DHash:    uint64(i) * 0x9E3779B97F4A7C15, // spread via Fibonacci hashing
			AHash:    uint64(i) * 0x6C62272E07BB0142,
			PHash:    uint64(i) * 0xD4A51F3EDB2E3D73,
		}
	}
	return imgs
}

// BenchmarkFindDuplicates measures the full duplicate-detection pipeline
// (exact + perceptual via BK-tree + union-find) on a growing library.
func BenchmarkFindDuplicates100(b *testing.B) {
	benchFindDuplicates(b, 100)
}

func BenchmarkFindDuplicates1000(b *testing.B) {
	benchFindDuplicates(b, 1000)
}

func BenchmarkFindDuplicates10000(b *testing.B) {
	benchFindDuplicates(b, 10_000)
}

func benchFindDuplicates(b *testing.B, n int) {
	b.Helper()
	images := makeBenchImages(n)
	b.ResetTimer()
	for range b.N {
		_ = FindDuplicates(images, DHash, 10)
	}
}

// BenchmarkBKTreeBuild measures BK-tree construction time.
func BenchmarkBKTreeBuild1000(b *testing.B) {
	images := makeBenchImages(1000)
	b.ResetTimer()
	for range b.N {
		t := &bkTree{}
		for i, img := range images {
			t.insert(i, img.DHash)
		}
	}
}

// BenchmarkBKTreeSearch measures BK-tree nearest-neighbor search.
func BenchmarkBKTreeSearch1000(b *testing.B) {
	images := makeBenchImages(1000)
	tree := &bkTree{}
	for i, img := range images {
		tree.insert(i, img.DHash)
	}
	query := images[len(images)/2].DHash
	b.ResetTimer()
	for range b.N {
		_ = tree.search(query, 10)
	}
}
