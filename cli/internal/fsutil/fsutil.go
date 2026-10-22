// Package fsutil holds the file helpers that several packages need and that
// depend on nothing else of the project.
package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileAtomic replaces path with data so that a reader (or a crash) sees
// either the old content or the new one, never a partial file. The data goes
// to a temporary file of the same directory (a unique name, so two writers do
// not step on each other), is flushed to disk, then renamed over path; the
// directory is flushed too where the system allows it (not on Windows).
//
// The final file has exactly perm (the umask does not apply). The temporary
// file is removed when anything fails. The directory of path must exist.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	return writeAtomic(path, data, perm, true)
}

// WriteFileAtomicNoSync is WriteFileAtomic without the flush to disk: the
// replacement is still atomic against a concurrent reader or a killed process,
// but a power cut may leave the old or an empty file. For frequent writes of
// content that can be rebuilt (traces, notifications), where an fsync on each
// write costs more than the risk.
func WriteFileAtomicNoSync(path string, data []byte, perm os.FileMode) error {
	return writeAtomic(path, data, perm, false)
}

func writeAtomic(path string, data []byte, perm os.FileMode, doSync bool) (err error) {
	dir, base := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	tmp, err := os.CreateTemp(dir, "."+base+".*.tmp")
	if err != nil {
		return fmt.Errorf("creating temporary file for %s: %w", path, err)
	}
	name := tmp.Name()
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(name)
		}
	}()

	if _, err = tmp.Write(data); err != nil {
		return fmt.Errorf("writing %s: %w", name, err)
	}
	if err = tmp.Chmod(perm); err != nil {
		return fmt.Errorf("setting permissions of %s: %w", name, err)
	}
	if doSync {
		if err = tmp.Sync(); err != nil {
			return fmt.Errorf("syncing %s: %w", name, err)
		}
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", name, err)
	}
	if err = os.Rename(name, path); err != nil {
		return fmt.Errorf("renaming %s to %s: %w", name, path, err)
	}
	if doSync {
		syncDir(dir)
	}
	return nil
}
