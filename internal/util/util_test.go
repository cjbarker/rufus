package util

import (
	"testing"
)

// TestFormatParseRoundTrip verifies that values emitted by FormatSize can be
// fed back into ParseSize without a unit-scale mismatch.
func TestFormatParseRoundTrip(t *testing.T) {
	inputs := []int64{512, 1024, 1536, 1024 * 1024, 4_509_450, 1024 * 1024 * 1024}
	for _, original := range inputs {
		formatted := FormatSize(original)
		parsed, err := ParseSize(formatted)
		if err != nil {
			t.Errorf("ParseSize(%q) error: %v", formatted, err)
			continue
		}
		// Allow ±1% tolerance for float rounding in the formatted string.
		delta := original - parsed
		if delta < 0 {
			delta = -delta
		}
		if delta > original/100+1 {
			t.Errorf("round-trip mismatch for %d bytes: FormatSize=%q, ParseSize=%d (delta %d)",
				original, formatted, parsed, delta)
		}
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0B"},
		{500, "500B"},
		{1023, "1023B"},
		{1024, "1.0KB"},
		{1536, "1.5KB"},
		{1024 * 1024, "1.0MB"},
		{4_509_450, "4.3MB"}, // ≈ 4.3 * 1024^2, rounds to 4.3MB
		{1024 * 1024 * 1024, "1.0GB"},
		{1_610_612_736, "1.5GB"}, // 1.5 * 1024^3
	}
	for _, tt := range tests {
		got := FormatSize(tt.bytes)
		if got != tt.want {
			t.Errorf("FormatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{"", 0, false},
		{"   ", 0, false},
		{"0", 0, false},
		{"500", 500, false},
		{"1024", 1024, false},
		{"500B", 500, false},
		{"1KB", 1024, false},
		{"1.5KB", 1536, false},         // 1.5 × 1024
		{"4MB", 4_194_304, false},      // 4 × 1024²
		{"4.3MB", 4_508_876, false},    // int64(4.3 × 1024²)
		{"1.5GB", 1_610_612_736, false}, // int64(1.5 × 1024³)
		{"2TB", 2_199_023_255_552, false}, // 2 × 1024⁴
		{"bad", 0, true},
		{"-1MB", 0, true},
		{"1.2.3MB", 0, true},
	}
	for _, tt := range tests {
		got, err := ParseSize(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseSize(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ParseSize(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
