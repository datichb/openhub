//go:build windows

// Package filelock provides advisory inter-process file locks.
package filelock

import (
	"os"
	"syscall"
)

// Lock is a no-op on Windows (single-process mode).
func Lock(string) (func(), error) { return func() {}, nil }

// ProcessAlive reports whether a process exists.
func ProcessAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil || pid <= 0 {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil || err.Error() != "os: process already finished"
}
