//go:build windows

package client

import (
	"os/exec"
	"syscall"
)

// SetDetached makes cmd outlive the oh process that starts it (its own
// session on Unix, a new process group on Windows): the daemon spawned by
// Ensure, the driver of a fan-out (daemon.SetDetached).
func SetDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000200}
}
