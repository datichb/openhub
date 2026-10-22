//go:build windows

package fsutil

func setUmask(m int) int { return 0 }
