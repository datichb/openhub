//go:build !windows

package cmd

import (
	"os"
	"syscall"
)

func ownedByMe(st os.FileInfo) bool {
	sys, ok := st.Sys().(*syscall.Stat_t)
	return !ok || int(sys.Uid) == os.Getuid()
}
