package cmd

import (
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cjbarker/rufus/internal/config"
	"github.com/cjbarker/rufus/internal/db"
)

// resetState resets all package-level command variables to their defaults and
// returns a temporary DB path unique to this test. Call this at the start of
// every integration test to prevent flag values from leaking between tests.
func resetState(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	*cfg = *config.Default()
	cfg.DBPath = dbPath

	// scan flags
	scanRecursive = true
	scanUpdate = false
	scanExcludes = nil

	// dupes flags
	dupesThreshold = 10
	dupesHash = "dhash"
	dupesFormat = "table"
	dupesYes = false

	// search flags
	searchTags = nil
	searchTagMode = "and"
	searchFace = ""
	searchMinSizeStr = ""
	searchMaxSizeStr = ""
	searchFormat = ""
	searchPath = ""
	searchBefore = ""
	searchAfter = ""
	searchSortBy = "path"
	searchSortDesc = false
	searchLimit = 50
	searchOffset = 0
	searchHasFaces = false
	searchNoFaces = false
	searchOutput = "table"

	// clean flags
	cleanDryRun = false
	cleanVacuum = false

	// export / import flags
	exportOutput = "json"
	exportFile = ""
	importFormat = ""

	// faces flags
	facesTolerance = 0.6
	facesForce = false

	return dbPath
}

// makeIntegrationJPEG creates a minimal JPEG file in dir and returns its path.
func makeIntegrationJPEG(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	path := filepath.Join(dir, name)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Pix[(y*img.Stride+x*4)+0] = uint8(x % 256)
			img.Pix[(y*img.Stride+x*4)+1] = uint8(y % 256)
			img.Pix[(y*img.Stride+x*4)+2] = 128
			img.Pix[(y*img.Stride+x*4)+3] = 255
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
	return path
}

// execute runs the root command with the given CLI args. It returns the error
// produced by the command (nil means success).
func execute(t *testing.T, args ...string) error {
	t.Helper()
	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestIntegrationScan(t *testing.T) {
	dbPath := resetState(t)
	dir := t.TempDir()
	makeIntegrationJPEG(t, dir, "a.jpg", 100, 100)
	makeIntegrationJPEG(t, dir, "b.jpg", 200, 200)
	makeIntegrationJPEG(t, dir, "c.jpg", 50, 50)

	if err := execute(t, "scan", "--db", dbPath, "--quiet", dir); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	count, err := store.ImageCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("expected 3 indexed images, got %d", count)
	}
}

func TestIntegrationScanRecursive(t *testing.T) {
	dbPath := resetState(t)
	dir := t.TempDir()
	makeIntegrationJPEG(t, dir, "root.jpg", 100, 100)
	subdir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	makeIntegrationJPEG(t, subdir, "nested.jpg", 100, 100)

	if err := execute(t, "scan", "--db", dbPath, "--quiet", dir); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	count, _ := store.ImageCount()
	if count != 2 {
		t.Errorf("expected 2 images (recursive), got %d", count)
	}
}

func TestIntegrationScanExclude(t *testing.T) {
	dbPath := resetState(t)
	dir := t.TempDir()
	makeIntegrationJPEG(t, dir, "keep.jpg", 100, 100)

	excluded := filepath.Join(dir, "skip")
	if err := os.MkdirAll(excluded, 0o755); err != nil {
		t.Fatal(err)
	}
	makeIntegrationJPEG(t, excluded, "skipped.jpg", 100, 100)

	if err := execute(t, "scan", "--db", dbPath, "--quiet", "--exclude", "skip", dir); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	count, _ := store.ImageCount()
	if count != 1 {
		t.Errorf("expected 1 image (excluded sub-dir), got %d", count)
	}
}

func TestIntegrationScanUpdate(t *testing.T) {
	dbPath := resetState(t)
	dir := t.TempDir()
	makeIntegrationJPEG(t, dir, "a.jpg", 100, 100)

	// Initial scan.
	if err := execute(t, "scan", "--db", dbPath, "--quiet", dir); err != nil {
		t.Fatalf("initial scan failed: %v", err)
	}

	// Add a second image.
	resetState(t)
	cfg.DBPath = dbPath
	makeIntegrationJPEG(t, dir, "b.jpg", 200, 200)

	if err := execute(t, "scan", "--db", dbPath, "--quiet", "--update", dir); err != nil {
		t.Fatalf("update scan failed: %v", err)
	}

	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	count, _ := store.ImageCount()
	if count != 2 {
		t.Errorf("expected 2 images after update scan, got %d", count)
	}
}

func TestIntegrationStats(t *testing.T) {
	dbPath := resetState(t)
	dir := t.TempDir()
	makeIntegrationJPEG(t, dir, "a.jpg", 100, 100)

	if err := execute(t, "scan", "--db", dbPath, "--quiet", dir); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	resetState(t)
	cfg.DBPath = dbPath
	if err := execute(t, "stats", "--db", dbPath, "--quiet"); err != nil {
		t.Errorf("stats failed: %v", err)
	}
}

func TestIntegrationSearch(t *testing.T) {
	dbPath := resetState(t)
	dir := t.TempDir()
	makeIntegrationJPEG(t, dir, "a.jpg", 100, 100)
	makeIntegrationJPEG(t, dir, "b.jpg", 200, 200)

	if err := execute(t, "scan", "--db", dbPath, "--quiet", dir); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	resetState(t)
	cfg.DBPath = dbPath
	// Search without filters should return all images.
	if err := execute(t, "search", "--db", dbPath, "--quiet", "--output", "json"); err != nil {
		t.Errorf("search failed: %v", err)
	}
}

