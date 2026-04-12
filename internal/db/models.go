package db

import "time"

// ImageRecord represents an indexed image in the database.
type ImageRecord struct {
	ID       int64
	FilePath string
	FileSize int64
	FileHash string // SHA-256 hex
	Width    int
	Height   int
	Format   string
	ModTime  time.Time
	ScannedAt time.Time
	AHash    uint64
	DHash    uint64
	PHash    uint64
}

// FaceRecord represents a detected face in an image.
type FaceRecord struct {
	ID        int64
	ImageID   int64
	Descriptor []byte // 128-dim float64 encoding as binary
	BoundsX   int
	BoundsY   int
	BoundsW   int
	BoundsH   int
	PersonID  *int64
}

// PersonRecord represents a named person for face recognition.
type PersonRecord struct {
	ID        int64
	Name      string
	CreatedAt time.Time
}

// TagRecord represents a tag on an image.
type TagRecord struct {
	ID      int64
	ImageID int64
	Tag     string
}

// FaceWithPerson pairs a FaceRecord with the assigned person's name (empty if unlabeled).
type FaceWithPerson struct {
	Face       FaceRecord
	PersonName string
}

// UnlabeledFace is a detected face that has not yet been assigned a person name.
type UnlabeledFace struct {
	FaceID   int64
	FilePath string
	BoundsX  int
	BoundsY  int
	BoundsW  int
	BoundsH  int
}

// DuplicateGroup represents a group of duplicate images.
type DuplicateGroup struct {
	Images   []ImageRecord
	Distance int // Hamming distance between members
	HashType string
}

// ScanStats holds statistics for a scan operation.
type ScanStats struct {
	FilesFound   int
	FilesIndexed int
	FilesSkipped int
	Errors       int
}

// Stats holds aggregate database statistics.
type Stats struct {
	Images   int64
	Faces    int64
	People   int64
	Tags     int64
	DBSizeBytes int64
}

// ExportRecord is a flat representation of an image with its tags used
// for JSON/CSV export and import round-trips.
type ExportRecord struct {
	FilePath  string   `json:"file_path"`
	FileSize  int64    `json:"file_size"`
	FileHash  string   `json:"file_hash"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	Format    string   `json:"format"`
	ModTime   string   `json:"mod_time"`
	Tags      []string `json:"tags,omitempty"`
}
