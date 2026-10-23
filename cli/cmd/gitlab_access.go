package cmd

import (
	"context"
	"os"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitlabapi"
	teamsvc "github.com/datichb/openhub/cli/internal/services/team"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// gitlabAccess is the GitLab that the commands on merge requests call (oh mr
// create, oh review feedback, the review inputs of the workflows): its URL
// and the token that serves it.
type gitlabAccess struct {
	URL   string
	Token teamsvc.GitLabTokenResult
}

// Client builds the API client; a refusal of the token names its source, its
// key and the server.
func (g gitlabAccess) Client() *gitlabapi.Client {
	origin := g.Token.Origin()
	return gitlabapi.NewClient(g.URL, g.Token.Token).WithAuthError(func(status int) error {
		return &tracker.TokenRejectedError{Origin: origin, Status: status}
	})
}

// resolveGitLabAccess resolves the GitLab of the MCP of a project and the
// token for it, with the unique cascade of the team service. Without a token
// declared for that server it fails with a *tracker.MissingTokenError that
// names the server, the sources read and how to declare the key.
func resolveGitLabAccess(ctx context.Context, a *app.App, project *domain.Project) (gitlabAccess, error) {
	svc := teamService(a)
	server := resolveGitLabURL(a, project)
	key, keyServer, implicit := svc.TrackerTokenDecl(project)
	res := svc.GitLabToken(ctx, svc.ResolveTeam(project).TeamID, server,
		teamsvc.WithProject(project), teamsvc.WithTrackerToken(key, keyServer, implicit))
	if !res.Found() {
		return gitlabAccess{URL: server, Token: res}, tracker.NewMissingTokenError(res, tracker.PurposeMergeRequest, "")
	}
	return gitlabAccess{URL: server, Token: res}, nil
}

// resolveGitLabURL is the GitLab URL of the MCP of a project (project, hub or
// team url), else GITLAB_URL, else gitlab.com.
func resolveGitLabURL(a *app.App, project *domain.Project) string {
	if u := teamService(a).GitLabMCPURL(project); u != "" {
		return u
	}
	if u := os.Getenv("GITLAB_URL"); u != "" {
		return u
	}
	return "https://gitlab.com"
}