func TestIntegrationSearchByFormat(t *testing.T) {
	dbPath := resetState(t)
	dir := t.TempDir()
	makeIntegrationJPEG(t, dir, "photo.jpg", 100, 100)

	if err := execute(t, "scan", "--db", dbPath, "--quiet", dir); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	resetState(t)
	cfg.DBPath = dbPath
	if err := execute(t, "search", "--db", dbPath, "--quiet", "--format", "jpeg"); err != nil {
		t.Errorf("search by format failed: %v", err)
	}
}

func TestIntegrationClean(t *testing.T) {
	dbPath := resetState(t)
	dir := t.TempDir()
	a := makeIntegrationJPEG(t, dir, "a.jpg", 100, 100)
	makeIntegrationJPEG(t, dir, "b.jpg", 200, 200)

	if err := execute(t, "scan", "--db", dbPath, "--quiet", dir); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	// Delete a.jpg from disk but leave it in the index.
	if err := os.Remove(a); err != nil {
		t.Fatal(err)
	}

	resetState(t)
	cfg.DBPath = dbPath
	if err := execute(t, "clean", "--db", dbPath, "--quiet"); err != nil {
		t.Errorf("clean failed: %v", err)
	}

	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	count, _ := store.ImageCount()
	if count != 1 {
		t.Errorf("expected 1 image after clean, got %d", count)
	}
}

func TestIntegrationCleanDryRun(t *testing.T) {
	dbPath := resetState(t)
	dir := t.TempDir()
	a := makeIntegrationJPEG(t, dir, "a.jpg", 100, 100)
	makeIntegrationJPEG(t, dir, "b.jpg", 200, 200)

	if err := execute(t, "scan", "--db", dbPath, "--quiet", dir); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if err := os.Remove(a); err != nil {
		t.Fatal(err)
	}

	resetState(t)
	cfg.DBPath = dbPath
	if err := execute(t, "clean", "--db", dbPath, "--quiet", "--dry-run"); err != nil {
		t.Errorf("clean --dry-run failed: %v", err)
	}

	// With dry-run, index should still have 2 records.
	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	count, _ := store.ImageCount()
	if count != 2 {
		t.Errorf("expected 2 images after dry-run clean, got %d", count)
	}
}

func TestIntegrationDupes(t *testing.T) {
	dbPath := resetState(t)

	// Insert two images with identical file hashes directly into the DB (exact dupe).
	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for i, path := range []string{"/tmp/a.jpg", "/tmp/b.jpg"} {
		_, err := store.InsertImage(&db.ImageRecord{
			FilePath: path,
			FileSize: 1000,
			FileHash: "samehash",
			Width:    100 + i,
			Height:   100 + i,
			Format:   "jpeg",
			ModTime:  now,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	_ = store.Close()

	resetState(t)
	cfg.DBPath = dbPath
	if err := execute(t, "dupes", "--db", dbPath, "--quiet", "--format", "json"); err != nil {
		t.Errorf("dupes failed: %v", err)
	}
}

func TestIntegrationExportJSON(t *testing.T) {
	dbPath := resetState(t)
	dir := t.TempDir()
	makeIntegrationJPEG(t, dir, "a.jpg", 100, 100)

	if err := execute(t, "scan", "--db", dbPath, "--quiet", dir); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	outFile := filepath.Join(t.TempDir(), "export.json")
	resetState(t)
	cfg.DBPath = dbPath
	if err := execute(t, "export", "--db", dbPath, "--quiet", "--format", "json", "--file", outFile); err != nil {
		t.Errorf("export failed: %v", err)
	}

	if _, err := os.Stat(outFile); err != nil {
		t.Errorf("export file not created: %v", err)
	}
}

func TestIntegrationExportCSV(t *testing.T) {
	dbPath := resetState(t)
	dir := t.TempDir()
	makeIntegrationJPEG(t, dir, "a.jpg", 100, 100)

	if err := execute(t, "scan", "--db", dbPath, "--quiet", dir); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	outFile := filepath.Join(t.TempDir(), "export.csv")
	resetState(t)
	cfg.DBPath = dbPath
	if err := execute(t, "export", "--db", dbPath, "--quiet", "--format", "csv", "--file", outFile); err != nil {
		t.Errorf("export CSV failed: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading CSV: %v", err)
	}
	if len(data) == 0 {
		t.Error("exported CSV is empty")
	}
}

func TestIntegrationImportJSON(t *testing.T) {
	// Set up source DB and export it.
	srcDBPath := resetState(t)
	dir := t.TempDir()
	makeIntegrationJPEG(t, dir, "a.jpg", 100, 100)
	makeIntegrationJPEG(t, dir, "b.jpg", 200, 200)

	if err := execute(t, "scan", "--db", srcDBPath, "--quiet", dir); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	outFile := filepath.Join(t.TempDir(), "export.json")
	resetState(t)
	cfg.DBPath = srcDBPath
	if err := execute(t, "export", "--db", srcDBPath, "--quiet", "--file", outFile); err != nil {
		t.Fatalf("export failed: %v", err)
	}

	// Import into a fresh DB.
	dstDBPath := filepath.Join(t.TempDir(), "dst.db")
	resetState(t)
	cfg.DBPath = dstDBPath
	if err := execute(t, "import", "--db", dstDBPath, "--quiet", outFile); err != nil {
		t.Fatalf("import failed: %v", err)
	}

	store, err := db.Open(dstDBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	count, _ := store.ImageCount()
	if count != 2 {
		t.Errorf("expected 2 images after import, got %d", count)
	}
}

func TestIntegrationVersion(t *testing.T) {
	resetState(t)
	if err := execute(t, "version", "--quiet"); err != nil {
		t.Errorf("version failed: %v", err)
	}
}
