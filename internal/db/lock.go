package db

import (
	"fmt"
	"os"
)

// AcquireLock creates an exclusive lock file alongside the database so that
// only one rufus scan process can run against the same database at a time.
//
// Returns a release function that removes the lock file. The caller must
// defer the release function to ensure cleanup on normal exit. On a crash the
// lock file is left behind; subsequent runs detect the stale file, verify the
// recorded PID is no longer alive, and remove it automatically.
//
// If the lock cannot be acquired (another process holds it), an error
// describing the lock file path is returned so the user knows how to recover.
func AcquireLock(dbPath string) (release func(), err error) {
	lockPath := dbPath + ".lock"

	// Attempt atomic exclusive creation.
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err == nil {
		// We own the lock — write our PID for diagnostics.
		_, _ = fmt.Fprintf(f, "%d", os.Getpid())
		_ = f.Close()
		return func() { _ = os.Remove(lockPath) }, nil
	}

	if !os.IsExist(err) {
		return nil, fmt.Errorf("creating lock file: %w", err)
	}

	// Lock file already exists — check whether it is stale.
	data, readErr := os.ReadFile(lockPath)
	if readErr != nil {
		return nil, fmt.Errorf("another rufus scan may be running\n  lock file: %s", lockPath)
	}

	var pid int
	if _, parseErr := fmt.Sscanf(string(data), "%d", &pid); parseErr == nil && pid > 0 {
		if isProcessAlive(pid) {
			return nil, fmt.Errorf(
				"another rufus scan is already running (PID %d)\n  if that process has ended, delete: %s",
				pid, lockPath,
			)
		}
		// Stale lock from a dead process — clean it up and retry once.
		_ = os.Remove(lockPath)
		return AcquireLock(dbPath)
	}

	// Unreadable or unparseable lock file — treat as stale.
	_ = os.Remove(lockPath)
	return AcquireLock(dbPath)
}
