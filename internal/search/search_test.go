package search

import (
	"testing"
	"time"

	"github.com/cjbarker/rufus/internal/db"
)

func TestSearchByFormat(t *testing.T) {
	store := setupTestStore(t)

	engine := NewEngine(store)
	results, err := engine.Search(&Query{Format: "jpeg"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 jpeg results, got %d", len(results))
	}

	results, err = engine.Search(&Query{Format: "png"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 png result, got %d", len(results))
	}
}

func TestSearchBySize(t *testing.T) {
	store := setupTestStore(t)

	engine := NewEngine(store)
	results, err := engine.Search(&Query{MinSize: 1500})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result with size >= 1500, got %d", len(results))
	}

	results, err = engine.Search(&Query{MaxSize: 500})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results with size <= 500, got %d", len(results))
	}
}

func TestSearchByPath(t *testing.T) {
	store := setupTestStore(t)

	engine := NewEngine(store)
	results, err := engine.Search(&Query{PathPattern: "photos"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results with 'photos' in path, got %d", len(results))
	}
}

func TestSearchByTag(t *testing.T) {
	store := setupTestStore(t)

	engine := NewEngine(store)
	results, err := engine.Search(&Query{Tags: []string{"nature"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result with tag 'nature', got %d", len(results))
	}
}

func TestSearchNoResults(t *testing.T) {
	store := setupTestStore(t)

	engine := NewEngine(store)
	results, err := engine.Search(&Query{Format: "tiff"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchWithLimit(t *testing.T) {
	store := setupTestStore(t)

	engine := NewEngine(store)
	results, err := engine.Search(&Query{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result with limit=1, got %d", len(results))
	}
}

func setupTestStore(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	images := []*db.ImageRecord{
		{FilePath: "/photos/sunset.jpg", FileSize: 1000, FileHash: "h1", Width: 800, Height: 600, Format: "jpeg", ModTime: time.Now()},
		{FilePath: "/photos/beach.jpg", FileSize: 2000, FileHash: "h2", Width: 1920, Height: 1080, Format: "jpeg", ModTime: time.Now()},
		{FilePath: "/docs/diagram.png", FileSize: 800, FileHash: "h3", Width: 400, Height: 300, Format: "png", ModTime: time.Now()},
	}

	for _, img := range images {
		id, err := store.InsertImage(img)
		if err != nil {
			t.Fatal(err)
		}
		if img.FilePath == "/photos/sunset.jpg" {
			if err := store.InsertTag(id, "nature"); err != nil {
				t.Fatal(err)
			}
		}
	}

	return store
}
