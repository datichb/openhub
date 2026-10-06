package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// Remote sessions in the Sessions view (P5-T18, 10 §7.4): « À récupérer »
// section, fetch (`g`), replay of the Beads journal with the conflict window.

// Remote statuses of SessionRow.Remote.
const (
	RemoteStatusSent     = "sent"
	RemoteStatusRunning  = "running"
	RemoteStatusReady    = "ready"
	RemoteStatusFailed   = "failed"
	RemoteStatusFetched  = "fetched"
	RemoteStatusResolved = "resolved"
)

// Conflict resolutions (ReplayConflict.Resolution).
const (
	ResolutionKeepLocal   = "keep_local"
	ResolutionApplyRemote = "apply_remote"
	ResolutionMergeNotes  = "merge_notes"
)

// RemoteFetchReport is the outcome of a fetch, display-ready.
type RemoteFetchReport struct {
	Lines   []string
	Journal int // Beads journal entries to replay
}

// ReplayField is a field changed remotely on a conflicting ticket.
type ReplayField struct{ Field, Local, Remote string }

// ReplayConflict is a ticket changed both on the machine and by the session.
type ReplayConflict struct {
	Ticket, Title string
	Fields        []ReplayField
	Notes         []string
	Resolution    string
}

// ReplayView is the replay of a journal, display-ready.
type ReplayView struct {
	Lines     []string // one per journal entry, with its state
	Conflicts []ReplayConflict
	Pending   int // entries left to apply
	Failed    int
}

// SessionRemote is implemented by backends that run sessions remotely.
type SessionRemote interface {
	FetchRemote(ctx context.Context, sessionID string) (RemoteFetchReport, error)
	PlanReplay(ctx context.Context, sessionID string) (*ReplayView, error)
	// ApplyReplay applies the pending entries; resolutions by ticket.
	ApplyReplay(ctx context.Context, sessionID string, resolutions map[string]string) (*ReplayView, error)
}

// toFetch reports a remote session waiting for the user (fetch or replay).
func toFetch(r SessionRow) bool {
	return r.Remote == RemoteStatusReady || r.Remote == RemoteStatusFailed || r.Remote == RemoteStatusFetched
}

// splitRemote separates the sessions to fetch from the others.
func splitRemote(rows []SessionRow) (rest, fetch []SessionRow) {
	for _, r := range rows {
		if toFetch(r) {
			fetch = append(fetch, r)
			continue
		}
		rest = append(rest, r)
	}
	return rest, fetch
}

// remoteSection adds the « À récupérer » section.
func remoteSection(items []widgets.SectionItem, rows []SessionRow) []widgets.SectionItem {
	if len(rows) == 0 {
		return items
	}
	items = append(items, widgets.SectionItem{MainText: i18n.Tf("tui.remote.section.fetch", len(rows)), IsHeader: true})
	for _, r := range rows {
		it := sessionItem(r)
		if r.RemoteDetail != "" {
			it.SecondaryText = "   " + r.RemoteDetail
		}
		items = append(items, it)
	}
	return items
}

// fetchSelected fetches a finished remote session (`g`), then offers the
// replay of its Beads journal.
func (v *SessionsView) fetchSelected(r *SessionRow) {
	rb, ok := v.cfg.Backend.(SessionRemote)
	if !ok || r == nil || v.shell == nil {
		return
	}
	id, label := r.ID, sessionLabel(*r)
	switch r.Remote {
	case RemoteStatusFetched:
		v.replayFlow(rb, id, label)
		return
	case RemoteStatusReady, RemoteStatusFailed:
	default:
		v.toast(i18n.T("tui.remote.not_ready"), false)
		return
	}
	v.toast(i18n.Tf("tui.remote.fetching", label), true)
	var rep RemoteFetchReport
	v.async(func(ctx context.Context) (string, error) {
		var err error
		rep, err = rb.FetchRemote(ctx, id)
		return "", err
	}, func(string) {
		v.reload()
		actions := []ModalAction{}
		if rep.Journal > 0 {
			actions = append(actions, ModalAction{Label: i18n.Tf("tui.remote.replay_action", rep.Journal), Callback: func() { v.replayFlow(rb, id, label) }})
		}
		actions = append(actions,
			ModalAction{Label: i18n.T("tui.remote.attach"), Callback: func() { v.cfg.Backend.Attach(id, "") }},
			ModalAction{Label: i18n.T("tui.remote.close"), Callback: func() {}, Separator: true})
		v.shell.ShowScrollableModal(i18n.Tf("tui.remote.fetch_title", label), strings.Join(rep.Lines, "\n"), actions)
	})
}

