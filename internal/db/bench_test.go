package db

import (
	"fmt"
	"testing"
	"time"
)

func makeBenchRecords(n int) []*ImageRecord {
	now := time.Now()
	recs := make([]*ImageRecord, n)
	for i := range n {
		recs[i] = &ImageRecord{
			FilePath: fmt.Sprintf("/bench/image%06d.jpg", i),
			FileSize: int64(i * 1000),
			FileHash: fmt.Sprintf("hash%06d", i),
			Width:    1920,
			Height:   1080,
			Format:   "jpeg",
			ModTime:  now,
			AHash:    uint64(i) * 0x9E3779B97F4A7C15,
			DHash:    uint64(i) * 0x6C62272E07BB0142,
			PHash:    uint64(i) * 0xD4A51F3EDB2E3D73,
		}
	}
	return recs
}

// BenchmarkInsertImageBatch measures batch insert throughput.
func BenchmarkInsertImageBatch100(b *testing.B) {
	benchInsertBatch(b, 100)
}

func BenchmarkInsertImageBatch1000(b *testing.B) {
	benchInsertBatch(b, 1000)
}

func benchInsertBatch(b *testing.B, batchSize int) {
	b.Helper()
	recs := makeBenchRecords(batchSize)
	b.ResetTimer()
	for range b.N {
		store, err := OpenMemory()
		if err != nil {
			b.Fatal(err)
		}
		if err := store.InsertImageBatch(recs); err != nil {
			b.Fatal(err)
		}
		_ = store.Close()
	}
}

// BenchmarkGetAllImages measures the cost of loading the full index.
func BenchmarkGetAllImages1000(b *testing.B) {
	store, err := OpenMemory()
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	if err := store.InsertImageBatch(makeBenchRecords(1000)); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for range b.N {
		if _, err := store.GetAllImages(); err != nil {
			b.Fatal(err)
		}
	}
}
