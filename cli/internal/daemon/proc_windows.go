//go:build windows

package daemon

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// ErrAlreadyRunning is returned by Run when another daemon holds the lock.
var ErrAlreadyRunning = errors.New("ohd is already running")

// lockFile takes an exclusive, non-blocking lock on path (LockFileEx).
func lockFile(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	ol := new(windows.Overlapped)
	h := windows.Handle(f.Fd())
	if err := windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ol); err != nil {
		f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, ErrAlreadyRunning
		}
		return nil, fmt.Errorf("locking %s: %w", path, err)
	}
	return func() {
		_ = windows.UnlockFileEx(h, 0, 1, 0, ol)
		f.Close()
	}, nil
}

func setDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000200}
}

func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil || err.Error() != "os: process already finished"
}
