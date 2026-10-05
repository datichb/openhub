package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// v5 session lifecycle in the TUI (E11): presence heartbeat while the TUI is
// open, recap of what ran while oh was closed, and the quit policy dialog.

func lastSeenPath() string { return filepath.Join(ohRunDir(), "last_seen") }

func readLastSeen() time.Time {
	data, err := os.ReadFile(lastSeenPath())
	if err != nil {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		return time.Time{}
	}
	return t
}

func writeLastSeen(t time.Time) {
	_ = os.MkdirAll(ohRunDir(), 0o700)
	_ = os.WriteFile(lastSeenPath(), []byte(t.Format(time.RFC3339)), 0o600)
}

// v5Sessions returns the open v5 sessions (not stopped/completed/failed).
func v5Sessions(ctx context.Context, a *app.App) []domain.Session {
	if a.Sessions == nil {
		return nil
	}
	all, err := a.Sessions.List(ctx, "")
	if err != nil {
		return nil
	}
	var out []domain.Session
	for _, s := range all {
		if s.GroupKey == "" {
			continue
		}
		switch s.State {
		case domain.RunStopped, domain.RunCompleted, domain.RunFailed:
			continue
		}
		out = append(out, s)
	}
	return out
}

// startTUIPresence declares the open TUI to the daemon (a pending decision then
// keeps its server awake) and returns a stop function.
func startTUIPresence(ctx context.Context) func() {
	if !v5Available(ctx) {
		return func() {}
	}
	pctx, cancel := context.WithCancel(ctx)
	go func() {
		// Heartbeats are retried every period: a daemon started (or
		// restarted) after the TUI opened still learns that oh is open.
		dc := daemon.NewClient(daemon.Paths{Dir: ohRunDir()})
		dc.KeepAlive(pctx, daemon.HeartbeatRequest{ClientID: "tui-" + sessionspec.NewSessionID(), Kind: daemon.ClientPresence}, 30*time.Second)
	}()
	return cancel
}

// showAbsenceRecap shows what changed in v5 sessions since the TUI was last closed.
func showAbsenceRecap(ctx context.Context, a *app.App, sh *shell.Shell) {
	since := readLastSeen()
	if since.IsZero() || a.Sessions == nil {
		return
	}
	all, err := a.Sessions.List(ctx, "")
	if err != nil {
		return
	}
	changed, waiting, sleeping := 0, 0, 0
	cost := 0.0
	for _, s := range all {
		if s.GroupKey == "" || s.StateChangedAt == nil || !s.StateChangedAt.After(since) {
			continue
		}
		changed++
		cost += s.Cost
		switch s.State {
		case domain.RunWaiting:
			waiting++
		case domain.RunSleeping:
			sleeping++
		}
	}
	if changed == 0 {
		return
	}
	sh.ShowToast(i18n.Tf("tui.v5.recap", changed, waiting, sleeping, cost), shell.ToastInfo)
}

// v5BeforeQuit applies the quit policy to the open v5 sessions (I7):
// inactive/waiting groups sleep now; working groups follow the user choice
// (finish the step then sleep — default —, keep running, or stop now).
func v5BeforeQuit(a *app.App, sh *shell.Shell) func(quit func()) {
	return func(quit func()) {
		ctx := context.Background()
		finish := func() {
			writeLastSeen(time.Now())
			quit()
		}
		sessions := v5Sessions(ctx, a)
		if len(sessions) == 0 || !v5Available(ctx) {
			finish()
			return
		}
		dc := daemon.NewClient(daemon.Paths{Dir: ohRunDir()})
		if _, err := dc.Health(ctx); err != nil {
			finish()
			return
		}
		working := map[string]bool{}
		groups := map[string]bool{}
		for _, s := range sessions {
			if s.State == domain.RunSleeping {
				continue
			}
			groups[s.GroupKey] = true
			if s.State == domain.RunActive {
				working[s.GroupKey] = true
			}
		}
		apply := func(workingPolicy daemon.QuitPolicy) {
			for g := range groups {
				p := daemon.PolicySleepWhenIdle
				if working[g] {
					p = workingPolicy
				}
				_ = dc.SetPolicy(ctx, g, p)
			}
			finish()
		}
		if len(working) == 0 {
			apply(daemon.PolicySleepWhenIdle)
			return
		}
		sh.ShowSelectModal(i18n.Tf("tui.v5.quit_title", len(working)), []views.SelectOption{
			{Label: i18n.T("tui.v5.quit_finish"), Value: string(daemon.PolicySleepWhenIdle)},
			{Label: i18n.T("tui.v5.quit_background"), Value: string(daemon.PolicyBackground)},
			{Label: i18n.T("tui.v5.quit_stop"), Value: string(daemon.PolicyStopNow)},
			{Label: i18n.T("tui.v5.quit_cancel"), Value: "cancel"},
		}, string(daemon.PolicySleepWhenIdle), func(v string) {
			if v == "cancel" {
				sh.CancelQuit()
				return
			}
			apply(daemon.QuitPolicy(v))
		})
	}
}
