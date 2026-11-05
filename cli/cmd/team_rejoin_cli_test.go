package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// rejoinOn runs oh team rejoin on repo with the answers of the user on its input
// and the flags given as --name=value. The application has a memory keychain
// and no team.
func rejoinOn(t *testing.T, repo, answers string, flags ...string) (*app.App, string, error) {
	t.Helper()
	return rejoinOnSecrets(t, repo, answers, nil, flags...)
}

// rejoinOnSecrets is rejoinOn with secrets already in the keychain.
func rejoinOnSecrets(t *testing.T, repo, answers string, secrets map[string]string, flags ...string) (a *app.App, out string, err error) {
	t.Helper()
	isolateGitAuth(t)
	i18ntest.Set(t, "en")
	configtest.Home(t)
	require.NoError(t, config.Save(&config.Config{}))
	a = newMockApp(nil, nil)
	a.Config = &config.Config{}
	a.Projects = &mockProjectStore{}
	a.Secrets = &mockSecretStore{secrets: map[string]string{}}
	for k, v := range secrets {
		a.Secrets.(*mockSecretStore).secrets[k] = v
	}
	io, stdout, _ := testIO()
	io.In = strings.NewReader(answers)
	a.IO = io
	application = a
	t.Cleanup(func() { application = nil })
	wireTestGitCredentials(t, a)

	c := &cobra.Command{Use: "rejoin", RunE: runTeamRejoin}
	addTeamRejoinFlags(c)
	c.SetContext(t.Context())
	require.NoError(t, c.ParseFlags(append([]string{"--repo=" + repo, "--no-retro-tag"}, flags...)))
	err = runTeamRejoin(c, nil)
	return a, stdout.String(), err
}

// A repository that asks for a token: oh probes it first, asks the token,
// checks it before keeping it, and only then clones.
func TestTeamRejoinAsksTheTokenBeforeTheClone(t *testing.T) {
	bare, _ := claimRepos(t)
	srv := gittest.ServeHTTP(t, bare, "team-state", "bob", "good")

	t.Run("a wrong token is refused, nothing is cloned", func(t *testing.T) {
		a, out, err := rejoinOn(t, srv.URL, "bob\nbad\nbob\nbad\nbob\nbad\n", "--member-id=testuser")
		require.Error(t, err)
		assert.Contains(t, out, "refuses this token")
		assert.NoDirExists(t, config.TeamStatePath(srv.URL), "no clone before the access")
		assert.Empty(t, a.Secrets.(*mockSecretStore).secrets, "a wrong token never reaches the keychain")
	})

	t.Run("the token is checked, kept, and the clone follows", func(t *testing.T) {
		a, out, err := rejoinOn(t, srv.URL, "bob\ngood\n", "--member-id=testuser")
		require.NoError(t, err, out)
		secrets := a.Secrets.(*mockSecretStore).secrets
		assert.Equal(t, "good", secrets[config.TeamGitTokenKey(srv.URL)])
		assert.Equal(t, "bob", secrets[config.TeamGitUserKey(srv.URL)])
		assert.True(t, teamstate.NewRepo(srv.URL, config.TeamStatePath(srv.URL)).IsCloned())
		assert.Equal(t, "testuser", a.Config.FindTeamByRepo(srv.URL).MemberID)
	})

	t.Run("no token typed is an error, not a loop", func(t *testing.T) {
		_, _, err := rejoinOn(t, srv.URL, "")
		require.Error(t, err)
	})
}

// An SSH repository whose key is refused: the command says what to check, and
// asks no token.
func TestTeamRejoinSSHRefusedAsksNoToken(t *testing.T) {
	fakeSSHServers(t, "gitlab.example.com") // known: the server is not what this test is about
	fakeSSHRefusing(t)
	_, out, err := rejoinOn(t, "git@gitlab.example.com:org/team.git", "token\n", "--member-id=testuser")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ssh-add")
	assert.NotContains(t, out, "Token")
}

