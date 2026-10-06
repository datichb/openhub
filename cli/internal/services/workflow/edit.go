package workflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/teamstate"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// Editing helpers for the CLI and the TUI (lot 2.C / 2.D): the text to edit,
// the preview of a publication.

// Text is a workflow document and its prompt template.
type Text struct {
	Ref wf.Ref `json:"ref"`
	// YAML is the document as written in its file.
	YAML []byte `json:"-"`
	// Prompt is the template the document uses (its own copy for a draft or
	// a history version; the referenced template otherwise; nil = none).
	Prompt []byte `json:"-"`
	// Path is the file of the document.
	Path string `json:"path"`
	// Draft: the text is the current member's draft.
	Draft bool `json:"draft,omitempty"`
}

// ErrNotFound is returned for a document absent from the requested place.
var ErrNotFound = errors.New("workflow document not found")

// DocumentText returns the text of ref ("<layer>:<id>"; bare id: most
// specific layer), drafts excluded.
func (s *Service) DocumentText(ctx context.Context, c Context, id string) (*Text, error) {
	cat, err := s.load(ctx, c)
	if err != nil {
		return nil, err
	}
	ref, ok := s.lookup(cat, id, "")
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownWorkflow, id)
	}
	doc, _ := cat.docs.Lookup(ref)
	return docText(doc, cat.env.Prompts)
}

// EditText returns what `edit` opens for layer:id: the current member's
// draft when there is one, else the published document of that layer.
func (s *Service) EditText(ctx context.Context, c Context, layer wf.Layer, id string) (*Text, error) {
	cat, err := s.load(ctx, c)
	if err != nil {
		return nil, err
	}
	ts := cat.team
	if ts == nil {
		return nil, ErrNoTeamState
	}
	scope, err := ts.scopeOf(layer)
	if err != nil {
		return nil, err
	}
	if rel, err := teamstate.DraftRel(scope, ts.Member, id); err == nil {
		path := filepath.Join(ts.Repo.Path(), rel)
		if data, err := os.ReadFile(path); err == nil {
			doc, _ := wf.Parse(data, wf.Source{Layer: layer, Path: path, Draft: true})
			t := &Text{Ref: wf.Ref{Layer: layer, ID: id}, YAML: data, Path: path, Draft: true}
			if doc != nil {
				if pt, err := docText(doc, cat.env.Prompts); err == nil {
					t.Prompt = pt.Prompt
				}
			}
			return t, nil
		}
	}
	doc, ok := cat.docs.Lookup(wf.Ref{Layer: layer, ID: id})
	if !ok {
		return nil, fmt.Errorf("%w: %s:%s", ErrNotFound, layer, id)
	}
	return docText(doc, cat.env.Prompts)
}

func docText(doc *wf.Document, prompts wf.PromptSource) (*Text, error) {
	t := &Text{Ref: doc.Ref(), Path: doc.Source.Path, Draft: doc.Source.Draft}
	data, err := os.ReadFile(doc.Source.Path)
	if err != nil {
		return nil, err
	}
	t.YAML = data
	if p := doc.Spec.Prompt; p != nil && p.Template != "" && prompts != nil {
		o := wf.Origin{Layer: doc.Source.Layer, ID: doc.Spec.ID, Source: doc.Source.Path, Draft: doc.Source.Draft}
		if pdata, err := prompts.ReadPrompt(o, p.Template); err == nil {
			t.Prompt = pdata
		}
	}
	return t, nil
}

// Preview is what a publication of a draft would do.
type Preview struct {
	Ref         wf.Ref `json:"ref"`
	NextVersion int    `json:"next_version"`
	// Published is the text of the current published version (nil: new).
	Published *Text `json:"published,omitempty"`
	// Draft is the text of the draft.
	Draft       *Text          `json:"draft"`
	Impact      ImpactReport   `json:"impact"`
	Diagnostics wf.Diagnostics `json:"diagnostics,omitempty"`
}

// Preview compares the current member's draft draftID with the published
// version (the publication itself revalidates after a pull).
func (s *Service) Preview(ctx context.Context, c Context, draftID string) (*Preview, error) {
	ts, err := s.requireTeam(ctx, c)
	if err != nil {
		return nil, err
	}
	scope, id, err := s.draftScope(ts, draftID)
	if err != nil {
		return nil, err
	}
	ref := wf.Ref{Layer: scope.Layer(), ID: id}
	cat, err := s.load(ctx, c)
	if err != nil {
		return nil, err
	}
	p := &Preview{Ref: ref}
	if doc, ok := cat.docs.Lookup(ref); ok {
		if p.Published, err = docText(doc, cat.env.Prompts); err != nil {
			return nil, err
		}
	}
	pub, _ := wf.Check(cat.docs, ref, nil, cat.env)

	if _, ds := ts.Repo.LoadDrafts(cat.docs, ts.Member, scope); len(ds) > 0 {
		cat.diags = append(cat.diags, ds...)
	}
	doc, ok := cat.docs.Lookup(ref)
	if !ok || !doc.Source.Draft {
		return nil, fmt.Errorf("%w: %s", ErrNoDraft, ref)
	}
	if p.Draft, err = docText(doc, cat.env.Prompts); err != nil {
		return nil, err
	}
	r, diags := wf.Check(cat.docs, ref, nil, cat.env)
	diags.Sort()
	p.Diagnostics = diags
	if r == nil || diags.HasErrors() {
		return p, &InvalidError{Ref: ref.String(), Diagnostics: diags}
	}
	var from *wf.Spec
	if pub != nil && p.Published != nil {
		from = pub.Spec
	}
	p.Impact = impactOf(from, r.Spec, cat.env.Agents)
	if from != nil && samePrompt(from.Prompt, r.Spec.Prompt) && string(p.Published.Prompt) != string(p.Draft.Prompt) {
		p.Impact.add(ImpactInfo, "prompt_changed", "prompt")
	}
	if cat.bricks != nil {
		p.Impact.NewBricks = missing(teamBricksUsed(r.Spec, cat.env.Agents, cat.bricks.Team), teamBricksUsed(from, cat.env.Agents, cat.bricks.Team))
	}

	lock, err := ts.Repo.ReadWorkflowLock()
	if err != nil {
		return nil, err
	}
	cur, _ := lock.Get(scope, id)
	p.NextVersion = cur.Version
	if hist, err := ts.Repo.ListHistory(scope, id); err == nil {
		for _, h := range hist {
			p.NextVersion = max(p.NextVersion, h.Version)
		}
	}
	p.NextVersion++
	return p, nil
}

// VersionText returns the text of a published or history version of id.
func (s *Service) VersionText(ctx context.Context, c Context, id string, version int) (*Text, error) {
	versions, err := s.History(ctx, c, id)
	if err != nil {
		return nil, err
	}
	for _, v := range versions {
		if v.Version != version {
			continue
		}
		data, err := os.ReadFile(v.Path)
		if err != nil {
			return nil, err
		}
		t := &Text{Ref: v.Ref, YAML: data, Path: v.Path}
		if own := filepath.Join(filepath.Dir(v.Path), fmt.Sprintf("%d%s", version, teamstate.OwnPromptSuffix)); !v.Current {
			t.Prompt, _ = os.ReadFile(own)
		}
		return t, nil
	}
	return nil, fmt.Errorf("%w: %s v%d", ErrUnknownVersion, id, version)
}
