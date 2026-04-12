# CLAUDE.md

## Project Overview

Rufus is a high-performance CLI photo manager for deduplication and image recognition, written in Go. It crawls directories to index images, detects duplicates using perceptual hashing (aHash, dHash, pHash) and SHA-256, recognizes faces, and provides advanced search across photo libraries.

## Build & Run

```bash
make build          # Compile binary with version injection → ./rufus
make test           # Run all tests: go test -v -race ./...
make lint           # Run golangci-lint: golangci-lint run ./...
make ci             # Full pipeline: lint → test → build
make clean          # Remove binary and caches
```

The project requires **Go 1.23+** (go.mod declares 1.25.0) and **golangci-lint** for linting. No CGO or external C libraries are needed — SQLite is pure Go (`modernc.org/sqlite`).

## Project Structure

```
cmd/            CLI commands (cobra): root, scan, dupes, search, faces, version
internal/
  config/       Application configuration with sensible defaults
  crawler/      Channel-based async directory traversal for image discovery
  hasher/       SHA-256 + perceptual hashing (AHash, DHash, PHash)
  db/           SQLite database (Store pattern, WAL mode, schema auto-init)
  duplicates/   Duplicate detection via Union-Find on Hamming distance
  faces/        Face detection/matching interface (dlib stub)
  search/       Query builder with filtering (tag, face, size, format, date, path)
testdata/       Test fixture images
```

## Key Dependencies

- `github.com/spf13/cobra` — CLI framework
- `github.com/corona10/goimagehash` — Perceptual hashing
- `modernc.org/sqlite` — Pure Go SQLite driver
- `golang.org/x/image` — Extended image format support (BMP, TIFF, WebP)

## Architecture & Patterns

- **Cobra CLI**: Root command with persistent flags (`--db`, `--workers`, `--verbose`); subcommands in `cmd/`
- **Store pattern**: `db.Store` wraps `*sql.DB` with typed methods; `OpenMemory()` for test isolation
- **Worker pools**: Fan-out via goroutines + buffered channels; `sync.WaitGroup` for coordination; `atomic.Int64` for counters
- **Channel pipelines**: `crawler.Crawl()` returns `<-chan Result` consumed by worker pools
- **Union-Find**: O(n^2) pairwise comparison for perceptual duplicate grouping
- **Output formats**: Table (tabwriter), JSON, CSV — selected via `--format` / `--output` flag

## Code Conventions

- Standard Go formatting (`gofmt`)
- Explicit error wrapping: `fmt.Errorf("context: %w", err)`
- Table-driven tests with `t.TempDir()` and in-memory databases
- Import order: stdlib, third-party, local (`github.com/cjbarker/rufus/...`)
- Blank imports for image format registration: `_ "image/jpeg"`
- PascalCase exports, camelCase unexported; lowercase package names

## Linting

Enabled linters (`.golangci.yml` v2): errcheck, govet (all except fieldalignment), ineffassign, staticcheck, unused, gosimple, gocritic (diagnostic + style tags), misspell. The `testdata/` directory is excluded.

## Database

SQLite with WAL mode and foreign keys enabled. Tables: `images` (with perceptual hash indexes), `faces`, `people`, `tags`. Schema auto-initializes on `db.Open()`. Hashes stored as INT64 (cast from uint64).

## Supported Image Formats

`.jpg`, `.jpeg`, `.png`, `.gif`, `.bmp`, `.tiff`, `.tif`, `.webp`

## CI

GitHub Actions: lint → test (matrix Go 1.23/1.24 with race detector) → build → release (goreleaser on `v*` tags).
