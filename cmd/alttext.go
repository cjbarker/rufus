package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cjbarker/rufus/internal/alttext"
	"github.com/cjbarker/rufus/internal/ui"
	"github.com/spf13/cobra"
)

var alttextTag bool

var alttextCmd = &cobra.Command{
	Use:   "alttext <image-path>",
	Short: "Generate alt-text keywords for an image using an LLM",
	Long: `Send an image to an LLM API to generate descriptive alt-text keywords.
The image must be a supported format (jpg, png, gif, bmp, tiff, webp).

Use --tag to also save the generated keywords as tags in the database
(requires the image to be indexed first with "rufus scan").

The LLM API endpoint, key, and model can be set via flags, environment
variables (RUFUS_LLM_API_URL, RUFUS_LLM_API_KEY, RUFUS_LLM_MODEL),
or the config file (~/.rufus/config.json).`,
	Args: cobra.ExactArgs(1),
	RunE: runAlttext,
}

func init() {
	alttextCmd.Flags().BoolVar(&alttextTag, "tag", false, "save keywords as tags on the indexed image")
	rootCmd.AddCommand(alttextCmd)
}

func runAlttext(cmd *cobra.Command, args []string) error {
	imagePath := args[0]

	// Validate image file exists.
	if _, err := os.Stat(imagePath); err != nil {
		return fmt.Errorf("image file: %w", err)
	}

	// Validate supported image extension.
	ext := strings.ToLower(filepath.Ext(imagePath))
	if _, ok := alttext.MimeForExt(ext); !ok {
		return fmt.Errorf("unsupported image format %q", ext)
	}

	// Validate required LLM config.
	if cfg.LLMApiKey == "" {
		return fmt.Errorf("LLM API key is required — set --api-key, RUFUS_LLM_API_KEY, or llm_api_key in config")
	}
	if cfg.LLMApiURL == "" {
		return fmt.Errorf("LLM API URL is required — set --api-url, RUFUS_LLM_API_URL, or llm_api_url in config")
	}

	// Call the LLM API.
	spinner := ui.NewSpinner("Generating alt text...")
	spinner.Start()

	result, err := alttext.Generate(context.Background(), alttext.Request{
		ImagePath: imagePath,
		ApiURL:    cfg.LLMApiURL,
		ApiKey:    cfg.LLMApiKey,
		Model:     cfg.LLMModel,
		Timeout:   30 * time.Second,
	})
	if err != nil {
		spinner.StopWithError("Failed to generate alt text")
		return fmt.Errorf("generating alt text: %w", err)
	}

	keywords := alttext.ParseKeywords(result)
	spinner.StopWithSuccess("Alt text generated")

	// Display keywords.
	fmt.Println()
	ui.SectionHeader(fmt.Sprintf("Alt Text for %s", ui.FileLink(imagePath)))
	fmt.Println()

	if len(keywords) == 0 {
		ui.WarningMessage("No keywords extracted from LLM response.")
		if cfg.Verbose {
			ui.InfoMessage(fmt.Sprintf("Raw response: %s", result))
		}
		fmt.Println()
		return nil
	}

	for _, kw := range keywords {
		fmt.Printf("  %s %s\n", ui.InfoStyle.Render("•"), ui.Highlight.Render(kw))
	}
	fmt.Println()

	// Optionally save as tags.
	if alttextTag {
		store, img, err := openStoreAndLookup(imagePath)
		if err != nil {
			return err
		}
		defer func() { _ = store.Close() }()

		var added []string
		for _, kw := range keywords {
			if err := store.InsertTag(img.ID, kw); err != nil {
				return fmt.Errorf("adding tag %q: %w", kw, err)
			}
			added = append(added, kw)
		}
		ui.SuccessMessage(fmt.Sprintf("Saved %d keyword(s) as tags on %s",
			len(added), ui.FileLink(imagePath)))
		fmt.Println()
	}

	return nil
}
