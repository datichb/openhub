package remote

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Packages are gzip tarballs written deterministically (sorted names, fixed
// time, no owner): the same content gives the same bytes and hash.

var epoch = time.Unix(0, 0).UTC()

// MaxUnpack bounds the size of an unpacked package.
const MaxUnpack = 512 << 20

// PackFiles packs name → content.
func PackFiles(files map[string][]byte) ([]byte, error) {
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	gz, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	gz.ModTime = epoch
	tw := tar.NewWriter(gz)
	for _, n := range names {
		if err := checkName(n); err != nil {
			return nil, err
		}
		if err := tw.WriteHeader(&tar.Header{Name: n, Mode: 0o644, Size: int64(len(files[n])), ModTime: epoch, Typeflag: tar.TypeReg, Format: tar.FormatPAX}); err != nil {
			return nil, err
		}
		if _, err := tw.Write(files[n]); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// PackDir packs the regular files of dir (symlinks and other types refused).
func PackDir(dir string) ([]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("pack: %s is not a regular file", p)
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = data
		return nil
	})
	if err != nil {
		return nil, err
	}
	return PackFiles(files)
}

// SHA256 returns the hex SHA-256 of data.
func SHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func checkName(n string) error {
	if n == "" || strings.HasPrefix(n, "/") || strings.Contains(n, "\\") || path.Clean(n) != n || n == ".." || strings.HasPrefix(n, "../") {
		return fmt.Errorf("pack: invalid file name %q", n)
	}
	return nil
}

// UnpackFiles reads a package into memory.
func UnpackFiles(r io.Reader) (map[string][]byte, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("unpack: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	out := map[string][]byte{}
	var total int64
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("unpack: %w", err)
		}
		if h.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("unpack: %s is not a regular file", h.Name)
		}
		if err := checkName(h.Name); err != nil {
			return nil, err
		}
		total += h.Size
		if total > MaxUnpack {
			return nil, errors.New("unpack: package too large")
		}
		data, err := io.ReadAll(io.LimitReader(tr, h.Size))
		if err != nil {
			return nil, err
		}
		out[h.Name] = data
	}
}

// UnpackDir writes a package under dir (files read-only).
func UnpackDir(r io.Reader, dir string) error {
	files, err := UnpackFiles(r)
	if err != nil {
		return err
	}
	for n, data := range files {
		p := filepath.Join(dir, filepath.FromSlash(n))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, data, 0o444); err != nil {
			return err
		}
	}
	return nil
}