// replayFlow shows the replay plan, asks a resolution for each conflict
// (conflict window), then applies after confirmation.
func (v *SessionsView) replayFlow(rb SessionRemote, id, label string) {
	var plan *ReplayView
	v.async(func(ctx context.Context) (string, error) {
		var err error
		plan, err = rb.PlanReplay(ctx, id)
		return "", err
	}, func(string) {
		if plan.Pending == 0 {
			v.applyReplay(rb, id, label, nil)
			return
		}
		v.resolveConflicts(rb, id, label, plan, 0, map[string]string{})
	})
}

func (v *SessionsView) resolveConflicts(rb SessionRemote, id, label string, plan *ReplayView, i int, res map[string]string) {
	for i < len(plan.Conflicts) && plan.Conflicts[i].Resolution != "" {
		i++
	}
	if i >= len(plan.Conflicts) {
		v.confirmReplay(rb, id, label, plan, res)
		return
	}
	c := plan.Conflicts[i]
	var b strings.Builder
	for _, f := range c.Fields {
		fmt.Fprintf(&b, "%-10s %s  |  %s\n", f.Field, i18n.Tf("tui.remote.conflict.local", f.Local), i18n.Tf("tui.remote.conflict.remote", f.Remote))
	}
	for _, n := range c.Notes {
		fmt.Fprintf(&b, "%s %s\n", i18n.T("tui.remote.conflict.note"), n)
	}
	choose := func(r string) func() {
		return func() {
			res[c.Ticket] = r
			plan.Conflicts[i].Resolution = r
			v.resolveConflicts(rb, id, label, plan, i+1, res)
		}
	}
	v.shell.ShowScrollableModal(i18n.Tf("tui.remote.conflict.title", c.Ticket, c.Title), b.String(), []ModalAction{
		{Label: i18n.T("tui.remote.conflict.keep_local"), Callback: choose(ResolutionKeepLocal)},
		{Label: i18n.T("tui.remote.conflict.apply_remote"), Callback: choose(ResolutionApplyRemote)},
		{Label: i18n.T("tui.remote.conflict.merge_notes"), Callback: choose(ResolutionMergeNotes)},
		{Label: i18n.T("tui.remote.conflict.later"), Callback: func() {}, Separator: true},
	})
}

func (v *SessionsView) confirmReplay(rb SessionRemote, id, label string, plan *ReplayView, res map[string]string) {
	v.shell.ShowScrollableModal(i18n.Tf("tui.remote.replay_title", label, plan.Pending), strings.Join(plan.Lines, "\n"), []ModalAction{
		{Label: i18n.Tf("tui.remote.replay_apply", plan.Pending), Callback: func() { v.applyReplay(rb, id, label, res) }},
		{Label: i18n.T("tui.remote.conflict.later"), Callback: func() {}, Separator: true},
	})
}

func (v *SessionsView) applyReplay(rb SessionRemote, id, label string, res map[string]string) {
	var out *ReplayView
	v.async(func(ctx context.Context) (string, error) {
		var err error
		out, err = rb.ApplyReplay(ctx, id, res)
		return "", err
	}, func(string) {
		v.reload()
		if out.Failed > 0 {
			v.shell.ShowScrollableModal(i18n.Tf("tui.remote.replay_failed", out.Failed), strings.Join(out.Lines, "\n"),
				[]ModalAction{{Label: i18n.T("tui.remote.close"), Callback: func() {}}})
			return
		}
		v.toast(i18n.Tf("tui.remote.replay_done", label), true)
	})
}
