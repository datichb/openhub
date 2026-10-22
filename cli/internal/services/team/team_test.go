package team

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
)

func TestMain(m *testing.M) { os.Exit(gittest.Main(m)) }

type fakeSecrets map[string]string

func (s fakeSecrets) Get(_ context.Context, key string) (string, error) { return s[key], nil }

type fakeProjects []domain.Project

func (p fakeProjects) List(context.Context, domain.ProjectStatus) ([]domain.Project, error) {
	return p, nil
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

// hubConfig has the teams "core" (the hub default), "ops" and a solo space.
func hubConfig() *config.Config {
	return &config.Config{Teams: []config.TeamConfig{
		{ID: "solo", Enabled: true, Solo: true, StateRepo: "git@h:me/solo.git", StatePath: "/s/solo", MemberID: "me"},
		{ID: "core", Enabled: true, StateRepo: "git@h:acme/core.git", StatePath: "/s/core", MemberID: "alice"},
		{ID: "ops", Enabled: true, StateRepo: "git@h:acme/ops.git", StatePath: "/s/ops", MemberID: "alice-ops"},
		{ID: "off", Enabled: false, StateRepo: "git@h:acme/off.git", StatePath: "/s/off"},
	}}
}

func TestResolveTeam(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *config.Config
		project    *domain.Project
		wantID     string
		wantEnable bool
		wantMember string
	}{
		{"no project is the hub team", hubConfig(), nil, "core", true, "alice"},
		{"project of another team keeps its team", hubConfig(), &domain.Project{TeamID: strPtr("ops")}, "ops", true, "alice-ops"},
		{"project without team falls back on the hub", hubConfig(), &domain.Project{ID: "p"}, "core", true, "alice"},
		{"project with an unknown team falls back on the hub", hubConfig(), &domain.Project{TeamID: strPtr("ghost")}, "core", true, "alice"},
		{"project of a disabled team falls back on the hub", hubConfig(), &domain.Project{TeamID: strPtr("off")}, "core", true, "alice"},
		{"project of a solo space falls back on the hub", hubConfig(), &domain.Project{TeamID: strPtr("solo")}, "core", true, "alice"},
		{"project with a legacy override disabled falls back on the hub", hubConfig(), &domain.Project{TeamConfig: &domain.ProjectTeamConfig{Mode: domain.ProjectTeamModeDisabled}}, "core", true, "alice"},
		{"no team at all", &config.Config{}, &domain.Project{}, "", false, ""},
		{"only a solo space", &config.Config{Teams: []config.TeamConfig{{ID: "solo", Enabled: true, Solo: true, StateRepo: "r"}}}, nil, "", false, ""},
		{"hub team disabled stays disabled", &config.Config{Teams: []config.TeamConfig{{ID: "off", StateRepo: "git@h:acme/off.git"}}}, &domain.Project{}, "off", false, ""},
		{"legacy [team] without ID takes the repository name", &config.Config{Team: config.TeamConfig{Enabled: true, StateRepo: "git@h:acme/legacy-state.git", MemberID: "bob"}}, nil, "legacy-state", true, "bob"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.cfg, nil, nil).ResolveTeam(tt.project)
			assert.Equal(t, tt.wantID, got.TeamID)
			assert.Equal(t, tt.wantEnable, got.Enabled)
			assert.Equal(t, tt.wantMember, got.MemberID)
		})
	}
}

func TestGitLabTokenUsesTheConfigAndTheOptions(t *testing.T) {
	t.Setenv(tracker.GitLabTokenEnv, "")
	cfg := hubConfig()
	cfg.MCP.Gitlab.Token = "my.gitlab.key"
	secrets := fakeSecrets{
		"my.gitlab.key":                          "from-configured-mcp-key",
		config.TeamGitLabTokenKey("core"):        "from-core",
		config.MCPProjectTokenKey("gitlab", "p"): "from-project",
		"tracker.key":                            "from-tracker-key",
	}
	svc := New(cfg, secrets, nil)
	ctx := t.Context()

	tok, src := svc.GitLabToken(ctx, "core")
	assert.Equal(t, "from-core", tok)
	assert.Equal(t, tracker.TokenSourceTeam, src)

	tok, src = svc.GitLabToken(ctx, "ops")
	assert.Equal(t, "from-configured-mcp-key", tok, "the key of [mcp.gitlab] is read")
	assert.Equal(t, tracker.TokenSourceMCP, src)

	tok, src = svc.GitLabToken(ctx, "core", WithProject("p"))
	assert.Equal(t, "from-project", tok)
	assert.Equal(t, tracker.TokenSourceProject, src)

	tok, src = svc.GitLabToken(ctx, "core", WithProject("p"), WithTrackerKey("tracker.key"))
	assert.Equal(t, "from-tracker-key", tok)
	assert.Equal(t, tracker.TokenSourceTrackerKey, src)

	t.Setenv(tracker.GitLabTokenEnv, "from-env")
	tok, src = svc.GitLabToken(ctx, "core", WithProject("p"), WithTrackerKey("tracker.key"))
	assert.Equal(t, "from-env", tok)
	assert.Equal(t, tracker.TokenSourceEnv, src)

	_, src = New(cfg, nil, nil).GitLabToken(ctx, "core")
	assert.Equal(t, tracker.TokenSourceEnv, src, "without keychain only the environment is read")
}

