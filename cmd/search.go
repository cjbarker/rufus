package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cjbarker/rufus/internal/db"
	"github.com/cjbarker/rufus/internal/search"
	"github.com/cjbarker/rufus/internal/ui"
	"github.com/spf13/cobra"
)

var (
	searchTag        string
	searchFace       string
	searchMinSizeStr string
	searchMaxSizeStr string
	searchFormat     string
	searchPath       string
	searchBefore     string
	searchAfter      string
	searchLimit      int
	searchOutput     string
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search the image index",
	Long: `Search indexed images by tag, face, size, format, date, or path.
Multiple filters can be combined.`,
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().StringVar(&searchTag, "tag", "", "filter by tag")
	searchCmd.Flags().StringVar(&searchFace, "face", "", "filter by person's face")
	searchCmd.Flags().StringVar(&searchMinSizeStr, "min-size", "", "minimum file size: bytes integer or value with unit (e.g. 500B, 4.3MB, 1.5GB, 2TB)")
	searchCmd.Flags().StringVar(&searchMaxSizeStr, "max-size", "", "maximum file size: bytes integer or value with unit (e.g. 500B, 4.3MB, 1.5GB, 2TB)")
	searchCmd.Flags().StringVar(&searchFormat, "format", "", "filter by image format (jpeg, png, etc.)")
	searchCmd.Flags().StringVar(&searchPath, "path", "", "filter by file path pattern")
	searchCmd.Flags().StringVar(&searchBefore, "before", "", "images modified before date (YYYY-MM-DD)")
	searchCmd.Flags().StringVar(&searchAfter, "after", "", "images modified after date (YYYY-MM-DD)")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 50, "maximum results")
	searchCmd.Flags().StringVar(&searchOutput, "output", "table", "output format: table, json")
	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = store.Close() }()

	minSize, err := parseSize(searchMinSizeStr)
	if err != nil {
		return fmt.Errorf("invalid --min-size: %w", err)
	}
	maxSize, err := parseSize(searchMaxSizeStr)
	if err != nil {
		return fmt.Errorf("invalid --max-size: %w", err)
	}

	q := &search.Query{
		Tag:         searchTag,
		Face:        searchFace,
		MinSize:     minSize,
		MaxSize:     maxSize,
		Format:      searchFormat,
		PathPattern: searchPath,
		Limit:       searchLimit,
	}

	if searchBefore != "" {
		t, parseErr := time.Parse("2006-01-02", searchBefore)
		if parseErr != nil {
			return fmt.Errorf("invalid --before date: %w", parseErr)
		}
		q.Before = &t
	}
	if searchAfter != "" {
		t, parseErr := time.Parse("2006-01-02", searchAfter)
		if parseErr != nil {
			return fmt.Errorf("invalid --after date: %w", parseErr)
		}
		q.After = &t
	}

	spinner := ui.NewSpinner("Searching...")
	spinner.Start()

	engine := search.NewEngine(store)
	results, err := engine.Search(q)
	if err != nil {
		spinner.StopWithError("Search failed")
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		spinner.StopWithSuccess("Search complete")
		fmt.Println()
		ui.InfoMessage("No results found.")
		return nil
	}

	spinner.StopWithSuccess(fmt.Sprintf("Found %s results",
		ui.Highlight.Render(fmt.Sprintf("%d", len(results)))))

	switch searchOutput {
	case "json":
		return outputSearchJSON(results)
	default:
		return outputSearchTable(results)
	}
}

func outputSearchTable(results []search.Result) error {
	fmt.Println()
	ui.SectionHeader("Search Results")
	fmt.Println()

	tbl := ui.NewTable("PATH", "SIZE", "RESOLUTION", "FORMAT")
	for _, r := range results {
		img := r.Image
		tbl.AddRow(
			ui.FileLink(img.FilePath),
			ui.SizeStyle.Render(formatSize(img.FileSize)),
			fmt.Sprintf("%dx%d", img.Width, img.Height),
			ui.FormatStyle.Render(img.Format),
		)
	}
	tbl.Render()
	fmt.Println()
	return nil
}

type searchJSONResult struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Format string `json:"format"`
}

func outputSearchJSON(results []search.Result) error {
	output := make([]searchJSONResult, len(results))
	for i, r := range results {
		output[i] = searchJSONResult{
			Path:   r.Image.FilePath,
			Size:   r.Image.FileSize,
			Width:  r.Image.Width,
			Height: r.Image.Height,
			Format: r.Image.Format,
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

// parseSize converts a size string to bytes. Accepts a plain integer (bytes)
// or a value with a unit suffix: B, MB, GB, TB using decimal multipliers
// (e.g. "4.3MB" → 4300000, "1.5GB" → 1500000000). Empty string returns 0.
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	// Plain integer — treat as bytes directly.
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n, nil
	}

	// Unit suffixes, longest first so "GB" is not matched by "B".
	units := []struct {
		suffix string
		factor int64
	}{
		{"TB", 1_000_000_000_000},
		{"GB", 1_000_000_000},
		{"MB", 1_000_000},
		{"B", 1},
	}

	upper := strings.ToUpper(s)
	for _, u := range units {
		if strings.HasSuffix(upper, u.suffix) {
			numStr := strings.TrimSpace(strings.TrimSuffix(upper, u.suffix))
			f, err := strconv.ParseFloat(numStr, 64)
			if err != nil || f < 0 {
				return 0, fmt.Errorf("invalid size %q", s)
			}
			return int64(f * float64(u.factor)), nil
		}
	}

	return 0, fmt.Errorf("invalid size %q: use a number in bytes or a value like 4.3MB, 1.5GB, 2TB", s)
}
