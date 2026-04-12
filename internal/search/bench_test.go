package search

import (
	"fmt"
	"testing"
	"time"

	"github.com/cjbarker/rufus/internal/db"
)

func setupBenchStore(b *testing.B, n int) *db.Store {
	b.Helper()
	store, err := db.OpenMemory()
	if err != nil {
		b.Fatal(err)
	}

	now := time.Now()
	recs := make([]*db.ImageRecord, n)
	for i := range n {
		format := "jpeg"
		if i%5 == 0 {
			format = "png"
		}
		recs[i] = &db.ImageRecord{
			FilePath: fmt.Sprintf("/photos/%04d/image.jpg", i),
			FileSize: int64(1000 + i*100),
			FileHash: fmt.Sprintf("h%d", i),
			Width:    1920,
			Height:   1080,
			Format:   format,
			ModTime:  now,
		}
	}

	if batchErr := store.InsertImageBatch(recs); batchErr != nil {
		b.Fatal(batchErr)
	}

	// Tag every 10th image.
	imgs, err := store.GetAllImages()
	if err != nil {
		b.Fatal(err)
	}
	for i, img := range imgs {
		if i%10 == 0 {
			if err := store.InsertTag(img.ID, "landscape"); err != nil {
				b.Fatal(err)
			}
		}
	}
	return store
}

// BenchmarkSearchAll measures a query with no filters (full table scan).
func BenchmarkSearchAll1000(b *testing.B) {
	store := setupBenchStore(b, 1000)
	defer func() { _ = store.Close() }()
	engine := NewEngine(store)
	b.ResetTimer()
	for range b.N {
		if _, err := engine.Search(&Query{Limit: 1000}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSearchByFormat measures a format filter.
func BenchmarkSearchByFormat1000(b *testing.B) {
	store := setupBenchStore(b, 1000)
	defer func() { _ = store.Close() }()
	engine := NewEngine(store)
	b.ResetTimer()
	for range b.N {
		if _, err := engine.Search(&Query{Format: "png", Limit: 1000}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSearchByTag measures a tag filter (JOIN + WHERE).
func BenchmarkSearchByTag1000(b *testing.B) {
	store := setupBenchStore(b, 1000)
	defer func() { _ = store.Close() }()
	engine := NewEngine(store)
	b.ResetTimer()
	for range b.N {
		if _, err := engine.Search(&Query{Tags: []string{"landscape"}, Limit: 1000}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSearchMultiTagAnd measures multi-tag AND mode.
func BenchmarkSearchMultiTagAnd1000(b *testing.B) {
	store := setupBenchStore(b, 1000)
	defer func() { _ = store.Close() }()
	engine := NewEngine(store)
	b.ResetTimer()
	for range b.N {
		if _, err := engine.Search(&Query{
			Tags:    []string{"landscape", "nature"},
			TagMode: TagModeAnd,
			Limit:   1000,
		}); err != nil {
			b.Fatal(err)
		}
	}
}
