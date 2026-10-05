//go:build !windows

// Package filelock provides advisory inter-process file locks.
package filelock

import (
	"os"
	"path/filepath"
	"syscall"
)

// Lock blocks until an exclusive lock on path is held and returns its release function.
func Lock(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}

// ProcessAlive reports whether a process exists.
func ProcessAlive(pid int) bool { return pid > 0 && syscall.Kill(pid, 0) != syscall.ESRCH }
