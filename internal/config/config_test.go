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
