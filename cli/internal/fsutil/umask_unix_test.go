//go:build !windows

package fsutil

import "syscall"

func setUmask(m int) int { return syscall.Umask(m) }