// The person creates their own card: no member to pick, the card is made with
// the flags, the team is joined with it.
func TestTeamRejoinCreatesTheCardWithFlags(t *testing.T) {
	bare, _ := claimRepos(t)
	a, out, err := rejoinOn(t, bare, "", "--new-member", "--member-id=newbie", "--display-name=New Bie", "--role=reviewer")
	require.NoError(t, err, out)
	card := cardsOf(t, bare)["newbie"]
	assert.Equal(t, "New Bie", card.DisplayName)
	assert.Equal(t, "reviewer", card.Role)
	assert.Equal(t, "semi-auto", card.DefaultMode)
	assert.Equal(t, "newbie", a.Config.FindTeamByRepo(bare).MemberID)
	assert.Contains(t, out, "newbie")

	_, _, err = rejoinOn(t, bare, "", "--new-member", "--member-id=other", "--role=boss")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boss")
	assert.NotContains(t, cardsOf(t, bare), "other")
}

// The list offers to create the card first; a taken ID is refused, and « C'est
// moi » attaches the card that exists without changing it.
func TestTeamRejoinCreatesTheCardFromTheListAndClaimsATakenOne(t *testing.T) {
	bare := teamWithColleagues(t)

	// 0 = create; the ID is a colleague's; « that's me » is answered no, then another ID
	a, out, err := rejoinOn(t, bare, "0\nben\nImpostor\n\nn\nnewbie\n")
	require.NoError(t, err, out)
	assert.Contains(t, out, "already taken")
	assert.Equal(t, "Benjamin", cardsOf(t, bare)["ben"].DisplayName)
	assert.Contains(t, cardsOf(t, bare), "newbie")
	assert.Equal(t, "newbie", a.Config.FindTeamByRepo(bare).MemberID)

	// the same, « that's me »
	bare = teamWithColleagues(t)
	a, out, err = rejoinOn(t, bare, "0\nben\nWhoever\n\ny\n\n")
	require.NoError(t, err, out)
	card := cardsOf(t, bare)["ben"]
	assert.Equal(t, "Benjamin", card.DisplayName, "attached as it is")
	assert.Equal(t, "lead", card.Role)
	assert.Equal(t, "ben", a.Config.FindTeamByRepo(bare).MemberID)
}

// The identity is verified before the card is published: a token that is
// somebody else's is told, and nothing is pushed unless the person says so.
func TestTeamRejoinVerifiesTheIdentityBeforePublishingTheCard(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GITLAB_PERSONAL_ACCESS_TOKEN", "")
	gitlab := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"username":"someone.else"}`))
	}))
	t.Cleanup(gitlab.Close)
	flags := []string{"--new-member", "--member-id=newbie", "--display-name=New", "--gitlab-username=new.bie", "--gitlab-url=" + gitlab.URL}

	for _, tc := range []struct {
		name, answers string
		published     bool
	}{
		{"declined", "\nn\n", false},
		{"published anyway", "\ny\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bare, _ := claimRepos(t)
			isolateGitAuth(t)
			// the token of the team in the keychain is "reused" (Enter)
			a, out, err := rejoinOnSecrets(t, bare, tc.answers, map[string]string{
				config.TeamGitLabTokenKey(config.RepoNameFromRemote(bare)): "glpat-x",
			}, flags...)
			assert.Contains(t, out, "someone.else", "the mismatch is told")
			if tc.published {
				require.NoError(t, err, out)
				assert.Contains(t, cardsOf(t, bare), "newbie")
				assert.NotNil(t, a.Config.FindTeamByRepo(bare))
				return
			}
			require.Error(t, err)
			assert.NotContains(t, cardsOf(t, bare), "newbie", "nothing was pushed")
		})
	}
}

// fakeSSHRefusing puts an ssh on the PATH that refuses the key the way OpenSSH
// does.
func fakeSSHRefusing(t *testing.T) {
	t.Helper()
	isolateGitAuth(t)
	dir := t.TempDir()
	script := "#!/bin/sh\necho 'git@gitlab.example.com: Permission denied (publickey).' >&2\nexit 255\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
