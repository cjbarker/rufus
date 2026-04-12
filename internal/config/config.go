package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Config holds the application configuration.
type Config struct {
	DBPath  string `json:"db"`
	Workers int    `json:"workers"`
	Verbose bool   `json:"verbose"`
	Quiet   bool   `json:"quiet"`
	NoColor bool   `json:"no_color"`
}

// DefaultDBPath returns the default database file path (~/.rufus/rufus.db).
func DefaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".rufus", "rufus.db")
}

// DefaultConfigPath returns the default config file path (~/.rufus/config.json).
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".rufus", "config.json")
}

// Default returns a Config with default values.
func Default() *Config {
	return &Config{
		DBPath:  DefaultDBPath(),
		Workers: runtime.NumCPU(),
		Verbose: false,
		Quiet:   false,
		NoColor: false,
	}
}

// LoadFile reads a JSON config file and returns a partial Config. Fields
// absent from the file are left at their zero value so callers can distinguish
// "explicitly set to false" from "not present" only when needed.
// Returns nil without error when the file does not exist.
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// ApplyEnv copies environment-variable overrides into dst. Only non-empty
// environment variables are applied; unset variables leave dst unchanged.
//
// Recognized variables:
//
//	RUFUS_DB       – database file path
//	RUFUS_WORKERS  – worker count (integer)
//	RUFUS_VERBOSE  – verbose flag (1/true/yes)
//	RUFUS_QUIET    – quiet flag   (1/true/yes)
//	RUFUS_NO_COLOR – no-color flag (1/true/yes)
func ApplyEnv(dst *Config) {
	if v := os.Getenv("RUFUS_DB"); v != "" {
		dst.DBPath = v
	}
	if v := os.Getenv("RUFUS_WORKERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			dst.Workers = n
		}
	}
	if v := os.Getenv("RUFUS_VERBOSE"); v != "" {
		dst.Verbose = isTruthy(v)
	}
	if v := os.Getenv("RUFUS_QUIET"); v != "" {
		dst.Quiet = isTruthy(v)
	}
	if v := os.Getenv("RUFUS_NO_COLOR"); v != "" {
		dst.NoColor = isTruthy(v)
	}
}

func isTruthy(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "1" || s == "true" || s == "yes"
}
