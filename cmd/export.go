package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cjbarker/rufus/internal/db"
	"github.com/cjbarker/rufus/internal/ui"
	"github.com/spf13/cobra"
)

var (
	exportOutput string
	exportFile   string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the image index",
	Long: `Export all indexed image metadata (file path, size, hash, dimensions, format, mod time, and tags)
to a JSON or CSV file for backup or migration purposes.`,
	RunE: runExport,
}

func init() {
	exportCmd.Flags().StringVar(&exportOutput, "format", "json", "output format: json, csv")
	exportCmd.Flags().StringVarP(&exportFile, "file", "f", "", "output file path (default: stdout)")
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = store.Close() }()

	spinner := ui.NewSpinner("Exporting index...")
	spinner.Start()

	records, err := store.ExportAll()
	if err != nil {
		spinner.StopWithError("Export failed")
		return fmt.Errorf("exporting records: %w", err)
	}
	spinner.StopWithSuccess(fmt.Sprintf("Exporting %s records", ui.Highlight.Render(fmt.Sprintf("%d", len(records)))))

	var out *os.File
	if exportFile != "" {
		out, err = os.Create(exportFile)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer func() { _ = out.Close() }()
	} else {
		out = os.Stdout
	}

	switch strings.ToLower(exportOutput) {
	case "csv":
		return writeExportCSV(out, records)
	default:
		return writeExportJSON(out, records)
	}
}

func writeExportJSON(out *os.File, records []db.ExportRecord) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(records); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return nil
}

func writeExportCSV(out *os.File, records []db.ExportRecord) error {
	w := csv.NewWriter(out)
	if err := w.Write([]string{"file_path", "file_size", "file_hash", "width", "height", "format", "mod_time", "tags"}); err != nil {
		return fmt.Errorf("writing CSV header: %w", err)
	}
	for _, r := range records {
		row := []string{
			r.FilePath,
			fmt.Sprintf("%d", r.FileSize),
			r.FileHash,
			fmt.Sprintf("%d", r.Width),
			fmt.Sprintf("%d", r.Height),
			r.Format,
			r.ModTime,
			strings.Join(r.Tags, ";"),
		}
		if err := w.Write(row); err != nil {
			return fmt.Errorf("writing CSV row: %w", err)
		}
	}
	w.Flush()
	return w.Error()
}
