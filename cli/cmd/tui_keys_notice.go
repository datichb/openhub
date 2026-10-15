package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/prefsvc"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Notice of the new keys (modernisation tui-keys): shown once, at the
// first launch of the TUI after the update; again from the help (?).

// keysNoticeDue reports whether the notice is to be shown. A first run (the
// configuration wizard opens) records it without showing it: the keys are
// not « new » there. When the cleanup screen of the former deployments is
// still to be offered, it comes first: the notice waits for a later launch.
func keysNoticeDue(ctx context.Context, a *app.App, firstRun bool) bool {
	if a == nil || a.Preferences == nil {
		return false
	}
	var seen bool
	ok, err := prefsvc.New(a.Preferences, nil).Get(ctx, domain.PreferenceScopeGlobal, views.KeysNoticePreference, &seen)
	if err != nil || (ok && seen) {
		return false
	}
	if firstRun {
		markKeysNoticeSeen(ctx, a)
		return false
	}
	return cleanupOffered(ctx, a)
}

func markKeysNoticeSeen(ctx context.Context, a *app.App) {
	_ = prefsvc.New(a.Preferences, nil).Set(ctx, domain.PreferenceScopeGlobal, views.KeysNoticePreference, true)
}

// offerKeysNoticeInTUI shows the notice when it is due (reads off the event
// loop; call it before offerCleanupInTUI, which records the cleanup offer).
func offerKeysNoticeInTUI(ctx context.Context, a *app.App, sh *shell.Shell, firstRun bool) {
	if sh == nil || !keysNoticeDue(ctx, a, firstRun) {
		return
	}
	sh.App().QueueUpdateDraw(func() {
		sh.OfferKeysNotice(func() { go markKeysNoticeSeen(context.Background(), a) })
	})
}
