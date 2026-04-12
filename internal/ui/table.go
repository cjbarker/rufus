package ui

import (
	"fmt"
	"strings"
)

// Table renders a visually appealing, colored table to stdout.
type Table struct {
	headers []string
	rows    [][]string
	colWidths []int
}

// NewTable creates a new table with the given headers.
func NewTable(headers ...string) *Table {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	return &Table{
		headers:   headers,
		colWidths: widths,
	}
}

// AddRow adds a row of values to the table.
func (t *Table) AddRow(values ...string) {
	// Pad or truncate to match header count
	row := make([]string, len(t.headers))
	for i := range row {
		if i < len(values) {
			row[i] = values[i]
		}
		if len(row[i]) > t.colWidths[i] {
			t.colWidths[i] = len(row[i])
		}
	}
	t.rows = append(t.rows, row)
}

// Render prints the styled table to stdout.
func (t *Table) Render() {
	if !IsTTY() {
		t.renderPlain()
		return
	}

	// Header
	headerParts := make([]string, len(t.headers))
	for i, h := range t.headers {
		headerParts[i] = TableHeaderStyle.Render(padRight(h, t.colWidths[i]))
	}
	fmt.Printf("  %s\n", strings.Join(headerParts, "  "))

	// Rows
	for i, row := range t.rows {
		parts := make([]string, len(row))
		for j, val := range row {
			padded := padRight(val, t.colWidths[j])
			if i%2 == 1 {
				parts[j] = TableAltStyle.Render(padded)
			} else {
				parts[j] = padded
			}
		}
		fmt.Printf("  %s\n", strings.Join(parts, "  "))
	}
}

// renderPlain prints a plain text table suitable for piping.
func (t *Table) renderPlain() {
	headerParts := make([]string, len(t.headers))
	for i, h := range t.headers {
		headerParts[i] = padRight(h, t.colWidths[i])
	}
	fmt.Println(strings.Join(headerParts, "  "))

	for _, row := range t.rows {
		parts := make([]string, len(row))
		for j, val := range row {
			parts[j] = padRight(val, t.colWidths[j])
		}
		fmt.Println(strings.Join(parts, "  "))
	}
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
