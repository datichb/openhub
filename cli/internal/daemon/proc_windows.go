//go:build windows

package daemon

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// ErrAlreadyRunning is returned by Run when another daemon holds the lock.
var ErrAlreadyRunning = errors.New("ohd is already running")

func lockFile(string) (func(), error) { return nil, ErrUnsupported }

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
