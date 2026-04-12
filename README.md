# Rufus

A high-performance CLI photo manager for deduplication and image recognition, written in Go.

Rufus crawls directories to index images, detects duplicates using perceptual hashing, recognizes faces, and provides advanced search across your photo library. It requires no external C libraries -- SQLite is pure Go.

## Features

- **Image Scanning** -- Concurrent directory crawling with worker pools to index images and compute hashes
- **Duplicate Detection** -- Finds duplicates using perceptual hashing (aHash, dHash, pHash) and SHA-256, with configurable similarity thresholds
- **Face Recognition** -- Detect faces in photos, label them, and search your library by person
- **Advanced Search** -- Filter by tag, face, file size, format, date range, and path pattern
- **Multiple Output Formats** -- Table, JSON, and CSV output for easy integration with other tools

For a detailed breakdown of all features and CLI usage, see [Features](docs/FEATURES.md). For technical internals, see [Architecture](docs/ARCHITECTURE.md).

## Requirements

- Go 1.23+
- golangci-lint (for linting)

## Build

```bash
make build
```

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
