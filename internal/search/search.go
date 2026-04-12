package search

import (
	"fmt"
	"strings"
	"time"

	"github.com/cjbarker/rufus/internal/db"
)

// Query represents a search query with multiple filters.
type Query struct {
	Tag         string
	Face        string
	MinSize     int64
	MaxSize     int64
	Format      string
	PathPattern string
	Before      *time.Time
	After       *time.Time
	Limit       int
}

// Result wraps a search result with optional relevance info.
type Result struct {
	Image    db.ImageRecord
	MatchedBy string
}

// Engine provides search capabilities over the image index.
type Engine struct {
	store *db.Store
}

// NewEngine creates a search engine backed by the given store.
func NewEngine(store *db.Store) *Engine {
	return &Engine{store: store}
}

// Search executes a search query and returns matching images.
func (e *Engine) Search(q *Query) ([]Result, error) {
	where, args := buildWhere(q)

	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT i.id, i.file_path, i.file_size, i.file_hash, i.width, i.height,
		       i.format, i.mod_time, i.scanned_at, i.ahash, i.dhash, i.phash
		FROM images i
		%s
		ORDER BY i.file_path
		LIMIT ?`, where)

	args = append(args, limit)

	rows, err := e.store.DB().Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var img db.ImageRecord
		var ahash, dhash, phash int64
		if err := rows.Scan(&img.ID, &img.FilePath, &img.FileSize, &img.FileHash,
			&img.Width, &img.Height, &img.Format, &img.ModTime, &img.ScannedAt,
			&ahash, &dhash, &phash); err != nil {
			return nil, fmt.Errorf("scanning result: %w", err)
		}
		img.AHash = uint64(ahash)
		img.DHash = uint64(dhash)
		img.PHash = uint64(phash)
		results = append(results, Result{Image: img})
	}
	return results, rows.Err()
}

func buildWhere(q *Query) (string, []any) {
	var joins []string
	var conditions []string
	var args []any

	if q.Tag != "" {
		joins = append(joins, "JOIN tags t ON t.image_id = i.id")
		conditions = append(conditions, "t.tag = ?")
		args = append(args, q.Tag)
	}

	if q.Face != "" {
		joins = append(joins, "JOIN faces f ON f.image_id = i.id")
		joins = append(joins, "JOIN people p ON p.id = f.person_id")
		conditions = append(conditions, "p.name = ?")
		args = append(args, q.Face)
	}

	if q.MinSize > 0 {
		conditions = append(conditions, "i.file_size >= ?")
		args = append(args, q.MinSize)
	}

	if q.MaxSize > 0 {
		conditions = append(conditions, "i.file_size <= ?")
		args = append(args, q.MaxSize)
	}

	if q.Format != "" {
		conditions = append(conditions, "i.format = ?")
		args = append(args, q.Format)
	}

	if q.PathPattern != "" {
		conditions = append(conditions, "i.file_path LIKE ?")
		args = append(args, "%"+q.PathPattern+"%")
	}

	if q.Before != nil {
		conditions = append(conditions, "i.mod_time < ?")
		args = append(args, *q.Before)
	}

	if q.After != nil {
		conditions = append(conditions, "i.mod_time > ?")
		args = append(args, *q.After)
	}

	clause := ""
	if len(joins) > 0 {
		clause += "\n" + strings.Join(joins, "\n")
	}
	if len(conditions) > 0 {
		clause += "\nWHERE " + strings.Join(conditions, " AND ")
	}

	return clause, args
}
