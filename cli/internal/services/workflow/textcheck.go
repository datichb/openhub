package workflow

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/datichb/openhub/cli/internal/teamstate"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// TextCheck is the validation of a draft being edited (TUI editor): nothing
// is written.
type TextCheck struct {
	// Ref is the document reference (layer:id; empty id when unreadable).
	Ref wf.Ref
	// Resolution is the resolved workflow (nil when the text cannot be
	// resolved: syntax error, unknown parent…).
	Resolution *Resolution
	// Diagnostics are every finding (parse and validation), sorted.
	Diagnostics wf.Diagnostics
	// Locked are the fields the parent chain locks (`enforce`; "*" = all).
	Locked []string
	// Path is the draft file the text would be saved to.
	Path string
	// TeamBricks are the team catalogue bricks the workflow uses.
	TeamBricks []string
}

// Valid reports whether the text can be saved (no error).
func (t *TextCheck) Valid() bool { return t.Resolution != nil && !t.Diagnostics.HasErrors() }

// IsLocked reports whether a top-level field is locked by the parent chain.
func (t *TextCheck) IsLocked(field string) bool {
	return slices.Contains(t.Locked, field) || slices.Contains(t.Locked, wf.EnforceAll)
}

// CheckText validates yaml (and its own prompt template; nil = the current
// one) as the current member's draft in layer, with the member's other
// drafts, as SaveDraft does before writing.
func (s *Service) CheckText(ctx context.Context, c Context, layer wf.Layer, yaml, prompt []byte) (*TextCheck, error) {
	cat, err := s.load(ctx, c)
	if err != nil {
		return nil, err
	}
	return s.checkDraftText(cat, layer, yaml, prompt)
}

func (s *Service) checkDraftText(cat *catalog, layer wf.Layer, yaml, prompt []byte) (*TextCheck, error) {
	if layer == wf.LayerHub {
		return nil, ErrHubReadOnly
	}
	ts := cat.team
	if ts == nil {
		return nil, ErrNoTeamState
	}
	scope, err := ts.scopeOf(layer)
	if err != nil {
		return nil, err
	}
	out := &TextCheck{Ref: wf.Ref{Layer: layer}}
	probe, diags := wf.Parse(yaml, wf.Source{Layer: layer})
	if probe == nil || diags.HasErrors() {
		out.Diagnostics = diags
		return out, nil
	}
	id := probe.Spec.ID
	out.Ref.ID = id
	if idErr := teamstate.ValidWorkflowID(id); idErr != nil {
		diags = append(diags, wf.Diagnostic{Severity: wf.SeverityError, Code: "id_invalid", Path: "id", Message: idErr.Error()})
		out.Diagnostics = diags
		return out, nil //nolint:nilerr // an invalid id is a finding of the text
	}
	rel, err := teamstate.DraftRel(scope, ts.Member, id)
	if err != nil {
		return nil, err
	}
	out.Path = filepath.Join(ts.Repo.Path(), rel)
	doc, diags := wf.Parse(yaml, wf.Source{Layer: layer, Path: out.Path, Draft: true})
	if doc == nil || diags.HasErrors() {
		out.Diagnostics = diags
		return out, nil
	}
	_, ds := ts.Repo.LoadDrafts(cat.docs, ts.Member, ts.scopes()...)
	cat.diags = append(cat.diags, ds...)
	cat.docs.Put(doc)
	env := cat.env
	if prompt != nil {
		env.Prompts = promptOverride{path: out.Path, data: prompt, next: env.Prompts}
	}
	r, vdiags := wf.Check(cat.docs, doc.Ref(), nil, env)
	diags = append(diags, vdiags...)
	diags.Sort()
	out.Diagnostics = diags
	if r != nil {
		out.Resolution = &Resolution{Resolved: r, Diagnostics: diags, env: env}
		if cat.bricks != nil {
			out.TeamBricks = teamBricksUsed(r.Spec, env.Agents, cat.bricks.Team)
		}
	}
	if ext := doc.Spec.Extends; ext != "" {
		if pref, err := wf.ParseRef(ext); err == nil {
			if parent, _ := wf.ResolveSpec(cat.docs, pref, nil); parent != nil {
				out.Locked = parent.Spec.Enforce
			}
		}
	}
	return out, nil
}

// AgentIDs lists the agents of the brick catalogue of c (hub merged with
// the team bricks), for the editor's choices.
func (s *Service) AgentIDs(ctx context.Context, c Context) ([]string, error) {
	cat, err := s.load(ctx, c)
	if err != nil {
		return nil, err
	}
	if l, ok := cat.env.Agents.(interface{ AgentIDs() []string }); ok {
		return l.AgentIDs(), nil
	}
	return nil, nil
}

// SampleValue is an example value of an input type (prompt preview).
func SampleValue(in wf.Input) any {
	if in.Default != nil {
		return nil // the default applies
	}
	switch in.Type {
	case wf.InputBool:
		return true
	case wf.InputInt:
		return 3
	case wf.InputEnum:
		if len(in.Values) > 0 {
			return in.Values[0]
		}
	case wf.InputBeadsID:
		return "bd-42"
	case wf.InputBeadsIDs:
		return "bd-42,bd-43"
	case wf.InputBranch:
		return "feat/example"
	case wf.InputPath:
		return "docs/example.md"
	case wf.InputText:
		return "Example text with several words."
	}
	return "example"
}

// PromptPreview renders the initial prompt of r with example values for
// the inputs without default ("" when the workflow has no prompt).
func PromptPreview(r *Resolution, pc PromptContext) (string, error) {
	if r == nil || !r.HasPrompt() {
		return "", nil
	}
	inputs := map[string]any{}
	for _, k := range r.Spec.Inputs.Keys() {
		in, _ := r.Spec.Inputs.Get(k)
		if v := SampleValue(in); v != nil {
			if in.Type == wf.InputBeadsIDs {
				v = strings.Split(fmt.Sprint(v), ",")
			}
			inputs[k] = v
		}
	}
	if pc.Project == "" {
		pc.Project = "example"
	}
	if pc.Location == "" {
		pc.Location = "."
	}
	return r.WithInputs(inputs).RenderPrompt(pc)
}
