# Rufus Architecture

Technical overview of Rufus internals, data flow, and design decisions.

## Project Structure

```
rufus/
├── main.go                  # Entry point
├── cmd/                     # CLI commands (Cobra)
│   ├── root.go              # Root command, global flags, config layering
│   ├── scan.go              # Image discovery and indexing
│   ├── dupes.go             # Duplicate detection and reporting
│   ├── search.go            # Query engine CLI
│   ├── stats.go             # Library statistics
│   ├── export.go            # Export index to JSON/CSV
│   ├── import.go            # Import index from JSON/CSV
│   ├── clean.go             # Remove stale database records
│   ├── faces.go             # Face detection/labeling subcommands
│   ├── version.go           # Version info
│   └── integration_test.go  # End-to-end CLI integration tests
├── internal/
│   ├── config/              # Application configuration with defaults and env-var loading
│   ├── crawler/             # Async directory traversal with exclude support
│   ├── hasher/              # SHA-256 + perceptual hashing (AHash, DHash, PHash)
│   ├── db/                  # SQLite storage layer (Store pattern, migrations)
│   ├── duplicates/          # Duplicate grouping via BK-tree + Union-Find
│   ├── faces/               # Face detection/matching interface (dlib stub)
│   ├── search/              # Query builder with filters
│   ├── ui/                  # Terminal UI (colors, spinners, tables)
│   └── util/                # Shared helpers (FormatSize, ParseSize)
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

1. **Crawler** (`crawler.Crawl`) walks the directory tree and emits `Result` structs on a buffered channel (capacity 256). Filters by image file extension. Directories matching the `--exclude` list (by name, absolute path, or glob) are pruned with `fs.SkipDir`.
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
│ Perceptual Dupes  │  O(n log n) BK-tree lookup on selected hash
│ (BK-tree +        │  (aHash/dHash/pHash); Union-Find clusters
│  Union-Find)      │  images within the Hamming distance threshold
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
- Perceptual duplicates use a **BK-tree** for O(n log n) Hamming-distance range queries. A Union-Find data structure then clusters images whose distance falls below the threshold.
- Groups are merged and each group is ranked to recommend which image to keep (highest resolution, then largest file size).

### Search Engine

The search command builds a SQL query dynamically from filter flags:

- `--tags` + `--tag-mode` -- AND mode uses a correlated subquery counting distinct matching tags; OR mode uses `EXISTS` on the tags table
- `--face` -- JOIN on `faces` + `people` tables
- `--has-faces` -- `EXISTS (SELECT 1 FROM faces WHERE image_id = i.id AND person_id IS NOT NULL)`
- `--no-faces` -- `NOT EXISTS (SELECT 1 FROM faces WHERE image_id = i.id)`
- `--min-size`, `--max-size` -- WHERE clauses on `file_size`
- `--format` -- WHERE on `format`
- `--before`, `--after` -- WHERE on `mod_time`
- `--path` -- LIKE pattern on `file_path`
- `--sort-by`, `--sort-desc` -- ORDER BY clause (path, size, date, format)
- `--limit`, `--offset` -- LIMIT / OFFSET for pagination

All filters are combinable and additive (AND logic between filter types).

### Configuration Layering

`PersistentPreRunE` in `cmd/root.go` runs before every subcommand and applies configuration in priority order:

1. Load `~/.rufus/config.json` (absent file is silently ignored)
2. Apply environment variables (`RUFUS_*`) via `config.ApplyEnv`
3. Re-apply CLI flags that were explicitly set (checked via `pflag.Changed`)

This ensures that a flag explicitly passed on the command line always wins, while still allowing the config file and env vars to set useful defaults without being overridden by Cobra's zero values.

## Database

SQLite with WAL mode and foreign keys. Schema auto-initializes on `db.Open()` via a versioned migration system.

### Tables

| Table | Purpose |
|-------|---------|
| `images` | Core index: path, size, SHA-256 hash, dimensions, format, perceptual hashes, face-scan timestamp |
| `people` | Named persons for face recognition |
| `faces` | Detected faces with 128-dim descriptor, bounding box, optional person link |
| `tags` | Image tags (many-to-many via image_id) |
| `schema_migrations` | Migration version log (version, description, applied_at) |

### Indexes

- Perceptual hash indexes (`idx_images_ahash`, `idx_images_dhash`, `idx_images_phash`) for fast duplicate lookups
- File hash index (`idx_images_file_hash`) for exact duplicate detection
- Face and tag indexes for search query performance

### Migration System

`db.Open()` runs `migrate()` which applies all pending entries in the `dbMigrations` slice in version order, recording each applied migration in `schema_migrations`. Migrations are idempotent:

- Already-applied versions are skipped.
- DDL statements that are expected to fail on new databases (e.g., `ALTER TABLE ADD COLUMN` for columns already in the base schema) carry a `skipOnErr` substring; if the SQLite error message contains that substring, the error is suppressed and the migration is still recorded as applied.

`OpenMemory()` also runs `migrate()`, ensuring in-memory test stores see the same schema as production.

### Design Decisions

- **Pure Go SQLite** (`modernc.org/sqlite`) -- No CGO dependency, simplifying cross-compilation and builds.
- **WAL mode** -- Enables concurrent reads during writes.
- **Perceptual hashes stored as INT64** -- SQLite has no UINT64 type. `hashToInt64` and `int64ToHash` helpers perform a bijective two's-complement conversion, making the fragile `int64(uint64val)` cast explicit, documented, and tested with a round-trip test covering values with the high bit set (`0x8000000000000000`, `0xFFFFFFFFFFFFFFFF`).

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

## Shared Utilities

`internal/util` provides helpers used across multiple `cmd/` files:

- **`FormatSize(bytes int64) string`** -- Human-readable binary size string (KB = 1024, MB = 1024², GB = 1024³).
- **`ParseSize(s string) (int64, error)`** -- Parse size strings with decimal unit suffixes (`KB`, `MB`, `GB`, `TB`). Accepts floats (e.g., `1.5MB`).

These replace earlier per-file copies in `cmd/dupes.go` and `cmd/search.go`.

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
