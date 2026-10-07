package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/provider"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// TUI wiring of the workflow editor (P2-T14): the WorkflowService check
// (resolution, origins, locks, diagnostics, prompt preview) and the bundle
// preview, mapped to the view.

// openWorkflowEditor pushes the editor of a draft (text t, in layer).
func openWorkflowEditor(c workflowsvc.Context, layer workflow.Layer, t *workflowsvc.Text, unsaved, promptOwn bool) {
	sh := tuiShell
	if sh == nil {
		return
	}
	ref := t.Ref
	ref.Layer = layer
	var agents []string
	go func() {
		ctx := sh.Context()
		agents, _ = newWorkflowService(ctx).AgentIDs(ctx, c)
		sh.App().QueueUpdateDraw(func() {
			sh.PushView(views.NewWorkflowEditorView(views.WorkflowEditorConfig{
				Ref: ref.String(), Layer: layer, YAML: t.YAML, Prompt: t.Prompt, Unsaved: unsaved, PromptOwn: promptOwn,
				Lang: i18n.Locale(), Agents: agents,
				Check: func(ctx context.Context, yaml, prompt []byte) (*views.EditorCheck, error) {
					return editorCheck(ctx, c, layer, yaml, prompt)
				},
				Save: func(ctx context.Context, yaml, prompt []byte) (*views.EditorSaved, error) {
					d, err := newWorkflowService(ctx).SaveDraft(ctx, c, workflowsvc.DraftInput{Layer: layer, YAML: yaml, Prompt: prompt})
					if err != nil {
						return nil, errors.New(tuiWorkflowError(err))
					}
					return &views.EditorSaved{Ref: d.Ref.String(), Pushed: d.Pushed}, nil
				},
				External: func(name string, content []byte, line int) ([]byte, error) {
					var out []byte
					err := sh.SuspendAndExec(func() (err error) {
						out, _, err = editTextAt(name, content, line)
						return err
					})
					return out, err
				},
				OnSaved: func(*views.EditorSaved) { refreshCatalog() },
			}))
		})
	}()
}

// editorCheck checks the edited text and builds the bundle preview.
func editorCheck(ctx context.Context, c workflowsvc.Context, layer workflow.Layer, yaml, prompt []byte) (*views.EditorCheck, error) {
	svc := newWorkflowService(ctx)
	chk, err := svc.CheckText(ctx, c, layer, yaml, prompt)
	if err != nil {
		return nil, errors.New(tuiWorkflowError(err))
	}
	out := &views.EditorCheck{Locked: chk.Locked}
	for _, d := range chk.Diagnostics {
		ed := views.EditorDiag{Error: d.Severity == workflow.SeverityError, Path: d.Path, Message: d.Message, Hint: d.Hint}
		if d.Source == "" || d.Source == chk.Path {
			ed.Line = d.Pos.Line
		}
		if d.Source != "" && d.Source != chk.Path {
			ed.Message += " (" + shortSource(d.Source) + ")"
		}
		out.Diagnostics = append(out.Diagnostics, ed)
	}
	res := chk.Resolution
	if res == nil {
		return out, nil
	}
	out.Spec, out.Origins = res.Spec, res.Origins
	if p, err := workflowsvc.PromptPreview(res, workflowsvc.PromptContext{Lang: i18n.Locale()}); err != nil {
		out.PromptErr = err.Error()
	} else {
		out.Prompt = p
	}
	if chk.Valid() {
		out.Bundle, out.BundleErr = editorBundlePreview(ctx, c, res, chk.TeamBricks)
	}
	return out, nil
}

func shortSource(p string) string {
	if i := strings.Index(p, "/workflows/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// editorBundlePreview builds the draft's bundle in a temporary directory
// and summarises it. Without an active project, the project MCP servers
// and models are unknown: listed servers are « project dependent ».
func editorBundlePreview(ctx context.Context, c workflowsvc.Context, res *workflowsvc.Resolution, teamBricks []string) (lines []string, failure string) {
	a := TryApp()
	if a == nil || a.Config == nil {
		return nil, ""
	}
	project, _ := projectByID(ctx, c.ProjectID)
	prov := a.Config.LLM.DefaultProvider
	if project != nil {
		prov = provider.ResolveProvider("", project.Provider, a.Config.LLM.DefaultProvider)
	}
	dir, err := os.MkdirTemp("", "oh-editor-bundle-")
	if err != nil {
		return nil, err.Error()
	}
	defer os.RemoveAll(dir)
	b, missing, err := buildWorkflowBundleIn(a, project, res, prov, dir)
	if err != nil {
		return nil, err.Error()
	}
	r := bundle.Show(b)
	if project == nil {
		lines = append(lines, "⚠ "+i18n.T("tui.editor.preview.no_project"))
	}
	var agents []string
	for _, ag := range r.Agents {
		s := ag.ID + " (" + ag.Mode + ")"
		if ag.Entry {
			s = "▶ " + s
		}
		agents = append(agents, s)
	}
	lines = append(lines,
		i18n.Tf("tui.editor.preview.agents", len(r.Agents), strings.Join(agents, " · ")),
		i18n.Tf("tui.editor.preview.skills", len(r.Skills), r.Budget.Skills),
		i18n.Tf("tui.editor.preview.budget", r.Budget.Initial, r.Budget.EntryAgent, r.Budget.SkillCatalog),
		i18n.Tf("tui.editor.preview.depth", r.MaxDepth, len(r.Permissions)),
		i18n.Tf("tui.editor.preview.isolation", string(r.Isolation), onOff(r.StrictIsolation), onOff(r.CodeMode)))
	mcp := strings.Join(r.MCP, ", ")
	if mcp == "" {
		mcp = "—"
	}
	lines = append(lines, i18n.Tf("tui.editor.preview.mcp", mcp))
	for _, m := range missing {
		if project == nil {
			lines = append(lines, "  · "+i18n.Tf("tui.editor.preview.mcp_project", m))
		} else {
			lines = append(lines, "  ⚠ "+i18n.Tf("tui.editor.preview.mcp_missing", m))
		}
	}
	if len(r.Plugins) > 0 {
		lines = append(lines, i18n.Tf("tui.editor.preview.plugins", strings.Join(r.Plugins, ", ")))
	}
	if r.DefaultModel != "" {
		lines = append(lines, i18n.Tf("tui.editor.preview.model", r.DefaultModel))
	}
	if len(teamBricks) > 0 {
		lines = append(lines, i18n.Tf("tui.editor.preview.team_bricks", strings.Join(teamBricks, ", ")))
	}
	return lines, ""
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// tuiOpenEditor opens the editor on the draft of ref (or on a new draft
// started from its published document).
func tuiOpenEditor(ref string) {
	c := catalogContext()
	layer, id := splitRef(ref)
	var t *workflowsvc.Text
	tuiAsync(func(ctx context.Context) (err error) {
		t, err = newWorkflowService(ctx).EditText(ctx, c, layer, id)
		return err
	}, func() { openWorkflowEditor(c, layer, t, false, false) })
}

// projectByID returns the project id ("" or unknown: nil).
func projectByID(ctx context.Context, id string) (*domain.Project, error) {
	a := TryApp()
	if id == "" || a == nil || a.Projects == nil {
		return nil, nil
	}
	p, err := a.Projects.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("project %s: %w", id, err)
	}
	return p, nil
}
