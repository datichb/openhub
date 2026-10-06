package cmd

import (
	"context"
	"errors"
	"io"
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/prefsvc"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// TUI wiring of the workflow catalogue (P1-T26, P2-T13): the WorkflowService
// workspace (catalogue, drafts, integrity, queue) mapped to the view, pins in
// the active context, editing actions calling the service.

const tuiCatalogProblems = 3

// newWorkflowCatalogView builds the catalogue view.
func newWorkflowCatalogView(start *tuiStart) *views.WorkflowCatalogView {
	return views.NewWorkflowCatalogView(views.WorkflowCatalogConfig{
		Load: func(ctx context.Context) (views.CatalogData, error) { return loadCatalogData(ctx, start) },
		Launch: func(id string) {
			openLaunchForm(start.a, tuiLaunchRequest{WorkflowID: id, ProjectID: activeScope().ProjectID})
		},
		TogglePin: func(id string) { start.togglePin(activeScope(), views.StartEntry{ID: id}) },
		New:       tuiNewWorkflow,
		Edit:      func(e views.CatalogEntry) { tuiEditWorkflow(e.Ref) },
		TestDraft: func(e views.CatalogEntry) {
			openLaunchForm(start.a, tuiLaunchRequest{WorkflowID: e.ID, ProjectID: activeScope().ProjectID, Draft: true})
		},
		Publish: func(e views.CatalogEntry) { openPublishView(e.Ref) },
		Diff:    func(e views.CatalogEntry) { showDraftDiff(e.Ref) },
		History: func(e views.CatalogEntry) { openHistoryView(e.Ref) },
		Archive: tuiArchiveWorkflow,
		Discard: tuiDiscardDraft,
	})
}

// openWorkflowCatalog shows the catalogue.
func openWorkflowCatalog() {
	if tuiShell != nil {
		tuiShell.NavigateTo("workflows")
	}
}

// activeScope is the « Démarrer » scope of the current TUI context.
func activeScope() views.StartScope {
	if tuiShell == nil {
		return views.StartScope{}
	}
	switch tuiShell.Mode() {
	case views.ModeProject:
		if p := tuiShell.ActiveProject(); p != nil {
			return views.StartScope{ProjectID: p.ID}
		}
	case views.ModeTeam:
		if t := tuiShell.ActiveTeam(); t != nil {
			return views.StartScope{TeamID: t.ID}
		}
	}
	return views.StartScope{}
}

// catalogContext is the WorkflowService context of the TUI.
func catalogContext() workflowsvc.Context {
	s := activeScope()
	return workflowsvc.Context{ProjectID: s.ProjectID, TeamID: s.TeamID}
}

func loadCatalogData(ctx context.Context, start *tuiStart) (views.CatalogData, error) {
	scope := activeScope()
	ws, err := newWorkflowService(ctx).Workspace(ctx, catalogContext())
	if err != nil {
		return views.CatalogData{}, err
	}
	pinned := map[string]bool{}
	if entries, err := start.prefs.Start(ctx, prefsvc.Context{ProjectID: scope.ProjectID, TeamID: scope.TeamID}, nil); err == nil {
		for _, p := range entries.Pinned {
			pinned[p.WorkflowID] = true
		}
	}
	data := views.CatalogData{Team: ws.TeamID, Member: ws.Member, Editable: ws.Editable, Queue: len(ws.Queue)}
	if ws.Project != "" {
		data.Project = ws.Project
		if tuiShell != nil {
			if p := tuiShell.ActiveProject(); p != nil && p.ID == ws.Project {
				data.Project = p.Name
			}
		}
	}
	drafts := map[string]bool{}
	for _, d := range ws.Drafts {
		drafts[d.Ref] = true
	}
	for _, s := range ws.Workflows {
		e := catalogEntry(s, pinned[s.ID])
		e.HasDraft = drafts[s.Ref]
		data.Entries = append(data.Entries, e)
	}
	for _, s := range ws.Drafts {
		data.Entries = append(data.Entries, catalogEntry(s, false))
	}
	for _, d := range ws.Integrity {
		data.Integrity = append(data.Integrity, views.CatalogIntegrity{Message: d.Message, Source: d.Source})
	}
	return data, nil
}

func catalogEntry(s workflowsvc.Summary, pinned bool) views.CatalogEntry {
	e := views.CatalogEntry{ID: s.ID, Ref: s.Ref, Layer: string(s.Layer), Version: s.Version, Label: s.Label, Desc: s.Description,
		Risk: string(s.Risk), Entry: s.EntryAgent, Chain: s.Chain, Valid: s.Valid, Pinned: pinned,
		ReadOnly: s.ReadOnly, Draft: s.Draft, Queued: s.Queued, Errors: s.Errors, Warnings: s.Warnings,
		NewBricks: s.NewBricks, TeamBricks: s.TeamBricks, Findings: diagLines(s.Diagnostics)}
	for _, r := range s.Runtimes {
		e.Runtimes = append(e.Runtimes, string(r))
	}
	for _, in := range s.Inputs {
		label := in.ID + " (" + string(in.Type) + ")"
		if in.Required {
			label += " *"
		}
		e.Inputs = append(e.Inputs, label)
	}
	for _, d := range s.Diagnostics.Errors() {
		if len(e.Problems) == tuiCatalogProblems {
			break
		}
		e.Problems = append(e.Problems, d.Message)
	}
	return e
}

// diagLines renders diagnostics as « ✗ path: message » lines.
func diagLines(ds workflow.Diagnostics) []string {
	var out []string
	for _, d := range ds {
		icon := "⚠"
		if d.Severity == workflow.SeverityError {
			icon = "✗"
		}
		line := icon + " "
		if d.Path != "" {
			line += d.Path + ": "
		}
		line += d.Message
		if d.Source != "" {
			loc := filepath.Base(d.Source)
			if p := d.Pos.String(); p != "" {
				loc += ":" + p
			}
			line += " (" + loc + ")"
		}
		out = append(out, line)
	}
	return out
}

// refreshCatalog reloads the catalogue when it is shown.
func refreshCatalog() {
	if tuiShell != nil {
		tuiShell.RemountIf("workflows")
	}
}

// tuiAsync runs fn off the event loop and then on the loop with its error
// (toasted when not nil).
func tuiAsync(fn func(ctx context.Context) error, then func()) {
	sh := tuiShell
	if sh == nil {
		return
	}
	go func() {
		err := fn(sh.Context())
		sh.App().QueueUpdateDraw(func() {
			if err != nil {
				sh.ShowToast(tuiWorkflowError(err), shell.ToastError)
				return
			}
			if then != nil {
				then()
			}
		})
	}()
}

// tuiWorkflowError is the message of a WorkflowService error.
func tuiWorkflowError(err error) string {
	var invalid *workflowsvc.InvalidError
	if errors.As(err, &invalid) {
		return i18n.Tf("teamstate.workflow.cli.invalid", invalid.Ref, len(invalid.Diagnostics.Errors()))
	}
	return workflowEditError(io.Discard, err).Error()
}

// tuiNewWorkflow creates a draft from the « new » form: its starting text
// opened in $EDITOR, then validated and saved.
func tuiNewWorkflow(n views.CatalogNew) {
	c := catalogContext()
	in := workflowsvc.NewDraft{ID: n.ID, Layer: workflow.Layer(n.Layer)}
	switch n.Kind {
	case views.CatalogNewExtends:
		in.Extends = n.Source
	case views.CatalogNewCopy:
		in.Copy = n.Source
	}
	var t *workflowsvc.Text
	tuiAsync(func(ctx context.Context) (err error) {
		t, err = newWorkflowService(ctx).NewDraftText(ctx, c, in)
		return err
	}, func() { editDraftInEditor(c, in.Layer, t.YAML, t.Prompt) })
}

// tuiEditWorkflow edits the draft of ref (or starts one from the published
// document) in $EDITOR. Replaced by the editor view in lot 3; kept for `y`.
func tuiEditWorkflow(ref string) {
	c := catalogContext()
	layer, id := splitRef(ref)
	var t *workflowsvc.Text
	tuiAsync(func(ctx context.Context) (err error) {
		t, err = newWorkflowService(ctx).EditText(ctx, c, layer, id)
		return err
	}, func() { editDraftInEditor(c, layer, t.YAML, t.Prompt) })
}

// editDraftInEditor opens yaml in $EDITOR (terminal suspended), then saves
// it as a draft; validation errors offer to reopen the editor.
func editDraftInEditor(c workflowsvc.Context, layer workflow.Layer, yaml, prompt []byte) {
	sh := tuiShell
	if sh == nil {
		return
	}
	var edited []byte
	err := sh.SuspendAndExec(func() (err error) {
		edited, _, err = editText("workflow.yaml", yaml)
		return err
	})
	if err != nil {
		sh.ShowToast(err.Error(), shell.ToastError)
		return
	}
	go func() {
		ctx := sh.Context()
		d, err := newWorkflowService(ctx).SaveDraft(ctx, c, workflowsvc.DraftInput{Layer: layer, YAML: edited, Prompt: prompt})
		sh.App().QueueUpdateDraw(func() {
			var invalid *workflowsvc.InvalidError
			switch {
			case errors.As(err, &invalid):
				body := ""
				for _, l := range diagLines(invalid.Diagnostics) {
					body += l + "\n"
				}
				sh.ShowScrollableModal(i18n.Tf("tui.catalog.edit.invalid_title", invalid.Ref), body, []views.ModalAction{
					{Label: i18n.T("tui.catalog.edit.reopen"), Callback: func() { editDraftInEditor(c, layer, edited, prompt) }},
					{Label: i18n.T("tui.catalog.edit.drop_changes"), Callback: func() {}, Separator: true},
				})
			case err != nil:
				sh.ShowToast(tuiWorkflowError(err), shell.ToastError)
			default:
				msg := i18n.Tf("tui.catalog.edit.saved", d.Ref.String())
				if !d.Pushed {
					msg = i18n.Tf("tui.catalog.edit.saved_local", d.Ref.String())
				}
				sh.ShowToast(msg, shell.ToastSuccess)
				refreshCatalog()
			}
		})
	}()
}

// tuiArchiveWorkflow archives a published workflow.
func tuiArchiveWorkflow(e views.CatalogEntry, message string) {
	c := catalogContext()
	var p *workflowsvc.Publication
	tuiAsync(func(ctx context.Context) (err error) {
		p, err = newWorkflowService(ctx).Archive(ctx, c, e.Ref, message)
		return err
	}, func() {
		msg := i18n.Tf("tui.catalog.edit.archived", e.Ref)
		if p != nil && p.Queued {
			msg = i18n.Tf("tui.publish.queued", e.Ref)
		}
		tuiShell.ShowToast(msg, shell.ToastSuccess)
		refreshCatalog()
	})
}

// tuiDiscardDraft deletes the current member's draft.
func tuiDiscardDraft(e views.CatalogEntry) {
	c := catalogContext()
	layer, id := splitRef(e.Ref)
	tuiAsync(func(ctx context.Context) error {
		return newWorkflowService(ctx).DiscardDraft(ctx, c, layer, id)
	}, func() {
		tuiShell.ShowToast(i18n.Tf("tui.catalog.edit.discarded", e.Ref), shell.ToastSuccess)
		refreshCatalog()
	})
}

// showDraftDiff shows the diff of the draft of ref against its published
// version, with the impact summary.
func showDraftDiff(ref string) {
	c := catalogContext()
	var p *views.PublishPreview
	tuiAsync(func(ctx context.Context) (err error) {
		p, err = loadPublishPreview(ctx, c, ref)
		return err
	}, func() {
		tuiShell.ShowScrollableModal(i18n.Tf("tui.publish.diff_title", p.Ref, p.Next), views.PublishText(p, nil), []views.ModalAction{
			{Label: i18n.T("tui.publish.publish"), Callback: func() { openPublishView(ref) }},
			{Label: i18n.T("tui.catalog.edit.close"), Callback: func() {}, Separator: true},
		})
	})
}
