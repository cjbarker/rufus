package crawler

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestIsImageFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"photo.jpg", true},
		{"photo.JPEG", true},
		{"image.png", true},
		{"anim.gif", true},
		{"pic.bmp", true},
		{"photo.tiff", true},
		{"photo.tif", true},
		{"photo.webp", true},
		{"doc.pdf", false},
		{"readme.txt", false},
		{"code.go", false},
		{"noext", false},
	}

	for _, tt := range tests {
		if got := isImageFile(tt.name); got != tt.want {
			t.Errorf("isImageFile(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestIsImageExtension(t *testing.T) {
	if !IsImageExtension(".jpg") {
		t.Error("expected .jpg to be a valid image extension")
	}
	if !IsImageExtension(".PNG") {
		t.Error("expected .PNG to be a valid image extension (case insensitive)")
	}
	if IsImageExtension(".txt") {
		t.Error("expected .txt to not be a valid image extension")
	}
}

func TestSupportedExtensions(t *testing.T) {
	exts := SupportedExtensions()
	if len(exts) == 0 {
		t.Fatal("expected at least one supported extension")
	}
	// Should contain at least jpg and png
	extMap := make(map[string]bool)
	for _, e := range exts {
		extMap[e] = true
	}
	if !extMap[".jpg"] {
		t.Error("expected .jpg in supported extensions")
	}
	if !extMap[".png"] {
		t.Error("expected .png in supported extensions")
	}
}

func TestCrawlRecursive(t *testing.T) {
	dir := t.TempDir()
	createTestFile(t, filepath.Join(dir, "a.jpg"))
	createTestFile(t, filepath.Join(dir, "b.png"))
	createTestFile(t, filepath.Join(dir, "c.txt")) // should be skipped

	subdir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	createTestFile(t, filepath.Join(subdir, "d.jpeg"))

	results := Crawl(dir, true, nil)
	var paths []string
	for r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		paths = append(paths, filepath.Base(r.Path))
	}

	sort.Strings(paths)
	expected := []string{"a.jpg", "b.png", "d.jpeg"}
	if len(paths) != len(expected) {
		t.Fatalf("got %d files, want %d: %v", len(paths), len(expected), paths)
	}
	for i, p := range paths {
		if p != expected[i] {
			t.Errorf("paths[%d] = %q, want %q", i, p, expected[i])
		}
	}
}

func TestCrawlNonRecursive(t *testing.T) {
	dir := t.TempDir()
	createTestFile(t, filepath.Join(dir, "a.jpg"))

	subdir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	createTestFile(t, filepath.Join(subdir, "b.jpg"))

	results := Crawl(dir, false, nil)
	var paths []string
	for r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		paths = append(paths, filepath.Base(r.Path))
	}

	if len(paths) != 1 || paths[0] != "a.jpg" {
		t.Errorf("expected [a.jpg], got %v", paths)
	}
}

func createTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
}
