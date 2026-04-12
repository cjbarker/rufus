package search

import (
	"fmt"
	"strings"
	"time"

	"github.com/cjbarker/rufus/internal/db"
)

// TagMode controls whether multi-tag filters use AND or OR logic.
type TagMode string

const (
	TagModeAnd TagMode = "and" // image must have ALL tags
	TagModeOr  TagMode = "or"  // image must have AT LEAST ONE tag
)

// SortField identifies the column used for ordering results.
type SortField string

const (
	SortByPath    SortField = "path"
	SortBySize    SortField = "size"
	SortByDate    SortField = "date"
	SortByFormat  SortField = "format"
)

// Query represents a search query with multiple filters.
type Query struct {
	// Tag filters (replaces the old single Tag field).
	Tags    []string // filter by one or more tags
	TagMode TagMode  // "and" (default) or "or"

	Face        string
	MinSize     int64
	MaxSize     int64
	Format      string
	PathPattern string
	Before      *time.Time
	After       *time.Time

	// Face-count filters.
	HasFaces bool // only images with at least one labeled face
	NoFaces  bool // only images with no detected faces

	// Pagination / ordering.
	SortBy     SortField
	SortDesc   bool
	Offset     int
	Limit      int
}

// Result wraps a search result with optional relevance info.
type Result struct {
	Image     db.ImageRecord
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

	orderCol := "i.file_path"
	switch q.SortBy {
	case SortBySize:
		orderCol = "i.file_size"
	case SortByDate:
		orderCol = "i.mod_time"
	case SortByFormat:
		orderCol = "i.format"
	}
	dir := "ASC"
	if q.SortDesc {
		dir = "DESC"
	}

	offset := q.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT i.id, i.file_path, i.file_size, i.file_hash, i.width, i.height,
		       i.format, i.mod_time, i.scanned_at, i.ahash, i.dhash, i.phash
		FROM images i
		%s
		ORDER BY %s %s
		LIMIT ? OFFSET ?`, where, orderCol, dir)

	args = append(args, limit, offset)

	rows, err := e.store.DB().Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

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

func buildWhere(q *Query) (sqlClause string, sqlArgs []any) {
	var joins []string
	var conditions []string
	var args []any

	// Multi-tag filtering.
	switch len(q.Tags) {
	case 0:
		// no tag filter
	case 1:
		joins = append(joins, "JOIN tags t ON t.image_id = i.id")
		conditions = append(conditions, "t.tag = ?")
		args = append(args, q.Tags[0])
	default:
		mode := q.TagMode
		if mode == "" {
			mode = TagModeAnd
		}
		placeholders := make([]string, len(q.Tags))
		for i, tag := range q.Tags {
			placeholders[i] = "?"
			args = append(args, tag)
		}
		inList := strings.Join(placeholders, ", ")
		if mode == TagModeAnd {
			// Correlated subquery: image must have ALL tags.
			conditions = append(conditions, fmt.Sprintf(
				`(SELECT COUNT(DISTINCT tag) FROM tags WHERE image_id = i.id AND tag IN (%s)) = %d`,
				inList, len(q.Tags)))
		} else {
			conditions = append(conditions, fmt.Sprintf(
				`EXISTS (SELECT 1 FROM tags WHERE image_id = i.id AND tag IN (%s))`, inList))
		}
	}

	if q.Face != "" {
		joins = append(joins, "JOIN faces f ON f.image_id = i.id")
		joins = append(joins, "JOIN people p ON p.id = f.person_id")
		conditions = append(conditions, "p.name = ?")
		args = append(args, q.Face)
	}

	if q.HasFaces {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM faces WHERE image_id = i.id AND person_id IS NOT NULL)")
	}
	if q.NoFaces {
		conditions = append(conditions, "NOT EXISTS (SELECT 1 FROM faces WHERE image_id = i.id)")
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
		conditions = append(conditions, "LOWER(i.file_path) LIKE LOWER(?)")
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
