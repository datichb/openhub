package cmd

import (
	"fmt"
	"io"
	"time"

	"github.com/datichb/openhub/cli/internal/crashlog"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// reportPanic prints the message of a panic that reached Execute and keeps
// its stack in a crash file of the state folder (internal/crashlog, which
// safego also writes for the panics it recovers): the message alone does not
// say where it happened. stack is taken in the recover, where the frames of
// the panic are still there (tview recovers a panic of its event loop to
// restore the terminal, then panics again).
func reportPanic(w io.Writer, r any, stack []byte) {
	fmt.Fprint(w, i18n.Tf("cmd.root.oh_internal_error_panic", r))
	if path, err := crashlog.Write(r, stack, time.Now()); err == nil {
		fmt.Fprint(w, i18n.Tf("cmd.root.crash_log_written", path))
	} else {
		fmt.Fprint(w, i18n.Tf("cmd.root.crash_log_failed", err))
		fmt.Fprintf(w, "%s\n", stack)
	}
	fmt.Fprint(w, i18n.T("cmd.root.please_report_this_bug_at_https"))
}
