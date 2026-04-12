package cmd

import (
	"fmt"
	"runtime"

	"github.com/cjbarker/rufus/internal/ui"
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		ui.Banner(Version)
		ui.StatusLine("Go", runtime.Version())
		ui.StatusLine("Platform", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))
		fmt.Println()
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
