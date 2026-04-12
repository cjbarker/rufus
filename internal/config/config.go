package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// Config holds the application configuration.
type Config struct {
	DBPath  string
	Workers int
	Verbose bool
}

// DefaultDBPath returns the default database file path (~/.rufus/rufus.db).
func DefaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".rufus", "rufus.db")
}

// Default returns a Config with default values.
func Default() *Config {
	return &Config{
		DBPath:  DefaultDBPath(),
		Workers: runtime.NumCPU(),
		Verbose: false,
	}
}
