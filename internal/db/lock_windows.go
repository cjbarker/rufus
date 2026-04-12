//go:build windows

package db

import (
	"os"
)

// isProcessAlive returns true if the process with the given PID is running.
// On Windows, os.FindProcess always succeeds; we probe liveness by opening
// the process handle via the standard library.
func isProcessAlive(pid int) bool {
	// os.FindProcess on Windows returns a handle; a subsequent Signal call
	// is not meaningful, so we use a different heuristic: attempt to open
	// the process directory under /proc — not available on Windows. Instead
	// we rely on OpenProcess via the os package indirectly: if FindProcess
	// returns a non-nil process and we can read its state, it is alive.
	// The most portable approach without cgo is to check whether a kill(0)
	// equivalent is feasible; on Windows we fall back to a best-effort check.
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Windows, Signal with os.Interrupt (SIGINT) is the only supported
	// signal and would actually interrupt the process. Instead, we treat any
	// non-error from FindProcess as "possibly alive" and let the user resolve
	// stale locks manually if needed.
	_ = proc
	return true
}
