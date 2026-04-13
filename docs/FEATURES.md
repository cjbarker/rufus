# Rufus Features

Complete reference for all Rufus CLI commands and capabilities.

## Global Flags

Available on every command.

| Flag | Default | Description |
|------|---------|-------------|
| `--db` | `~/.rufus/rufus.db` | Path to SQLite database |
| `--workers` | CPU count | Number of concurrent workers |
| `-v, --verbose` | `false` | Verbose output |
| `-q, --quiet` | `false` | Suppress all non-error output (useful for scripting) |
| `--no-color` | `false` | Disable ANSI color output |
| `--api-url` | `https://api.openai.com/v1/chat/completions` | LLM API endpoint URL |
| `--api-key` | | LLM API key |
| `--model` | `gpt-4o` | LLM model name |

## Configuration

Rufus supports layered configuration. Values are applied in this priority order (highest wins):

1. **CLI flags** -- explicit flags on the command line
2. **Environment variables** -- `RUFUS_DB`, `RUFUS_WORKERS`, `RUFUS_VERBOSE`, `RUFUS_QUIET`, `RUFUS_NO_COLOR`, `RUFUS_LLM_API_URL`, `RUFUS_LLM_API_KEY`, `RUFUS_LLM_MODEL`
3. **Config file** -- `~/.rufus/config.json` (JSON, created automatically on first run)
4. **Built-in defaults**

Example config file:

```json
{
  "db": "/data/photos/rufus.db",
  "workers": 8,
  "quiet": false,
  "no_color": false,
  "llm_api_url": "https://api.openai.com/v1/chat/completions",
  "llm_api_key": "",
  "llm_model": "gpt-4o"
}
```

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
| `--exclude` | | Exclude a directory by name, path, or glob (repeatable) |

**Examples:**

```bash
# Scan a single directory
rufus scan ~/Photos

# Scan multiple directories
rufus scan ~/Photos ~/Backup/images

# Incremental scan (skip already-indexed files)
rufus scan --update ~/Photos

# Exclude directories by name
rufus scan --exclude Trash --exclude .thumbnails ~/Photos

# Exclude using a glob pattern
rufus scan --exclude "20??-*-tmp" ~/Photos

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
| `-y, --yes` | `false` | Auto-confirm deletion without interactive prompt |

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

# Delete duplicates without confirmation prompt
rufus dupes --yes
```

**Hash algorithms:**

- **aHash (Average Hash)** -- Fast, tolerant of minor changes. Compares each pixel to the mean.
- **dHash (Difference Hash)** -- Default. Good balance of speed and accuracy. Compares adjacent pixels.
- **pHash (Perceptual Hash)** -- Most accurate. Uses DCT to capture frequency information. Slower but best for near-identical images.

### `search` -- Query the Index

Search indexed images with combinable filters for tags, face, file size, format, date, path, and face presence.

```bash
rufus search
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--tags` | | Filter by tag (repeatable) |
| `--tag-mode` | `and` | How multiple tags are combined: `and` (all must match) or `or` (any must match) |
| `--face` | | Filter by person's name |
| `--has-faces` | `false` | Only show images that have at least one labeled face |
| `--no-faces` | `false` | Only show images with no detected faces |
| `--min-size` | | Minimum file size (e.g. `500KB`, `2MB`) |
| `--max-size` | | Maximum file size (e.g. `10MB`, `1GB`) |
| `--format` | | Filter by image format (`jpeg`, `png`, etc.) |
| `--path` | | Filter by file path substring |
| `--before` | | Images modified before this date (`YYYY-MM-DD`) |
| `--after` | | Images modified after this date (`YYYY-MM-DD`) |
| `--sort-by` | `path` | Sort field: `path`, `size`, `date`, `format` |
| `--sort-desc` | `false` | Sort in descending order |
| `--limit` | `50` | Maximum results to return |
| `--offset` | `0` | Number of results to skip (for pagination) |
| `--output` | `table` | Output format: `table`, `json`, `csv` |

**Examples:**

