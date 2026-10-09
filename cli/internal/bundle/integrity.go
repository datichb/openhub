package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Integrity of the session bundles (v5 critical N23, audit S-03, S-10). A
// bundle is shared by every session of the same hash: an agent that changes
// one of its files (a skill, an agent body, bundle.json with the checkpoint
// behaviors) would change all of them. The hash of a bundle is recomputed
// when it is reused by Build and when it is loaded (memoized while no file
// changes), and its files are read-only once built (Seal).

// ErrTampered refuses a bundle whose content no longer matches its hash.
var ErrTampered = errors.New("the session bundle was modified after it was built")

// verified memoizes the bundles checked in this process: dir → signature
// of the files (path, size, modification time) when the hash matched.
var verified sync.Map

// Verify recomputes the hash of the bundle in dir and compares it to hash.
func Verify(dir, hash string) error {
	spec, err := readSpec(dir)
	if err != nil {
		return err
	}
	return verify(dir, spec, hash)
}

func verify(dir string, spec sessionspec.BundleSpec, hash string) error {
	if !contentHash(hash) {
		return nil // not a built bundle (hand-written test fixtures)
	}
	sig, err := signature(dir)
	if err != nil {
		return err
	}
	if v, ok := verified.Load(dir); ok && v.(string) == sig {
		return nil
	}
	got, err := hashBundle(dir, spec)
	if err != nil {
		return err
	}
	if got != hash {
		verified.Delete(dir)
		return fmt.Errorf("%w: %s", ErrTampered, dir)
	}
	verified.Store(dir, sig)
	return nil
}

// contentHash reports a bundle name made by Build (hex SHA-256).
func contentHash(h string) bool {
	if len(h) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(h)
	return err == nil
}

// signature summarizes the files of dir without reading them.
func signature(dir string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%d\x00%d\x00%o\x00", p, info.Size(), info.ModTime().UnixNano(), info.Mode())
		return nil
	})
	return hex.EncodeToString(h.Sum(nil)), err
}

// Seal makes the files of a bundle read-only (0444, directories 0555,
// deepest first).
func Seal(dir string) error {
	var dirs []string
	if err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirs = append(dirs, p)
			return nil
		}
		return os.Chmod(p, bundleReadOnly)
	}); err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Chmod(dirs[i], 0o555); err != nil {
			return err
		}
	}
	return nil
}

// Unseal makes a bundle directory (or a directory holding bundles) writable
// again by its owner, so that it can be removed.
func Unseal(dir string) error {
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrPermission) {
				_ = os.Chmod(p, 0o755)
				return nil
			}
			return err
		}
		if d.IsDir() {
			return os.Chmod(p, 0o755)
		}
		return os.Chmod(p, 0o644)
	})
}

// RemoveBundle removes a bundle directory, sealed or not.
func RemoveBundle(dir string) error {
	_ = Unseal(dir)
	verified.Delete(dir)
	return os.RemoveAll(dir)
}

const bundleReadOnly fs.FileMode = 0o444
