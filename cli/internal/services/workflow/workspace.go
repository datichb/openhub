package workflow

import (
	"context"
	"path/filepath"
	"slices"
	"strings"

	"github.com/datichb/openhub/cli/internal/teamstate"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// Workspace is everything the editable catalogue shows (TUI, `oh workflow
// list`), read in one load: the published workflows, the current member's
// drafts, the integrity warnings and the offline queue.
type Workspace struct {
	// Workflows is the catalogue (Catalog).
	Workflows []Summary `json:"workflows"`
	// Drafts are the current member's drafts, each validated with the
	// member's other drafts (Summary.Draft is set).
	Drafts []Summary `json:"drafts,omitempty"`
	// Integrity lists the skipped team-state files and the refused team
	// bricks (Integrity).
	Integrity wf.Diagnostics `json:"integrity,omitempty"`
	// Queue is the current member's operations waiting for the network.
	Queue []teamstate.QueuedOp `json:"queue,omitempty"`
	// TeamID and Member identify the team-state ("" without one).
	TeamID string `json:"team_id,omitempty"`
	Member string `json:"member,omitempty"`
	// Project is the project whose layer is loaded ("" = team only).
	Project string `json:"project,omitempty"`
	// Editable: a team-state is available (drafts can be created).
	Editable bool `json:"editable"`
}

// Workspace returns the catalogue of c with the current member's drafts,
// the integrity warnings and the offline queue.
func (s *Service) Workspace(ctx context.Context, c Context) (*Workspace, error) {
	cat, err := s.load(ctx, c)
	if err != nil {
		return nil, err
	}
	w := &Workspace{Workflows: s.catalogOf(cat)}
	for _, d := range cat.diags {
		if teamstate.IsIntegrityDiag(d) || isBrickDiag(d) {
			w.Integrity = append(w.Integrity, d)
		}
	}
	ts := cat.team
	if ts == nil {
		return w, nil
	}
	w.TeamID, w.Member, w.Project, w.Editable = ts.TeamID, ts.Member, ts.Project, true

	queued := map[wf.Ref]bool{}
	if ops, err := ts.Repo.Queue(); err == nil {
		for _, op := range ops {
			if op.Member != ts.Member {
				continue
			}
			w.Queue = append(w.Queue, op)
			queued[queueRef(op)] = true
		}
	}
	for i := range w.Workflows {
		if r, err := wf.ParseRef(w.Workflows[i].Ref); err == nil && queued[r] {
			w.Workflows[i].Queued = true
		}
	}

	refs, ds := ts.Repo.LoadDrafts(cat.docs, ts.Member, ts.scopes()...)
	loaded := map[string]bool{}
	for _, ref := range refs {
		if doc, ok := cat.docs.Lookup(ref); ok {
			loaded[doc.Source.Path] = true
		}
	}
	for _, ref := range refs {
		sum := s.summarize(cat, ref)
		sum.Draft, sum.ReadOnly, sum.Queued = true, false, queued[ref]
		if pub, ok := Find(w.Workflows, ref.ID); ok {
			sum.NewBricks = missing(sum.TeamBricks, pub.TeamBricks)
		} else {
			sum.NewBricks = sum.TeamBricks
		}
		w.Drafts = append(w.Drafts, sum)
	}
	// Drafts that could not be parsed are listed as invalid.
	bySource := map[string]*Summary{}
	for _, d := range ds {
		if d.Source == "" || loaded[d.Source] {
			continue
		}
		if teamstate.IsIntegrityDiag(d) {
			w.Integrity = append(w.Integrity, d)
			continue
		}
		sum, ok := bySource[d.Source]
		if !ok {
			id := strings.TrimSuffix(filepath.Base(d.Source), filepath.Ext(d.Source))
			layer := cat.sourceLayer(d.Source)
			sum = &Summary{ID: id, Ref: wf.Ref{Layer: layer, ID: id}.String(), Layer: layer, Label: id, Source: d.Source, Draft: true}
			bySource[d.Source] = sum
		}
		sum.Diagnostics = append(sum.Diagnostics, d)
	}
	for _, sum := range bySource {
		sum.count()
		w.Drafts = append(w.Drafts, *sum)
	}
	slices.SortFunc(w.Drafts, func(a, b Summary) int { return strings.Compare(a.Ref, b.Ref) })
	return w, nil
}

// queueRef is the document a queued operation applies to.
func queueRef(op teamstate.QueuedOp) wf.Ref {
	scope, _ := teamstate.ParseScope(op.Scope)
	return wf.Ref{Layer: scope.Layer(), ID: op.ID}
}

// QueuedOps returns the current member's operations waiting for the network
// (cheap: no git command), to decide whether FlushQueue is worth running.
func (s *Service) QueuedOps(ctx context.Context, c Context) ([]teamstate.QueuedOp, error) {
	ts, err := s.teamState(ctx, c)
	if err != nil || ts == nil {
		return nil, err
	}
	ops, err := ts.Repo.Queue()
	if err != nil {
		return nil, err
	}
	var out []teamstate.QueuedOp
	for _, op := range ops {
		if op.Member == ts.Member {
			out = append(out, op)
		}
	}
	return out, nil
}
