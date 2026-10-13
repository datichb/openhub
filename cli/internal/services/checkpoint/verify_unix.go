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

// runAs runs cmd as acct (nil: the account of oh), its group only.
func runAs(cmd *exec.Cmd, acct *Account) error {
	if acct == nil {
		return nil
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Credential = &syscall.Credential{Uid: acct.UID, Gid: acct.GID, Groups: []uint32{}}
	return nil
}
