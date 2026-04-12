package config

import (
	"runtime"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Workers != runtime.NumCPU() {
		t.Errorf("Workers = %d, want %d", cfg.Workers, runtime.NumCPU())
	}
	if cfg.Verbose {
		t.Error("Verbose should default to false")
	}
	if cfg.DBPath == "" {
		t.Error("DBPath should not be empty")
	}
}

func TestDefaultDBPath(t *testing.T) {
	path := DefaultDBPath()
	if !strings.HasSuffix(path, "rufus.db") {
		t.Errorf("DBPath should end with rufus.db, got %q", path)
	}
	if !strings.Contains(path, ".rufus") {
		t.Errorf("DBPath should contain .rufus directory, got %q", path)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "valid default config",
			cfg:     Config{DBPath: "/tmp/test.db", Workers: 4},
			wantErr: false,
		},
		{
			name:    "empty DBPath",
			cfg:     Config{DBPath: "", Workers: 4},
			wantErr: true,
		},
		{
			name:    "zero workers",
			cfg:     Config{DBPath: "/tmp/test.db", Workers: 0},
			wantErr: true,
		},
		{
			name:    "negative workers",
			cfg:     Config{DBPath: "/tmp/test.db", Workers: -1},
			wantErr: true,
		},
		{
			name:    "single worker is valid",
			cfg:     Config{DBPath: "/tmp/test.db", Workers: 1},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
