//go:build unix

package peercheck

import (
	"os"
	"syscall"
)

// FileOwner returns the owner of a file (false where the system does not tell).
func FileOwner(fi os.FileInfo) (int, bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return -1, false
	}
	return int(st.Uid), true
}
