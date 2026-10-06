package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Shared helpers of the editing commands (oh workflow new|edit|diff|publish|
// history|restore|archive, v5 phase 2, P2-T12).

// addWorkflowContextFlags registers --project (team-state of the project).
func addWorkflowContextFlags(c *cobra.Command) {
	c.Flags().StringP("project", "p", "", i18n.T("teamstate.workflow.cli.flag_project"))
	c.Flags().String("team", "", i18n.T("teamstate.workflow.cli.flag_team"))
}

// workflowCmdContext returns the WorkflowService context of an editing
// command: --project, else the project of the current directory, else the
// active team.
func workflowCmdContext(cmd *cobra.Command) (workflowsvc.Context, error) {
	a := TryApp()
	if a == nil {
		return workflowsvc.Context{}, nil
	}
	ref, _ := cmd.Flags().GetString("project")
	if team, _ := cmd.Flags().GetString("team"); team != "" {
		if ref != "" {
			return workflowsvc.Context{}, errors.New(i18n.T("teamstate.workflow.cli.team_and_project"))
		}
		if a.Config.FindTeam(team) == nil {
			return workflowsvc.Context{}, errors.New(i18n.Tf("cmd.team.promote.unknown_team", team))
		}
		return workflowsvc.Context{TeamID: team}, nil
	}
	p, err := bundleProject(ctxOf(cmd), a, ref)
	if err != nil || p == nil {
		return workflowsvc.Context{}, err
	}
	if tc := config.ResolveTeamForProject(a.Config, p); !tc.Enabled && !tc.Solo {
		if ref != "" {
			return workflowsvc.Context{}, errors.New(i18n.Tf("teamstate.workflow.cli.project_without_team", p.Name))
		}
		return workflowsvc.Context{}, nil // project without team: the active team
	}
	return workflowsvc.Context{ProjectID: p.ID}, nil
}

// workflowEditError explains the errors of the editing service.
func workflowEditError(w io.Writer, err error) error {
	var invalid *workflowsvc.InvalidError
	var loose *workflowsvc.DraftLoosensError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &invalid):
		printDiagnostics(w, invalid.Diagnostics)
		return errors.New(i18n.Tf("teamstate.workflow.cli.invalid", invalid.Ref, len(invalid.Diagnostics.Errors())))
	case errors.As(err, &loose):
		return err
	case errors.Is(err, workflowsvc.ErrNoTeamState):
		return errors.New(i18n.T("teamstate.workflow.cli.no_team_state"))
	case errors.Is(err, workflowsvc.ErrHubReadOnly):
		return errors.New(i18n.T("teamstate.workflow.cli.hub_read_only"))
	case errors.Is(err, workflowsvc.ErrNoDraft):
		return errors.New(i18n.Tf("teamstate.workflow.cli.no_draft", err.Error()))
	case errors.Is(err, workflowsvc.ErrNotPublished), errors.Is(err, workflowsvc.ErrNotFound):
		return errors.New(i18n.Tf("teamstate.workflow.cli.not_published", err.Error()))
	case errors.Is(err, workflowsvc.ErrUnknownVersion):
		return errors.New(i18n.Tf("teamstate.workflow.cli.unknown_version", err.Error()))
	case errors.Is(err, workflowsvc.ErrUnknownWorkflow):
		return errors.New(i18n.Tf("cmd.workflow.show.unknown", strings.TrimPrefix(err.Error(), workflowsvc.ErrUnknownWorkflow.Error()+": ")))
	case errors.Is(err, workflowsvc.ErrSharedPrompt):
		return errors.New(i18n.Tf("teamstate.workflow.cli.shared_prompt", err.Error()))
	case errors.Is(err, teamstate.ErrPushRejected):
		return errors.New(i18n.T("teamstate.workflow.cli.push_rejected"))
	}
	return err
}

// layerFlag reads --layer (team | project).
func layerFlag(cmd *cobra.Command) (workflow.Layer, error) {
	v, _ := cmd.Flags().GetString("layer")
	l := workflow.Layer(v)
	if l != workflow.LayerTeam && l != workflow.LayerProject {
		return "", errors.New(i18n.Tf("teamstate.workflow.cli.bad_layer", v))
	}
	return l, nil
}