// ── Tracker engine ───────────────────────────────────────────────────────────

// trackerEnv is a hub whose team core has a cloned team-state (a local git
// repository) and a GitLab fake server.
type trackerEnv struct {
	cfg      *config.Config
	repo     *teamstate.Repo
	server   *httptest.Server
	requests *atomic.Int32
}

func newTrackerEnv(t *testing.T, trackerTOML string) *trackerEnv {
	t.Helper()
	t.Setenv("OH_HOME", t.TempDir())
	t.Setenv(tracker.GitLabTokenEnv, "")
	t.Setenv("GITLAB_URL", "")

	n := &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("PRIVATE-TOKEN") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(srv.Close)

	statePath := filepath.Join(t.TempDir(), "state")
	repo, err := teamstate.InitSolo(t.Context(), statePath, teamstate.Member{ID: "alice"})
	require.NoError(t, err)
	if trackerTOML != "" {
		body := "[tracker]\n" + trackerTOML
		require.NoError(t, os.WriteFile(filepath.Join(statePath, "config.toml"), []byte(body), 0o644))
	}
	return &trackerEnv{
		cfg: &config.Config{Teams: []config.TeamConfig{
			{ID: "core", Enabled: true, StateRepo: "git@h:acme/core.git", StatePath: statePath, MemberID: "alice"},
		}},
		repo: repo, server: srv, requests: n,
	}
}

func (e *trackerEnv) service(projects ProjectLister, secrets tracker.SecretGetter) *Service {
	s := New(e.cfg, secrets, projects)
	s.HubDir = filepath.Join(filepath.Dir(e.repo.Path()), "hub")
	return s
}

func (e *trackerEnv) trackerTOML(extra string) string {
	return "type = \"gitlab\"\nenabled = true\ntracker_url = \"" + e.server.URL + "\"\ntracker_project = \"acme/app\"\n" + extra
}

func linkedProjects() fakeProjects { return fakeProjects{{ID: "p1", Name: "app"}} }

func TestTrackerEngineTypedErrors(t *testing.T) {
	t.Run("no team", func(t *testing.T) {
		_, err := New(&config.Config{}, nil, nil).TrackerEngine(t.Context(), nil)
		assert.ErrorIs(t, err, ErrTeamNotConfigured)
	})
	t.Run("team not enabled", func(t *testing.T) {
		cfg := &config.Config{Teams: []config.TeamConfig{{ID: "off", StateRepo: "r"}}}
		_, err := New(cfg, nil, nil).TrackerEngine(t.Context(), nil)
		assert.ErrorIs(t, err, ErrTeamNotConfigured)
	})
	t.Run("not cloned", func(t *testing.T) {
		cfg := &config.Config{Teams: []config.TeamConfig{{ID: "core", Enabled: true, StateRepo: "r", StatePath: filepath.Join(t.TempDir(), "nothing")}}}
		_, err := New(cfg, nil, nil).TrackerEngine(t.Context(), nil)
		assert.ErrorIs(t, err, ErrNotCloned)
	})
	t.Run("config.toml unreadable", func(t *testing.T) {
		e := newTrackerEnv(t, "")
		require.NoError(t, os.WriteFile(filepath.Join(e.repo.Path(), "config.toml"), []byte("[tracker\n"), 0o644))
		_, err := e.service(nil, nil).TrackerEngine(t.Context(), nil)
		var ce *ConfigError
		require.ErrorAs(t, err, &ce)
	})
	t.Run("no tracker type", func(t *testing.T) {
		e := newTrackerEnv(t, "")
		_, err := e.service(nil, nil).TrackerEngine(t.Context(), nil)
		assert.ErrorIs(t, err, ErrTrackerNotConfigured)
	})
	t.Run("disabled in the team config", func(t *testing.T) {
		e := newTrackerEnv(t, "type = \"gitlab\"\nenabled = false\n")
		_, err := e.service(nil, nil).TrackerEngine(t.Context(), nil)
		assert.ErrorIs(t, err, ErrTrackerDisabled)
	})
	t.Run("disabled locally in hub.toml", func(t *testing.T) {
		e := newTrackerEnv(t, "")
		e.writeTracker(t, "")
		e.cfg.Tracker.Enabled = boolPtr(false)
		_, err := e.service(linkedProjects(), fakeSecrets{}).TrackerEngine(t.Context(), nil)
		assert.ErrorIs(t, err, ErrTrackerDisabled)
		_, err = e.service(linkedProjects(), fakeSecrets{}).SyncTracker(t.Context(), SyncOptions{Pull: true})
		assert.ErrorIs(t, err, ErrTrackerDisabled, "the sync (CLI and screen) honors enabled = false")
		assert.Zero(t, e.requests.Load(), "nothing reaches the tracker")
	})
	t.Run("credentials missing", func(t *testing.T) {
		e := newTrackerEnv(t, "")
		e.writeTracker(t, "")
		_, err := e.service(linkedProjects(), fakeSecrets{}).TrackerEngine(t.Context(), nil)
		var ce *CredentialsError
		require.ErrorAs(t, err, &ce)
		assert.Equal(t, "gitlab", ce.TrackerType)
		assert.Equal(t, "GITLAB_TOKEN", ce.EnvVar)
	})
}

