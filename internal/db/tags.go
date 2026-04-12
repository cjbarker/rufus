package db

import "fmt"

// InsertTag adds a tag to an image. Silently ignores duplicates.
func (s *Store) InsertTag(imageID int64, tag string) error {
	_, err := s.db.Exec(
		"INSERT OR IGNORE INTO tags (image_id, tag) VALUES (?, ?)",
		imageID, tag,
	)
	return err
}

// RemoveTag removes a single tag from an image. No-ops if the tag does not exist.
func (s *Store) RemoveTag(imageID int64, tag string) error {
	_, err := s.db.Exec("DELETE FROM tags WHERE image_id = ? AND tag = ?", imageID, tag)
	if err != nil {
		return fmt.Errorf("removing tag %q from image %d: %w", tag, imageID, err)
	}
	return nil
}

// GetTagsForImage returns all tag strings for a given image ID, ordered alphabetically.
func (s *Store) GetTagsForImage(imageID int64) ([]string, error) {
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

// SearchByTag returns images that have the given tag, ordered by file path.
func (s *Store) SearchByTag(tag string) ([]ImageRecord, error) {
	rows, err := s.db.Query(`
		SELECT i.id, i.file_path, i.file_size, i.file_hash, i.width, i.height,
		       i.format, i.mod_time, i.scanned_at, i.ahash, i.dhash, i.phash, i.face_scanned_at
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
