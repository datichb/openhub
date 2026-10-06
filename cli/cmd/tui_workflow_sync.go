package cmd

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
)

// Replay of the offline workflow publication queue (O14) after each
// successful team-state synchronization of the TUI (views.SetTeamSyncHook).
// The CLI does not replay at startup: `oh workflow publish --retry`.

// tuiQueueFlushing guards one replay per team-state at a time.
var tuiQueueFlushing sync.Map

// tuiReplayWorkflowQueue replays the current member's queued publications
// of the team-state at repoPath (called off the event loop).
func tuiReplayWorkflowQueue(ctx context.Context, repoPath string) {
	a := TryApp()
	if a == nil || a.Config == nil {
		return
	}
	team := teamByStatePath(a.Config, repoPath)
	if team == nil {
		return
	}
	c := workflowsvc.Context{TeamID: team.ID}
	svc := newWorkflowService(ctx)
	if ops, err := svc.QueuedOps(ctx, c); err != nil || len(ops) == 0 {
		return
	}
	if _, busy := tuiQueueFlushing.LoadOrStore(team.ID, true); busy {
		return
	}
	defer tuiQueueFlushing.Delete(team.ID)
	pubs, errs := svc.FlushQueue(ctx, c)
	if ctx.Err() != nil || (len(pubs) == 0 && len(errs) == 0) {
		return
	}
	notifyQueueReplay(pubs, errs)
}

// notifyQueueReplay toasts the result of a replay and refreshes the
// catalogue.
func notifyQueueReplay(pubs []*workflowsvc.Publication, errs []error) {
	sh := tuiShell
	if sh == nil {
		return
	}
	var refs []string
	for _, p := range pubs {
		refs = append(refs, p.Ref.String())
	}
	sh.App().QueueUpdateDraw(func() {
		if len(pubs) > 0 {
			sh.ShowToast(i18n.Tf("tui.catalog.edit.queue_replayed", len(pubs), strings.Join(refs, ", ")), shell.ToastSuccess)
		}
		if len(errs) > 0 {
			sh.ShowToast(i18n.Tf("tui.catalog.edit.queue_failed", errors.Join(errs...).Error()), shell.ToastError)
		}
		sh.RemountIf("workflows")
	})
}

// teamByStatePath returns the team (solo included) whose team-state is at
// path.
func teamByStatePath(cfg *config.Config, path string) *config.TeamConfig {
	want := filepath.Clean(path)
	for i := range cfg.Teams {
		t := &cfg.Teams[i]
		p := t.StatePath
		if p == "" {
			p = config.DefaultTeamStatePath()
		}
		if t.Enabled && filepath.Clean(p) == want {
			return t
		}
	}
	return nil
}
