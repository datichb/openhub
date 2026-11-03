//go:build !windows

package fsutil

import "os"

// rename replaces newpath with oldpath; the rename is atomic on Unix.
func rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}
