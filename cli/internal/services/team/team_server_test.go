package team

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// The team-state repository is reached through an SSH alias (git@mon-alias:…):
// the alias is a name of ~/.ssh/config, not a server. gitlab_url of the team
// says which GitLab it is, and the token of the team is then valid for it.
func TestTeamTokenFollowsTheGitLabURLOfAnAliasedRepository(t *testing.T) {
	t.Setenv(tracker.GitLabTokenEnv, "")
	t.Setenv(tracker.GitLabServerEnv, "")
	secrets := fakeSecrets{config.TeamGitLabTokenKey("core"): "tok-core"}
	cfg := &config.Config{Teams: []config.TeamConfig{
		{ID: "core", Enabled: true, StateRepo: "git@mon-alias:acme/core.git", StatePath: "/s/core", MemberID: "alice"},
	}}
	svc := New(cfg, secrets, nil)
	const gitlab = "https://gitlab.corp.example"

	assert.Empty(t, svc.GitLabToken(t.Context(), "core", gitlab).Token, "the alias is not the GitLab: no token for it")
	assert.Equal(t, "mon-alias", svc.TeamRepoServer(nil))

	cfg.Teams[0].GitLabURL = gitlab + "/"
	res := svc.GitLabToken(t.Context(), "core", gitlab)
	assert.Equal(t, "tok-core", res.Token)
	assert.Equal(t, tracker.TokenSourceTeam, res.Source)
	assert.Equal(t, "gitlab.corp.example", svc.TeamRepoServer(nil))
	assert.Empty(t, svc.GitLabToken(t.Context(), "core", "https://mon-alias").Token, "the declared GitLab replaces the host of the remote")
	assert.Equal(t, "gitlab.corp.example", svc.CredentialSource(tracker.CredentialSource{}, nil).TeamServer)
}

// A team that is not registered yet (oh team rejoin) gets its GitLab from the
// option; the setting of a registered team has the priority over the remote
// given with WithTeamRepo.
func TestGitLabURLOfATeamNotYetInHubToml(t *testing.T) {
	t.Setenv(tracker.GitLabTokenEnv, "")
	t.Setenv(tracker.GitLabServerEnv, "")
	svc := New(&config.Config{}, fakeSecrets{config.TeamGitLabTokenKey("new"): "tok"}, nil)
	const gitlab = "https://gitlab.new.example"
	remote := "git@new-alias:acme/new.git"

	assert.Empty(t, svc.GitLabToken(t.Context(), "new", gitlab, WithTeamRepo(remote)).Token)
	assert.Equal(t, "tok", svc.GitLabToken(t.Context(), "new", gitlab, WithTeamRepo(remote), WithTeamServer(gitlab)).Token)

	cfg := &config.Config{Teams: []config.TeamConfig{{ID: "new", Enabled: true, StateRepo: remote, StatePath: "/s/new", GitLabURL: gitlab}}}
	svc = New(cfg, fakeSecrets{config.TeamGitLabTokenKey("new"): "tok"}, nil)
	assert.Equal(t, "tok", svc.GitLabToken(t.Context(), "new", gitlab, WithTeamRepo(remote)).Token)
}
