# Rufus Features

Complete reference for all Rufus CLI commands and capabilities.

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--db` | `~/.rufus/rufus.db` | Path to SQLite database |
| `--workers` | CPU count | Number of concurrent workers |
| `-v, --verbose` | `false` | Verbose output |

## Commands

### `scan` -- Index Images

Crawls directories, discovers image files, computes perceptual hashes (aHash, dHash, pHash) and SHA-256 file hashes, and stores results in the database.

```bash
rufus scan <path> [paths...]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `-r, --recursive` | `true` | Recurse into subdirectories |
| `--update` | `false` | Only process new or modified files (incremental mode) |

**Examples:**

```bash
# Scan a single directory
rufus scan ~/Photos

# Scan multiple directories
rufus scan ~/Photos ~/Backup/images

# Incremental scan (skip already-indexed files)
rufus scan --update ~/Photos

# Scan with 8 workers and verbose output
rufus scan --workers 8 -v ~/Photos
```

### `dupes` -- Find Duplicates

Analyzes indexed images to find duplicates using perceptual hashing. Images are grouped by visual similarity based on hash algorithm and Hamming distance threshold. Each group recommends which image to keep based on resolution and file size.

```bash
rufus dupes
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--threshold` | `10` | Hamming distance threshold for similarity |
| `--hash` | `dhash` | Hash algorithm: `ahash`, `dhash`, `phash` |
| `--format` | `table` | Output format: `table`, `json`, `csv` |

**Examples:**

```bash
# Find duplicates with default settings
rufus dupes

# Strict matching (lower threshold = more similar)
rufus dupes --threshold 5

# Use perceptual hash and output JSON
rufus dupes --hash phash --format json

# Export duplicates as CSV for processing
rufus dupes --format csv > duplicates.csv
```

**Hash algorithms:**

- **aHash (Average Hash)** -- Fast, tolerant of minor changes. Compares each pixel to the mean.
- **dHash (Difference Hash)** -- Default. Good balance of speed and accuracy. Compares adjacent pixels.
- **pHash (Perceptual Hash)** -- Most accurate. Uses DCT to capture frequency information. Slower but best for near-identical images.

### `search` -- Query the Index

Search indexed images with combinable filters for tag, face, file size, format, date, and path.

```bash
rufus search
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--tag` | | Filter by tag |
| `--face` | | Filter by person's face |
| `--min-size` | `0` | Minimum file size in bytes |
| `--max-size` | `0` | Maximum file size in bytes |
| `--format` | | Filter by image format (jpeg, png, etc.) |
| `--path` | | Filter by file path pattern |
| `--before` | | Images modified before date (YYYY-MM-DD) |
| `--after` | | Images modified after date (YYYY-MM-DD) |
| `--limit` | `50` | Maximum results |
| `--output` | `table` | Output format: `table`, `json` |

**Examples:**

```bash
# Find large JPEG files
rufus search --format jpeg --min-size 5000000

# Find images from a specific date range
rufus search --after 2024-01-01 --before 2024-12-31

# Search by path pattern with JSON output
rufus search --path "vacation" --output json

# Combine multiple filters
rufus search --format png --min-size 1000000 --after 2024-06-01 --limit 100
```

### `faces` -- Face Recognition

Detect and manage faces in your photo library. Supports detection, labeling, searching by person, and listing known people.

#### `faces detect`

Run face detection on all indexed images not yet processed. Requires dlib models to be installed.

```bash
rufus faces detect
```

#### `faces label`

Assign a name to a detected face.

```bash
rufus faces label <face-id> <name>
```

#### `faces find`

Find all images containing a named person.

```bash
rufus faces find <name>
```

#### `faces list`

List all known labeled people.

```bash
rufus faces list
```

**Shared flag:**

| Flag | Default | Description |
|------|---------|-------------|
| `--tolerance` | `0.6` | Face match tolerance (lower = stricter) |

### `version`

Print version, Go runtime, and platform information.

```bash
rufus version
```

## Supported Image Formats

| Format | Extensions |
|--------|------------|
| JPEG | `.jpg`, `.jpeg` |
| PNG | `.png` |
| GIF | `.gif` |
| BMP | `.bmp` |
| TIFF | `.tiff`, `.tif` |
| WebP | `.webp` |

## Output Formats

Most commands support multiple output formats:

- **table** -- Human-readable table with colored output (default)
- **json** -- Machine-readable JSON for scripting and pipelines
- **csv** -- Comma-separated values (available for `dupes`)
