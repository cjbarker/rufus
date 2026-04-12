package ui

import "os"

var quietMode bool

// SetQuiet suppresses all non-error output when q is true.
// ErrorMessage always prints regardless of this setting.
func SetQuiet(q bool) { quietMode = q }

// IsQuiet reports whether quiet mode is active.
func IsQuiet() bool { return quietMode }

// SetNoColor disables ANSI color output by setting the NO_COLOR environment
// variable, which is respected by lipgloss and termenv at render time.
func SetNoColor(nc bool) {
	if nc {
		os.Setenv("NO_COLOR", "1")
	}
}