func (e *trackerEnv) writeTracker(t *testing.T, extra string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(e.repo.Path(), "config.toml"), []byte("[tracker]\n"+e.trackerTOML(extra)), 0o644))
}

func TestTrackerEngineTokenFromTheTeamKey(t *testing.T) {
	e := newTrackerEnv(t, "")
	e.writeTracker(t, "")
	secrets := fakeSecrets{config.TeamGitLabTokenKey("core"): "team-token"}

	eng, err := e.service(linkedProjects(), secrets).TrackerEngine(t.Context(), nil)
	require.NoError(t, err)
	assert.NotNil(t, eng)
}

func TestTrackerEngineWithoutLinkedProjectStillBuilds(t *testing.T) {
	e := newTrackerEnv(t, "")
	require.NoError(t, os.WriteFile(filepath.Join(e.repo.Path(), "config.toml"),
		[]byte("[tracker]\ntype = \"gitlab\"\nenabled = true\ntracker_url = \""+e.server.URL+"\"\n"), 0o644))
	secrets := fakeSecrets{config.TeamGitLabTokenKey("core"): "team-token"}

	eng, err := e.service(linkedProjects(), secrets).TrackerEngine(t.Context(), nil)
	require.NoError(t, err, "the auto engine still serves ticket details without a linked project")
	assert.NotNil(t, eng)

	_, err = e.service(linkedProjects(), secrets).SyncTracker(t.Context(), SyncOptions{})
	assert.ErrorIs(t, err, ErrNoProjects)
}

func TestSyncTrackerRunsAgainstTheFakeServer(t *testing.T) {
	e := newTrackerEnv(t, "")
	e.writeTracker(t, "")
	secrets := fakeSecrets{config.TeamGitLabTokenKey("core"): "team-token"}

	var started, finished string
	res, err := e.service(linkedProjects(), secrets).SyncTracker(t.Context(), SyncOptions{
		Pull: true,
		OnRun: func(trackerType string) func() {
			started = trackerType
			return func() { finished = trackerType }
		},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Len(t, res.Projects, 1)
	assert.Equal(t, "p1", res.Projects[0].ProjectID)
	assert.Equal(t, "acme/app", res.Projects[0].TrackerProject)
	assert.Equal(t, "gitlab", started)
	assert.Equal(t, "gitlab", finished)
}

func TestSyncTrackerProjectOverrideAndTeamOfTheProject(t *testing.T) {
	e := newTrackerEnv(t, "")
	e.writeTracker(t, "")
	e.cfg.Teams = append(e.cfg.Teams, config.TeamConfig{ID: "ops", Enabled: true, StateRepo: "git@h:acme/ops.git", StatePath: filepath.Join(t.TempDir(), "missing")})
	svc := e.service(fakeProjects{{ID: "p1", TrackerConfig: &domain.ProjectTrackerConfig{TrackerProject: "acme/other"}}}, fakeSecrets{config.TeamGitLabTokenKey("core"): "tok"})

	res, err := svc.SyncTracker(t.Context(), SyncOptions{})
	require.NoError(t, err)
	assert.Equal(t, "acme/other", res.Projects[0].TrackerProject, "the project override wins over the team default")

	// A project of the team ops (not cloned) uses ops, not the hub team.
	_, err = svc.SyncTracker(t.Context(), SyncOptions{Project: &domain.Project{TeamID: strPtr("ops")}})
	assert.ErrorIs(t, err, ErrNotCloned)

	// A project without team falls back on the hub team.
	res, err = svc.SyncTracker(t.Context(), SyncOptions{Project: &domain.Project{ID: "x"}})
	require.NoError(t, err)
	assert.NotNil(t, res)
}
