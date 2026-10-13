//go:build !windows

package docstest

import (
	"os/exec"
	"syscall"
)

// setProcessGroup runs cmd in its own process group, killed as a whole
// when its context ends: mmdc and the headless browser it starts (left
// alive, the browser keeps writing to the temporary directory of the test).
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
