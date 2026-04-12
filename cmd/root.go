package cmd

import (
	"github.com/cjbarker/rufus/internal/config"
	"github.com/spf13/cobra"
)

var cfg = config.Default()

var rootCmd = &cobra.Command{
	Use:   "rufus",
	Short: "High-performance photo manager for deduplication and image recognition",
	Long: `Rufus crawls directories to index images, detect duplicates using
perceptual hashing, recognize faces, and provide advanced search
across your photo library.`,
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfg.DBPath, "db", cfg.DBPath, "path to SQLite database")
	rootCmd.PersistentFlags().IntVar(&cfg.Workers, "workers", cfg.Workers, "number of concurrent workers")
	rootCmd.PersistentFlags().BoolVarP(&cfg.Verbose, "verbose", "v", cfg.Verbose, "verbose output")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
