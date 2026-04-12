# Rufus

A high-performance CLI photo manager for deduplication and image recognition, written in Go.

Rufus crawls directories to index images, detects duplicates using perceptual hashing, recognizes faces, and provides advanced search across your photo library. SQLite is pure Go and requires no external C libraries. Face detection requires dlib (installed automatically by `make build-faces`).

## Features

- **Image Scanning** -- Concurrent directory crawling with worker pools to index images and compute hashes
- **Duplicate Detection** -- Finds duplicates using perceptual hashing (aHash, dHash, pHash) and SHA-256, with configurable similarity thresholds
- **Face Recognition** -- Detect faces in photos, label them, merge identities, and search your library by person
- **Library Stats** -- Quick summary of indexed images, faces, people, tags, and database size
- **Export / Import** -- Export the full index to JSON or CSV; import it back into any Rufus database
- **Advanced Search** -- Filter by tags (AND/OR), face, file size, format, date range, path pattern, and face presence

  **Pipeline:**
  1. **Build** -- Run `make build`. dlib is installed automatically if missing. To build without face detection, use `make build-no-faces`.
  2. **Scan** -- Run `rufus scan <dir>` to index images into the local database.
  3. **Detect** -- Run `rufus faces detect` to scan indexed images for faces. On first run, dlib is installed automatically if missing and model files are downloaded to `~/.rufus/models/`. Each detected face is stored with a bounding box and a 128-dimensional descriptor. Already-scanned images are skipped unless `--force` is passed.
  4. **Auto-match** -- Detected faces are automatically compared against previously labeled people. Faces within the similarity threshold (default `0.6`, tunable via `--tolerance`) are assigned to the matching person without any manual step.
  5. **Label** -- Use `rufus faces unlabeled` to list detected faces with no name assigned. Assign a name with `rufus faces label <face-id> <name>`. Re-running `rufus faces detect` after labeling will re-match all remaining unlabeled faces against the new labels — no `--force` needed.
  6. **Search** -- Use `rufus faces find <name>` to list every image containing that person.
- **Multiple Output Formats** -- Table, JSON, and CSV output for easy integration with other tools

For a detailed breakdown of all features and CLI usage, see [Features](docs/FEATURES.md). For technical internals, see [Architecture](docs/ARCHITECTURE.md).

## Requirements

- Go 1.23+
- golangci-lint (for linting)

## Build

### With face detection (default)

```bash
make build
```

Installs dlib automatically if missing, then compiles with face detection enabled (`-tags dlib`, `CGO_ENABLED=1`). This is the standard build — `make build` delegates to `make build-faces`.

**macOS** -- dlib is installed via Homebrew (`brew install dlib jpeg-turbo`).  
**Linux** -- dlib is installed via apt-get (`sudo apt-get install libdlib-dev libjpeg-dev`).

### Without face detection

```bash
make build-no-faces
```

Produces a lighter binary with no CGO dependency and no dlib requirement. Face-related commands will prompt the user to rebuild with `make build`.

This compiles the binary with version injection to `./rufus`.

## Test

```bash
make test
```

Runs all tests with the race detector enabled (`go test -v -race ./...`).

## Lint

```bash
make lint
```

## Quick Start

```bash
# Build
make build

# Scan a photo directory
./rufus scan ~/Photos

# Scan and exclude specific subdirectories
./rufus scan --exclude Trash --exclude .thumbnails ~/Photos

# Find duplicates (prompt before deleting)
./rufus dupes

# Find duplicates and auto-confirm deletion
./rufus dupes --yes

# Search your library
./rufus search --format jpeg --min-size 1000000

# Search with multiple tags (AND mode) and sort by date
./rufus search --tags landscape --tags nature --tag-mode and --sort-by date

# Show database statistics
./rufus stats

# Export library index to JSON
./rufus export --format json --file library.json

# Import a previously exported index
./rufus import library.json

# Suppress all non-error output (e.g. for scripting)
./rufus --quiet scan ~/Photos

# Check version
./rufus version
```

### Face Detection Workflow

```bash
# Detect faces in all indexed images (downloads dlib models on first run)
./rufus faces detect

# List faces that have not been assigned a name yet
./rufus faces unlabeled

# Assign a name to a face by ID
./rufus faces label 12 "Alice"

# Remove a label from a face (revert to unlabeled)
./rufus faces unlabel 12

# Merge two people records into one (moves all faces from merge-id to keep-id)
./rufus faces merge 3 7

# Re-run detect — propagates the new label to all similar unlabeled faces
./rufus faces detect

# Find every photo containing a person
./rufus faces find "Alice"

# List all known people
./rufus faces list

# Re-scan all images from scratch (clears existing face records)
./rufus faces detect --force
```

## Configuration

Rufus reads a configuration file at `~/.rufus/config.json` (created automatically on first run). Any value in the config file can be overridden by an environment variable or a CLI flag.

| Priority | Source |
|----------|--------|
| 1 (highest) | CLI flags |
| 2 | Environment variables (`RUFUS_DB`, `RUFUS_WORKERS`, `RUFUS_VERBOSE`, `RUFUS_QUIET`, `RUFUS_NO_COLOR`) |
| 3 | Config file (`~/.rufus/config.json`) |
| 4 (lowest) | Built-in defaults |

## Supported Image Formats

JPEG, PNG, GIF, BMP, TIFF, WebP

## License

See [LICENSE](LICENSE) for details.
