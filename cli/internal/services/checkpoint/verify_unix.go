//go:build !windows

package checkpoint

import (
	"os/exec"
	"syscall"
)

// setProcessGroup runs cmd in its own process group, killed as a whole
// when the context ends (a test runner and its children).
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