```bash
# Find large JPEG files
rufus search --format jpeg --min-size 5MB

# Find images from a specific date range, sorted newest first
rufus search --after 2024-01-01 --before 2024-12-31 --sort-by date --sort-desc

# Find images tagged with both "landscape" AND "nature"
rufus search --tags landscape --tags nature --tag-mode and

# Find images tagged with "sunset" OR "sunrise"
rufus search --tags sunset --tags sunrise --tag-mode or

# Only images that have labeled faces
rufus search --has-faces

# Only images with no detected faces
rufus search --no-faces

# Search by path pattern with JSON output
rufus search --path "vacation" --output json

# Paginate results
rufus search --limit 20 --offset 40

# Combine multiple filters
rufus search --format png --min-size 1MB --after 2024-06-01 --limit 100
```

### `stats` -- Library Statistics

Display a quick summary of the database contents.

```bash
rufus stats
```

**Output includes:**

- Total indexed images
- Total detected faces
- Total known people (labeled)
- Total tags applied
- Database file size on disk

**Example:**

```bash
rufus stats
```

```
Images    1 234
Faces       456
People       12
Tags        789
DB Size    4.2 MB
```

### `export` -- Export Library Index

Export all indexed image records (with tags) to a file for backup or migration.

```bash
rufus export --format json --file <path>
rufus export --format csv  --file <path>
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `json` | Output format: `json` or `csv` |
| `--file` | | Output file path (required) |

**Examples:**

```bash
# Export to JSON
rufus export --format json --file library.json

# Export to CSV
rufus export --format csv --file library.csv
```

**CSV columns:** `file_path`, `file_size`, `file_hash`, `width`, `height`, `format`, `mod_time`, `tags` (semicolon-separated).

### `import` -- Import Library Index

Import image records from a previously exported JSON or CSV file into the database. The format is auto-detected from the file extension (`.csv` → CSV, everything else → JSON). Records are upserted so running import multiple times is safe.

```bash
rufus import <file>
```

**Examples:**

```bash
# Import from JSON
rufus import library.json

# Import from CSV
rufus import library.csv
```

### `info` -- Inspect an Indexed Image

Display all stored metadata for a single indexed image. The image must be indexed first with `rufus scan`.

```bash
rufus info <image-path>
```

**Output includes:**

- File size, resolution, and format
- SHA-256 file hash (truncated) and perceptual hashes (aHash, dHash, pHash)
- Last modified time, scan timestamp, and face-scan timestamp
- All tags applied to the image
- All detected faces with ID, person name (or "unlabeled"), and bounding-box coordinates

**Example:**

```bash
rufus info ~/Photos/party.jpg
```

### `clean` -- Remove Stale Records

Removes database records for images that no longer exist on disk. Before deleting a stale record, Rufus checks whether another indexed record has the same SHA-256 hash and its file is present on disk. If so, the file is treated as **moved** rather than deleted: its tags and detected faces are migrated to the live record atomically and the stale record is removed without data loss.

```bash
rufus clean
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | `false` | Preview moves and removals without making changes |
| `--vacuum` | `false` | Run `VACUUM` after deletion to reclaim disk space |

**Output sections:**

- **Moved** -- files whose content was found at a new path; metadata is preserved
- **Stale** -- files with no matching live copy; records are deleted

**Examples:**

```bash
# Preview stale records and detected moves without making changes
rufus clean --dry-run

# Remove stale records (migrates metadata for moved files)
rufus clean

# Remove and reclaim disk space
rufus clean --vacuum
```

### `faces` -- Face Recognition

Detect and manage faces in your photo library. Supports detection, labeling, identity merging, searching by person, and listing known people.

**Shared flag (all `faces` subcommands):**

| Flag | Default | Description |
|------|---------|-------------|
| `--tolerance` | `0.6` | Maximum Euclidean distance between face descriptors to count as a match. Lower values are stricter (fewer auto-assignments); higher values match across more variation in lighting and angle but risk false positives. Recommended range: `0.5`–`0.65`. |

#### `faces detect`

Run face detection on all indexed images not yet processed. On first run, dlib models are downloaded automatically to `~/.rufus/models/`. Already-scanned images are skipped unless `--force` is passed.

After detection, a rematch pass runs automatically: all unlabeled faces in the database are compared against any known labels, so running `detect` again after labeling a face propagates that label to similar unscanned faces without needing `--force`.