// splitRef returns the layer and id of "<layer>:<id>" (layer "" for a bare id).
func splitRef(s string) (layer workflow.Layer, id string) {
	if r, err := workflow.ParseRef(s); err == nil {
		return r.Layer, r.ID
	}
	return "", s
}

// interactive reports whether the command may ask questions.
func interactive() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

// confirm asks a yes/no question (true without a terminal or with --yes).
func confirm(cmd *cobra.Command, question string) (bool, error) {
	if yes, _ := cmd.Flags().GetBool("yes"); yes || !interactive() {
		return true, nil
	}
	return launcher.NewCLIUI(cmd.ErrOrStderr()).Confirm(question)
}

// editorArgv returns the editor command ($VISUAL, $EDITOR, else vi).
func editorArgv() []string {
	for _, env := range []string{"VISUAL", "EDITOR"} {
		if v := strings.Fields(os.Getenv(env)); len(v) > 0 {
			return v
		}
	}
	return []string{"vi"}
}

// editText opens content in the editor (file named name) and returns the
// edited text.
func editText(name string, content []byte) (edited []byte, path string, err error) {
	dir, err := os.MkdirTemp("", "oh-workflow-")
	if err != nil {
		return nil, "", err
	}
	path = filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return nil, "", err
	}
	argv := append(editorArgv(), path)
	c := exec.Command(argv[0], argv[1:]...) //nolint:gosec // the user's editor
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return nil, path, fmt.Errorf("%s: %w", strings.Join(argv, " "), err)
	}
	edited, err = os.ReadFile(path)
	return edited, path, err
}

// draftEdit is the content to save as a draft.
type draftEdit struct {
	Layer  workflow.Layer
	YAML   []byte
	Prompt []byte // nil = keep
	// EditYAML / EditPrompt open the editor on the document / the prompt.
	EditYAML, EditPrompt bool
	// Name of the temporary YAML file (<id>.yaml).
	Name string
}

// saveDraftEdit opens the editor when asked, saves the draft and, on
// validation errors, offers to edit again (interactive) or keeps the edited
// file and fails.
func saveDraftEdit(cmd *cobra.Command, svc *workflowsvc.Service, c workflowsvc.Context, e draftEdit) (*workflowsvc.Draft, error) {
	out := cmd.OutOrStdout()
	original := append([]byte(nil), e.YAML...)
	for {
		var path string
		if e.EditYAML {
			data, p, err := editText(e.Name, e.YAML)
			if err != nil {
				return nil, err
			}
			e.YAML, path = data, p
		}
		if e.EditPrompt {
			body := e.Prompt
			if body == nil {
				body = []byte{}
			}
			data, _, err := editText(strings.TrimSuffix(e.Name, ".yaml")+".md.tmpl", body)
			if err != nil {
				return nil, err
			}
			e.Prompt = data
		}
		if e.EditYAML && !e.EditPrompt && bytes.Equal(e.YAML, original) && e.Prompt == nil {
			fmt.Fprintln(out, i18n.T("teamstate.workflow.cli.unchanged"))
		}
		d, err := svc.SaveDraft(cmd.Context(), c, workflowsvc.DraftInput{Layer: e.Layer, YAML: e.YAML, Prompt: e.Prompt})
		var invalid *workflowsvc.InvalidError
		if err == nil || !errors.As(err, &invalid) || (!e.EditYAML && !e.EditPrompt) || !interactive() {
			if err != nil && path != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), theme.Subtitle.Render(i18n.Tf("teamstate.workflow.cli.kept", path)))
			}
			return d, err
		}
		printDiagnostics(cmd.ErrOrStderr(), invalid.Diagnostics)
		again, cerr := launcher.NewCLIUI(cmd.ErrOrStderr()).Confirm(i18n.T("teamstate.workflow.cli.edit_again"))
		if cerr != nil || !again {
			if path != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), theme.Subtitle.Render(i18n.Tf("teamstate.workflow.cli.kept", path)))
			}
			return nil, err
		}
	}
}

