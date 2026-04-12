// Package db provides the SQLite storage layer for rufus.
// The Store type and its methods are split across focused files:
//
//   - db.go      — Store type, Open/Close, schema, migrations
//   - images.go  — image CRUD (InsertImage, GetAllImages, …)
//   - faces.go   — face and person CRUD
//   - tags.go    — tag operations (InsertTag, RemoveTag, GetTagsForImage, …)
//   - export.go  — ExportAll, ImportRecords
//   - stats.go   — GetStats, Vacuum
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

// hashToInt64 converts a uint64 perceptual hash to int64 for SQLite storage
// using two's-complement bit reinterpretation. The conversion is bijective:
// every uint64 maps to a unique int64, preserving hashes with the high bit set.
func hashToInt64(h uint64) int64 { return int64(h) }

// int64ToHash converts an int64 retrieved from SQLite back to the original
// uint64 perceptual hash. This is the exact inverse of hashToInt64.
func int64ToHash(v int64) uint64 { return uint64(v) }

// migration describes a single schema change identified by a monotonically
// increasing version number. If skipOnErr is non-empty, errors whose message
// contains that substring are treated as a no-op (the migration is still
// recorded as applied). This handles cases like ALTER TABLE ADD COLUMN on a
// database that already has the column from the base schema.
type migration struct {
	version     int
	description string
	sql         string
	skipOnErr   string // optional: ignore errors containing this substring
}

// dbMigrations is the ordered list of all schema changes applied after the
// baseline schema. Add new migrations to the end; never edit existing ones.
var dbMigrations = []migration{
	{
		version:     1,
		description: "add face_scanned_at to images",
		sql:         "ALTER TABLE images ADD COLUMN face_scanned_at DATETIME",
		// New databases already have this column in the base schema; ignore the
		// "duplicate column name" error so the migration is still recorded.
		skipOnErr: "duplicate column name",
	},
}

// Store provides database operations for rufus.
type Store struct {
	db *sql.DB
}

// Open opens or creates the SQLite database at the given path, initializes
// the schema, and runs any pending migrations.
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

// OpenMemory opens an in-memory SQLite database. Useful for testing.
func OpenMemory() (*Store, error) {
	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(on)")
	if err != nil {
		return nil, fmt.Errorf("opening in-memory database: %w", err)
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

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for advanced queries (e.g. from the search engine).
func (s *Store) DB() *sql.DB {
	return s.db
}

// migrate runs any outstanding schema migrations in version order. Each
// migration is recorded in the schema_migrations table so it runs exactly once.
func (s *Store) migrate() error {
	// Bootstrap the migrations table. This is idempotent.
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version     INTEGER PRIMARY KEY,
		description TEXT    NOT NULL,
		applied_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	for _, m := range dbMigrations {
		var count int
		if err := s.db.QueryRow(
			"SELECT COUNT(*) FROM schema_migrations WHERE version = ?", m.version,
		).Scan(&count); err != nil {
			return fmt.Errorf("checking migration %d: %w", m.version, err)
		}
		if count > 0 {
			continue // already applied
		}

		if _, err := s.db.Exec(m.sql); err != nil {
			if m.skipOnErr != "" && strings.Contains(err.Error(), m.skipOnErr) {
				// Expected non-fatal error (e.g. column already exists); proceed.
			} else {
				return fmt.Errorf("applying migration %d (%s): %w", m.version, m.description, err)
			}
		}
		if _, err := s.db.Exec(
			"INSERT INTO schema_migrations (version, description) VALUES (?, ?)",
			m.version, m.description,
		); err != nil {
			return fmt.Errorf("recording migration %d: %w", m.version, err)
		}
	}
	return nil
}
