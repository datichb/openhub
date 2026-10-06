package cmd

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/runsvc"
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

// showAbsenceRecap shows what changed in v5 sessions since the TUI was last
// closed. The stores are read off the event loop.
func showAbsenceRecap(ctx context.Context, a *app.App, sh *shell.Shell) {
	since := readLastSeen()
	if since.IsZero() || a.Sessions == nil {
		return
	}
	go func() {
		svc, err := newSessionService(ctx, a)
		if err != nil {
			return
		}
		r, err := svc.Recap(ctx, since)
		if err != nil || r.Empty() {
			return
		}
		msg := i18n.Tf("tui.sessions.recap", r.Changed, r.Waiting, r.Sleeping, r.Finished, r.Cost)
		if r.NewDecisions > 0 {
			msg += " · " + i18n.Tf("tui.sessions.recap_decisions", r.NewDecisions, r.OpenDecisions)
		}
		level := shell.ToastInfo
		if r.OpenDecisions > 0 {
			level = shell.ToastWarning
		}
		sh.App().QueueUpdateDraw(func() { sh.ShowToast(msg, level) })
	}()
}

// v5BeforeQuit applies the quit policy to the open v5 sessions (I7):
// inactive/waiting sessions sleep now; for each working session the user
// chooses — finish the step then sleep (default), keep running in the
// background, or stop now. Stores and daemon are read off the event loop;
// Esc cancels the quit.
func v5BeforeQuit(a *app.App, sh *shell.Shell) func(quit func()) {
	return func(quit func()) {
		finish := func() {
			writeLastSeen(time.Now())
			quit()
		}
		go func() {
			// Bounded reads: an unresponsive daemon must not hold the quit.
			ctx, cancel := context.WithTimeout(context.Background(), quitReadTimeout)
			defer cancel()
			if !v5Available(ctx) {
				sh.App().QueueUpdateDraw(finish)
				return
			}
			if _, err := daemon.NewClient(daemon.Paths{Dir: ohRunDir()}).Health(ctx); err != nil {
				sh.App().QueueUpdateDraw(finish)
				return
			}
			svc, err := newRunService(ctx, a)
			if err != nil {
				sh.App().QueueUpdateDraw(finish)
				return
			}
			plan, err := svc.QuitPlan(ctx)
			if err != nil || len(plan.Groups) == 0 {
				sh.App().QueueUpdateDraw(finish)
				return
			}
			apply := func(choices map[string]runsvc.QuitChoice) {
				go func() {
					if err := svc.ApplyQuit(context.Background(), plan, choices); err != nil {
						slog.Warn("quit policy not fully applied", "error", err)
					}
					if inProcessDaemonRunning() && len(plan.Working) > 0 {
						// The daemon stops with this process (Windows): let the
						// chosen steps finish first.
						sh.App().QueueUpdateDraw(func() { sh.ShowToast(i18n.T("cmd.daemon.inprocess_waiting"), shell.ToastInfo) })
						wctx, cancel := context.WithTimeout(context.Background(), quitStepWait)
						_ = svc.WaitFinished(wctx, plan, choices)
						cancel()
					}
					sh.App().QueueUpdateDraw(finish)
				}()
			}
			if len(plan.Working) == 0 {
				apply(nil)
				return
			}
			fields := quitFields(ctx, a, plan)
			title := i18n.Tf("tui.sessions.quit_title", len(plan.Working))
			if n := len(plan.Resting); n > 0 {
				title += " · " + i18n.Tf("tui.sessions.quit_resting", n)
			}
			sh.App().QueueUpdateDraw(func() {
				sh.ShowInlineForm(views.InlineFormConfig{
					Title:  title,
					Fields: fields,
					OnSubmit: func(values map[string]string, _ map[string][]string) {
						choices := map[string]runsvc.QuitChoice{}
						for id, v := range values {
							choices[id] = runsvc.QuitChoice(v)
						}
						apply(choices)
					},
					OnCancel: sh.CancelQuit,
				})
			})
		}()
	}
}

// quitStepWait bounds the wait for the working steps when the daemon lives
// in this process (Windows); a second Ctrl+Q quits at once.
const quitStepWait = 30 * time.Minute

// quitReadTimeout bounds the reads of the quit dialog (daemon, stores).
const quitReadTimeout = 10 * time.Second

// quitFields builds one choice per working session (10 §7.5). "Keep running
// in the background" is not offered when the daemon lives in this oh process
// (Windows): it stops with it.
func quitFields(ctx context.Context, a *app.App, plan runsvc.QuitPlan) []views.FormField {
	opts := []views.SelectOption{{Label: i18n.T("tui.sessions.quit_opt.finish"), Value: string(runsvc.QuitFinish)}}
	if !inProcessDaemonRunning() {
		opts = append(opts, views.SelectOption{Label: i18n.T("tui.sessions.quit_opt.background"), Value: string(runsvc.QuitBackground)})
	}
	opts = append(opts, views.SelectOption{Label: i18n.T("tui.sessions.quit_opt.stop"), Value: string(runsvc.QuitStop)})
	names := map[string]string{}
	fields := make([]views.FormField, 0, len(plan.Working))
	for _, s := range plan.Working {
		name, ok := names[s.ProjectID]
		if !ok && a.Projects != nil {
			if p, err := a.Projects.Get(ctx, s.ProjectID); err == nil {
				name = p.Name
			}
			names[s.ProjectID] = name
		}
		label := "● " + firstNonEmpty(s.WorkflowID, s.EntryAgent)
		if name != "" {
			label += " · " + name
		}
		fields = append(fields, views.FormField{Key: s.ID, Label: label, Type: views.FieldSelect, Options: opts, Default: string(runsvc.QuitFinish)})
	}
	return fields
}