```bash
rufus faces detect            # scan new images only; rematch unlabeled faces against known labels
rufus faces detect --force    # re-scan all images, clearing existing face records first
```

| Flag | Default | Description |
|------|---------|-------------|
| `--force` / `-f` | `false` | Re-scan all images, ignoring the face-scan cache |

#### `faces unlabeled`

List all detected faces that have not yet been assigned a person name. Use the face IDs shown here with `faces label`.

```bash
rufus faces unlabeled
```

#### `faces label`

Assign a name to a detected face. Creates the person if they do not already exist. Re-run `faces detect` afterward to propagate the new label to similar unlabeled faces.

```bash
rufus faces label <face-id> <name>
```

#### `faces unlabel`

Remove the person assignment from a face, reverting it to unlabeled. Useful when a face was assigned to the wrong person.

```bash
rufus faces unlabel <face-id>
```

#### `faces merge`

Merge two people records into one. All faces associated with `<merge-id>` are reassigned to `<keep-id>`, then the `<merge-id>` person record is deleted. Use this to consolidate duplicate identities (e.g., "Alice" and "Alice Smith" detected as separate people).

```bash
rufus faces merge <keep-id> <merge-id>
```

**Example:**

```bash
# List people to find their IDs
rufus faces list

# Merge person 7 into person 3 (keeps person 3)
rufus faces merge 3 7
```

#### `faces show`

Crop the bounding box for a detected face out of its source image and open it in your default image viewer. Useful for identifying which face in a photo corresponds to a given face ID before labeling.

```bash
rufus faces show <face-id>
```

| Flag | Default | Description |
|------|---------|-------------|
| `-o, --output` | temp file | Path to save the cropped PNG |
| `--padding` | `40` | Pixels to add around the face bounds for context |
| `--no-open` | `false` | Save the file but skip opening the viewer |

**Examples:**

```bash
# Open face 12 in the default image viewer
rufus faces show 12

# Add extra padding around the face and save to a specific file
rufus faces show 12 --padding 80 --output ~/Desktop/face12.png

# Save only (useful in scripts or headless environments)
rufus faces show 12 --no-open --output /tmp/face12.png
```

**Output:**

```
  Face ID   12
  Person    Alice
  Bounds    (120,80)–(220,180)
  Source    /photos/party.jpg
  Saved     /tmp/rufus-face-12.png
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

### `alttext` -- Generate Image Alt-Text

Send an image to an OpenAI-compatible LLM vision API to generate descriptive alt-text keywords following W3C guidelines. The image is automatically resized to fit within 512x512 pixels before sending to optimize speed and memory.

```bash
rufus alttext <image-path>
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--tag` | `false` | Save generated keywords as tags on the indexed image |

The LLM endpoint, API key, and model are configured via the global `--api-url`, `--api-key`, and `--model` flags (or the corresponding environment variables / config file entries).

**Examples:**

```bash
# Generate alt-text keywords for an image
rufus alttext --api-key $OPENAI_API_KEY ~/Photos/sunset.jpg

# Generate keywords and auto-save as tags
rufus alttext --api-key $OPENAI_API_KEY --tag ~/Photos/sunset.jpg

# Use a custom LLM endpoint (Ollama, Azure OpenAI, LiteLLM, etc.)
rufus alttext --api-url http://localhost:11434/v1/chat/completions --api-key local --model llava ~/Photos/sunset.jpg

# Use environment variables instead of flags
export RUFUS_LLM_API_KEY="sk-..."
export RUFUS_LLM_MODEL="gpt-4o"
rufus alttext ~/Photos/sunset.jpg
```

**Output:**

```
Alt Text for ~/Photos/sunset.jpg

  • sunset
  • ocean
  • golden hour
  • warm
  • peaceful
  • horizon
  • waves
  • beach
  • serene
  • dusk
```

With `--tag`, the keywords are also saved as database tags. The image must be indexed first with `rufus scan` when using `--tag`.

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

Most commands support multiple output formats via `--output` or `--format`:

- **table** -- Human-readable table with colored output (default)
- **json** -- Machine-readable JSON for scripting and pipelines
- **csv** -- Comma-separated values (available for `dupes`, `search`, `export`)
