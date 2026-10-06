//go:build !windows

package opencodev2

import (
	"os/exec"
	"syscall"
)

// setDetached puts the server in its own session/process group so it is not
// killed when the terminal or the oh process goes away.
func setDetached(cmd *exec.Cmd) {
	// Keep the attributes set by the runtime (credentials of the oh runner).
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
}

func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) != syscall.ESRCH
}

func terminateGroup(pid int) {
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
}

func killGroup(pid int) {
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}
