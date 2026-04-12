package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/cjbarker/rufus/internal/db"
	"github.com/cjbarker/rufus/internal/ui"
	"github.com/spf13/cobra"
)

var importFormat string

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import image index from a file",
	Long: `Import image metadata from a JSON or CSV file previously created with "rufus export".
Existing records are updated; new records are inserted. Tags are merged (never removed).`,
	Args: cobra.ExactArgs(1),
	RunE: runImport,
}

func init() {
	importCmd.Flags().StringVar(&importFormat, "format", "", "input format: json, csv (auto-detected from file extension if omitted)")
	rootCmd.AddCommand(importCmd)
}

func runImport(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening import file: %w", err)
	}
	defer func() { _ = f.Close() }()

	// Auto-detect format from extension if not specified.
	format := strings.ToLower(importFormat)
	if format == "" {
		lower := strings.ToLower(filePath)
		if strings.HasSuffix(lower, ".csv") {
			format = "csv"
		} else {
			format = "json"
		}
	}

	spinner := ui.NewSpinner(fmt.Sprintf("Parsing %s...", format))
	spinner.Start()

	var records []db.ExportRecord
	switch format {
	case "csv":
		records, err = readImportCSV(f)
	default:
		records, err = readImportJSON(f)
	}
	if err != nil {
		spinner.StopWithError("Parse failed")
		return fmt.Errorf("parsing import file: %w", err)
	}
	spinner.StopWithSuccess(fmt.Sprintf("Parsed %s records", ui.Highlight.Render(fmt.Sprintf("%d", len(records)))))

	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = store.Close() }()

	spinner = ui.NewSpinner("Importing records...")
	spinner.Start()
	if err := store.ImportRecords(records); err != nil {
		spinner.StopWithError("Import failed")
		return fmt.Errorf("importing records: %w", err)
	}
	spinner.StopWithSuccess(fmt.Sprintf("Imported %s records", ui.Highlight.Render(fmt.Sprintf("%d", len(records)))))
	fmt.Println()
	return nil
}

func readImportJSON(r io.Reader) ([]db.ExportRecord, error) {
	var records []db.ExportRecord
	if err := json.NewDecoder(r).Decode(&records); err != nil {
		return nil, fmt.Errorf("decoding JSON: %w", err)
	}
	return records, nil
}

func readImportCSV(r io.Reader) ([]db.ExportRecord, error) {
	cr := csv.NewReader(r)
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV header: %w", err)
	}
	// Build column index map for flexibility.
	colIdx := make(map[string]int, len(header))
	for i, h := range header {
		colIdx[strings.TrimSpace(strings.ToLower(h))] = i
	}
	get := func(row []string, col string) string {
		if i, ok := colIdx[col]; ok && i < len(row) {
			return row[i]
		}
		return ""
	}

	var records []db.ExportRecord
	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading CSV row: %w", err)
		}
		fileSize, _ := strconv.ParseInt(get(row, "file_size"), 10, 64)
		width, _ := strconv.Atoi(get(row, "width"))
		height, _ := strconv.Atoi(get(row, "height"))
		var tags []string
		if ts := get(row, "tags"); ts != "" {
			tags = strings.Split(ts, ";")
		}
		records = append(records, db.ExportRecord{
			FilePath: get(row, "file_path"),
			FileSize: fileSize,
			FileHash: get(row, "file_hash"),
			Width:    width,
			Height:   height,
			Format:   get(row, "format"),
			ModTime:  get(row, "mod_time"),
			Tags:     tags,
		})
	}
	return records, nil
}
