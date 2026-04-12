package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/cjbarker/rufus/internal/db"
	"github.com/cjbarker/rufus/internal/search"
	"github.com/cjbarker/rufus/internal/ui"
	"github.com/spf13/cobra"
)

var (
	searchTag     string
	searchFace    string
	searchMinSize int64
	searchMaxSize int64
	searchFormat  string
	searchPath    string
	searchBefore  string
	searchAfter   string
	searchLimit   int
	searchOutput  string
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
	searchCmd.Flags().Int64Var(&searchMinSize, "min-size", 0, "minimum file size in bytes")
	searchCmd.Flags().Int64Var(&searchMaxSize, "max-size", 0, "maximum file size in bytes")
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

	q := &search.Query{
		Tag:         searchTag,
		Face:        searchFace,
		MinSize:     searchMinSize,
		MaxSize:     searchMaxSize,
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
