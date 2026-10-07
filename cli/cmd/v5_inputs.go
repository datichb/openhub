package cmd

import (
	"context"
	"errors"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// inputSources are the sources of computed inputs (`from:`, workflow
// InputSources) on this machine: the merge requests of the project forge
// (GitLab today) and the team space of the project.
func inputSources(a *app.App) map[string]workflowsvc.InputSource {
	out := gitlabInputSources(a)
	out["ticket.brief"] = func(ctx context.Context, c workflowsvc.Context, ticketID string) (string, error) {
		return ticketBrief(ctx, a, c.ProjectID, ticketID)
	}
	return out
}

// ticketBrief reads the takeover brief of a ticket (the enriched one first)
// in the team space of the project.
func ticketBrief(ctx context.Context, a *app.App, projectID, ticketID string) (string, error) {
	p, err := a.Projects.Get(ctx, projectID)
	if err != nil {
		return "", err
	}
	var team *config.TeamConfig
	if p.TeamID != nil {
		team = a.Config.FindTeam(*p.TeamID)
	}
	if team == nil {
		active := a.Config.ActiveTeam()
		team = &active
	}
	if team == nil || team.StatePath == "" {
		return "", errors.New(i18n.T("cmd.workflow.input.no_team_space"))
	}
	content, err := teamstate.NewRepo(team.StateRepo, team.StatePath).ReadBrief(p.Name, ticketID)
	if errors.Is(err, teamstate.ErrBriefNotFound) {
		return "", errors.New(i18n.Tf("cmd.workflow.input.no_brief", p.Name, ticketID))
	}
	return content, err
}
