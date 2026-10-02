//go:build windows

package opencode

import "os"

// isProcessAlive checks whether a process with the given PID is still running.
// On Windows, os.FindProcess always succeeds (it doesn't verify existence).
// We probe liveness by sending signal 0 — on Windows this calls OpenProcess
// and returns a non-nil error if the process does not exist or is not accessible.
func isProcessAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal(os.Signal(syscall.Signal(0))) on Windows calls OpenProcess internally.
	// It returns a non-nil error when the process does not exist.
	err = p.Signal(os.Signal(nil))
	// err == nil is not possible (nil signal is not implemented), but if the
	// error message is "os: process already finished" the process is dead.
	// Any other error (including "not supported by windows") means the process
	// handle was valid, i.e. the process exists.
	if err == nil {
		return true
	}
	return err.Error() != "os: process already finished"
}
