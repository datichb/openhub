package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTeamGitKeys(t *testing.T) {
	for _, tt := range []struct {
		remote, token, user string
	}{
		{"https://gitlab.example.com/org/team-state.git", "openhub.git.gitlab.example.com/org/team-state.token", "openhub.git.gitlab.example.com/org/team-state.user"},
		{"https://GitLab.Example.com/org/team-state/", "openhub.git.gitlab.example.com/org/team-state.token", "openhub.git.gitlab.example.com/org/team-state.user"},
		{"  http://127.0.0.1:8080/team.git  ", "openhub.git.127.0.0.1:8080/team.token", "openhub.git.127.0.0.1:8080/team.user"},
		{"https://me@gitlab.example.com/g/sub/team.git", "openhub.git.gitlab.example.com/g/sub/team.token", "openhub.git.gitlab.example.com/g/sub/team.user"},
		{"git@gitlab.example.com:org/team.git", "", ""},
		{"ssh://git@gitlab.example.com/org/team.git", "", ""},
		{"/tmp/bare", "", ""},
		{"", "", ""},
	} {
		assert.Equal(t, tt.token, TeamGitTokenKey(tt.remote), tt.remote)
		assert.Equal(t, tt.user, TeamGitUserKey(tt.remote), tt.remote)
	}
	assert.NotEqual(t, TeamGitTokenKey("https://a.example/org/team.git"), TeamGitTokenKey("https://b.example/org/team.git"),
		"the same repository name on two servers does not share a token")
	assert.NotEqual(t, TeamGitTokenKey("https://a.example/org/team.git"), TeamGitLabTokenKey("team"),
		"not the key of the GitLab API token")
}
