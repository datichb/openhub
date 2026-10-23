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
// team service, the step of the team included.
func TestTrackerCredentialsUseTheTeamToken(t *testing.T) {
	t.Setenv(tracker.GitLabTokenEnv, "")
	cfg := &config.Config{Teams: []config.TeamConfig{{ID: "core", Enabled: true, StateRepo: "git@h:acme/core.git", StatePath: "/s/core", MemberID: "alice"}}}
	a := &app.App{Config: cfg, Secrets: &mockSecretStore{secrets: map[string]string{config.TeamGitLabTokenKey("core"): "from-team"}}}
	teamCfg := &teamstate.TeamConfig{}
	teamCfg.Tracker.TrackerURL = "https://gitlab.example.com"
	teamCfg.Tracker.TrackerTokenKey = "openhub.tracker.gitlab.token"

	creds, err := tracker.ResolveCredentials(context.Background(), trackerCredentials(a, teamCfg), tracker.TypeGitLab)
	require.NoError(t, err)
	assert.Equal(t, "from-team", creds.Token)
	assert.Equal(t, "https://gitlab.example.com", creds.BaseURL)

	// Before, only the tracker key was read: without it, no token.
	_, err = tracker.ResolveCredentials(context.Background(), tracker.CredentialSource{
		URL: teamCfg.Tracker.TrackerURL, TokenKey: teamCfg.Tracker.TrackerTokenKey, Secrets: a.Secrets,
	}, tracker.TypeGitLab)
	assert.Error(t, err)
}
