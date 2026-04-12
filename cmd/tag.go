package cmd

import (
	"fmt"
	"strings"

	"github.com/cjbarker/rufus/internal/db"
	"github.com/cjbarker/rufus/internal/ui"
	"github.com/spf13/cobra"
)

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Add, remove, or list tags on indexed images",
	Long: `Manage tags on images already in the rufus index.

Tags are used by "rufus search --tags" to filter results.
The image must be indexed first with "rufus scan".`,
}

var tagAddCmd = &cobra.Command{
	Use:   "add <image-path> <tag> [tags...]",
	Short: "Add one or more tags to an indexed image",
	Args:  cobra.MinimumNArgs(2),
	RunE:  runTagAdd,
}

var tagRemoveCmd = &cobra.Command{
	Use:   "remove <image-path> <tag> [tags...]",
	Short: "Remove one or more tags from an indexed image",
	Args:  cobra.MinimumNArgs(2),
	RunE:  runTagRemove,
}

var tagListCmd = &cobra.Command{
	Use:   "list <image-path>",
	Short: "List all tags on an indexed image",
	Args:  cobra.ExactArgs(1),
	RunE:  runTagList,
}

func init() {
	tagCmd.AddCommand(tagAddCmd)
	tagCmd.AddCommand(tagRemoveCmd)
	tagCmd.AddCommand(tagListCmd)
	rootCmd.AddCommand(tagCmd)
}

func openStoreAndLookup(imagePath string) (*db.Store, *db.ImageRecord, error) {
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return nil, nil, fmt.Errorf("opening database: %w", err)
	}
	img, err := store.GetImageByPath(imagePath)
	if err != nil {
		_ = store.Close()
		return nil, nil, fmt.Errorf("querying image: %w", err)
	}
	if img == nil {
		_ = store.Close()
		return nil, nil, fmt.Errorf("%q is not in the index — run 'rufus scan' first", imagePath)
	}
	return store, img, nil
}

func runTagAdd(cmd *cobra.Command, args []string) error {
	imagePath := args[0]
	tags := args[1:]

	store, img, err := openStoreAndLookup(imagePath)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	var added []string
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if err := store.InsertTag(img.ID, tag); err != nil {
			return fmt.Errorf("adding tag %q: %w", tag, err)
		}
		added = append(added, tag)
	}

	if len(added) == 0 {
		ui.WarningMessage("No tags provided.")
		return nil
	}
	ui.SuccessMessage(fmt.Sprintf("Added %d tag(s) to %s: %s",
		len(added),
		ui.FileLink(imagePath),
		ui.Highlight.Render(strings.Join(added, ", "))))
	return nil
}

func runTagRemove(cmd *cobra.Command, args []string) error {
	imagePath := args[0]
	tags := args[1:]

	store, img, err := openStoreAndLookup(imagePath)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	var removed []string
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if err := store.RemoveTag(img.ID, tag); err != nil {
			return fmt.Errorf("removing tag %q: %w", tag, err)
		}
		removed = append(removed, tag)
	}

	if len(removed) == 0 {
		ui.WarningMessage("No tags provided.")
		return nil
	}
	ui.SuccessMessage(fmt.Sprintf("Removed %d tag(s) from %s: %s",
		len(removed),
		ui.FileLink(imagePath),
		ui.Highlight.Render(strings.Join(removed, ", "))))
	return nil
}

func runTagList(cmd *cobra.Command, args []string) error {
	imagePath := args[0]

	store, img, err := openStoreAndLookup(imagePath)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	tags, err := store.GetTagsForImage(img.ID)
	if err != nil {
		return fmt.Errorf("listing tags: %w", err)
	}

	fmt.Println()
	ui.SectionHeader(fmt.Sprintf("Tags for %s", ui.FileLink(imagePath)))
	fmt.Println()

	if len(tags) == 0 {
		ui.InfoMessage("No tags — use 'rufus tag add' to add some.")
	} else {
		tbl := ui.NewTable("TAG")
		for _, t := range tags {
			tbl.AddRow(ui.Highlight.Render(t))
		}
		tbl.Render()
	}
	fmt.Println()
	return nil
}
