package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS images (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_path TEXT UNIQUE NOT NULL,
    file_size INTEGER NOT NULL,
    file_hash TEXT NOT NULL,
    width INTEGER,
    height INTEGER,
    format TEXT,
    mod_time DATETIME NOT NULL,
    scanned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    ahash INTEGER DEFAULT 0,
    dhash INTEGER DEFAULT 0,
    phash INTEGER DEFAULT 0,
    face_scanned_at DATETIME
);

CREATE TABLE IF NOT EXISTS people (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS faces (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    image_id INTEGER NOT NULL REFERENCES images(id) ON DELETE CASCADE,
    descriptor BLOB NOT NULL,
    bounds_x INTEGER,
    bounds_y INTEGER,
    bounds_w INTEGER,
    bounds_h INTEGER,
    person_id INTEGER REFERENCES people(id)
);

CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    image_id INTEGER NOT NULL REFERENCES images(id) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    UNIQUE(image_id, tag)
);

CREATE INDEX IF NOT EXISTS idx_images_dhash ON images(dhash);
CREATE INDEX IF NOT EXISTS idx_images_phash ON images(phash);
CREATE INDEX IF NOT EXISTS idx_images_ahash ON images(ahash);
CREATE INDEX IF NOT EXISTS idx_images_file_hash ON images(file_hash);
CREATE INDEX IF NOT EXISTS idx_faces_person ON faces(person_id);
CREATE INDEX IF NOT EXISTS idx_faces_image ON faces(image_id);
CREATE INDEX IF NOT EXISTS idx_tags_tag ON tags(tag);
CREATE INDEX IF NOT EXISTS idx_tags_image ON tags(image_id);
`

// Store provides database operations for rufus.
type Store struct {
	db *sql.DB
}

// Open opens or creates the SQLite database at the given path and initializes the schema.
func Open(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(wal)&_pragma=foreign_keys(on)")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing schema: %w", err)
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	return store, nil
}

// migrate applies any schema changes needed for existing databases.
func (s *Store) migrate() error {
	// Add face_scanned_at to images if it doesn't exist (added after initial release).
	_, err := s.db.Exec("ALTER TABLE images ADD COLUMN face_scanned_at DATETIME")
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return fmt.Errorf("adding face_scanned_at column: %w", err)
	}
	return nil
}

// OpenMemory opens an in-memory SQLite database (useful for testing).
func OpenMemory() (*Store, error) {
	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(on)")
	if err != nil {
		return nil, fmt.Errorf("opening in-memory database: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing schema: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for advanced queries.
func (s *Store) DB() *sql.DB {
	return s.db
}

// InsertImage inserts or updates an image record. Returns the image ID.
func (s *Store) InsertImage(rec *ImageRecord) (int64, error) {
	// Cast uint64 hashes to int64 for SQLite compatibility
	result, err := s.db.Exec(`
		INSERT INTO images (file_path, file_size, file_hash, width, height, format, mod_time, ahash, dhash, phash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(file_path) DO UPDATE SET
			file_size=excluded.file_size,
			file_hash=excluded.file_hash,
			width=excluded.width,
			height=excluded.height,
			format=excluded.format,
			mod_time=excluded.mod_time,
			ahash=excluded.ahash,
			dhash=excluded.dhash,
			phash=excluded.phash,
			scanned_at=CURRENT_TIMESTAMP`,
		rec.FilePath, rec.FileSize, rec.FileHash, rec.Width, rec.Height,
		rec.Format, rec.ModTime, int64(rec.AHash), int64(rec.DHash), int64(rec.PHash),
	)
	if err != nil {
		return 0, fmt.Errorf("inserting image: %w", err)
	}
	return result.LastInsertId()
}

// InsertImageBatch inserts or upserts multiple image records in a single
// transaction using a shared prepared statement. This is significantly faster
// than calling InsertImage in a loop because it amortises the per-transaction
// overhead across the whole batch.
func (s *Store) InsertImageBatch(recs []*ImageRecord) error {
	if len(recs) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("starting batch transaction: %w", err)
	}
	stmt, err := tx.Prepare(`
		INSERT INTO images (file_path, file_size, file_hash, width, height, format, mod_time, ahash, dhash, phash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(file_path) DO UPDATE SET
			file_size=excluded.file_size,
			file_hash=excluded.file_hash,
			width=excluded.width,
			height=excluded.height,
			format=excluded.format,
			mod_time=excluded.mod_time,
			ahash=excluded.ahash,
			dhash=excluded.dhash,
			phash=excluded.phash,
			scanned_at=CURRENT_TIMESTAMP`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("preparing batch statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, rec := range recs {
		if _, err := stmt.Exec(
			rec.FilePath, rec.FileSize, rec.FileHash, rec.Width, rec.Height,
			rec.Format, rec.ModTime, int64(rec.AHash), int64(rec.DHash), int64(rec.PHash),
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("inserting %s: %w", rec.FilePath, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing batch: %w", err)
	}
	return nil
}

// WalkUnlabeledFacesWithDescriptors streams face records that have no assigned
// person, invoking fn for each one without loading the full result set into
// memory. fn may call other Store methods safely (WAL mode allows concurrent
// reads and writes). Return a non-nil error from fn to abort the walk.
func (s *Store) WalkUnlabeledFacesWithDescriptors(fn func(FaceRecord) error) error {
	rows, err := s.db.Query(`
		SELECT id, image_id, descriptor, bounds_x, bounds_y, bounds_w, bounds_h, person_id
		FROM faces WHERE person_id IS NULL ORDER BY id`)
	if err != nil {
		return fmt.Errorf("querying unlabeled faces: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var f FaceRecord
		if err := rows.Scan(&f.ID, &f.ImageID, &f.Descriptor,
			&f.BoundsX, &f.BoundsY, &f.BoundsW, &f.BoundsH, &f.PersonID); err != nil {
			return fmt.Errorf("scanning face row: %w", err)
		}
		if err := fn(f); err != nil {
			return err
		}
	}
	return rows.Err()
}

// scanImage scans an image row into an ImageRecord, handling int64-to-uint64 hash conversion.
func scanImage(scanner interface{ Scan(...any) error }) (ImageRecord, error) {
	var img ImageRecord
	var ahash, dhash, phash int64
	err := scanner.Scan(&img.ID, &img.FilePath, &img.FileSize, &img.FileHash,
		&img.Width, &img.Height, &img.Format, &img.ModTime, &img.ScannedAt,
		&ahash, &dhash, &phash)
	img.AHash = uint64(ahash)
	img.DHash = uint64(dhash)
	img.PHash = uint64(phash)
	return img, err
}

// GetAllImages returns all image records from the database.
func (s *Store) GetAllImages() ([]ImageRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, file_path, file_size, file_hash, width, height, format, mod_time, scanned_at, ahash, dhash, phash
		FROM images ORDER BY file_path`)
	if err != nil {
		return nil, fmt.Errorf("querying images: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var images []ImageRecord
	for rows.Next() {
		img, err := scanImage(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning image row: %w", err)
		}
		images = append(images, img)
	}
	return images, rows.Err()
}

// GetImageByPath returns an image record by file path, or nil if not found.
func (s *Store) GetImageByPath(path string) (*ImageRecord, error) {
	row := s.db.QueryRow(`
		SELECT id, file_path, file_size, file_hash, width, height, format, mod_time, scanned_at, ahash, dhash, phash
		FROM images WHERE file_path = ?`, path)
	img, err := scanImage(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying image by path: %w", err)
	}
	return &img, nil
}

// ImageCount returns the total number of indexed images.
func (s *Store) ImageCount() (int64, error) {
	var count int64
	err := s.db.QueryRow("SELECT COUNT(*) FROM images").Scan(&count)
	return count, err
}

// GetUnscannedImages returns all images that have not yet had face detection run on them,
// identified by a NULL face_scanned_at timestamp.
func (s *Store) GetUnscannedImages() ([]ImageRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, file_path, file_size, file_hash, width, height, format, mod_time, scanned_at, ahash, dhash, phash
		FROM images
		WHERE face_scanned_at IS NULL
		ORDER BY file_path`)
	if err != nil {
		return nil, fmt.Errorf("querying unscanned images: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var images []ImageRecord
	for rows.Next() {
		img, err := scanImage(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning image row: %w", err)
		}
		images = append(images, img)
	}
	return images, rows.Err()
}

// MarkImageFaceScanned sets face_scanned_at to the current time for the given image,
// indicating that face detection has been run (regardless of how many faces were found).
func (s *Store) MarkImageFaceScanned(id int64) error {
	_, err := s.db.Exec("UPDATE images SET face_scanned_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("marking image %d as face-scanned: %w", id, err)
	}
	return nil
}

// DeleteFacesByImage removes all face records for a given image, used when
// force-rescanning to avoid accumulating duplicate detections.
func (s *Store) DeleteFacesByImage(imageID int64) error {
	_, err := s.db.Exec("DELETE FROM faces WHERE image_id = ?", imageID)
	if err != nil {
		return fmt.Errorf("deleting faces for image %d: %w", imageID, err)
	}
	return nil
}

// GetAllFacesWithPerson returns all face records joined with their person name (if labeled).
func (s *Store) GetAllFacesWithPerson() ([]FaceWithPerson, error) {
	rows, err := s.db.Query(`
		SELECT f.id, f.image_id, f.descriptor, f.bounds_x, f.bounds_y, f.bounds_w, f.bounds_h,
		       f.person_id, COALESCE(p.name, '') as person_name
		FROM faces f
		LEFT JOIN people p ON p.id = f.person_id
		ORDER BY f.id`)
	if err != nil {
		return nil, fmt.Errorf("querying faces: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []FaceWithPerson
	for rows.Next() {
		var fw FaceWithPerson
		if err := rows.Scan(&fw.Face.ID, &fw.Face.ImageID, &fw.Face.Descriptor,
			&fw.Face.BoundsX, &fw.Face.BoundsY, &fw.Face.BoundsW, &fw.Face.BoundsH,
			&fw.Face.PersonID, &fw.PersonName); err != nil {
			return nil, fmt.Errorf("scanning face row: %w", err)
		}
		results = append(results, fw)
	}
	return results, rows.Err()
}

// GetUnlabeledFaces returns all detected faces that have not yet been assigned a person,
// joined with their image file path, ordered by file path then face ID.
func (s *Store) GetUnlabeledFaces() ([]UnlabeledFace, error) {
	rows, err := s.db.Query(`
		SELECT f.id, i.file_path, f.bounds_x, f.bounds_y, f.bounds_w, f.bounds_h
		FROM faces f
		JOIN images i ON i.id = f.image_id
		WHERE f.person_id IS NULL
		ORDER BY i.file_path, f.id`)
	if err != nil {
		return nil, fmt.Errorf("querying unlabeled faces: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var faces []UnlabeledFace
	for rows.Next() {
		var f UnlabeledFace
		if err := rows.Scan(&f.FaceID, &f.FilePath, &f.BoundsX, &f.BoundsY, &f.BoundsW, &f.BoundsH); err != nil {
			return nil, fmt.Errorf("scanning unlabeled face row: %w", err)
		}
		faces = append(faces, f)
	}
	return faces, rows.Err()
}

// GetUnlabeledFacesWithDescriptors returns all face records with no assigned person,
// including their descriptor blobs for re-matching against known labels.
func (s *Store) GetUnlabeledFacesWithDescriptors() ([]FaceRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, image_id, descriptor, bounds_x, bounds_y, bounds_w, bounds_h, person_id
		FROM faces WHERE person_id IS NULL ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("querying unlabeled faces: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var recs []FaceRecord
	for rows.Next() {
		var f FaceRecord
		if err := rows.Scan(&f.ID, &f.ImageID, &f.Descriptor,
			&f.BoundsX, &f.BoundsY, &f.BoundsW, &f.BoundsH, &f.PersonID); err != nil {
			return nil, fmt.Errorf("scanning face row: %w", err)
		}
		recs = append(recs, f)
	}
	return recs, rows.Err()
}

// DeleteImage removes an image record from the database by ID.
// The caller is responsible for deleting the actual file from the filesystem.
func (s *Store) DeleteImage(id int64) error {
	_, err := s.db.Exec("DELETE FROM images WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting image %d: %w", id, err)
	}
	return nil
}

// InsertFace inserts a face detection record.
func (s *Store) InsertFace(rec *FaceRecord) (int64, error) {
	result, err := s.db.Exec(`
		INSERT INTO faces (image_id, descriptor, bounds_x, bounds_y, bounds_w, bounds_h, person_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rec.ImageID, rec.Descriptor, rec.BoundsX, rec.BoundsY, rec.BoundsW, rec.BoundsH, rec.PersonID,
	)
	if err != nil {
		return 0, fmt.Errorf("inserting face: %w", err)
	}
	return result.LastInsertId()
}

// InsertPerson inserts a named person. Returns the person ID.
func (s *Store) InsertPerson(name string) (int64, error) {
	result, err := s.db.Exec("INSERT INTO people (name) VALUES (?)", name)
	if err != nil {
		return 0, fmt.Errorf("inserting person: %w", err)
	}
	return result.LastInsertId()
}

// GetPersonByName returns a person by name, or nil if not found.
func (s *Store) GetPersonByName(name string) (*PersonRecord, error) {
	var p PersonRecord
	err := s.db.QueryRow("SELECT id, name, created_at FROM people WHERE name = ?", name).
		Scan(&p.ID, &p.Name, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying person: %w", err)
	}
	return &p, nil
}

// GetAllPeople returns all named people.
func (s *Store) GetAllPeople() ([]PersonRecord, error) {
	rows, err := s.db.Query("SELECT id, name, created_at FROM people ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("querying people: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var people []PersonRecord
	for rows.Next() {
		var p PersonRecord
		if err := rows.Scan(&p.ID, &p.Name, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning person row: %w", err)
		}
		people = append(people, p)
	}
	return people, rows.Err()
}

// InsertTag adds a tag to an image. Ignores duplicates.
func (s *Store) InsertTag(imageID int64, tag string) error {
	_, err := s.db.Exec(
		"INSERT OR IGNORE INTO tags (image_id, tag) VALUES (?, ?)",
		imageID, tag,
	)
	return err
}

// SearchByTag returns images matching the given tag.
func (s *Store) SearchByTag(tag string) ([]ImageRecord, error) {
	rows, err := s.db.Query(`
		SELECT i.id, i.file_path, i.file_size, i.file_hash, i.width, i.height,
		       i.format, i.mod_time, i.scanned_at, i.ahash, i.dhash, i.phash
		FROM images i
		JOIN tags t ON t.image_id = i.id
		WHERE t.tag = ?
		ORDER BY i.file_path`, tag)
	if err != nil {
		return nil, fmt.Errorf("searching by tag: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var images []ImageRecord
	for rows.Next() {
		img, err := scanImage(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning image row: %w", err)
		}
		images = append(images, img)
	}
	return images, rows.Err()
}

// GetFacesByImage returns all face records for a given image.
func (s *Store) GetFacesByImage(imageID int64) ([]FaceRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, image_id, descriptor, bounds_x, bounds_y, bounds_w, bounds_h, person_id
		FROM faces WHERE image_id = ?`, imageID)
	if err != nil {
		return nil, fmt.Errorf("querying faces: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var faces []FaceRecord
	for rows.Next() {
		var f FaceRecord
		if err := rows.Scan(&f.ID, &f.ImageID, &f.Descriptor,
			&f.BoundsX, &f.BoundsY, &f.BoundsW, &f.BoundsH, &f.PersonID); err != nil {
			return nil, fmt.Errorf("scanning face row: %w", err)
		}
		faces = append(faces, f)
	}
	return faces, rows.Err()
}

// UpdateFacePerson assigns a person ID to a face.
func (s *Store) UpdateFacePerson(faceID, personID int64) error {
	_, err := s.db.Exec("UPDATE faces SET person_id = ? WHERE id = ?", personID, faceID)
	return err
}

// UnlabelFace removes the person assignment from a face (sets person_id = NULL).
func (s *Store) UnlabelFace(faceID int64) error {
	_, err := s.db.Exec("UPDATE faces SET person_id = NULL WHERE id = ?", faceID)
	if err != nil {
		return fmt.Errorf("unlabeling face %d: %w", faceID, err)
	}
	return nil
}

// MergePeople reassigns all faces from mergeID to keepID, then deletes mergeID.
// Both operations run inside a single transaction.
func (s *Store) MergePeople(keepID, mergeID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning merge transaction: %w", err)
	}
	if _, err := tx.Exec("UPDATE faces SET person_id = ? WHERE person_id = ?", keepID, mergeID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("reassigning faces: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM people WHERE id = ?", mergeID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting merged person: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing merge: %w", err)
	}
	return nil
}

// GetStats returns aggregate counts for images, faces, people, and tags,
// plus the on-disk size of the database file.
func (s *Store) GetStats(dbPath string) (Stats, error) {
	var st Stats
	row := s.db.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM images),
			(SELECT COUNT(*) FROM faces),
			(SELECT COUNT(*) FROM people),
			(SELECT COUNT(*) FROM tags)`)
	if err := row.Scan(&st.Images, &st.Faces, &st.People, &st.Tags); err != nil {
		return st, fmt.Errorf("querying stats: %w", err)
	}
	if dbPath != "" && dbPath != ":memory:" {
		info, err := os.Stat(dbPath)
		if err == nil {
			st.DBSizeBytes = info.Size()
		}
	}
	return st, nil
}

// Vacuum runs SQLite's VACUUM command to reclaim unused space after deletions.
func (s *Store) Vacuum() error {
	_, err := s.db.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("vacuuming database: %w", err)
	}
	return nil
}

// ExportAll returns all images with their tags as ExportRecords, suitable for
// JSON or CSV serialization.
func (s *Store) ExportAll() ([]ExportRecord, error) {
	images, err := s.GetAllImages()
	if err != nil {
		return nil, err
	}
	records := make([]ExportRecord, 0, len(images))
	for _, img := range images {
		tags, err := s.getTagsForImage(img.ID)
		if err != nil {
			return nil, err
		}
		records = append(records, ExportRecord{
			FilePath: img.FilePath,
			FileSize: img.FileSize,
			FileHash: img.FileHash,
			Width:    img.Width,
			Height:   img.Height,
			Format:   img.Format,
			ModTime:  img.ModTime.Format("2006-01-02T15:04:05Z07:00"),
			Tags:     tags,
		})
	}
	return records, nil
}

// getTagsForImage returns all tag strings for a given image ID.
func (s *Store) getTagsForImage(imageID int64) ([]string, error) {
	rows, err := s.db.Query("SELECT tag FROM tags WHERE image_id = ? ORDER BY tag", imageID)
	if err != nil {
		return nil, fmt.Errorf("querying tags for image %d: %w", imageID, err)
	}
	defer func() { _ = rows.Close() }()
	var tags []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("scanning tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// ImportRecords bulk-inserts ExportRecords (image metadata + tags) in a single
// transaction. Existing images are updated; tags are inserted with OR IGNORE.
func (s *Store) ImportRecords(recs []ExportRecord) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning import transaction: %w", err)
	}
	imgStmt, err := tx.Prepare(`
		INSERT INTO images (file_path, file_size, file_hash, width, height, format, mod_time)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(file_path) DO UPDATE SET
			file_size=excluded.file_size,
			file_hash=excluded.file_hash,
			width=excluded.width,
			height=excluded.height,
			format=excluded.format,
			mod_time=excluded.mod_time,
			scanned_at=CURRENT_TIMESTAMP`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("preparing import statement: %w", err)
	}
	defer func() { _ = imgStmt.Close() }()

	tagStmt, err := tx.Prepare("INSERT OR IGNORE INTO tags (image_id, tag) VALUES (?, ?)")
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("preparing tag insert statement: %w", err)
	}
	defer func() { _ = tagStmt.Close() }()

	for _, rec := range recs {
		res, err := imgStmt.Exec(rec.FilePath, rec.FileSize, rec.FileHash, rec.Width, rec.Height, rec.Format, rec.ModTime)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("importing %s: %w", rec.FilePath, err)
		}
		imgID, err := res.LastInsertId()
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("getting insert id for %s: %w", rec.FilePath, err)
		}
		for _, tag := range rec.Tags {
			if _, err := tagStmt.Exec(imgID, tag); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("importing tag %q for %s: %w", tag, rec.FilePath, err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing import: %w", err)
	}
	return nil
}

// GetImagesByPerson returns all images containing a named person's face.
func (s *Store) GetImagesByPerson(personID int64) ([]ImageRecord, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT i.id, i.file_path, i.file_size, i.file_hash, i.width, i.height,
		       i.format, i.mod_time, i.scanned_at, i.ahash, i.dhash, i.phash
		FROM images i
		JOIN faces f ON f.image_id = i.id
		WHERE f.person_id = ?
		ORDER BY i.file_path`, personID)
	if err != nil {
		return nil, fmt.Errorf("querying images by person: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var images []ImageRecord
	for rows.Next() {
		img, err := scanImage(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning image row: %w", err)
		}
		images = append(images, img)
	}
	return images, rows.Err()
}
