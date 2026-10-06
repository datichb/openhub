package cmd

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	remotesvc "github.com/datichb/openhub/cli/internal/services/remote"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Remote sessions in the TUI (P5-T18): rows decorated with the remote
// status, fetch and replay through the remote service.

var _ views.SessionRemote = (*tuiSessions)(nil)

// remoteTrack throttles the pipeline checks of the TUI.
var remoteTrack struct {
	sync.Mutex
	last time.Time
}

const remoteTrackEvery = 30 * time.Second

// decorateRemote adds the remote status of the rows, and checks the running
// pipelines (at most every 30 s).
func (t *tuiSessions) decorateRemote(ctx context.Context, rows []views.SessionRow) []views.SessionRow {
	if store == nil {
		return rows
	}
	rs := sqlite.NewRemoteStore(store)
	refs, err := rs.ListRemote(ctx)
	if err != nil || len(refs) == 0 {
		return rows
	}
	pending := false
	for _, r := range refs {
		pending = pending || remotesvc.Pending(r)
	}
	remoteTrack.Lock()
	due := pending && time.Since(remoteTrack.last) > remoteTrackEvery
	if due {
		remoteTrack.last = time.Now()
	}
	remoteTrack.Unlock()
	if due {
		tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		_, _ = newRemoteReturnService(tctx, t.a).Track(tctx)
		cancel()
		if fresh, err := rs.ListRemote(ctx); err == nil {
			refs = fresh
		}
	}
	for i := range rows {
		ref, ok := refs[rows[i].ID]
		if !ok {
			continue
		}
		rows[i].Remote = string(ref.Status)
		rows[i].RemoteDetail = remoteDetail(ref)
		if remotesvc.Pending(ref) {
			rows[i].StateLabel = rows[i].RemoteDetail
		}
	}
	return rows
}

// remoteDetail is the line of a remote session: pipeline, MR, journal.
func remoteDetail(ref domain.RemoteRef) string {
	parts := []string{i18n.Tf("tui.remote.pipeline", ref.Pipeline, i18n.T("tui.remote.status."+string(ref.Status)))}
	if ref.MRURL != "" {
		parts = append(parts, i18n.T("tui.remote.mr_ready"))
	}
	if ref.Error != "" && ref.Status == domain.RemoteFailed {
		parts = append(parts, ref.Error)
	}
	if ref.Status == domain.RemoteFetched {
		parts = append(parts, i18n.T("tui.remote.journal_to_replay"))
	}
	return strings.Join(parts, " · ")
}

// FetchRemote implements views.SessionRemote.
func (t *tuiSessions) FetchRemote(ctx context.Context, id string) (views.RemoteFetchReport, error) {
	res, err := newRemoteReturnService(ctx, t.a).Fetch(ctx, id, remotesvc.FetchOptions{})
	if err != nil {
		return views.RemoteFetchReport{}, err
	}
	s := res.Summary
	lines := []string{"✔ " + i18n.Tf("cmd.session.fetch.summary", i18n.T("cmd.session.fetch.outcome."+s.Outcome), s.Cost, len(res.Journal))}
	if s.Error != "" {
		lines = append(lines, "⚠ "+s.Error)
	}
	if s.MRURL != "" {
		lines = append(lines, "✔ "+i18n.Tf("cmd.session.fetch.mr", s.MRURL))
	}
	if res.Imported {
		lines = append(lines, "✔ "+i18n.Tf("tui.remote.imported", res.Location))
	}
	if s.Deferred != nil {
		lines = append(lines, "⏸ "+i18n.Tf("cmd.session.fetch.deferred", s.Deferred.ID, firstNonEmpty(s.Deferred.Label, s.Deferred.ID)))
	}
	if s.Question != nil {
		lines = append(lines, "? "+i18n.Tf("cmd.session.fetch.question", s.Question.Label))
	}
	return views.RemoteFetchReport{Lines: lines, Journal: len(res.Journal)}, nil
}

func toReplayView(p *remotesvc.ReplayPlan) *views.ReplayView {
	out := &views.ReplayView{Pending: p.Pending()}
	for _, it := range p.Items {
		icon := map[string]string{remotesvc.ItemApplied: "✔", remotesvc.ItemRefused: "✗", remotesvc.ItemFailed: "✗", remotesvc.ItemSkipped: "◌"}[it.State]
		if icon == "" {
			icon = "○"
		}
		line := fmt.Sprintf("%s bd %s — %s", icon, strings.Join(it.Argv, " "), i18n.T("cmd.session.resolve.state."+it.State))
		if it.Reason != "" {
			line += " (" + it.Reason + ")"
		}
		if it.State == remotesvc.ItemFailed {
			out.Failed++
		}
		out.Lines = append(out.Lines, line)
	}
	for _, c := range p.Conflicts {
		vc := views.ReplayConflict{Ticket: c.Ticket, Title: c.Title, Notes: c.Notes, Resolution: string(c.Resolution)}
		for _, f := range c.Fields {
			vc.Fields = append(vc.Fields, views.ReplayField{Field: f.Field, Local: f.Local, Remote: f.Remote})
		}
		out.Conflicts = append(out.Conflicts, vc)
	}
	return out
}

// PlanReplay implements views.SessionRemote.
func (t *tuiSessions) PlanReplay(ctx context.Context, id string) (*views.ReplayView, error) {
	p, err := newRemoteReturnService(ctx, t.a).PlanReplay(ctx, id)
	if err != nil {
		return nil, err
	}
	return toReplayView(p), nil
}

// ApplyReplay implements views.SessionRemote.
func (t *tuiSessions) ApplyReplay(ctx context.Context, id string, res map[string]string) (*views.ReplayView, error) {
	r := map[string]remotesvc.Resolution{}
	for k, v := range res {
		r[k] = remotesvc.Resolution(v)
	}
	p, err := newRemoteReturnService(ctx, t.a).ApplyReplay(ctx, id, r)
	if err != nil {
		return nil, err
	}
	return toReplayView(p), nil
}
