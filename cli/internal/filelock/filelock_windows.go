//go:build windows

// Package filelock provides advisory inter-process file locks.
package filelock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// ErrLocked is returned by TryLock when another holder has the lock.
var ErrLocked = errors.New("filelock: already locked")

func lock(path string, flags uint32) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	ol := new(windows.Overlapped)
	h := windows.Handle(f.Fd())
	if err := windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK|flags, 0, 1, 0, ol); err != nil {
		f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, ErrLocked
		}
		return nil, err
	}
	return func() {
		_ = windows.UnlockFileEx(h, 0, 1, 0, ol)
		f.Close()
	}, nil
}

// Lock blocks until an exclusive lock on path is held and returns its release function.
func Lock(path string) (func(), error) { return lock(path, 0) }

// TryLock takes an exclusive lock on path without waiting; it returns
// ErrLocked when the lock is held elsewhere.
func TryLock(path string) (func(), error) { return lock(path, windows.LOCKFILE_FAIL_IMMEDIATELY) }

// LockContext waits for an exclusive lock on path until ctx is done.
func LockContext(ctx context.Context, path string) (func(), error) {
	for {
		unlock, err := TryLock(path)
		if !errors.Is(err, ErrLocked) {
			return unlock, err
		}
		t := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil, fmt.Errorf("filelock: waiting for %s: %w", path, ctx.Err())
		case <-t.C:
		}
	}
}

// ProcessAlive reports whether a process exists.
func ProcessAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil || pid <= 0 {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil || err.Error() != "os: process already finished"
}
