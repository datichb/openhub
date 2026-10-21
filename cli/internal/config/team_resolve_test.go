package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
)

var hubTeam = config.TeamConfig{
	Enabled:   true,
	StateRepo: "git@gitlab.com:acme/team-state.git",
	StatePath: "/home/user/.oh/team-state",
	MemberID:  "alice",
}

func TestResolveTeamConfig(t *testing.T) {
	t.Parallel()
	custom := func(repo, member, path string) *domain.ProjectTeamConfig {
		return &domain.ProjectTeamConfig{Mode: domain.ProjectTeamModeCustom, StateRepo: repo, MemberID: member, StatePath: path}
	}
	tests := []struct {
		name        string
		proj        *domain.ProjectTeamConfig
		wantEnabled bool
		wantRepo    string
		wantMember  string
		wantPath    string // "" : not checked; "~name": contains name
	}{
		{"no project config inherits the hub", nil, true, hubTeam.StateRepo, "alice", hubTeam.StatePath},
		{"inherit mode mirrors the hub", &domain.ProjectTeamConfig{Mode: domain.ProjectTeamModeInherit}, true, hubTeam.StateRepo, "alice", hubTeam.StatePath},
		{"disabled mode wins over an enabled hub", &domain.ProjectTeamConfig{Mode: domain.ProjectTeamModeDisabled}, false, "", "", ""},
		{"custom mode overrides the repo and the member", custom("git@github.com:beta/other-team.git", "bob", ""), true, "git@github.com:beta/other-team.git", "bob", "~other-team"},
		{"custom mode falls back to the member of the hub", custom("git@github.com:beta/other-team.git", "", ""), true, "git@github.com:beta/other-team.git", "alice", ""},
		{"custom mode derives the state path from the repo", custom("git@github.com:acme/my-team.git", "", ""), true, "", "", "~my-team"},
		{"custom mode keeps an explicit state path", custom("git@github.com:acme/my-team.git", "", "/custom/path"), true, "", "", "/custom/path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.ResolveTeamConfig(hubTeam, tt.proj)
			assert.Equal(t, tt.wantEnabled, got.Enabled, "enabled")
			if !tt.wantEnabled {
				return
			}
			if tt.wantRepo != "" {
				assert.Equal(t, tt.wantRepo, got.StateRepo, "state repo")
			}
			if tt.wantMember != "" {
				assert.Equal(t, tt.wantMember, got.MemberID, "member")
			}
			if name, ok := strings.CutPrefix(tt.wantPath, "~"); ok {
				assert.Contains(t, got.StatePath, name, "state path")
			} else if tt.wantPath != "" {
				assert.Equal(t, tt.wantPath, got.StatePath, "state path")
			}
		})
	}
}

func TestTeamStatePath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		remote   string
		wantHost string
		wantName string
	}{
		{"scp style on github", "git@github.com:acme/team-state.git", "github.com", "team-state"},
		{"scp style with a group path", "git@gitlab.com:org/sub/my-team.git", "gitlab.com", "my-team"},
		{"scp style with no group", "git@host:repo.git", "host", "repo"},
		{"https", "https://github.com/acme/my-team.git", "github.com", "my-team"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.TeamStatePath(tt.remote)
			assert.Contains(t, got, "team-states", "the clone is inside the team-states folder")
			assert.Contains(t, got, tt.wantHost, "the host is part of the path")
			assert.True(t, strings.HasSuffix(got, tt.wantName), "path %q ends with %q", got, tt.wantName)
		})
	}

	assert.NotEqual(t, config.TeamStatePath("git@gitlab.com:acme/state.git"), config.TeamStatePath("git@github.com:other/state.git"),
		"the same repo name on two hosts never collides")
}

func TestResolveTeamForProject(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Teams: []config.TeamConfig{
		{ID: "acme", Enabled: true, StateRepo: "git@gitlab.com:acme/ts.git", StatePath: "/path/acme", MemberID: "alice"},
		{ID: "beta", Enabled: true, StateRepo: "git@github.com:beta/ts.git", StatePath: "/path/beta", MemberID: "bob"},
		{ID: "off", Enabled: false, StateRepo: "git@gitlab.com:off/ts.git", MemberID: "carol"},
	}}
	id := func(s string) *string { return &s }
	tests := []struct {
		name        string
		teamID      *string
		wantEnabled bool
		wantTeamID  string
		wantMember  string
		wantPath    string
	}{
		{"no team: a solo project", nil, false, "", "", ""},
		{"an empty team id is no team", id(""), false, "", "", ""},
		{"a team of the config", id("beta"), true, "beta", "bob", "/path/beta"},
		{"an unknown team disables gracefully", id("nonexistent"), false, "", "", ""},
		{"a disabled team stays disabled but keeps its id", id("off"), false, "off", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.ResolveTeamForProject(cfg, &domain.Project{ID: "proj1", TeamID: tt.teamID})
			assert.Equal(t, tt.wantEnabled, got.Enabled, "enabled")
			assert.Equal(t, tt.wantTeamID, got.TeamID, "team id")
			if tt.wantEnabled {
				assert.Equal(t, tt.wantMember, got.MemberID, "member")
				assert.Equal(t, tt.wantPath, got.StatePath, "state path")
			}
		})
	}
}

