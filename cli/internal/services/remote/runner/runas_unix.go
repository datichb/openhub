//go:build unix

package runner

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func runAs(cmd *exec.Cmd, u *User) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: u.UID, Gid: u.GID}}
}

// openNoFollow opens a file for reading without following a final symbolic
// link nor blocking on a FIFO.
func openNoFollow(p string) (*os.File, error) {
	return os.OpenFile(p, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}

func isSymlinkErr(err error) bool { return errors.Is(err, syscall.ELOOP) }

// ownedBy reports whether u owns the file (nil: any owner).
func ownedBy(info os.FileInfo, u *User) bool {
	if u == nil {
		return true
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && st.Uid == u.UID
}
