package views

import (
	"context"
	"time"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

const pullSlowThreshold = time.Second

// syncAsync triggers an asynchronous git pull on repo.
//
// Behaviour:
//   - Returns immediately; the pull runs in a background goroutine.
//   - If the pull takes longer than pullSlowThreshold (1 s), a
//     "Synchronisation..." toast is shown via shell (if not nil).
//   - When the pull completes (success or failure), onDone is called
//     on the tview event loop via app.QueueUpdateDraw.
//   - On pull error: shows a warning toast ("Sync impossible — données locales")
//     so the user knows data may be stale, then still calls onDone so the view
//     can render with whatever local data is available.
//   - If app is nil or repo is nil, onDone is invoked synchronously with nil.
//   - The shell's lifecycle context is used to cancel the pull on app exit.
func syncAsync(
	app *tview.Application,
	repo teamstate.TeamStateWriter,
	shell ShellAccess,
	onDone func(pullErr error),
) {
	var ctx context.Context
	if shell != nil {
		ctx = shell.Context()
	} else {
		ctx = context.Background()
	}
	pullFn := func() error { return repo.Pull(ctx) }
	syncFuncAsync(app, pullFn, shell, onDone)
}

// syncFuncAsync is the low-level variant used when a bare sync function
// (not a teamstate.TeamStateReader) is available — e.g. in TeamBoardView where the
// repo reference is owned by the caller.
//
// pullFn is called in a background goroutine. If pullFn is nil, onDone is
// called synchronously with nil. All other behaviour is identical to syncAsync.
func syncFuncAsync(
	app *tview.Application,
	pullFn func() error,
	shell ShellAccess,
	onDone func(pullErr error),
) {
	if app == nil || pullFn == nil {
		if onDone != nil {
			onDone(nil)
		}
		return
	}

	// Slow-pull indicator: fires only if the pull blocks for > 1 s.
	timer := time.AfterFunc(pullSlowThreshold, func() {
		app.QueueUpdateDraw(func() {
			if shell != nil {
				shell.ShowToastMsg("Synchronisation...", true)
			}
		})
	})

	// Determine lifecycle context for early-out on app exit.
	var ctx context.Context
	if shell != nil {
		ctx = shell.Context()
	} else {
		ctx = context.Background()
	}

	go func() {
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		default:
		}

		err := pullFn()
		timer.Stop()

		select {
		case <-ctx.Done():
			return
		default:
		}

		app.QueueUpdateDraw(func() {
			if err != nil && !teamstate.IsPullWarning(err) {
				// Hard error (auth failure, no network) — warn the user.
				if shell != nil {
					shell.ShowToastMsg("Sync impossible — données locales", false)
				}
			}
			// Always call onDone so the view refreshes from local data.
			if onDone != nil {
				onDone(err)
			}
		})
	}()
}
