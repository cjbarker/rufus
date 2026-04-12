# Rufus

A high-performance CLI photo manager for deduplication and image recognition, written in Go.

Rufus crawls directories to index images, detects duplicates using perceptual hashing, recognizes faces, and provides advanced search across your photo library. SQLite is pure Go and requires no external C libraries. Face detection requires dlib (installed automatically by `make build-faces`).

## Features

- **Image Scanning** -- Concurrent directory crawling with worker pools to index images and compute hashes
- **Duplicate Detection** -- Finds duplicates using perceptual hashing (aHash, dHash, pHash) and SHA-256, with configurable similarity thresholds
- **Face Recognition** -- Detect faces in photos, label them, and search your library by person

  **Pipeline:**
  1. **Build** -- Run `make build`. dlib is installed automatically if missing. To build without face detection, use `make build-no-faces`.
  2. **Scan** -- Run `rufus scan <dir>` to index images into the local database.
  3. **Detect** -- Run `rufus faces detect` to scan indexed images for faces. On first run, dlib is installed automatically if missing and model files are downloaded to `~/.rufus/models/`. Each detected face is stored with a bounding box and a 128-dimensional descriptor.
  4. **Auto-match** -- Detected faces are automatically compared against previously labeled people. Faces within the similarity threshold are assigned to the matching person without any manual step.
  5. **Label** -- Unmatched faces are stored unlabeled. Assign a name with `rufus faces label <face-id> <name>`. Future detections of similar faces are auto-assigned to that person.
  6. **Search** -- Use `rufus faces find <name>` to list every image containing that person.
- **Advanced Search** -- Filter by tag, face, file size, format, date range, and path pattern
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

# Find duplicates
./rufus dupes

# Search your library
./rufus search --format jpeg --min-size 1000000

# Check version
./rufus version
```

## Supported Image Formats

JPEG, PNG, GIF, BMP, TIFF, WebP

## License

See [LICENSE](LICENSE) for details.
