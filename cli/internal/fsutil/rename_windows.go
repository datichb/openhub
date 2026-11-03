//go:build windows

package fsutil

import (
	"errors"
	"os"
	"syscall"
	"time"
)

// Windows errors that a rename over a file returns while another process or
// goroutine holds or replaces the target at the same moment.
const (
	errorAccessDenied     syscall.Errno = 5
	errorSharingViolation syscall.Errno = 32
)

// rename replaces newpath with oldpath. On Windows, replacing a file that
// another writer is replacing at the same time fails for a short while
// (access denied, sharing violation) instead of waiting: retry for up to
// two seconds, as the Go toolchain does (cmd/go/internal/robustio).
func rename(oldpath, newpath string) error {
	delay := time.Millisecond
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := os.Rename(oldpath, newpath)
		if err == nil || !transientRenameError(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(delay)
		if delay < 32*time.Millisecond {
			delay *= 2
		}
	}
}

func transientRenameError(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	return errno == errorAccessDenied || errno == errorSharingViolation
}
