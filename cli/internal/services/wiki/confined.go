package wiki

import (
	"io/fs"
	"os"
	"path/filepath"
)

// Reads confined to the project (p3-remote-wiki): in a remote job the oh
// daemon serves the wiki as root from a clone that belongs to the account
// of the agents. A symbolic link of the clone, even one swapped in between
// a check and the read, never makes oh read or stat a file outside the
// project (os.Root refuses to leave its directory).

// readConfined reads rel, a slash path relative to root, inside root.
func readConfined(root, rel string) ([]byte, error) {
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return r.ReadFile(filepath.FromSlash(rel))
}

// statConfined stats rel, a slash path relative to root, inside root.
func statConfined(root, rel string) (fs.FileInfo, error) {
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return r.Stat(filepath.FromSlash(rel))
}
