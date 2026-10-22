package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

func gitlabHub(secrets map[string]string) *app.App {
	return &app.App{
		Config: &config.Config{Teams: []config.TeamConfig{
			{ID: "core", Enabled: true, StateRepo: "git@h:acme/core.git", StatePath: "/s/core", MemberID: "alice"},
			{ID: "ops", Enabled: true, StateRepo: "git@h:acme/ops.git", StatePath: "/s/ops", MemberID: "alice"},
		}},
		Secrets: &mockSecretStore{secrets: secrets},
	}
}

func teamOf(id string) *string { return &id }

// oh review publish / oh mr create / oh review feedback find the GitLab token
// of the team of the project, with the same cascade as the tracker.
func TestResolveGitLabTokenFollowsTheUniqueCascade(t *testing.T) {
	const (
		coreKey = "openhub.team.core.gitlab.token"
		opsKey  = "openhub.team.ops.gitlab.token"
		mcpKey  = "openhub.mcp.gitlab.token"
	)
	inOps := &domain.Project{ID: "p1", TeamID: teamOf("ops")}
	solo := &domain.Project{ID: "p2"}

	tests := []struct {
		name    string
		env     string
		secrets map[string]string
		project *domain.Project
		want    string
	}{
		{"env first", "from-env", map[string]string{opsKey: "ops", mcpKey: "mcp"}, inOps, "from-env"},
		{"team key of the project (was ignored)", "", map[string]string{opsKey: "ops", coreKey: "core"}, inOps, "ops"},
		{"team key before the hub MCP token (new order)", "", map[string]string{opsKey: "ops", mcpKey: "mcp"}, inOps, "ops"},
		{"project token before the team key", "", map[string]string{"openhub.mcp.gitlab.token.p1": "proj", opsKey: "ops"}, inOps, "proj"},
		{"hub MCP token when the team has none", "", map[string]string{coreKey: "core", mcpKey: "mcp"}, inOps, "mcp"},
		{"project without team uses the hub team key", "", map[string]string{coreKey: "core", mcpKey: "mcp"}, solo, "core"},
		{"the key set in [mcp.gitlab] is read", "", map[string]string{"my.key": "custom"}, solo, "custom"},
		{"former tracker key as last resort", "", map[string]string{"openhub.tracker.gitlab.token": "trk"}, solo, "trk"},
		{"nothing", "", map[string]string{}, solo, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITLAB_TOKEN", tt.env)
			a := gitlabHub(tt.secrets)
			a.Config.MCP.Gitlab.Token = "my.key"
			assert.Equal(t, tt.want, resolveGitLabToken(t.Context(), a, tt.project))
		})
	}
}

func TestResolveGitLabTokenWithoutProjectOrKeychain(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	a := gitlabHub(map[string]string{"openhub.team.core.gitlab.token": "core"})
	assert.Equal(t, "core", resolveGitLabToken(t.Context(), a, nil), "no project: the hub team")
	a.Secrets = nil
	assert.Empty(t, resolveGitLabToken(t.Context(), a, nil))
	t.Setenv("GITLAB_TOKEN", "env")
	assert.Equal(t, "env", resolveGitLabToken(t.Context(), a, nil))
}

// The identity check and the "reuse the token" prompt of oh team init/rejoin
// read the same cascade; the label says which source answers.
func TestGitlabTokenSourceLabelsTheCascade(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		secrets map[string]string
		want    string
	}{
		{"env", "x", map[string]string{"openhub.team.core.gitlab.token": "t"}, i18n.T("cmd.init.wizard_rejoin_token_source_env")},
		{"team", "", map[string]string{"openhub.team.core.gitlab.token": "t", "openhub.mcp.gitlab.token": "m"}, i18n.Tf("cmd.init.wizard_rejoin_token_source_team", "core")},
		{"mcp", "", map[string]string{"openhub.mcp.gitlab.token": "m", "openhub.tracker.gitlab.token": "k"}, i18n.T("cmd.init.wizard_rejoin_token_source_mcp")},
		{"mcp legacy name", "", map[string]string{"gitlab-token": "m"}, i18n.T("cmd.init.wizard_rejoin_token_source_mcp")},
		{"tracker", "", map[string]string{"openhub.tracker.gitlab.token": "k"}, i18n.T("cmd.init.wizard_rejoin_token_source_tracker")},
		{"none", "", map[string]string{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITLAB_TOKEN", tt.env)
			a := gitlabHub(tt.secrets)
			assert.Equal(t, tt.want, gitlabTokenSource(t.Context(), a, "core"))
			wantTok := resolveGitLabTokenForIdentity(t.Context(), a, "core")
			assert.Equal(t, tt.want != "", wantTok != "", "the label and the identity token agree")
		})
	}
	t.Setenv("GITLAB_TOKEN", "")
	assert.Empty(t, gitlabTokenSource(t.Context(), nil, "core"), "no app: nothing")
}

// oh mcp enable --project only counts what the MCP server is given: not the
// token of a team nor the former tracker key.
func TestCheckProjectTokenForGitLabIgnoresTeamAndTrackerKeys(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	p := &domain.Project{ID: "p1", TeamID: teamOf("core")}
	for key, want := range map[string]bool{
		"openhub.team.core.gitlab.token": false,
		"openhub.tracker.gitlab.token":   false,
		"openhub.mcp.gitlab.token":       true,
		"openhub.mcp.gitlab.token.p1":    true,
	} {
		a := gitlabHub(map[string]string{key: "tok"})
		assert.Equal(t, want, checkProjectToken(t.Context(), a, "gitlab", p), key)
	}
	t.Setenv("GITLAB_TOKEN", "env")
	assert.True(t, checkProjectToken(t.Context(), gitlabHub(nil), "gitlab", p))
}

// resolveGitLabProject reads the tracker project of the team of the project.
func TestResolveGitLabProjectUsesTheTeamOfTheProject(t *testing.T) {
	mkState := func(trackerProject string) string {
		path := filepath.Join(t.TempDir(), "state")
		_, err := teamstate.InitSolo(t.Context(), path, teamstate.Member{ID: "alice"})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(path, "config.toml"),
			[]byte("[tracker]\ntype = \"gitlab\"\ntracker_project = \""+trackerProject+"\"\n"), 0o644))
		return path
	}
	a := gitlabHub(nil)
	a.Config.Teams[0].StatePath = mkState("acme/core-app")
	a.Config.Teams[1].StatePath = mkState("acme/ops-app")

	assert.Equal(t, "acme/ops-app", resolveGitLabProject(a, &domain.Project{ID: "p1", TeamID: teamOf("ops")}))
	assert.Equal(t, "acme/core-app", resolveGitLabProject(a, &domain.Project{ID: "p2"}), "no team: the hub team")
	assert.Equal(t, "acme/mine", resolveGitLabProject(a, &domain.Project{ID: "p3", TeamID: teamOf("ops"),
		TrackerConfig: &domain.ProjectTrackerConfig{TrackerProject: "acme/mine"}}), "the project override wins")
}
