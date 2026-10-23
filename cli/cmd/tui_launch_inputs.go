package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/launch"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/textfmt"
	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// Inputs of the launch form (modernisation tui-launch): the workflow an
// alias opens, what oh computes for an input with `from:`, the checks of a
// path or a branch, the notes of the inputs.

// tuiLaunchAlias returns the workflow to open for req and the alias asked
// ("" when req.WorkflowID is a workflow): the target chosen in the omnibar,
// else the one oh run picks from the inputs given (`with`: feature with
// tickets → implement-spec).
func tuiLaunchAlias(ctx context.Context, wsvc *workflowsvc.Service, projectID string, req tuiLaunchRequest) (id, alias string) {
	if req.Draft {
		return req.WorkflowID, ""
	}
	if req.AliasTarget != "" {
		return req.AliasTarget, req.WorkflowID
	}
	target, ok := wsvc.ResolveAlias(ctx, workflowsvc.Context{ProjectID: projectID}, req.WorkflowID, func(input string) bool {
		return strings.TrimSpace(req.Prefill[input]) != "" || (len(req.Tickets) > 0 && launch.AliasTicketInput(ctx, wsvc, projectID, req.WorkflowID, input))
	})
	if !ok {
		return req.WorkflowID, ""
	}
	return target, req.WorkflowID
}

// aliasRoute is the input that routes alias to sp (`with`; "" = default
// target, or not an alias of sp).
func aliasRoute(sp *workflow.Spec, alias string) string {
	if alias == "" {
		return ""
	}
	for _, a := range sp.Aliases {
		if a.ID == alias {
			return a.With
		}
	}
	return ""
}

// fromPreviewMax bounds a value shown in the preview of a computed input.
const fromPreviewMax = 48

// tuiFromPreview tells what oh will compute for an input with `from:`:
// the session read (« session.output(spec.spec) → ses_41c0 (docs/…) »), the
// sessions named by an input, or, for the sources of the forge, that it is
// read at launch. It never fails: the launch computes the value again.
func tuiFromPreview(a *app.App, project *domain.Project, parent string, sp *workflow.Spec) func(ctx context.Context, k string, values map[string]string) string {
	return func(ctx context.Context, k string, values map[string]string) string {
		in, ok := sp.Inputs.Get(k)
		if !ok || in.From == "" {
			return ""
		}
		f, ok := workflow.ParseSessionFrom(in.From)
		if !ok {
			return i18n.Tf("tui.launch.input.from_at_launch", in.From)
		}
		c := workflowsvc.Context{ProjectID: project.ID, FromSession: parent, Workflow: sp.ID}
		switch f.Source {
		case workflow.SourceSessionOutput:
			s, err := pickSession(ctx, a, c, f.Workflow, func(s *domain.Session) bool {
				if outputText(s.Outputs[f.Output]) == "" {
					return false
				}
				for out, want := range in.FromEquals {
					if outputText(s.Outputs[out]) != want {
						return false
					}
				}
				return true
			})
			if err != nil {
				return i18n.Tf("tui.launch.input.from_none", in.From)
			}
			return i18n.Tf("tui.launch.input.from_value", in.From, s.ID, textfmt.Clip(outputText(s.Outputs[f.Output]), fromPreviewMax))
		case workflow.SourceSessionTraces:
			if v := strings.TrimSpace(values[f.Input]); v != "" {
				return i18n.Tf("tui.launch.input.from_traces", in.From, v)
			}
			f.Workflow = ""
		}
		s, err := pickSession(ctx, a, c, f.Workflow, func(s *domain.Session) bool { return s.State != domain.RunActive })
		if err != nil {
			return i18n.Tf("tui.launch.input.from_none", in.From)
		}
		return i18n.Tf("tui.launch.input.from_session", in.From, s.ID)
	}
}

// tuiCheckInput checks a path (relative to the project) or a branch (local
// or of a remote) typed in the form: a warning, never a refusal (a path or
// a branch may be created by the session).
func tuiCheckInput(dir string) func(ctx context.Context, in workflow.Input, value string) string {
	return func(ctx context.Context, in workflow.Input, value string) string {
		switch in.Type {
		case workflow.InputPath:
			p := value
			if !filepath.IsAbs(p) {
				p = filepath.Join(dir, p)
			}
			if _, err := os.Stat(p); err != nil {
				return i18n.T("tui.launch.input.path_missing")
			}
		case workflow.InputBranch:
			if !worktree.IsGitRepo(dir) || worktree.BranchExists(dir, value) {
				return ""
			}
			out, err := gitutil.Command(ctx, dir, "for-each-ref", "--count=1", "--format=%(refname)", "refs/remotes/*/"+value).Output()
			if err != nil || strings.TrimSpace(string(out)) == "" {
				return i18n.Tf("tui.launch.input.branch_missing", value)
			}
		}
		return ""
	}
}

// tuiInputNotes are the notes of the inputs: the fan-out input while the
// fan-out is not enabled on this machine.
func tuiInputNotes(a *app.App, sp *workflow.Spec) map[string]string {
	if sp.Fanout == nil || sp.Fanout.Input == "" || (a.Config != nil && a.Config.Fanout.Experimental) {
		return nil
	}
	return map[string]string{sp.Fanout.Input: i18n.T("tui.launch.input.fanout_off")}
}
