package launch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/runsvc"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// gitlabDeps is newDeps for a project whose tickets are on GitLab (tracker)
// and whose session has, or not, the GitLab MCP server.
func gitlabDeps(t *testing.T, tr TrackerInfo, mcp ...string) (Deps, *fakes) {
	t.Helper()
	d, f := newDeps(t)
	d.Tracker = func(context.Context, *domain.Project) TrackerInfo { return tr }
	d.BuildBundle = func(*domain.Project, *workflowsvc.Resolution, string) (*bundle.Bundle, []string, error) {
		b := &bundle.Bundle{}
		for _, m := range mcp {
			b.Spec.MCP = append(b.Spec.MCP, sessionspec.MCPServerDef{Name: m})
		}
		return b, nil, nil
	}
	return d, f
}

func warningCodes(p *Prepared) []string {
	var out []string
	for _, w := range p.Plan.Warnings {
		out = append(out, w.Code)
	}
	return out
}

// Bug 20: « #623 » was taken for a Beads ticket (bd show 623) because nothing
// told the session it was a GitLab issue, nor with which tool to read it.
func TestPromptNamesTheGitLabTicketOfTheRequest(t *testing.T) {
	d, f := gitlabDeps(t, TrackerInfo{Type: "gitlab", Project: "group/app"}, "gitlab")
	p, err := Prepare(t.Context(), d, Options{Workflow: "ask", Project: f.project,
		Inputs: map[string]string{"request": "implémente #623 et vois la !70"}})
	require.NoError(t, err)

	prompt := p.Plan.Sessions[0].Prompt
	assert.Contains(t, prompt, "tracker=gitlab project=group/app mcp=true")
	assert.Contains(t, prompt, workflowsvc.GitLabRefsTitle)
	assert.Contains(t, prompt, "- `#623` : outil `gitlab_get_issue` (`project_id` = `group/app`, `iid` = 623)")
	assert.Contains(t, prompt, "- `!70` : outil `gitlab_get_mr` (`project_id` = `group/app`, `mr_iid` = 70)")
	assert.NotContains(t, prompt, "n'est pas activé", "the session has the MCP")
	assert.NotContains(t, warningCodes(p), "gitlab_mcp_off")
}

func TestGitLabURLsNameTheirOwnProject(t *testing.T) {
	d, f := gitlabDeps(t, TrackerInfo{Type: "gitlab", Project: "group/app"}, "gitlab")
	p, err := Prepare(t.Context(), d, Options{Workflow: "ask", Project: f.project,
		Inputs: map[string]string{"request": "voir https://gitlab.example.com/other/lib/-/issues/9"}})
	require.NoError(t, err)
	assert.Contains(t, p.Plan.Sessions[0].Prompt, "- `https://gitlab.example.com/other/lib/-/issues/9` : outil `gitlab_get_issue` (`project_id` = `other/lib`, `iid` = 9)")
}

// The ticket is GitLab but the session has no GitLab MCP: say it at launch.
func TestLaunchWarnsWhenAGitLabTicketCannotBeRead(t *testing.T) {
	d, f := gitlabDeps(t, TrackerInfo{Type: "gitlab", Project: "group/app"})
	p, err := Prepare(t.Context(), d, Options{Workflow: "ask", Project: f.project,
		Inputs: map[string]string{"request": "implémente #623"}})
	require.NoError(t, err)

	assert.Contains(t, p.Plan.Warnings, runsvc.Warning{Code: "gitlab_mcp_off", Args: []any{"#623"}})
	assert.Contains(t, p.Plan.Sessions[0].Prompt, "mcp=false")
	assert.Contains(t, p.Plan.Sessions[0].Prompt, "Le MCP GitLab n'est pas activé dans cette session", "the session is told it cannot read #623")
}

// Without a ticket in the request, the tracker alone warns only a workflow
// whose prompt tells the agents how to read tickets.
func TestLaunchWarnsOfAGitLabTrackerWithoutMCPOnlyWhenTicketsAreRead(t *testing.T) {
	d, f := gitlabDeps(t, TrackerInfo{Type: "gitlab", Project: "group/app"})
	p, err := Prepare(t.Context(), d, Options{Workflow: "ask", Project: f.project, Inputs: map[string]string{"request": "ajoute un export"}})
	require.NoError(t, err)
	assert.Contains(t, p.Plan.Warnings, runsvc.Warning{Code: "gitlab_mcp_off_tracker"})

	p, err = Prepare(t.Context(), d, Options{Workflow: "quick", Project: f.project})
	require.NoError(t, err)
	assert.NotContains(t, warningCodes(p), "gitlab_mcp_off_tracker", "quick does not read tickets from the tracker")
	assert.NotContains(t, warningCodes(p), "gitlab_mcp_off")
}

func TestShortReferencesNeedAGitLabTracker(t *testing.T) {
	d, f := gitlabDeps(t, TrackerInfo{}, "gitlab")
	p, err := Prepare(t.Context(), d, Options{Workflow: "ask", Project: f.project,
		Inputs: map[string]string{"request": "ticket #623 et https://gitlab.example.com/g/a/-/issues/4"}})
	require.NoError(t, err)

	prompt := p.Plan.Sessions[0].Prompt
	assert.NotContains(t, prompt, "`#623`", "#623 is GitLab's syntax only where the tracker is GitLab")
	assert.Contains(t, prompt, "- `https://gitlab.example.com/g/a/-/issues/4` : outil `gitlab_get_issue`", "an issue URL says what it is")
	assert.Contains(t, prompt, "tracker= ")
}

func TestBeadsIDsAreNotGitLabReferences(t *testing.T) {
	d, f := gitlabDeps(t, TrackerInfo{Type: "gitlab", Project: "group/app"}, "gitlab")
	p, err := Prepare(t.Context(), d, Options{Workflow: "ask", Project: f.project,
		Inputs: map[string]string{"request": "reprends bd-12 et proj-a1b.3"}})
	require.NoError(t, err)
	assert.NotContains(t, p.Plan.Sessions[0].Prompt, workflowsvc.GitLabRefsTitle)
}

func TestNoTrackerHookMeansNoGitLabContext(t *testing.T) {
	d, f := newDeps(t)
	p, err := Prepare(t.Context(), d, Options{Workflow: "ask", Project: f.project, Inputs: map[string]string{"request": "#623"}})
	require.NoError(t, err)
	assert.Contains(t, p.Plan.Sessions[0].Prompt, "tracker= ")
	assert.NotContains(t, p.Plan.Sessions[0].Prompt, workflowsvc.GitLabRefsTitle)
}
