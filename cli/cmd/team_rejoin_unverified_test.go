package cmd

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// rejoinWithGitLabUser rejoins as a member who has a GitLab handle, with an API
// token of the team in the keychain and a GitLab (fake) that answers
// GET /api/v4/user with status.
func rejoinWithGitLabUser(t *testing.T, status int, user string) (*teamRejoinResult, *int32, error) {
	t.Helper()
	i18ntest.Set(t, "en")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GITLAB_PERSONAL_ACCESS_TOKEN", "")
	a, repo, _ := setupClaimTestApp(t)
	configtest.Home(t)
	gittest.Commit(t, repo.Path(), "members.toml",
		"[members.testuser]\ndisplay_name = \"Test User\"\nrole = \"dev\"\ngitlab_username = \"testuser\"\n", "gitlab handle")
	gittest.Run(t, repo.Path(), "push", "-q")
	a.Config.Teams = nil

	var asked int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&asked, 1)
		assert.Equal(t, "/api/v4/user", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"username":"` + user + `"}`))
	}))
	t.Cleanup(srv.Close)

	a.Secrets = &mockSecretStore{secrets: map[string]string{
		config.TeamGitLabTokenKey(config.RepoNameFromRemote(repo.Remote())): "glpat-git-scopes-only",
	}}
	res, err := teamRejoinCore(t.Context(), a, teamRejoinParams{
		StateRepo: repo.Remote(), StatePath: repo.Path(), MemberID: "testuser", GitLabURL: srv.URL,
	})
	return res, &asked, err
}

// A token with the git scopes only is refused by GET /user (403): the rejoin
// goes through and says the identity was not verified, instead of failing in a
// loop.
func TestRejoinWithATokenGitLabRefusesIsUnverifiedNotBlocked(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusUnauthorized, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			res, asked, err := rejoinWithGitLabUser(t, status, "")
			require.NoError(t, err)
			require.NotNil(t, res)
			assert.EqualValues(t, 1, atomic.LoadInt32(asked), "the identity was asked")
			require.NotNil(t, res.IdentityUnverified, "the warning is carried to the caller")
			assert.Contains(t, res.IdentityUnverified.Error(), "Identity not verified")
			assert.Contains(t, res.IdentityUnverified.Error(), "read_user")
			assert.Nil(t, res.IdentityMismatch)
			assert.Equal(t, "testuser", res.Member.ID)
		})
	}
}

// A GitLab that cannot be reached is the same: not verified, not blocked.
func TestRejoinWithGitLabUnreachableIsUnverifiedNotBlocked(t *testing.T) {
	i18ntest.Set(t, "en")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GITLAB_PERSONAL_ACCESS_TOKEN", "")
	a, repo, _ := setupClaimTestApp(t)
	configtest.Home(t)
	gittest.Commit(t, repo.Path(), "members.toml",
		"[members.testuser]\ndisplay_name = \"Test User\"\nrole = \"dev\"\ngitlab_username = \"testuser\"\n", "gitlab handle")
	gittest.Run(t, repo.Path(), "push", "-q")
	a.Config.Teams = nil
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	a.Secrets = &mockSecretStore{secrets: map[string]string{
		config.TeamGitLabTokenKey(config.RepoNameFromRemote(repo.Remote())): "glpat",
	}}
	res, err := teamRejoinCore(t.Context(), a, teamRejoinParams{
		StateRepo: repo.Remote(), StatePath: repo.Path(), MemberID: "testuser", GitLabURL: url,
	})
	require.NoError(t, err)
	require.NotNil(t, res.IdentityUnverified)
}

// A token that answers is still checked: the right user passes, another one is
// a mismatch.
func TestRejoinIdentityStillVerifiedWhenGitLabAnswers(t *testing.T) {
	res, _, err := rejoinWithGitLabUser(t, http.StatusOK, "testuser")
	require.NoError(t, err)
	assert.Nil(t, res.IdentityUnverified)
	assert.Nil(t, res.IdentityMismatch)

	res, _, err = rejoinWithGitLabUser(t, http.StatusOK, "someone-else")
	require.NoError(t, err)
	assert.Nil(t, res.IdentityUnverified)
	require.NotNil(t, res.IdentityMismatch)
}

// The git token goes to the git key and nowhere else; the API token of the team
// is neither created from it nor overwritten by it.
func TestSaveTeamGitTokenLeavesTheAPIKeyAlone(t *testing.T) {
	i18ntest.Set(t, "en")
	a := newMockApp(nil, nil)
	wireTestGitCredentials(t, a)
	remote := "https://gitlab.example.com/org/team-state.git"
	store := a.Secrets.(*mockSecretStore).secrets
	apiKey := config.TeamGitLabTokenKey(config.RepoNameFromRemote(remote))

	require.NoError(t, saveTeamGitToken(t.Context(), a, remote, "", "git-token"))
	assert.Equal(t, "git-token", store[config.TeamGitTokenKey(remote)])
	assert.NotContains(t, store, apiKey, "no API token is made of the git token")

	store[apiKey] = "api-token"
	require.NoError(t, saveTeamGitToken(t.Context(), a, remote, "", "git-token-2"))
	assert.Equal(t, "api-token", store[apiKey], "an API token is not overwritten")
	assert.Equal(t, "git-token-2", store[config.TeamGitTokenKey(remote)])
}

// The step of the GitLab API token is asked for an HTTPS repository too, even
// when the access step kept a git token: that token is not an API token.
func TestRejoinAsksTheAPITokenAfterAGitToken(t *testing.T) {
	i18ntest.Set(t, "en")
	w := &rejoinWizard{
		stateRepo: "https://gitlab.example.com/org/team-state.git",
		memberID:  "alice",
		members:   []teamstate.Member{{ID: "alice", GitLabUsername: "alice"}},
		access:    &teamAccess{TokenStored: true},
	}
	assert.False(t, w.gitlabTokenStep().SkipIf(), "asked after a stored git token")

	w.members = []teamstate.Member{{ID: "alice"}}
	assert.True(t, w.gitlabTokenStep().SkipIf(), "no GitLab handle: nothing to verify")
}
