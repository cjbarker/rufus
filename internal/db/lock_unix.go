//go:build !windows

package db

import (
	"os"
	"syscall"
)

// isProcessAlive returns true if the process with the given PID is running.
// On Unix we send signal 0, which checks existence without disturbing the process.
func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
