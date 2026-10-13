//go:build !windows

package gitutil

import (
	"os"
	"os/exec"
	"syscall"
)

// statOwner returns the uid owning path.
func statOwner(path string) (uint32, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return st.Uid, true
}

// runAsOwner runs cmd as o, its group only (no supplementary group of oh).
func runAsOwner(cmd *exec.Cmd, o Owner) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Credential = &syscall.Credential{Uid: o.UID, Gid: o.GID, Groups: []uint32{}}
	return nil
}

// openNoBlock opens a file for reading without blocking on a FIFO.
func openNoBlock(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
