package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/cjbarker/rufus/internal/db"
	"github.com/cjbarker/rufus/internal/search"
	"github.com/cjbarker/rufus/internal/ui"
	"github.com/cjbarker/rufus/internal/util"
	"github.com/spf13/cobra"
)

var (
	searchTags       []string
	searchTagMode    string
	searchFace       string
	searchMinSizeStr string
	searchMaxSizeStr string
	searchFormat     string
	searchPath       string
	searchBefore     string
	searchAfter      string
	searchLimit      int
	searchOffset     int
	searchSortBy     string
	searchSortDesc   bool
	searchHasFaces   bool
	searchNoFaces    bool
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
	searchCmd.Flags().StringArrayVar(&searchTags, "tag", nil, "filter by tag (repeatable; use --tag-mode to control AND/OR logic)")
	searchCmd.Flags().StringVar(&searchTagMode, "tag-mode", "and", "tag match mode: and (image has ALL tags) or or (image has ANY tag)")
	searchCmd.Flags().StringVar(&searchFace, "face", "", "filter by person's face")
	searchCmd.Flags().BoolVar(&searchHasFaces, "has-faces", false, "only images with at least one labeled face")
	searchCmd.Flags().BoolVar(&searchNoFaces, "no-faces", false, "only images with no detected faces")
	searchCmd.Flags().StringVar(&searchMinSizeStr, "min-size", "", "minimum file size: bytes integer or value with unit (e.g. 500B, 4.3MB, 1.5GB, 2TB)")
	searchCmd.Flags().StringVar(&searchMaxSizeStr, "max-size", "", "maximum file size: bytes integer or value with unit (e.g. 500B, 4.3MB, 1.5GB, 2TB)")
	searchCmd.Flags().StringVar(&searchFormat, "format", "", "filter by image format (jpeg, png, etc.)")
	searchCmd.Flags().StringVar(&searchPath, "path", "", "filter by file path pattern")
	searchCmd.Flags().StringVar(&searchBefore, "before", "", "images modified before date (YYYY-MM-DD)")
	searchCmd.Flags().StringVar(&searchAfter, "after", "", "images modified after date (YYYY-MM-DD)")
	searchCmd.Flags().StringVar(&searchSortBy, "sort", "path", "sort field: path, size, date, format")
	searchCmd.Flags().BoolVar(&searchSortDesc, "desc", false, "sort in descending order")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 50, "maximum results")
	searchCmd.Flags().IntVar(&searchOffset, "offset", 0, "skip first N results (for pagination)")
	searchCmd.Flags().StringVar(&searchOutput, "output", "table", "output format: table, json")
	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = store.Close() }()

	minSize, err := util.ParseSize(searchMinSizeStr)
	if err != nil {
		return fmt.Errorf("invalid --min-size: %w", err)
	}
	maxSize, err := util.ParseSize(searchMaxSizeStr)
	if err != nil {
		return fmt.Errorf("invalid --max-size: %w", err)
	}

	q := &search.Query{
		Tags:        searchTags,
		TagMode:     search.TagMode(searchTagMode),
		Face:        searchFace,
		HasFaces:    searchHasFaces,
		NoFaces:     searchNoFaces,
		MinSize:     minSize,
		MaxSize:     maxSize,
		Format:      searchFormat,
		PathPattern: searchPath,
		SortBy:      search.SortField(searchSortBy),
		SortDesc:    searchSortDesc,
		Limit:       searchLimit,
		Offset:      searchOffset,
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
			ui.SizeStyle.Render(util.FormatSize(img.FileSize)),
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

