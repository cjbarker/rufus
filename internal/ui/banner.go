package ui

import (
	"fmt"
)

// Banner prints the rufus application banner with styled branding.
func Banner(version string) {
	if !IsTTY() {
		fmt.Printf("rufus %s\n", version)
		return
	}

	logo := `
    ┏━━━━━━━━━━━━━━━━━━━━━━━━━━┓
    ┃   ` + TitleStyle.Render("  RUFUS  ") + `              ┃
    ┃   ` + Dim.Render("Photo Manager") + `              ┃
    ┗━━━━━━━━━━━━━━━━━━━━━━━━━━┛`

	fmt.Println(SeparatorStyle.Render(logo))
	if version != "" {
		fmt.Printf("    %s %s\n", Dim.Render("version"), Highlight.Render(version))
	}
	fmt.Println()
}

// SectionHeader prints a styled section header.
func SectionHeader(title string) {
	if !IsTTY() {
		fmt.Printf("\n%s\n", title)
		return
	}
	fmt.Printf("\n  %s\n", TitleStyle.Render(fmt.Sprintf(" %s ", title)))
}

// StatusLine prints a labeled status line with colored value.
func StatusLine(label, value string) {
	if !IsTTY() {
		fmt.Printf("  %s: %s\n", label, value)
		return
	}
	fmt.Printf("  %s %s\n", Dim.Render(label+":"), Highlight.Render(value))
}

// SuccessMessage prints a success message with a checkmark.
func SuccessMessage(msg string) {
	if !IsTTY() {
		fmt.Printf("[OK] %s\n", msg)
		return
	}
	fmt.Printf("  %s %s\n", SuccessStyle.Render("✔"), msg)
}

// WarningMessage prints a warning message.
func WarningMessage(msg string) {
	if !IsTTY() {
		fmt.Printf("[WARN] %s\n", msg)
		return
	}
	fmt.Printf("  %s %s\n", WarningStyle.Render("⚠"), msg)
}

// ErrorMessage prints an error message with an X.
func ErrorMessage(msg string) {
	if !IsTTY() {
		fmt.Printf("[ERROR] %s\n", msg)
		return
	}
	fmt.Printf("  %s %s\n", ErrorStyle.Render("✖"), msg)
}

// InfoMessage prints an informational message.
func InfoMessage(msg string) {
	if !IsTTY() {
		fmt.Printf("[INFO] %s\n", msg)
		return
	}
	fmt.Printf("  %s %s\n", InfoStyle.Render("ℹ"), msg)
}

// Separator prints a visual separator line.
func Separator() {
	if !IsTTY() {
		fmt.Println("---")
		return
	}
	fmt.Printf("  %s\n", SeparatorStyle.Render("─────────────────────────────────────────"))
}

// GroupHeader prints a styled group header (for duplicate groups, etc.).
func GroupHeader(title string) {
	if !IsTTY() {
		fmt.Printf("\n%s\n", title)
		return
	}
	boxed := BoxStyle.Render(title)
	fmt.Printf("\n%s\n", boxed)
}