func TestHostFromRemote(t *testing.T) {
	t.Parallel()
	cases := []struct {
		remote string
		want   string
	}{
		{"git@gitlab.com:acme/repo.git", "gitlab.com"},
		{"git@github.com:org/state.git", "github.com"},
		{"https://github.com/acme/repo.git", "github.com"},
		{"https://gitlab.company.io/team/state.git", "gitlab.company.io"},
		{"ssh://git@bitbucket.org/acme/repo.git", "bitbucket.org"},
		{"invalid", "local"},
	}
	for _, c := range cases {
		got := config.HostFromRemote(c.remote)
		if got != c.want {
			t.Errorf("HostFromRemote(%q): want %q, got %q", c.remote, c.want, got)
		}
	}
}

// --- Legacy (project-level TeamConfig) ---

func TestResolveTeamForProject_LegacyFallback(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Team: config.TeamConfig{
			ID: "legacy", Enabled: true, StateRepo: "git@gitlab.com:old/ts.git",
			StatePath: "/old/path", MemberID: "charlie",
		},
	}
	project := &domain.Project{
		ID:     "proj1",
		TeamID: nil,
		TeamConfig: &domain.ProjectTeamConfig{
			Mode: domain.ProjectTeamModeInherit,
		},
	}

	got := config.ResolveTeamForProject(cfg, project)
	if !got.Enabled {
		t.Fatal("legacy inherit mode should resolve to hub team")
	}
	if got.MemberID != "charlie" {
		t.Errorf("expected MemberID 'charlie' from legacy hub, got %q", got.MemberID)
	}
}

// ─── Legacy TeamConfig migration tests ─────────────────────────────────────

func TestResolveTeamForProject_LegacyInherit(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Teams: []config.TeamConfig{
			{ID: "acme", Enabled: true, StateRepo: "git@gitlab.com:acme/ts.git", StatePath: "/tmp/ts", MemberID: "alice"},
		},
	}
	// TeamID is nil, but legacy TeamConfig exists with inherit mode
	project := &domain.Project{
		ID:     "proj-legacy",
		TeamID: nil,
		TeamConfig: &domain.ProjectTeamConfig{
			Mode: "inherit",
		},
	}

	got := config.ResolveTeamForProject(cfg, project)
	if !got.Enabled {
		t.Fatal("legacy inherit should resolve to the active team")
	}
	if got.MemberID != "alice" {
		t.Fatalf("expected MemberID=alice, got %s", got.MemberID)
	}
	if got.StateRepo != "git@gitlab.com:acme/ts.git" {
		t.Fatalf("expected inherited StateRepo, got %s", got.StateRepo)
	}
}

func TestResolveTeamForProject_LegacyDisabled(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Teams: []config.TeamConfig{
			{ID: "acme", Enabled: true, StateRepo: "git@gitlab.com:acme/ts.git", MemberID: "alice"},
		},
	}
	project := &domain.Project{
		ID:     "proj-solo",
		TeamID: nil,
		TeamConfig: &domain.ProjectTeamConfig{
			Mode: "disabled",
		},
	}

	got := config.ResolveTeamForProject(cfg, project)
	if got.Enabled {
		t.Fatal("legacy disabled mode should resolve to disabled")
	}
}

func TestResolveTeamForProject_LegacyCustom(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Teams: []config.TeamConfig{
			{ID: "acme", Enabled: true, StateRepo: "git@gitlab.com:acme/ts.git", MemberID: "alice"},
		},
	}
	project := &domain.Project{
		ID:     "proj-custom",
		TeamID: nil,
		TeamConfig: &domain.ProjectTeamConfig{
			Mode:      "custom",
			StateRepo: "git@gitlab.com:other/custom-ts.git",
			MemberID:  "bob",
		},
	}

	got := config.ResolveTeamForProject(cfg, project)
	if !got.Enabled {
		t.Fatal("legacy custom should be enabled")
	}
	if got.MemberID != "bob" {
		t.Fatalf("expected custom MemberID=bob, got %s", got.MemberID)
	}
	if got.StateRepo != "git@gitlab.com:other/custom-ts.git" {
		t.Fatalf("expected custom StateRepo, got %s", got.StateRepo)
	}
}

// A3: team-state clones follow OH_HOME (isolated environments) and still
// find a former clone without the host level.
func TestTeamStatePath_UnderOHHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	got := config.TeamStatePath("git@gitlab.com:acme/team-state.git")
	want := filepath.Join(home, "team-states", "gitlab.com", "team-state")
	if got != want {
		t.Fatalf("TeamStatePath = %q, want %q", got, want)
	}
	legacy := filepath.Join(home, "team-states", "team-state")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := config.TeamStatePath("git@gitlab.com:acme/team-state.git"); got != legacy {
		t.Fatalf("TeamStatePath = %q, want the former clone %q", got, legacy)
	}
}
