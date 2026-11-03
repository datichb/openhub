package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/datichb/openhub/cli/internal/buildinfo"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// reportPanic prints the message of a panic that reached Execute and keeps
// its stack in a crash file of the state folder: the message alone does not
// say where it happened. stack is taken in the recover, where the frames of
// the panic are still there (tview recovers a panic of its event loop to
// restore the terminal, then panics again).
func reportPanic(w io.Writer, r any, stack []byte) {
	fmt.Fprint(w, i18n.Tf("cmd.root.oh_internal_error_panic", r))
	if path, err := writeCrashLog(r, stack, time.Now()); err == nil {
		fmt.Fprint(w, i18n.Tf("cmd.root.crash_log_written", path))
	} else {
		fmt.Fprint(w, i18n.Tf("cmd.root.crash_log_failed", err))
		fmt.Fprintf(w, "%s\n", stack)
	}
	fmt.Fprint(w, i18n.T("cmd.root.please_report_this_bug_at_https"))
}

// writeCrashLog writes crash-<time>.log in the run folder of the hub and
// returns its path. It is private to the user: the stack can hold values.
func writeCrashLog(r any, stack []byte, now time.Time) (string, error) {
	dir := ohRunDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "crash-"+now.UTC().Format("20060102T150405.000Z")+".log")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
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
	return path, nil
}
