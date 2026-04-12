package db

import (
	"fmt"
	"os"
)

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
