// Package crashlog writes the crash report of a panic: crash-<time>.log in
// the run folder of the hub, with the version, the panic and its stack.
//
// Two places write it: cmd.Execute, for a panic of the main goroutine, and
// internal/safego, for a panic it recovers in another goroutine. The package
// depends on neither, so both can import it.
package crashlog

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/datichb/openhub/cli/internal/buildinfo"
	"github.com/datichb/openhub/cli/internal/config"
)

// keep is how many crash files stay in the run folder: a loop that panics
// again and again must not fill the disk of the user.
const keep = 20

const (
	prefix = "crash-"
	suffix = ".log"
)

// Dir is the folder of the crash files.
func Dir() string { return filepath.Join(config.HubDir(), "run") }

// Enabled reports whether Write writes anything. A test binary that did not
// give the hub a folder of its own (OH_HOME) writes nothing: a test that
// panics on purpose must not leave files in the hub of the developer.
func Enabled() bool {
	return !testing.Testing() || os.Getenv("OH_HOME") != ""
}

// Write writes the crash report of the panic r, whose stack was read in the
// recover (the frames of the panic are still there), and returns its path.
// The file is private to the user: the stack can hold values. It is
// ErrDisabled when Enabled is false.
func Write(r any, stack []byte, now time.Time) (string, error) {
	if !Enabled() {
		return "", ErrDisabled
	}
	dir := Dir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	stamp := now.UTC().Format("20060102T150405.000Z")
	var (
		f    *os.File
		path string
		err  error
	)
	// Two panics in the same millisecond get two files.
	for n := 0; n < 100; n++ {
		name := prefix + stamp
		if n > 0 {
			name += fmt.Sprintf("-%d", n)
		}
		path = filepath.Join(dir, name+suffix)
		f, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if !errors.Is(err, fs.ErrExist) {
			break
		}
	}
	if err != nil {
		return "", err
	}
	_, werr := fmt.Fprintf(f, "oh %s (commit %s, built %s)\n%s %s/%s\ntime: %s\n\npanic: %v\n\n%s",
		buildinfo.Version, buildinfo.Commit, buildinfo.BuildDate, runtime.Version(), runtime.GOOS, runtime.GOARCH,
		now.UTC().Format(time.RFC3339), r, stack)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return "", werr
	}
	prune(dir)
	return path, nil
}

// ErrDisabled is the error of Write when it writes nothing (see Enabled).
var ErrDisabled = errors.New("crash reports are not written by test binaries without OH_HOME")

// prune removes the oldest crash files beyond keep. The names hold the time,
// so the name order is the age order.
func prune(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		if n := e.Name(); !e.IsDir() && strings.HasPrefix(n, prefix) && strings.HasSuffix(n, suffix) {
			names = append(names, n)
		}
	}
	if len(names) <= keep {
		return
	}
	sort.Strings(names)
	for _, n := range names[:len(names)-keep] {
		_ = os.Remove(filepath.Join(dir, n))
	}
}
