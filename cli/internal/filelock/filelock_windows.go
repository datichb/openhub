//go:build windows

// Package filelock provides advisory inter-process file locks.
package filelock

import (
	"context"
	"errors"
	"os"
	"syscall"
)

// Lock is a no-op on Windows (single-process mode).
func Lock(string) (func(), error) { return func() {}, nil }

// LockContext is a no-op on Windows (single-process mode).
func LockContext(context.Context, string) (func(), error) { return func() {}, nil }

// ErrLocked is returned by TryLock when another holder has the lock.
var ErrLocked = errors.New("filelock: already locked")

// TryLock is a no-op on Windows (single-process mode).
func TryLock(string) (func(), error) { return func() {}, nil }

// ProcessAlive reports whether a process exists.
func ProcessAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil || pid <= 0 {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil || err.Error() != "os: process already finished"
}
