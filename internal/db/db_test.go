package db

import (
	"testing"
	"time"
)

func TestOpenMemory(t *testing.T) {
	store, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	count, err := store.ImageCount()
	if err != nil {
		t.Fatalf("ImageCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 images, got %d", count)
	}
}

func TestInsertAndGetImage(t *testing.T) {
	store, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	rec := &ImageRecord{
		FilePath: "/photos/test.jpg",
		FileSize: 1024,
		FileHash: "abc123",
		Width:    800,
		Height:   600,
		Format:   "jpeg",
		ModTime:  time.Now(),
		AHash:    111,
		DHash:    222,
		PHash:    333,
	}

	id, err := store.InsertImage(rec)
	if err != nil {
		t.Fatalf("InsertImage failed: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive ID, got %d", id)
	}

	got, err := store.GetImageByPath("/photos/test.jpg")
	if err != nil {
		t.Fatalf("GetImageByPath failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected image, got nil")
	}
	if got.FileSize != 1024 {
		t.Errorf("FileSize = %d, want 1024", got.FileSize)
	}
	if got.Width != 800 || got.Height != 600 {
		t.Errorf("dimensions = %dx%d, want 800x600", got.Width, got.Height)
	}
	if got.AHash != 111 || got.DHash != 222 || got.PHash != 333 {
		t.Error("hashes don't match")
	}
}

func TestInsertImageUpsert(t *testing.T) {
	store, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	rec := &ImageRecord{
		FilePath: "/photos/test.jpg",
		FileSize: 1024,
		FileHash: "abc123",
		Width:    800,
		Height:   600,
		Format:   "jpeg",
		ModTime:  time.Now(),
	}

	_, err = store.InsertImage(rec)
	if err != nil {
		t.Fatal(err)
	}

	// Update with different size
	rec.FileSize = 2048
	rec.Width = 1600
	_, err = store.InsertImage(rec)
	if err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	got, _ := store.GetImageByPath("/photos/test.jpg")
	if got.FileSize != 2048 {
		t.Errorf("upsert FileSize = %d, want 2048", got.FileSize)
	}
	if got.Width != 1600 {
		t.Errorf("upsert Width = %d, want 1600", got.Width)
	}

	count, _ := store.ImageCount()
	if count != 1 {
		t.Errorf("expected 1 image after upsert, got %d", count)
	}
}

func TestGetImageByPathNotFound(t *testing.T) {
	store, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	got, err := store.GetImageByPath("/nonexistent.jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent image")
	}
}

func TestGetAllImages(t *testing.T) {
	store, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	for i := 0; i < 3; i++ {
		rec := &ImageRecord{
			FilePath: "/photos/" + string(rune('a'+i)) + ".jpg",
			FileSize: int64(i * 100),
			FileHash: "hash" + string(rune('a'+i)),
			Format:   "jpeg",
			ModTime:  time.Now(),
		}
		if _, insertErr := store.InsertImage(rec); insertErr != nil {
			t.Fatal(insertErr)
		}
	}

	images, err := store.GetAllImages()
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 3 {
		t.Errorf("expected 3 images, got %d", len(images))
	}
}

func TestPersonCRUD(t *testing.T) {
	store, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	id, err := store.InsertPerson("Alice")
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Error("expected positive ID")
	}

	person, err := store.GetPersonByName("Alice")
	if err != nil {
		t.Fatal(err)
	}
	if person == nil || person.Name != "Alice" {
		t.Error("expected to find Alice")
	}

	notFound, err := store.GetPersonByName("Bob")
	if err != nil {
		t.Fatal(err)
	}
	if notFound != nil {
		t.Error("expected nil for Bob")
	}

	people, err := store.GetAllPeople()
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 1 {
		t.Errorf("expected 1 person, got %d", len(people))
	}
}

func TestTagOperations(t *testing.T) {
	store, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	rec := &ImageRecord{
		FilePath: "/photos/test.jpg",
		FileSize: 1024,
		FileHash: "abc",
		Format:   "jpeg",
		ModTime:  time.Now(),
	}
	imgID, _ := store.InsertImage(rec)

	if tagErr := store.InsertTag(imgID, "landscape"); tagErr != nil {
		t.Fatal(tagErr)
	}
	// Duplicate tag should not error
	if tagErr := store.InsertTag(imgID, "landscape"); tagErr != nil {
		t.Fatalf("duplicate tag should be ignored: %v", tagErr)
	}

	images, err := store.SearchByTag("landscape")
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 1 {
		t.Errorf("expected 1 image with tag, got %d", len(images))
	}

	images, err = store.SearchByTag("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 0 {
		t.Errorf("expected 0 images for nonexistent tag, got %d", len(images))
	}
}
