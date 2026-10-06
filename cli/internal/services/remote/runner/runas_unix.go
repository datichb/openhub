//go:build unix

package runner

import (
	"os/exec"
	"syscall"
)

func runAs(cmd *exec.Cmd, u *User) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: u.UID, Gid: u.GID}}
}
