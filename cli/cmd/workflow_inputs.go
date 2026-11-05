package cmd

import (
	"context"
	"errors"
	"os/exec"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// inputSources are the sources of computed inputs (`from:`, workflow
// InputSources) on this machine: the merge requests of the project forge
// (GitLab today) and the team space of the project.
func inputSources(a *app.App, note func(string)) map[string]workflowsvc.InputSource {
	out := map[string]workflowsvc.InputSource{}
	for k, fn := range gitlabInputSources(a) {
		out[k] = gitlabOnly(fn, func(ctx context.Context, projectID string) string { return githubRemoteOf(ctx, a, projectID) })
	}
	for k, fn := range sessionInputSources(a, note) {
		out[k] = fn
	}
	out["ticket.brief"] = func(ctx context.Context, c workflowsvc.Context, ticketID string) (string, error) {
		return ticketBrief(ctx, a, c.ProjectID, ticketID)
	}
	return out
}

// gitlabOnly makes a source of merge request inputs (GitLab) fail with a clear
// message on a project whose forge is GitHub (F-07), instead of a token or
// project error: remoteOf returns the GitHub remote of the project, "" when
// it is not one.
func gitlabOnly(fn workflowsvc.InputSource, remoteOf func(ctx context.Context, projectID string) string) workflowsvc.InputSource {
	return func(ctx context.Context, c workflowsvc.Context, ref string) (string, error) {
		if remote := remoteOf(ctx, c.ProjectID); remote != "" {
			return "", errors.New(i18n.Tf("cmd.inputs.github.mr_gitlab_only", remote))
		}
		return fn(ctx, c, ref)
	}
}

// githubRemoteOf returns the origin remote of a project (host/path) when it
// is on GitHub; "" otherwise or when it cannot be read.
func githubRemoteOf(ctx context.Context, a *app.App, projectID string) string {
	p, err := a.Projects.Get(ctx, projectID)
	if err != nil || p == nil {
		return ""
	}
	return githubRemote(ctx, p.Path)
}

// githubRemote returns "host/path" when the origin remote of the repository at
// dir is on GitHub (github.com or a GitHub Enterprise host).
func githubRemote(ctx context.Context, dir string) string {
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	gr, err := config.ParseGitRemote(string(out))
	if err != nil || (gr.Host != "github.com" && !strings.HasPrefix(gr.Host, "github.")) {
		return ""
	}
	return gr.Host + "/" + gr.Path
}

// ticketBrief reads the takeover brief of a ticket (the enriched one first)
// in the team space of the project.
func ticketBrief(ctx context.Context, a *app.App, projectID, ticketID string) (string, error) {
	p, err := a.Projects.Get(ctx, projectID)
	if err != nil {
		return "", err
	}
	// The brief is in the team space of the project (solo spaces included);
	// a project without one follows the team ResolveTeam gives.
	team, err := ticketBriefTeam(ctx, a, p)
	if err != nil {
		return "", err
	}
	if team.StatePath == "" {
		return "", errors.New(i18n.T("cmd.workflow.input.no_team_space"))
	}
	content, err := teamstate.NewRepo(team.StateRepo, team.StatePath).ReadBrief(ctx, p.Name, ticketID)
	if errors.Is(err, teamstate.ErrBriefNotFound) {
		return "", errors.New(i18n.Tf("cmd.workflow.input.no_brief", p.Name, ticketID))
	}
	return content, err
}

// ticketBriefTeam is the team space the briefs of project p are in.
func ticketBriefTeam(ctx context.Context, a *app.App, p *domain.Project) (config.ResolvedTeamConfig, error) {
	if p.TeamID != nil {
		if t := a.Config.FindTeam(*p.TeamID); t != nil {
			return t.Resolved(), nil
		}
	}
	t, err := ResolveTeam(ctx, a)
	return t.ResolvedTeamConfig, err
}
