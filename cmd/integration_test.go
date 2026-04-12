package cmd

import (
	"fmt"
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

// ─── Faces integration tests ─────────────────────────────────────────────────
//
// These tests exercise the faces subcommand routing and DB interactions using
// the stub build (no dlib). Face records are seeded directly via db.Open so
// the label/unlabel/merge/find paths can be covered without real detection.

func TestIntegrationFacesDetect(t *testing.T) {
	dbPath := resetState(t)
	dir := t.TempDir()
	makeIntegrationJPEG(t, dir, "a.jpg", 100, 100)

	if err := execute(t, "scan", "--db", dbPath, "--quiet", dir); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	resetState(t)
	cfg.DBPath = dbPath
	// Stub binary: DlibAvailable() returns false, so detect prints an info
	// message and returns nil — no error expected.
	if err := execute(t, "faces", "--db", dbPath, "--quiet", "detect"); err != nil {
		t.Errorf("faces detect failed: %v", err)
	}
}

func TestIntegrationFacesList(t *testing.T) {
	dbPath := resetState(t)
	// Empty database — should print "no people" message without error.
	if err := execute(t, "faces", "--db", dbPath, "--quiet", "list"); err != nil {
		t.Errorf("faces list (empty) failed: %v", err)
	}
}

func TestIntegrationFacesUnlabeled(t *testing.T) {
	dbPath := resetState(t)
	// Empty database — should print "no unlabeled faces" message without error.
	if err := execute(t, "faces", "--db", dbPath, "--quiet", "unlabeled"); err != nil {
		t.Errorf("faces unlabeled (empty) failed: %v", err)
	}
}

// seedFace inserts an image + bare face record into the database and returns the face ID.
func seedFace(t *testing.T, dbPath string) (faceID int64) {
	t.Helper()
	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	imgID, err := store.InsertImage(&db.ImageRecord{
		FilePath: "/tmp/integration_test.jpg",
		FileSize: 1000,
		FileHash: "testhash",
		Width:    100,
		Height:   100,
		Format:   "jpeg",
		ModTime:  time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Descriptor: 128 float64 values encoded as 1024 zero bytes.
	faceID, err = store.InsertFace(&db.FaceRecord{
		ImageID:    imgID,
		Descriptor: make([]byte, 128*8),
	})
	if err != nil {
		t.Fatal(err)
	}
	return faceID
}

func TestIntegrationFacesLabel(t *testing.T) {
	dbPath := resetState(t)
	faceID := seedFace(t, dbPath)

	resetState(t)
	cfg.DBPath = dbPath
	if err := execute(t, "faces", "--db", dbPath, "--quiet", "label",
		fmt.Sprintf("%d", faceID), "Alice"); err != nil {
		t.Fatalf("faces label failed: %v", err)
	}

	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	person, err := store.GetPersonByName("Alice")
	if err != nil {
		t.Fatal(err)
	}
	if person == nil {
		t.Error("expected person 'Alice' to be created by faces label")
	}
}

func TestIntegrationFacesUnlabel(t *testing.T) {
	dbPath := resetState(t)
	faceID := seedFace(t, dbPath)

	// Label first.
	resetState(t)
	cfg.DBPath = dbPath
	if err := execute(t, "faces", "--db", dbPath, "--quiet", "label",
		fmt.Sprintf("%d", faceID), "Bob"); err != nil {
		t.Fatalf("faces label failed: %v", err)
	}

	// Then unlabel.
	resetState(t)
	cfg.DBPath = dbPath
	if err := execute(t, "faces", "--db", dbPath, "--quiet", "unlabel",
		fmt.Sprintf("%d", faceID)); err != nil {
		t.Errorf("faces unlabel failed: %v", err)
	}

	// Verify the face appears in the unlabeled list again.
	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	unlabeled, err := store.GetUnlabeledFaces()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, u := range unlabeled {
		if u.FaceID == faceID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("face %d should be unlabeled after 'faces unlabel'", faceID)
	}
}

func TestIntegrationFacesMerge(t *testing.T) {
	dbPath := resetState(t)

	// Insert two people directly.
	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	keepID, err := store.InsertPerson("Alice")
	if err != nil {
		t.Fatal(err)
	}
	mergeID, err := store.InsertPerson("Bob")
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Close()

	resetState(t)
	cfg.DBPath = dbPath
	if mergeErr := execute(t, "faces", "--db", dbPath, "--quiet", "merge",
		fmt.Sprintf("%d", keepID), fmt.Sprintf("%d", mergeID)); mergeErr != nil {
		t.Fatalf("faces merge failed: %v", mergeErr)
	}

	store, err = db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	people, err := store.GetAllPeople()
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 1 {
		t.Errorf("expected 1 person after merge, got %d", len(people))
	}
	if len(people) == 1 && people[0].Name != "Alice" {
		t.Errorf("expected 'Alice' to survive merge, got %q", people[0].Name)
	}
}

func TestIntegrationFacesFind(t *testing.T) {
	dbPath := resetState(t)

	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	imgID, err := store.InsertImage(&db.ImageRecord{
		FilePath: "/tmp/alice_photo.jpg",
		FileSize: 1000,
		FileHash: "hash_alice",
		Width:    100,
		Height:   100,
		Format:   "jpeg",
		ModTime:  time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	personID, err := store.InsertPerson("Alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertFace(&db.FaceRecord{
		ImageID:    imgID,
		Descriptor: make([]byte, 128*8),
		PersonID:   &personID,
	}); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()

	resetState(t)
	cfg.DBPath = dbPath
	if err := execute(t, "faces", "--db", dbPath, "--quiet", "find", "Alice"); err != nil {
		t.Errorf("faces find failed: %v", err)
	}
}

// ─── Tag integration tests ────────────────────────────────────────────────────

func TestIntegrationTagAdd(t *testing.T) {
	dbPath := resetState(t)
	dir := t.TempDir()
	path := makeIntegrationJPEG(t, dir, "photo.jpg", 100, 100)

	if err := execute(t, "scan", "--db", dbPath, "--quiet", dir); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	resetState(t)
	cfg.DBPath = dbPath
	if err := execute(t, "tag", "--db", dbPath, "--quiet", "add", path, "landscape", "nature"); err != nil {
		t.Fatalf("tag add failed: %v", err)
	}

	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	img, err := store.GetImageByPath(path)
	if err != nil || img == nil {
		t.Fatalf("image not found: %v", err)
	}
	tags, err := store.GetTagsForImage(img.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"landscape": true, "nature": true}
	for _, tag := range tags {
		delete(want, tag)
	}
	if len(want) > 0 {
		t.Errorf("missing tags after add: %v", want)
	}
}

func TestIntegrationTagRemove(t *testing.T) {
	dbPath := resetState(t)
	dir := t.TempDir()
	path := makeIntegrationJPEG(t, dir, "photo.jpg", 100, 100)

	if err := execute(t, "scan", "--db", dbPath, "--quiet", dir); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	resetState(t)
	cfg.DBPath = dbPath
	if err := execute(t, "tag", "--db", dbPath, "--quiet", "add", path, "landscape", "nature"); err != nil {
		t.Fatalf("tag add failed: %v", err)
	}

	resetState(t)
	cfg.DBPath = dbPath
	if err := execute(t, "tag", "--db", dbPath, "--quiet", "remove", path, "landscape"); err != nil {
		t.Fatalf("tag remove failed: %v", err)
	}

	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	img, _ := store.GetImageByPath(path)
	tags, _ := store.GetTagsForImage(img.ID)
	for _, tag := range tags {
		if tag == "landscape" {
			t.Error("expected 'landscape' to be removed")
		}
	}
	found := false
	for _, tag := range tags {
		if tag == "nature" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'nature' to remain after partial remove")
	}
}

func TestIntegrationTagList(t *testing.T) {
	dbPath := resetState(t)
	dir := t.TempDir()
	path := makeIntegrationJPEG(t, dir, "photo.jpg", 100, 100)

	if err := execute(t, "scan", "--db", dbPath, "--quiet", dir); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	resetState(t)
	cfg.DBPath = dbPath
	if err := execute(t, "tag", "--db", dbPath, "--quiet", "add", path, "sunset"); err != nil {
		t.Fatalf("tag add failed: %v", err)
	}

	resetState(t)
	cfg.DBPath = dbPath
	if err := execute(t, "tag", "--db", dbPath, "--quiet", "list", path); err != nil {
		t.Errorf("tag list failed: %v", err)
	}
}

func TestIntegrationTagAddUnindexed(t *testing.T) {
	dbPath := resetState(t)

	// Attempt to tag an image that was never scanned — should fail clearly.
	resetState(t)
	cfg.DBPath = dbPath
	err := execute(t, "tag", "--db", dbPath, "--quiet", "add", "/nonexistent/photo.jpg", "test")
	if err == nil {
		t.Error("expected error when tagging an unindexed image, got nil")
	}
}

// ─── info tests ───────────────────────────────────────────────────────────────

func TestIntegrationInfo(t *testing.T) {
	dbPath := resetState(t)
	dir := t.TempDir()
	path := makeIntegrationJPEG(t, dir, "photo.jpg", 120, 80)

	if err := execute(t, "scan", "--db", dbPath, "--quiet", dir); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	resetState(t)
	cfg.DBPath = dbPath
	if err := execute(t, "info", "--db", dbPath, path); err != nil {
		t.Errorf("info failed: %v", err)
	}
}

func TestIntegrationInfoUnindexed(t *testing.T) {
	dbPath := resetState(t)
	cfg.DBPath = dbPath

	err := execute(t, "info", "--db", dbPath, "/nonexistent/photo.jpg")
	if err == nil {
		t.Error("expected error for unindexed path, got nil")
	}
}

// ─── clean move-detection tests ───────────────────────────────────────────────

// seedImageWithHash inserts an image record with a specific file hash and
// returns its ID. The filePath need not exist on disk.
func seedImageWithHash(t *testing.T, store *db.Store, filePath, fileHash string) int64 {
	t.Helper()
	id, err := store.InsertImage(&db.ImageRecord{
		FilePath: filePath,
		FileSize: 1000,
		FileHash: fileHash,
		Width:    100,
		Height:   100,
		Format:   "jpeg",
		ModTime:  time.Now(),
	})
	if err != nil {
		t.Fatalf("inserting image %s: %v", filePath, err)
	}
	return id
}

func TestIntegrationCleanDetectMove(t *testing.T) {
	dbPath := resetState(t)
	dir := t.TempDir()

	// Create a real JPEG on disk at the "new" location.
	newPath := makeIntegrationJPEG(t, dir, "moved.jpg", 100, 100)

	// Scan the new path so Rufus has a DB record for it (with a real hash).
	if err := execute(t, "scan", "--db", dbPath, "--quiet", dir); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	// Retrieve the hash that scan computed, then seed the stale record.
	var realHash string
	var newID int64
	oldPath := filepath.Join(dir, "old_location", "moved.jpg")
	func() {
		store, openErr := db.Open(dbPath)
		if openErr != nil {
			t.Fatal(openErr)
		}
		defer func() { _ = store.Close() }()

		img, queryErr := store.GetImageByPath(newPath)
		if queryErr != nil || img == nil {
			t.Fatalf("image not found after scan: %v", queryErr)
		}
		realHash = img.FileHash
		newID = img.ID

		// Seed a "stale" record at an old path with the same hash and add a tag.
		oldID := seedImageWithHash(t, store, oldPath, realHash)
		if tagErr := store.InsertTag(oldID, "vacation"); tagErr != nil {
			t.Fatalf("inserting tag: %v", tagErr)
		}
	}()

	// Run clean — old path doesn't exist on disk, new path does.
	resetState(t)
	cfg.DBPath = dbPath
	cleanDryRun = false
	cleanVacuum = false
	if err := execute(t, "clean", "--db", dbPath); err != nil {
		t.Fatalf("clean failed: %v", err)
	}

	// Old record should be gone; new record should have the migrated tag.
	store2, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store2.Close() }()

	// Old record must be deleted.
	byPath, err := store2.GetImageByPath(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	if byPath != nil {
		t.Error("old record should have been removed after move detection")
	}

	// New record must still exist.
	byNew, err := store2.GetImageByPath(newPath)
	if err != nil || byNew == nil {
		t.Fatalf("new record missing after clean: %v", err)
	}
	if byNew.ID != newID {
		t.Errorf("new record ID changed unexpectedly: got %d, want %d", byNew.ID, newID)
	}

	// Tag "vacation" must have been migrated to the new record.
	tags, err := store2.GetTagsForImage(newID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tg := range tags {
		if tg == "vacation" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("tag 'vacation' was not migrated to new record; tags: %v", tags)
	}
}
