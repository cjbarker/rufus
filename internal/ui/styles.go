package ui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
)

// Theme colors for consistent styling throughout the app.
var (
	// Brand colors
	Primary   = lipgloss.Color("#7C3AED") // vibrant purple
	Secondary = lipgloss.Color("#06B6D4") // cyan
	Accent    = lipgloss.Color("#F59E0B") // amber

	// Status colors
	Success = lipgloss.Color("#10B981") // emerald green
	Warning = lipgloss.Color("#F59E0B") // amber
	Error   = lipgloss.Color("#EF4444") // red
	Info    = lipgloss.Color("#3B82F6") // blue
	Muted   = lipgloss.Color("#6B7280") // gray

	// Text styles
	Bold      = lipgloss.NewStyle().Bold(true)
	Dim       = lipgloss.NewStyle().Foreground(Muted)
	Highlight = lipgloss.NewStyle().Foreground(Secondary).Bold(true)

	// Header / title bar style
	TitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(Primary).
			Bold(true).
			Padding(0, 1)

	// Status message styles
	SuccessStyle = lipgloss.NewStyle().Foreground(Success).Bold(true)
	WarningStyle = lipgloss.NewStyle().Foreground(Warning).Bold(true)
	ErrorStyle   = lipgloss.NewStyle().Foreground(Error).Bold(true)
	InfoStyle    = lipgloss.NewStyle().Foreground(Info)

	// Path and file styles
	PathStyle   = lipgloss.NewStyle().Foreground(Secondary)
	FormatStyle = lipgloss.NewStyle().Foreground(Accent)
	SizeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#A78BFA"))

	// Table styles
	TableHeaderStyle = lipgloss.NewStyle().
				Foreground(Primary).
				Bold(true).
				Underline(true)
	TableRowStyle   = lipgloss.NewStyle()
	TableAltStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#D1D5DB"))
	TableBorderChar = lipgloss.NewStyle().Foreground(Muted).Render("│")

	// Badge styles
	KeepBadge   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(Success).Bold(true).Padding(0, 1).Render(" KEEP ")
	RemoveBadge = lipgloss.NewStyle().Foreground(Muted).Render("  --  ")

	// Box styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Primary).
			Padding(0, 1)

	// Summary box
	SummaryStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Success).
			Padding(0, 2).
			MarginTop(1)

	// Separator
	SeparatorStyle = lipgloss.NewStyle().Foreground(Muted)
)

// IsTTY reports whether stdout is connected to a terminal.
func IsTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// FileLink wraps path in an OSC 8 terminal hyperlink (file:// URL) so the
// user can Cmd/Ctrl+click to open the file directly from the terminal.
// Falls back to styled plain text when stdout is not a TTY.
func FileLink(path string) string {
	styled := PathStyle.Render(path)
	if !IsTTY() {
		return styled
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return fmt.Sprintf("\033]8;;file://%s\033\\%s\033]8;;\033\\", abs, styled)
}
