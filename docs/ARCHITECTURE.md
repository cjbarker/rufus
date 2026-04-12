# Rufus Architecture

Technical overview of Rufus internals, data flow, and design decisions.

## Project Structure

```
rufus/
├── main.go                  # Entry point
├── cmd/                     # CLI commands (Cobra)
│   ├── root.go              # Root command, global flags
│   ├── scan.go              # Image discovery and indexing
│   ├── dupes.go             # Duplicate detection and reporting
│   ├── search.go            # Query engine CLI
│   ├── faces.go             # Face detection/labeling subcommands
│   └── version.go           # Version info
├── internal/
│   ├── config/              # Application configuration with defaults
│   ├── crawler/             # Async directory traversal
│   ├── hasher/              # SHA-256 + perceptual hashing
│   ├── db/                  # SQLite storage layer
│   ├── duplicates/          # Duplicate grouping via Union-Find
│   ├── faces/               # Face detection/matching interface
│   ├── search/              # Query builder with filters
│   └── ui/                  # Terminal UI (colors, spinners, tables)
└── testdata/                # Test fixture images
```

## Data Flow

### Scan Pipeline

The scan command uses a channel-based pipeline with concurrent workers:

```
Directory
   │
   ▼
┌──────────┐     chan Result     ┌──────────────┐     chan indexResult     ┌──────────┐
│ Crawler  │ ──────────────────► │ Worker Pool  │ ──────────────────────► │ DB Writer│
│ (walk)   │                     │ (hash files) │                         │ (insert) │
└──────────┘                     └──────────────┘                         └──────────┘
```

1. **Crawler** (`crawler.Crawl`) walks the directory tree and emits `Result` structs on a buffered channel (capacity 256). Filters by image file extension.
2. **Worker Pool** -- `N` goroutines (configurable via `--workers`) read from a jobs channel, compute SHA-256 and perceptual hashes using `hasher.HashFile`, and emit results.
3. **DB Writer** -- A single goroutine batches results into SQLite inserts.
4. Coordination uses `sync.WaitGroup` for workers and an `atomic.Int64` for progress counters.

### Duplicate Detection

```
All images from DB
        │
        ▼
┌───────────────────┐
│ Exact Duplicates  │  Group by SHA-256 file hash
│ (hash map)        │
└───────┬───────────┘
        │
        ▼
┌───────────────────┐
│ Perceptual Dupes  │  O(n²) pairwise Hamming distance
│ (Union-Find)      │  on selected hash (aHash/dHash/pHash)
└───────┬───────────┘
        │
        ▼
┌───────────────────┐
│ Merge Groups      │  Combine exact + perceptual groups
└───────┬───────────┘
        │
        ▼
  Ranked output (keep recommendation by resolution/size)
```

- Exact duplicates are found first via SHA-256 hash map grouping.
- Perceptual duplicates use O(n^2) pairwise comparison with a Union-Find data structure to cluster images whose Hamming distance falls below the threshold.
- Groups are merged and each group is ranked to recommend which image to keep (highest resolution, then largest file size).

### Search Engine

The search command builds a SQL query dynamically from filter flags:

- `--tag` -- JOIN on `tags` table
- `--face` -- JOIN on `faces` + `people` tables
- `--min-size`, `--max-size` -- WHERE clauses on `file_size`
- `--format` -- WHERE on `format`
- `--before`, `--after` -- WHERE on `mod_time`
- `--path` -- LIKE pattern on `file_path`

All filters are combinable and additive (AND logic).

## Database

SQLite with WAL mode and foreign keys. Schema auto-initializes on `db.Open()`.

### Tables

| Table | Purpose |
|-------|---------|
| `images` | Core index: path, size, SHA-256 hash, dimensions, format, perceptual hashes |
| `people` | Named persons for face recognition |
| `faces` | Detected faces with 128-dim descriptor, bounding box, optional person link |
| `tags` | Image tags (many-to-many via image_id) |

### Indexes

- Perceptual hash indexes (`idx_images_ahash`, `idx_images_dhash`, `idx_images_phash`) for fast duplicate lookups
- File hash index (`idx_images_file_hash`) for exact duplicate detection
- Face and tag indexes for search query performance

### Design Decisions

- **Pure Go SQLite** (`modernc.org/sqlite`) -- No CGO dependency, simplifying cross-compilation and builds.
- **WAL mode** -- Enables concurrent reads during writes.
- **Perceptual hashes stored as INT64** -- Cast from uint64, enabling direct comparison in SQL and efficient indexing.

## Concurrency Model

- **Fan-out worker pool** -- Configurable worker count for CPU-bound hashing operations.
- **Buffered channels** (capacity 256) -- Decouple producer (crawler) from consumers (hashers) to prevent backpressure stalls.
- **Single DB writer** -- Serializes writes to avoid SQLite contention.
- **Atomic counters** (`atomic.Int64`) -- Lock-free progress tracking across goroutines.

## Hashing

Each image is hashed in two ways:

1. **SHA-256** -- Cryptographic hash of file bytes for exact duplicate detection.
2. **Perceptual hashes** via `goimagehash`:
   - **aHash** -- Average hash. Compares pixel luminance to the mean.
   - **dHash** -- Difference hash. Encodes relative brightness between adjacent pixels.
   - **pHash** -- Perceptual hash. Uses DCT for frequency-domain comparison.

Similarity is measured by Hamming distance (number of differing bits). Lower distance = more similar.

## Dependencies

| Dependency | Purpose |
|------------|---------|
| `spf13/cobra` | CLI framework with subcommands and flag parsing |
| `corona10/goimagehash` | Perceptual hashing (aHash, dHash, pHash) |
| `modernc.org/sqlite` | Pure Go SQLite driver (no CGO) |
| `golang.org/x/image` | BMP, TIFF, WebP format decoders |
| `charmbracelet/lipgloss` | Terminal styling (colors, borders, tables) |

## CI/CD

GitHub Actions pipeline: lint -> test (Go 1.23/1.24 matrix with race detector) -> build -> release (goreleaser on `v*` tags).