// printDraftSaved reports a saved draft.
func printDraftSaved(w io.Writer, d *workflowsvc.Draft) {
	printDiagnostics(w, d.Diagnostics)
	fmt.Fprintf(w, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("teamstate.workflow.cli.draft_saved", d.Ref.String(), d.Path))
	if !d.Pushed {
		fmt.Fprintf(w, "  %s\n", theme.WarningStyle.Render(i18n.Tf("teamstate.workflow.cli.draft_not_pushed", d.PushError)))
	}
	fmt.Fprintf(w, "  %s\n", theme.Subtitle.Render(i18n.Tf("teamstate.workflow.cli.draft_next", d.Ref.ID)))
}

// printImpact prints an impact summary.
func printImpact(w io.Writer, r workflowsvc.ImpactReport) {
	if len(r.Items) == 0 && len(r.NewBricks) == 0 {
		fmt.Fprintf(w, "  %s\n", theme.Subtitle.Render(i18n.T("teamstate.workflow.cli.impact_none")))
		return
	}
	fmt.Fprintln(w, theme.Bold.Render(i18n.T("teamstate.workflow.cli.impact_title")))
	for _, it := range r.Items {
		icon := theme.Subtitle.Render("•")
		if it.Level == workflowsvc.ImpactWiden {
			icon = theme.WarningStyle.Render(theme.IconWarning)
		}
		fmt.Fprintf(w, "  %s %s\n", icon, it.Message)
	}
	for _, b := range r.NewBricks {
		fmt.Fprintf(w, "  %s %s\n", theme.SuccessStyle.Render("✦"), i18n.Tf("teamstate.workflow.cli.new_brick", b))
	}
}

// printPublication reports a publication, restore or archive.
func printPublication(w io.Writer, p *workflowsvc.Publication) {
	if p.Queued {
		fmt.Fprintf(w, "%s %s\n", theme.WarningStyle.Render(theme.IconWarning), i18n.Tf("teamstate.workflow.cli.queued", p.Kind, p.Ref.String()))
		return
	}
	printDiagnostics(w, p.Diagnostics)
	key := map[string]string{workflowsvc.OpPublish: "teamstate.workflow.cli.published", workflowsvc.OpRestore: "teamstate.workflow.cli.restored", workflowsvc.OpArchive: "teamstate.workflow.cli.archived"}[p.Kind]
	fmt.Fprintf(w, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf(key, p.Ref.String(), p.Version))
	if p.Kind != workflowsvc.OpArchive {
		printImpact(w, p.Impact)
	}
}

// ctxOf is the command context (tests may run RunE directly).
func ctxOf(cmd *cobra.Command) context.Context {
	if c := cmd.Context(); c != nil {
		return c
	}
	return context.Background()
}

// readWorkflowFile reads a document from a file ("-" = standard input).
func readWorkflowFile(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

// editTarget finds what `edit`/`diff` work on: an explicit layer (prefix or
// --layer), else the project layer when the context has one and it holds
// the workflow (draft or published), else the team layer.
func editTarget(cmd *cobra.Command, svc *workflowsvc.Service, c workflowsvc.Context, arg string) (*workflowsvc.Text, workflow.Layer, error) {
	layer, id := splitRef(arg)
	if layer == "" {
		if v, _ := cmd.Flags().GetString("layer"); v != "" {
			layer = workflow.Layer(v)
		}
	}
	if layer == workflow.LayerHub {
		return nil, layer, workflowsvc.ErrHubReadOnly
	}
	if layer != "" {
		t, err := svc.EditText(ctxOf(cmd), c, layer, id)
		return t, layer, err
	}
	var last error
	for _, l := range []workflow.Layer{workflow.LayerProject, workflow.LayerTeam} {
		t, err := svc.EditText(ctxOf(cmd), c, l, id)
		if err == nil {
			return t, l, nil
		}
		last = err
	}
	return nil, workflow.LayerTeam, last
}
