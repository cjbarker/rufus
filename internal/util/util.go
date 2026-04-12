// Package util provides shared helper functions used across the cmd layer.
package util

import (
	"fmt"
	"strconv"
	"strings"
)

// FormatSize converts a byte count to a human-readable string using binary
// prefixes (KB = 1024, MB = 1024², GB = 1024³).
func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1fGB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

// ParseSize converts a size string to a byte count. Accepts a plain integer
// (bytes) or a value with a decimal unit suffix: B, MB, GB, TB
// (e.g. "4.3MB" → 4_300_000). Empty string returns 0.
func ParseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	// Plain integer — treat as bytes.
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n, nil
	}

	// Unit suffixes, longest first so "GB" is not matched by "B".
	units := []struct {
		suffix string
		factor int64
	}{
		{"TB", 1_000_000_000_000},
		{"GB", 1_000_000_000},
		{"MB", 1_000_000},
		{"B", 1},
	}

	upper := strings.ToUpper(s)
	for _, u := range units {
		if strings.HasSuffix(upper, u.suffix) {
			numStr := strings.TrimSpace(strings.TrimSuffix(upper, u.suffix))
			f, err := strconv.ParseFloat(numStr, 64)
			if err != nil || f < 0 {
				return 0, fmt.Errorf("invalid size %q", s)
			}
			return int64(f * float64(u.factor)), nil
		}
	}

	return 0, fmt.Errorf("invalid size %q: use a number in bytes or a value like 4.3MB, 1.5GB, 2TB", s)
}
