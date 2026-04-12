package ui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Confirm asks the user a yes/no question and returns the result.
// Falls back to defaultVal if not a TTY.
func Confirm(question string, defaultVal bool) bool {
	if !IsTTY() {
		return defaultVal
	}

	hint := "y/N"
	if defaultVal {
		hint = "Y/n"
	}

	fmt.Printf("  %s %s %s ",
		InfoStyle.Render("?"),
		Bold.Render(question),
		Dim.Render(fmt.Sprintf("[%s]", hint)),
	)

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	switch input {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		return defaultVal
	}
}

// Select presents a list of options and returns the selected index.
// Falls back to defaultIdx if not a TTY.
func Select(question string, options []string, defaultIdx int) int {
	if !IsTTY() || len(options) == 0 {
		return defaultIdx
	}

	fmt.Printf("  %s %s\n", InfoStyle.Render("?"), Bold.Render(question))
	for i, opt := range options {
		marker := Dim.Render("  ")
		if i == defaultIdx {
			marker = InfoStyle.Render("> ")
		}
		idx := Dim.Render(fmt.Sprintf("%d)", i+1))
		fmt.Printf("    %s %s %s\n", marker, idx, opt)
	}

	fmt.Printf("\n  %s ", Dim.Render("Enter choice:"))

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if idx, err := strconv.Atoi(input); err == nil && idx >= 1 && idx <= len(options) {
		return idx - 1
	}
	return defaultIdx
}
