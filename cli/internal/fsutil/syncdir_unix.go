//go:build !windows

package fsutil

import "os"

// syncDir flushes the directory entry of the renamed file. The new content is
// already in place, so a failure (some file systems refuse to sync a
// directory) is not reported.
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}
