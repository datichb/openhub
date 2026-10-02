//go:build !windows

package opencode

import "syscall"

// isProcessAlive checks whether a process with the given PID is still running.
// On Unix, we use the kill(pid, 0) trick: it does not send a signal but checks
// for the existence of the process.
//
//   - err == nil    → process exists and we can signal it
//   - err == EPERM  → process exists but we lack permission (still alive)
//   - err == ESRCH  → no such process (dead)
func isProcessAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err != syscall.ESRCH
}
