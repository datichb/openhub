package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// The wizard of the tracker finds the GitLab token with the cascade of the
// team service, the step of the team included, for the server of the tracker.
func TestTrackerCredentialsUseTheTeamToken(t *testing.T) {
	t.Setenv(tracker.GitLabTokenEnv, "")
	t.Setenv(tracker.GitLabServerEnv, "")
	t.Setenv("GITLAB_URL", "")
	cfg := &config.Config{Teams: []config.TeamConfig{{ID: "core", Enabled: true, StateRepo: "git@gitlab.example.com:acme/core.git", StatePath: "/s/core", MemberID: "alice"}}}
	a := &app.App{Config: cfg, Secrets: &mockSecretStore{secrets: map[string]string{config.TeamGitLabTokenKey("core"): "from-team"}}}
	teamCfg := &teamstate.TeamConfig{}
	teamCfg.Tracker.TrackerURL = "https://gitlab.example.com"
	teamCfg.Tracker.TrackerTokenKey = "openhub.tracker.gitlab.token"

	creds, err := tracker.ResolveCredentials(context.Background(), trackerCredentials(a, teamCfg), tracker.TypeGitLab)
	require.NoError(t, err)
	assert.Equal(t, "from-team", creds.Token)
	assert.Equal(t, "https://gitlab.example.com", creds.BaseURL)
	assert.Equal(t, tracker.TokenSourceTeam, creds.Origin.Source)

	// Before, only the tracker key was read: without it, no token.
	_, err = tracker.ResolveCredentials(context.Background(), tracker.CredentialSource{
		URL: teamCfg.Tracker.TrackerURL, TokenKey: teamCfg.Tracker.TrackerTokenKey, Secrets: a.Secrets,
	}, tracker.TypeGitLab)
	assert.Error(t, err)
}

// The team repository and the tracker are on two GitLab: the token of the team
// is not offered to the tracker (the wizard, the status and the sync read the
// same cascade), and the error says which server is missing a token.
func TestTrackerCredentialsDoNotSendTheTeamTokenToAnotherServer(t *testing.T) {
	t.Setenv(tracker.GitLabTokenEnv, "")
	t.Setenv(tracker.GitLabServerEnv, "")
	t.Setenv("GITLAB_URL", "")
	cfg := &config.Config{Teams: []config.TeamConfig{{ID: "core", Enabled: true, StateRepo: "git@gitlab.octo.example:acme/core.git", StatePath: "/s/core", MemberID: "alice"}}}
	a := &app.App{Config: cfg, Secrets: &mockSecretStore{secrets: map[string]string{
		config.TeamGitLabTokenKey("core"): "token-octo",
		"forge.token":                     "token-forge",
	}}}
	teamCfg := &teamstate.TeamConfig{}
	teamCfg.Tracker.TrackerURL = "https://gitlab.forge.example"

	_, err := tracker.ResolveCredentials(context.Background(), trackerCredentials(a, teamCfg), tracker.TypeGitLab)
	var missing *tracker.MissingTokenError
	require.ErrorAs(t, err, &missing)
	assert.Equal(t, "https://gitlab.forge.example", missing.Server)

	// tracker_token_key of hub.toml (bug 11: the status ignored it) and its server
	a.Config.Tracker.SetTokenDecl("forge.token", "https://gitlab.forge.example")
	creds, err := tracker.ResolveCredentials(context.Background(), trackerCredentials(a, teamCfg), tracker.TypeGitLab)
	require.NoError(t, err)
	assert.Equal(t, "token-forge", creds.Token)

	// the status reads the same cascade
	eff := tracker.ResolveTrackerConfig(&teamCfg.Tracker, a.Config.Tracker)
	creds, err = tracker.ResolveCredentials(context.Background(), buildCredentialSource(a, eff), tracker.TypeGitLab)
	require.NoError(t, err)
	assert.Equal(t, "token-forge", creds.Token)

	// a key without a declared server serves the tracker it is set for
	a.Config.Tracker.SetTokenDecl("forge.token", "")
	creds, err = tracker.ResolveCredentials(context.Background(), trackerCredentials(a, teamCfg), tracker.TypeGitLab)
	require.NoError(t, err)
	assert.Equal(t, "token-forge", creds.Token)

	// ... but not another one: the key of the team was set next to the tracker_url of the team,
	// which hub.toml overrides
	a.Config.Tracker.SetTokenDecl("", "")
	teamCfg.Tracker.TrackerTokenKey = "forge.token"
	a.Config.Tracker.TrackerURL = "https://gitlab.other.example"
	_, err = tracker.ResolveCredentials(context.Background(), trackerCredentials(a, teamCfg), tracker.TypeGitLab)
	require.ErrorAs(t, err, &missing)
	assert.Contains(t, err.Error(), "forge.token")
}
