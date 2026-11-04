package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// wireTestGitCredentials makes the secrets of a the source of the git tokens
// of the team-state for the length of the test (the keychain of the machine is
// never touched: a.Secrets is a memory store).
func wireTestGitCredentials(t *testing.T, a *app.App) {
	t.Helper()
	teamGitCreds = wireTeamGitCredentials(func() domain.SecretStore { return a.Secrets })
	t.Cleanup(func() {
		teamstate.SetCredentialSource(nil)
		teamGitCreds = nil
	})
}

// The token of a repository goes to the keychain of oh under a key of its own
// and comes back to git; an SSH repository has none.
func TestTeamGitTokenIsKeptInTheKeychainOfOh(t *testing.T) {
	i18ntest.Set(t, "en")
	a := newMockApp(nil, nil)
	wireTestGitCredentials(t, a)
	remote := "https://gitlab.example.com/org/team-state.git"
	ctx := t.Context()

	require.NoError(t, saveTeamGitToken(ctx, a, remote, "", "  glpat-abc "))

	store := a.Secrets.(*mockSecretStore).secrets
	assert.Equal(t, "glpat-abc", store["openhub.git.gitlab.example.com/org/team-state.token"])
	assert.Equal(t, "oauth2", store["openhub.git.gitlab.example.com/org/team-state.user"], "oauth2 by default")
	assert.NotContains(t, store, config.TeamGitLabTokenKey("team-state"), "the git token is not the API token of the team")

	got := teamGitCreds.source(ctx, remote)
	assert.Equal(t, teamstate.GitCredentials{Username: "oauth2", Token: "glpat-abc"}, got)
	assert.Equal(t, got, teamGitCreds.source(ctx, "https://gitlab.example.com/org/team-state"), "written without .git")
	assert.Zero(t, teamGitCreds.source(ctx, "https://gitlab.example.com/org/other.git"), "another repository")
	assert.Zero(t, teamGitCreds.source(ctx, "git@gitlab.example.com:org/team-state.git"), "SSH has no token")

	require.NoError(t, saveTeamGitToken(ctx, a, remote, "bob", "t2"))
	assert.Equal(t, teamstate.GitCredentials{Username: "bob", Token: "t2"}, teamGitCreds.source(ctx, remote))
}

// A repository without a token is remembered for a while, and forgotten as
// soon as a token is stored.
func TestTeamGitCredentialsMissIsForgottenOnStore(t *testing.T) {
	a := newMockApp(nil, nil)
	wireTestGitCredentials(t, a)
	remote := "https://git.example.com/org/team.git"
	assert.Zero(t, teamGitCreds.source(t.Context(), remote))
	// the keychain gets a token behind our back: not seen before the memory expires
	a.Secrets.(*mockSecretStore).secrets[config.TeamGitTokenKey(remote)] = "late"
	assert.Zero(t, teamGitCreds.source(t.Context(), remote), "the miss is remembered")
	require.NoError(t, saveTeamGitToken(t.Context(), a, remote, "", "tok"))
	assert.Equal(t, "tok", teamGitCreds.source(t.Context(), remote).Token)
}

func TestSaveTeamGitTokenRefusals(t *testing.T) {
	i18ntest.Set(t, "en")
	a := newMockApp(nil, nil)
	wireTestGitCredentials(t, a)
	err := saveTeamGitToken(t.Context(), a, "git@gitlab.example.com:org/team.git", "", "x")
	require.Error(t, err)
	assert.Equal(t, i18n.T("v532w.access.not_https"), err.Error())

	a.Secrets = nil
	err = saveTeamGitToken(t.Context(), a, "https://gitlab.example.com/org/team.git", "", "x")
	require.Error(t, err)
	assert.Equal(t, i18n.T("v532w.access.no_keychain"), err.Error())
	assert.Zero(t, teamGitCreds.source(t.Context(), "https://gitlab.example.com/org/team.git"), "no keychain: no token")
}
