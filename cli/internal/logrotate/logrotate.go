// Package logrotate keeps the log files of oh bounded: a file larger than a
// limit is moved to <file>.1 (the older copies to .2, .3…; the last one
// dropped). Rotate renames the file (nobody writes it: before a process
// opens it); CopyTruncate copies then truncates it in place (a live process
// keeps it open with O_APPEND and goes on writing at the new end).
package logrotate

import (
	"fmt"
	"io"
	"os"
)

// Defaults of the oh logs ([gc] log_max_mb, log_keep).
const (
	DefaultMaxBytes int64 = 20 << 20
	DefaultKeep           = 3
)

// Rotate moves path to path.1 when it is larger than maxBytes, after
// shifting the older copies (keep of them are kept). It reports whether it
// rotated. A missing file is not an error.
func Rotate(path string, maxBytes int64, keep int) (bool, error) {
	if !tooLarge(path, maxBytes) {
		return false, nil
	}
	if err := shift(path, keep); err != nil {
		return false, err
	}
	if keep <= 0 {
		return true, os.Remove(path)
	}
	return true, os.Rename(path, path+".1")
}

// CopyTruncate copies path to path.1 when it is larger than maxBytes, then
// truncates it to zero. Lines written between the copy and the truncation
// are lost (the usual trade-off of copy-truncate).
func CopyTruncate(path string, maxBytes int64, keep int) (bool, error) {
	if !tooLarge(path, maxBytes) {
		return false, nil
	}
	if err := shift(path, keep); err != nil {
		return false, err
	}
	if keep > 0 {
		if err := copyFile(path, path+".1"); err != nil {
			return false, err
		}
	}
	return true, os.Truncate(path, 0)
}

func tooLarge(path string, maxBytes int64) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular() && maxBytes > 0 && fi.Size() > maxBytes
}

// shift drops path.<keep> and renames path.<n> to path.<n+1>.
func shift(path string, keep int) error {
	if keep <= 0 {
		return nil
	}
	if err := os.Remove(fmt.Sprintf("%s.%d", path, keep)); err != nil && !os.IsNotExist(err) {
		return err
	}
	for n := keep - 1; n >= 1; n-- {
		from := fmt.Sprintf("%s.%d", path, n)
		if err := os.Rename(from, fmt.Sprintf("%s.%d", path, n+1)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	mode := os.FileMode(0o600)
	if fi, err := in.Stat(); err == nil {
		mode = fi.Mode().Perm()
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
