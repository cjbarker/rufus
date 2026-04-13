package cmd

import (
	"github.com/cjbarker/rufus/internal/config"
	"github.com/cjbarker/rufus/internal/ui"
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
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Load config file, then apply env vars. CLI flags win over both.
		fileCfg, err := config.LoadFile(config.DefaultConfigPath())
		if err != nil {
			return err
		}
		pf := cmd.Root().PersistentFlags()
		if fileCfg != nil {
			if !pf.Changed("db") && fileCfg.DBPath != "" {
				cfg.DBPath = fileCfg.DBPath
			}
			if !pf.Changed("workers") && fileCfg.Workers > 0 {
				cfg.Workers = fileCfg.Workers
			}
			if !pf.Changed("verbose") {
				cfg.Verbose = fileCfg.Verbose
			}
			if !pf.Changed("quiet") {
				cfg.Quiet = fileCfg.Quiet
			}
			if !pf.Changed("no-color") {
				cfg.NoColor = fileCfg.NoColor
			}
			if !pf.Changed("api-url") && fileCfg.LLMApiURL != "" {
				cfg.LLMApiURL = fileCfg.LLMApiURL
			}
			if !pf.Changed("api-key") && fileCfg.LLMApiKey != "" {
				cfg.LLMApiKey = fileCfg.LLMApiKey
			}
			if !pf.Changed("model") && fileCfg.LLMModel != "" {
				cfg.LLMModel = fileCfg.LLMModel
			}
		}
		config.ApplyEnv(cfg)
		// CLI flags override env vars for booleans (re-apply if explicitly set).
		if pf.Changed("verbose") {
			cfg.Verbose, _ = pf.GetBool("verbose")
		}
		if pf.Changed("quiet") {
			cfg.Quiet, _ = pf.GetBool("quiet")
		}
		if pf.Changed("no-color") {
			cfg.NoColor, _ = pf.GetBool("no-color")
		}
		if pf.Changed("api-url") {
			cfg.LLMApiURL, _ = pf.GetString("api-url")
		}
		if pf.Changed("api-key") {
			cfg.LLMApiKey, _ = pf.GetString("api-key")
		}
		if pf.Changed("model") {
			cfg.LLMModel, _ = pf.GetString("model")
		}
		ui.SetQuiet(cfg.Quiet)
		ui.SetNoColor(cfg.NoColor)
		return cfg.Validate()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfg.DBPath, "db", cfg.DBPath, "path to SQLite database")
	rootCmd.PersistentFlags().IntVar(&cfg.Workers, "workers", cfg.Workers, "number of concurrent workers")
	rootCmd.PersistentFlags().BoolVarP(&cfg.Verbose, "verbose", "v", cfg.Verbose, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&cfg.Quiet, "quiet", "q", cfg.Quiet, "suppress all non-error output")
	rootCmd.PersistentFlags().BoolVar(&cfg.NoColor, "no-color", cfg.NoColor, "disable color output")
	rootCmd.PersistentFlags().StringVar(&cfg.LLMApiURL, "api-url", cfg.LLMApiURL, "LLM API endpoint URL")
	rootCmd.PersistentFlags().StringVar(&cfg.LLMApiKey, "api-key", cfg.LLMApiKey, "LLM API key")
	rootCmd.PersistentFlags().StringVar(&cfg.LLMModel, "model", cfg.LLMModel, "LLM model name")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
