package cmd

import (
	"context"
	"errors"
	"os"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/mcpresolve"
	teamsvc "github.com/datichb/openhub/cli/internal/services/team"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// prepareMCPTest is the connection test of an MCP service of the team MCP view
// (t). It runs on the event loop, which owns the shared Config: it takes a
// snapshot of it, and the function it returns makes the calls (off the loop)
// with the same cascade and the same clients as the commands that call the
// server. It returns nil for a service that has no connection test.
//
//   - gitlab: GET /user on the GitLab of the MCP, with the token of the
//     cascade of the team service (the one of oh mr create);
//   - jira: GET /myself on the Jira of the MCP, with the MCP token;
//   - figma, gslides: none (the code has no call that checks their token).
func prepareMCPTest(a *app.App, service string) func(ctx context.Context) (string, error) {
	cfg := *a.Config
	svc := teamsvc.New(&cfg, a.Secrets, a.Projects)
	switch service {
	case "gitlab":
		return func(ctx context.Context) (string, error) { return testGitLabMCP(ctx, svc) }
	case "jira":
		return func(ctx context.Context) (string, error) { return testJiraMCP(ctx, svc, a.Secrets) }
	}
	return nil
}

func testGitLabMCP(ctx context.Context, svc *teamsvc.Service) (string, error) {
	access, err := resolveGitLabAccessWith(ctx, svc, nil)
	if err != nil {
		return "", err
	}
	user, err := access.Client().CurrentUser(ctx)
	if err != nil {
		return "", callError(access.URL, access.Token.Origin(), err)
	}
	return i18n.Tf("tui.team_mcp.test_detail", user, access.Token.Origin().DescribeWithServer()), nil
}

func testJiraMCP(ctx context.Context, svc *teamsvc.Service, secrets config.SecretReadWriter) (string, error) {
	tc := svc.ResolveTeam(nil)
	eff := mcpresolve.ResolveFull(sharedMCPService(tc, "jira"), svc.Config.MCP.Jira, nil, tc.TeamID)

	url := eff.URL
	if url == "" {
		url = os.Getenv("JIRA_URL")
	}
	if url == "" {
		return "", errors.New(i18n.T("tui.team_mcp.test_jira_no_url"))
	}

	origin := tracker.TokenOrigin{Source: tracker.TokenSourceEnv, Key: "JIRA_TOKEN", Server: url}
	token := os.Getenv("JIRA_TOKEN")
	if token == "" {
		key := eff.TokenKey
		if key == "" {
			key = config.DefaultTokenKeyForService("jira")
		}
		origin = tracker.TokenOrigin{Source: tracker.TokenSourceMCP, Key: key, Server: url}
		token, _ = config.ReadMCPToken(ctx, secrets, "jira", eff.TokenKey)
		if token == "" {
			return "", errors.New(i18n.Tf("tui.team_mcp.test_jira_no_token", url, key))
		}
	}

	t, err := tracker.New(tracker.Config{Type: tracker.TypeJira, BaseURL: url, Token: token, Origin: origin})
	if err != nil {
		return "", callError(url, origin, err)
	}
	user, err := t.TestConnection(ctx)
	if err != nil {
		return "", callError(url, origin, err)
	}
	return i18n.Tf("tui.team_mcp.test_detail", user, origin.DescribeWithServer()), nil
}

// callError names the server and the token of a call that failed. A refusal of
// the token already does (it comes with its source, key and server).
func callError(server string, origin tracker.TokenOrigin, err error) error {
	var rejected *tracker.TokenRejectedError
	if errors.As(err, &rejected) {
		return err
	}
	return errors.New(i18n.Tf("tui.team_mcp.test_call_failed", server, origin.Describe(), err))
}

// sharedMCPService reads the recommendation of the team for an MCP service
// from the team-state clone (nil without team, clone or recommendation).
func sharedMCPService(tc config.ResolvedTeamConfig, name string) *teamstate.SharedMCPConfig {
	if !tc.Enabled || tc.StatePath == "" {
		return nil
	}
	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	if !repo.IsCloned() {
		return nil
	}
	cfg, err := repo.LoadConfig()
	if err != nil {
		return nil
	}
	if shared, ok := cfg.MCP[name]; ok {
		return &shared
	}
	return nil
}
