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
	"github.com/datichb/openhub/cli/internal/tracker"
)

func gitlabHub(secrets map[string]string) *app.App {
	return &app.App{
		Config: &config.Config{Teams: []config.TeamConfig{
			{ID: "core", Enabled: true, StateRepo: "git@gitlab.core.example:acme/core.git", StatePath: "/s/core", MemberID: "alice"},
			{ID: "ops", Enabled: true, StateRepo: "git@gitlab.ops.example:acme/ops.git", StatePath: "/s/ops", MemberID: "alice"},
		}},
		Secrets: &mockSecretStore{secrets: secrets},
	}
}

func teamOf(id string) *string { return &id }

// oh review publish / oh mr create / oh review feedback find the GitLab token
// of the team of the project for the GitLab they call, with the same cascade
// as the tracker.
func TestResolveGitLabAccessFollowsTheUniqueCascade(t *testing.T) {
	const (
		coreKey = "openhub.team.core.gitlab.token"
		opsKey  = "openhub.team.ops.gitlab.token"
		mcpKey  = "openhub.mcp.gitlab.token"
	)
	inOps := &domain.Project{ID: "p1", TeamID: teamOf("ops")}
	solo := &domain.Project{ID: "p2"}

	tests := []struct {
		name      string
		env       string
		envServer string
		mcpURL    string
		secrets   map[string]string
		project   *domain.Project
		want      string
	}{
		{"env declared for the server first", "from-env", "https://gitlab.ops.example", "https://gitlab.ops.example", map[string]string{opsKey: "ops", mcpKey: "mcp"}, inOps, "from-env"},
		{"env without a declared server is not sent", "from-env", "", "https://gitlab.ops.example", map[string]string{opsKey: "ops"}, inOps, "ops"},
		{"team key of the project on the server of its repository", "", "", "https://gitlab.ops.example", map[string]string{opsKey: "ops", coreKey: "core"}, inOps, "ops"},
		{"team key before the hub MCP token on the same server", "", "", "https://gitlab.ops.example", map[string]string{opsKey: "ops", mcpKey: "mcp"}, inOps, "ops"},
		{"project token before the team key", "", "", "https://gitlab.ops.example", map[string]string{"openhub.mcp.gitlab.token.p1": "proj", opsKey: "ops"}, inOps, "proj"},
		{"hub MCP token when the team has none", "", "", "https://gitlab.ops.example", map[string]string{coreKey: "core", mcpKey: "mcp"}, inOps, "mcp"},
		{"the team key is not sent to another GitLab", "", "", "https://gitlab.forge.example", map[string]string{opsKey: "ops", coreKey: "core"}, inOps, ""},
		{"the hub MCP token serves the url of the MCP", "", "", "https://gitlab.forge.example", map[string]string{mcpKey: "mcp"}, solo, "mcp"},
		{"project without team uses the hub team key", "", "", "https://gitlab.core.example", map[string]string{coreKey: "core", mcpKey: "mcp"}, solo, "core"},
		{"the key set in [mcp.gitlab] is read", "", "", "https://gitlab.forge.example", map[string]string{"my.key": "custom"}, solo, "custom"},
		{"former tracker key for its declared server", "", "", "https://gitlab.forge.example", map[string]string{"openhub.tracker.gitlab.token": "trk"}, solo, "trk"},
		{"nothing", "", "", "https://gitlab.forge.example", map[string]string{}, solo, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITLAB_TOKEN", tt.env)
			t.Setenv("OH_GITLAB_URL", tt.envServer)
			t.Setenv("GITLAB_URL", "")
			a := gitlabHub(tt.secrets)
			a.Config.MCP.Gitlab.Token = "my.key"
			a.Config.MCP.Gitlab.URL = tt.mcpURL
			a.Config.Tracker.SetTokenDecl("", "")
			if tt.name == "former tracker key for its declared server" {
				a.Config.Tracker.TrackerTokenURL = tt.mcpURL // declares the server of the former key
				a.Config.Tracker.TrackerTokenKey = "unused.key"
			}
			access, err := resolveGitLabAccess(t.Context(), a, tt.project)
			assert.Equal(t, tt.want, access.Token.Token)
			if tt.want == "" {
				assert.True(t, tracker.IsMissingToken(err), "no token: the error names the server and the sources")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestResolveGitLabAccessUsesTheURLOfTheMCP(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("OH_GITLAB_URL", "")
	t.Setenv("GITLAB_URL", "")
	a := gitlabHub(map[string]string{"openhub.team.core.gitlab.token": "core"})

	_, err := resolveGitLabAccess(t.Context(), a, nil)
	var missing *tracker.MissingTokenError
	require.ErrorAs(t, err, &missing)
	assert.Equal(t, "https://gitlab.com", missing.Server, "no url anywhere: gitlab.com, which is not the server of the team")

	a.Config.MCP.Gitlab.URL = "https://gitlab.core.example/"
	access, err := resolveGitLabAccess(t.Context(), a, nil)
	require.NoError(t, err)
	assert.Equal(t, "core", access.Token.Token, "no project: the hub team")
	assert.Equal(t, "https://gitlab.core.example/", access.URL)

	t.Setenv("GITLAB_URL", "https://gitlab.core.example")
	a.Config.MCP.Gitlab.URL = ""
	access, err = resolveGitLabAccess(t.Context(), a, nil)
	require.NoError(t, err)
	assert.Equal(t, "https://gitlab.core.example", access.URL, "GITLAB_URL is the fallback of the url")

	a.Secrets = nil
	_, err = resolveGitLabAccess(t.Context(), a, nil)
	require.Error(t, err)
	t.Setenv("GITLAB_TOKEN", "env")
	_, err = resolveGitLabAccess(t.Context(), a, nil)
	require.NoError(t, err, "GITLAB_URL declares the server of GITLAB_TOKEN")
}

// The identity check and the "reuse the token" prompt of oh team init/rejoin
// read the same cascade; the label says which source answers.
func TestGitlabTokenSourceLabelsTheCascade(t *testing.T) {
	const repo = "git@gitlab.core.example:acme/core.git"
	tests := []struct {
		name      string
		env       string
		envServer string
		secrets   map[string]string
		mutate    func(*app.App)
		want      string
	}{
		{"env declared for the server", "x", "https://gitlab.core.example", map[string]string{"openhub.team.core.gitlab.token": "t"}, nil, i18n.T("cmd.init.wizard_rejoin_token_source_env")},
		{"env without a declared server", "x", "", map[string]string{"openhub.team.core.gitlab.token": "t"}, nil, i18n.Tf("cmd.init.wizard_rejoin_token_source_team", "core")},
		{"team", "", "", map[string]string{"openhub.team.core.gitlab.token": "t", "openhub.mcp.gitlab.token": "m"}, nil, i18n.Tf("cmd.init.wizard_rejoin_token_source_team", "core")},
		{"mcp", "", "", map[string]string{"openhub.mcp.gitlab.token": "m", "openhub.tracker.gitlab.token": "k"}, func(a *app.App) { a.Config.MCP.Gitlab.URL = "https://gitlab.core.example" }, i18n.T("cmd.init.wizard_rejoin_token_source_mcp")},
		{"mcp token of another GitLab", "", "", map[string]string{"openhub.mcp.gitlab.token": "m"}, func(a *app.App) { a.Config.MCP.Gitlab.URL = "https://gitlab.forge.example" }, ""},
		{"mcp legacy name", "", "", map[string]string{"gitlab-token": "m"}, func(a *app.App) { a.Config.MCP.Gitlab.URL = "https://gitlab.core.example" }, i18n.T("cmd.init.wizard_rejoin_token_source_mcp")},
		{"none", "", "", map[string]string{}, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITLAB_TOKEN", tt.env)
			t.Setenv("OH_GITLAB_URL", tt.envServer)
			t.Setenv("GITLAB_URL", "")
			a := gitlabHub(tt.secrets)
			if tt.mutate != nil {
				tt.mutate(a)
			}
			assert.Equal(t, tt.want, gitlabTokenSource(t.Context(), a, "core", repo))
			assert.Equal(t, tt.want != "", gitlabTeamToken(t.Context(), a, "core", repo).Found(), "the label and the identity token agree")
		})
	}
	t.Setenv("GITLAB_TOKEN", "")
	assert.Empty(t, gitlabTokenSource(t.Context(), nil, "core", repo), "no app: nothing")
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

// oh config set tracker.tracker_token_url declares the server of the key.
func TestConfigSetTrackerTokenURL(t *testing.T) {
	c := &config.Config{}
	field := configFieldMap["tracker.tracker_token_url"]
	require.NoError(t, field.Set(c, "https://gitlab.forge.example"))
	assert.Equal(t, "https://gitlab.forge.example", c.Tracker.TrackerTokenURL)
	require.Error(t, field.Set(c, "https://"), "a value that designates no server is refused")
	assert.Equal(t, "https://gitlab.forge.example", c.Tracker.TrackerTokenURL)
	field.Unset(c)
	assert.Empty(t, c.Tracker.TrackerTokenURL)
}

// A token that the GitLab refuses is named with its source, its key and the
// server by oh mr create and oh review feedback (the same client).
func TestGitLabAccessClientNamesTheRefusedToken(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("OH_GITLAB_URL", "")
	t.Setenv("GITLAB_URL", "")
	srv, seen := fakeGitLab(t, "other")

	a := gitlabHub(map[string]string{"my.key": "expired"})
	a.Config.MCP.Gitlab.Token = "my.key"
	a.Config.MCP.Gitlab.URL = srv.URL
	access, err := resolveGitLabAccess(t.Context(), a, nil)
	require.NoError(t, err)

	_, err = access.Client().FindMRByBranch(t.Context(), "acme/app", "feat/x")
	require.Error(t, err)
	assert.Equal(t, []string{"expired"}, seen())
	assert.Contains(t, err.Error(), "refused the token (HTTP 401)")
	assert.Contains(t, err.Error(), "MCP token (key my.key)")
	assert.Contains(t, err.Error(), "oh secrets set my.key")
	assert.ErrorIs(t, err, tracker.ErrTokenInvalid)
}

// Servers that a source implies: the MCP tokens serve gitlab.com when the MCP has
// no url; the key of the tracker serves the tracker only.
func TestResolveGitLabAccessImplicitServers(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("OH_GITLAB_URL", "")
	t.Setenv("GITLAB_URL", "")

	t.Run("MCP token without url: gitlab.com", func(t *testing.T) {
		a := gitlabHub(map[string]string{"openhub.mcp.gitlab.token": "mcp"})
		access, err := resolveGitLabAccess(t.Context(), a, nil)
		require.NoError(t, err)
		assert.Equal(t, "mcp", access.Token.Token)
		assert.Equal(t, "https://gitlab.com", access.URL)
	})
	t.Run("MCP token without url: not sent to GITLAB_URL", func(t *testing.T) {
		t.Setenv("GITLAB_URL", "https://gitlab.forge.example")
		a := gitlabHub(map[string]string{"openhub.mcp.gitlab.token": "mcp"})
		_, err := resolveGitLabAccess(t.Context(), a, nil)
		require.True(t, tracker.IsMissingToken(err))
	})
	t.Run("key of the tracker on the same server", func(t *testing.T) {
		a := gitlabHub(map[string]string{"forge.token": "trk"})
		a.Config.MCP.Gitlab.URL = "https://gitlab.forge.example"
		a.Config.Tracker.TrackerTokenKey = "forge.token"
		a.Config.Tracker.TrackerURL = "https://gitlab.forge.example/"
		access, err := resolveGitLabAccess(t.Context(), a, nil)
		require.NoError(t, err)
		assert.Equal(t, "trk", access.Token.Token)
	})
	t.Run("key of the tracker, tracker on another server: refused", func(t *testing.T) {
		a := gitlabHub(map[string]string{"forge.token": "trk"})
		a.Config.MCP.Gitlab.URL = "https://gitlab.other.example"
		a.Config.Tracker.TrackerTokenKey = "forge.token"
		a.Config.Tracker.TrackerURL = "https://gitlab.forge.example"
		_, err := resolveGitLabAccess(t.Context(), a, nil)
		require.True(t, tracker.IsMissingToken(err))
		assert.Contains(t, err.Error(), "forge.token")
	})
}
