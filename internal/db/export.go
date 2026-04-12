package db

import "fmt"

// ExportAll returns all images with their tags as ExportRecords, suitable for
// JSON or CSV serialization.
func (s *Store) ExportAll() ([]ExportRecord, error) {
	images, err := s.GetAllImages()
	if err != nil {
		return nil, err
	}
	records := make([]ExportRecord, 0, len(images))
	for _, img := range images {
		tags, err := s.GetTagsForImage(img.ID)
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

// ImportRecords bulk-inserts ExportRecords (image metadata + tags) in a single
// transaction. Existing images are updated (upsert); tags are inserted with OR IGNORE.
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
