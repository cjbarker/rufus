package db

import (
	"database/sql"
	"fmt"
)

// scanImage scans an image row into an ImageRecord, converting int64 hash
// values back to uint64.
func scanImage(scanner interface{ Scan(...any) error }) (ImageRecord, error) {
	var img ImageRecord
	var ahash, dhash, phash int64
	err := scanner.Scan(&img.ID, &img.FilePath, &img.FileSize, &img.FileHash,
		&img.Width, &img.Height, &img.Format, &img.ModTime, &img.ScannedAt,
		&ahash, &dhash, &phash, &img.FaceScannedAt)
	img.AHash = int64ToHash(ahash)
	img.DHash = int64ToHash(dhash)
	img.PHash = int64ToHash(phash)
	return img, err
}

// InsertImage inserts or updates an image record. Returns the image ID.
func (s *Store) InsertImage(rec *ImageRecord) (int64, error) {
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
		rec.Format, rec.ModTime, hashToInt64(rec.AHash), hashToInt64(rec.DHash), hashToInt64(rec.PHash),
	)
	if err != nil {
		return 0, fmt.Errorf("inserting image: %w", err)
	}
	return result.LastInsertId()
}

// InsertImageBatch inserts or upserts multiple image records in a single
// transaction using a shared prepared statement. Significantly faster than
// calling InsertImage in a loop because it amortises per-transaction overhead.
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
			rec.Format, rec.ModTime, hashToInt64(rec.AHash), hashToInt64(rec.DHash), hashToInt64(rec.PHash),
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

// GetAllImages returns all image records ordered by file path.
func (s *Store) GetAllImages() ([]ImageRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, file_path, file_size, file_hash, width, height, format, mod_time, scanned_at, ahash, dhash, phash, face_scanned_at
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
		SELECT id, file_path, file_size, file_hash, width, height, format, mod_time, scanned_at, ahash, dhash, phash, face_scanned_at
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

// GetUnscannedImages returns images that have not yet had face detection run,
// identified by a NULL face_scanned_at value.
func (s *Store) GetUnscannedImages() ([]ImageRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, file_path, file_size, file_hash, width, height, format, mod_time, scanned_at, ahash, dhash, phash, face_scanned_at
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

// MarkImageFaceScanned sets face_scanned_at to the current time, indicating
// that face detection has completed for this image.
func (s *Store) MarkImageFaceScanned(id int64) error {
	_, err := s.db.Exec("UPDATE images SET face_scanned_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("marking image %d as face-scanned: %w", id, err)
	}
	return nil
}

// DeleteImage removes an image record by ID. Cascades to faces and tags.
func (s *Store) DeleteImage(id int64) error {
	_, err := s.db.Exec("DELETE FROM images WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting image %d: %w", id, err)
	}
	return nil
}

// GetImagesByHash returns all image records with the given SHA-256 hash.
// Typically returns one result, but may return multiple if the same file
// content was indexed from different paths.
func (s *Store) GetImagesByHash(fileHash string) ([]ImageRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, file_path, file_size, file_hash, width, height, format, mod_time, scanned_at, ahash, dhash, phash, face_scanned_at
		FROM images WHERE file_hash = ? ORDER BY file_path`, fileHash)
	if err != nil {
		return nil, fmt.Errorf("querying images by hash: %w", err)
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

// MigrateImageMetadata transfers faces and tags from one image record to
// another, then deletes the source record. Used by clean to preserve metadata
// when a file has been moved and the destination is already indexed.
// Tags that already exist on the destination are silently skipped.
func (s *Store) MigrateImageMetadata(fromID, toID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("starting migration transaction: %w", err)
	}
	if _, err := tx.Exec("UPDATE OR IGNORE tags SET image_id = ? WHERE image_id = ?", toID, fromID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("migrating tags: %w", err)
	}
	if _, err := tx.Exec("UPDATE faces SET image_id = ? WHERE image_id = ?", toID, fromID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("migrating faces: %w", err)
	}
	// Delete source record; CASCADE removes any remaining tags/faces that were
	// skipped due to conflicts (e.g. duplicate tags already on destination).
	if _, err := tx.Exec("DELETE FROM images WHERE id = ?", fromID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting source image %d: %w", fromID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing migration: %w", err)
	}
	return nil
}

// GetImagesByPerson returns all images containing a given person's face.
func (s *Store) GetImagesByPerson(personID int64) ([]ImageRecord, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT i.id, i.file_path, i.file_size, i.file_hash, i.width, i.height,
		       i.format, i.mod_time, i.scanned_at, i.ahash, i.dhash, i.phash, i.face_scanned_at
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
