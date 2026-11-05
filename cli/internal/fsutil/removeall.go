package fsutil

import (
	"io/fs"
	"os"
	"path/filepath"
)

// RemoveAllWritable removes the tree at path like os.RemoveAll, and makes
// writable what refuses to go: git writes the objects of a clone read-only,
// which Windows will not delete. It returns the error of the second attempt.
func RemoveAllWritable(path string) error {
	err := os.RemoveAll(path)
	if err == nil {
		return nil
	}
	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return nil //nolint:nilerr // best effort: the retry reports what remains
		}
		mode := os.FileMode(0o600)
		if d.IsDir() {
			mode = 0o700
		}
		_ = os.Chmod(p, mode)
		return nil
	})
	return os.RemoveAll(path)
}
