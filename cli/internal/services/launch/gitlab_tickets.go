package launch

import (
	"context"
	"strings"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitlabapi"
	"github.com/datichb/openhub/cli/internal/runsvc"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Where the tickets of a project live decides how a reference is read:
// « #623 » is an issue of the GitLab of the project, never a Beads ticket
// (Beads ids look like proj-a1b). The prompts tell the session how to read
// each kind, and the launch warns when the session cannot.

// TrackerInfo is the tracker of the tickets of a project as far as oh knows
// it: the team tracker, else the forge of the origin remote.
type TrackerInfo struct {
	// Type is "gitlab" or "jira"; empty when oh knows none (Beads only).
	Type string
	// Project is the project of the tracker: the GitLab path or ID.
	Project string
}

// GitLab reports whether the tickets of the project are on GitLab.
func (t TrackerInfo) GitLab() bool { return t.Type == "gitlab" }

const (
	// gitlabMCPServer is the name of the GitLab MCP server in a bundle.
	gitlabMCPServer = "gitlab"
	// mcpToolIssue and mcpToolMR are the tools that read an issue and a
	// merge request, as the model sees them (<server>_<tool>).
	mcpToolIssue = gitlabMCPServer + "_get_issue"
	mcpToolMR    = gitlabMCPServer + "_get_mr"
	// readsTicketsMarker tells that a prompt template gives the session the
	// way to read a ticket by its source.
	readsTicketsMarker = ".oh.gitlab_mcp"
)

func (d Deps) trackerOf(ctx context.Context, project *domain.Project) TrackerInfo {
	if d.Tracker == nil || project == nil {
		return TrackerInfo{}
	}
	return d.Tracker(ctx, project)
}

// hasMCP reports whether the bundle carries the MCP server name.
func hasMCP(b *bundle.Bundle, name string) bool {
	if b == nil {
		return false
	}
	for _, m := range b.Spec.MCP {
		if m.Name == name {
			return true
		}
	}
	return false
}

// GitLabRefs returns the GitLab issues and merge requests the free-text
// inputs name. A URL is always a reference; #<iid> and !<iid> only where the
// tracker of the project is GitLab. A reference without project in its URL
// belongs to the project of the tracker.
func GitLabRefs(sp *workflow.Spec, inputs map[string]any, tr TrackerInfo) []workflowsvc.GitLabRef {
	var out []workflowsvc.GitLabRef
	seen := map[string]bool{}
	for _, k := range sp.Inputs.Keys() {
		in, _ := sp.Inputs.Get(k)
		if in.Type != workflow.InputText && in.Type != workflow.InputString {
			continue
		}
		text, _ := inputs[k].(string)
		for _, r := range gitlabapi.FindRefs(text) {
			if r.Host == "" && !tr.GitLab() {
				continue
			}
			if seen[r.Text] {
				continue
			}
			seen[r.Text] = true
			ref := workflowsvc.GitLabRef{Text: r.Text, Kind: string(r.Kind), IID: r.IID, Project: r.Project, Tool: mcpToolIssue}
			if r.Kind == gitlabapi.RefMR {
				ref.Tool = mcpToolMR
			}
			if ref.Project == "" {
				ref.Project = tr.Project
			}
			out = append(out, ref)
		}
	}
	return out
}

// gitlabMCPWarnings tells what a session without the GitLab MCP server
// cannot do: read a GitLab ticket the launch names, or, for a workflow whose
// prompt tells how to read tickets, the tickets of a GitLab tracker.
func (p *Prepared) gitlabMCPWarnings() []runsvc.Warning {
	if hasMCP(p.Bundle, gitlabMCPServer) {
		return nil
	}
	var refs []string
	seen := map[string]bool{}
	for _, rs := range p.gitlabRefs {
		for _, r := range rs {
			if !seen[r.Text] {
				seen[r.Text] = true
				refs = append(refs, r.Text)
			}
		}
	}
	if len(refs) > 0 {
		return []runsvc.Warning{{Code: "gitlab_mcp_off", Args: []any{strings.Join(refs, ", ")}}}
	}
	if !p.tracker.GitLab() {
		return nil
	}
	if tmpl, err := p.Resolution.PromptTemplate(); err == nil && strings.Contains(tmpl, readsTicketsMarker) {
		return []runsvc.Warning{{Code: "gitlab_mcp_off_tracker"}}
	}
	return nil
}
