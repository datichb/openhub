package workflow

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// Drafts (P2-T03): a member's unpublished version of a team or project
// workflow, validated when saved, launchable locally with --draft.

var (
	// ErrHubReadOnly is returned when saving a hub-layer document.
	ErrHubReadOnly = errors.New("hub workflows are read-only")
	// ErrNoDraft is returned by ResolveDraft when the chain has no draft.
	ErrNoDraft = errors.New("no draft")
	// ErrDraftRemote is returned for a draft launch on a remote runtime.
	ErrDraftRemote = errors.New("drafts run locally only")
)

// DraftLoosensError refuses a draft launch that would widen the published
// version (security may only be hardened by a draft under test).
type DraftLoosensError struct {
	Ref   string
	Items []ImpactItem
}

func (e *DraftLoosensError) Error() string {
	lines := make([]string, len(e.Items))
	for i, it := range e.Items {
		lines[i] = "  - " + it.Message
	}
	return i18n.Tf("teamstate.workflow.draft.loosens", e.Ref) + "\n" + strings.Join(lines, "\n")
}

// DraftInput is a draft to save.
type DraftInput struct {
	// Layer is team or project.
	Layer wf.Layer
	// YAML is the oh/v1 document; its id names the draft.
	YAML []byte
	// Prompt is the draft's own prompt template: nil keeps the current one,
	// empty removes it.
	Prompt []byte
}

// Draft is a saved draft.
type Draft struct {
	Ref    wf.Ref `json:"ref"`
	Member string `json:"member"`
	Path   string `json:"path"`
	// Diagnostics are the warnings of the validation.
	Diagnostics wf.Diagnostics `json:"diagnostics,omitempty"`
	// Pushed is false when the commit could not be pushed (offline): it is
	// pushed with the next team-state synchronization.
	Pushed    bool   `json:"pushed"`
	PushError string `json:"push_error,omitempty"`
}

// SaveDraft validates and saves a draft of the current member. A draft with
// errors is not saved (*InvalidError with every finding).
func (s *Service) SaveDraft(ctx context.Context, c Context, in DraftInput) (*Draft, error) {
	if in.Layer == wf.LayerHub {
		return nil, ErrHubReadOnly
	}
	cat, err := s.load(ctx, c)
	if err != nil {
		return nil, err
	}
	chk, err := s.checkDraftText(cat, in.Layer, in.YAML, in.Prompt)
	if err != nil {
		return nil, err
	}
	if !chk.Valid() {
		ref := string(in.Layer)
		if chk.Ref.ID != "" {
			ref = chk.Ref.String()
		}
		return nil, &InvalidError{Ref: ref, Diagnostics: chk.Diagnostics}
	}
	ts := cat.team
	scope, _ := ts.scopeOf(in.Layer)
	id, path, diags := chk.Ref.ID, chk.Path, chk.Diagnostics
	files, err := ts.Repo.WriteDraftLocal(scope, ts.Member, id, in.YAML, in.Prompt)
	if err != nil {
		return nil, err
	}
	d := &Draft{Ref: chk.Ref, Member: ts.Member, Path: path, Diagnostics: diags, Pushed: true}
	if err := ts.Repo.CommitAndPush(ctx, fmt.Sprintf("workflow: draft %s:%s by %s", scope, id, ts.Member), files...); err != nil {
		d.Pushed, d.PushError = false, err.Error()
	}
	return d, nil
}

// Drafts lists the drafts of the current member.
func (s *Service) Drafts(ctx context.Context, c Context) ([]teamstate.DraftInfo, error) {
	ts, err := s.teamState(ctx, c)
	if err != nil {
		return nil, err
	}
	if ts == nil {
		return nil, ErrNoTeamState
	}
	return ts.Repo.ListDrafts(ts.Member, ts.scopes()...)
}

// DiscardDraft deletes the current member's draft of id in layer.
func (s *Service) DiscardDraft(ctx context.Context, c Context, layer wf.Layer, id string) error {
	ts, err := s.teamState(ctx, c)
	if err != nil {
		return err
	}
	if ts == nil {
		return ErrNoTeamState
	}
	scope, err := ts.scopeOf(layer)
	if err != nil {
		return err
	}
	removed, err := ts.Repo.DeleteDraftLocal(scope, ts.Member, id)
	if err != nil {
		return err
	}
	if len(removed) == 0 {
		return fmt.Errorf("%w: %s:%s", ErrNoDraft, layer, id)
	}
	return ts.Repo.CommitAndPush(ctx, fmt.Sprintf("workflow: discard draft %s:%s by %s", scope, id, ts.Member), removed...)
}

// ResolveDraft resolves id with the current member's drafts in place of the
// published documents (`oh run --draft`). The draft is refused on a remote
// runtime and when it widens the published version (security may not be
// loosened by a draft under test).
func (s *Service) ResolveDraft(ctx context.Context, c Context, id string, opts ResolveOpts) (*Resolution, error) {
	if opts.Session != nil && opts.Session.Runtime == wf.RuntimeRemote {
		return nil, ErrDraftRemote
	}
	cat, err := s.load(ctx, c)
	if err != nil {
		return nil, err
	}
	ts := cat.team
	if ts == nil {
		return nil, ErrNoTeamState
	}
	drafts, ds := ts.Repo.LoadDrafts(cat.docs, ts.Member, ts.scopes()...)
	cat.diags = append(cat.diags, ds...)
	ref, ok := s.lookup(cat, id, opts.Layer)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownWorkflow, id)
	}
	r, diags := wf.Check(cat.docs, ref, opts.Session, cat.env)
	if r == nil {
		return nil, &InvalidError{Ref: ref.String(), Diagnostics: diags}
	}
	if !slices.ContainsFunc(r.Chain, func(c wf.Ref) bool { return slices.Contains(drafts, c) }) {
		return nil, fmt.Errorf("%w: %s", ErrNoDraft, ref)
	}
	diags.Sort()
	res := &Resolution{Resolved: r, Diagnostics: diags, env: cat.env}
	if diags.HasErrors() {
		return res, &InvalidError{Ref: ref.String(), Diagnostics: diags}
	}
	if pub, err := s.Resolve(ctx, c, ref.String(), ResolveOpts{}); err == nil {
		rep := impactOf(pub.Spec, r.Spec, cat.env.Agents)
		if w := rep.Widenings(); len(w) > 0 {
			return res, &DraftLoosensError{Ref: ref.String(), Items: w}
		}
	}
	return res, nil
}

// promptOverride serves the prompt of a document being saved before it is
// written to disk.
type promptOverride struct {
	path string
	data []byte
	next wf.PromptSource
}

func (p promptOverride) ReadPrompt(origin wf.Origin, path string) ([]byte, error) {
	if origin.Source == p.path && len(p.data) > 0 {
		return p.data, nil
	}
	if p.next == nil {
		return nil, fmt.Errorf("no prompt source for %s", path)
	}
	return p.next.ReadPrompt(origin, path)
}
