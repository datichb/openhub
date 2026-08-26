package config_test

import (
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
)

var hubTeam = config.TeamConfig{
	Enabled:   true,
	StateRepo: "git@gitlab.com:acme/team-state.git",
	StatePath: "/home/user/.oh/team-state",
	MemberID:  "alice",
}

func TestResolveTeamConfig_NilInheritsHub(t *testing.T) {
	got := config.ResolveTeamConfig(hubTeam, nil)
	if !got.Enabled || got.MemberID != "alice" || got.StateRepo != hubTeam.StateRepo {
		t.Fatalf("nil project should inherit hub, got %+v", got)
	}
}

func TestResolveTeamConfig_InheritMode(t *testing.T) {
	proj := &domain.ProjectTeamConfig{Mode: domain.ProjectTeamModeInherit}
	got := config.ResolveTeamConfig(hubTeam, proj)
	if !got.Enabled || got.MemberID != "alice" {
		t.Fatalf("inherit mode should mirror hub, got %+v", got)
	}
}

func TestResolveTeamConfig_DisabledMode(t *testing.T) {
	proj := &domain.ProjectTeamConfig{Mode: domain.ProjectTeamModeDisabled}
	got := config.ResolveTeamConfig(hubTeam, proj)
	if got.Enabled {
		t.Fatal("disabled mode must return Enabled=false")
	}
}

func TestResolveTeamConfig_DisabledOverridesHubEnabled(t *testing.T) {
	hub := hubTeam
	hub.Enabled = true
	proj := &domain.ProjectTeamConfig{Mode: domain.ProjectTeamModeDisabled}
	got := config.ResolveTeamConfig(hub, proj)
	if got.Enabled {
		t.Fatal("disabled project must win over enabled hub")
	}
}

func TestResolveTeamConfig_CustomOverridesRepo(t *testing.T) {
	proj := &domain.ProjectTeamConfig{
		Mode:      domain.ProjectTeamModeCustom,
		StateRepo: "git@github.com:beta/other-team.git",
		MemberID:  "bob",
	}
	got := config.ResolveTeamConfig(hubTeam, proj)
	if !got.Enabled {
		t.Fatal("custom mode must be enabled")
	}
	if got.StateRepo != proj.StateRepo {
		t.Errorf("expected StateRepo %s, got %s", proj.StateRepo, got.StateRepo)
	}
	if got.MemberID != "bob" {
		t.Errorf("expected MemberID bob, got %s", got.MemberID)
	}
}

func TestResolveTeamConfig_CustomFallsBackToHubMemberID(t *testing.T) {
	proj := &domain.ProjectTeamConfig{
		Mode:      domain.ProjectTeamModeCustom,
		StateRepo: "git@github.com:beta/other-team.git",
		// MemberID intentionally empty → should fall back to hub
	}
	got := config.ResolveTeamConfig(hubTeam, proj)
	if got.MemberID != "alice" {
		t.Errorf("expected hub fallback MemberID alice, got %s", got.MemberID)
	}
}

func TestResolveTeamConfig_CustomAutoStatePath(t *testing.T) {
	proj := &domain.ProjectTeamConfig{
		Mode:      domain.ProjectTeamModeCustom,
		StateRepo: "git@github.com:acme/my-team.git",
		// StatePath empty → auto-derived
	}
	got := config.ResolveTeamConfig(hubTeam, proj)
	if !strings.Contains(got.StatePath, "my-team") {
		t.Errorf("expected StatePath to contain repo name 'my-team', got %s", got.StatePath)
	}
}

func TestResolveTeamConfig_CustomExplicitStatePath(t *testing.T) {
	proj := &domain.ProjectTeamConfig{
		Mode:      domain.ProjectTeamModeCustom,
		StateRepo: "git@github.com:acme/my-team.git",
		StatePath: "/custom/path",
	}
	got := config.ResolveTeamConfig(hubTeam, proj)
	if got.StatePath != "/custom/path" {
		t.Errorf("expected explicit StatePath, got %s", got.StatePath)
	}
}

func TestTeamStatePath_SCPStyle(t *testing.T) {
	cases := []struct {
		remote   string
		wantHost string
		wantName string
	}{
		{"git@github.com:acme/team-state.git", "github.com", "team-state"},
		{"git@gitlab.com:org/sub/my-team.git", "gitlab.com", "my-team"},
		{"git@host:repo.git", "host", "repo"},
	}
	for _, c := range cases {
		got := config.TeamStatePath(c.remote)
		if !strings.Contains(got, c.wantHost) {
			t.Errorf("TeamStatePath(%q): want host %q in path, got %q", c.remote, c.wantHost, got)
		}
		if !strings.HasSuffix(got, c.wantName) {
			t.Errorf("TeamStatePath(%q): want suffix %q, got %q", c.remote, c.wantName, got)
		}
	}
}

func TestTeamStatePath_HTTPS(t *testing.T) {
	got := config.TeamStatePath("https://github.com/acme/my-team.git")
	if !strings.HasSuffix(got, "my-team") {
		t.Errorf("expected suffix 'my-team', got %s", got)
	}
	if !strings.Contains(got, "github.com") {
		t.Errorf("expected host 'github.com' in path, got %s", got)
	}
}

func TestTeamStatePath_IsInsideTeamStatesDir(t *testing.T) {
	got := config.TeamStatePath("git@github.com:acme/team-state.git")
	if !strings.Contains(got, "team-states") {
		t.Errorf("path should be inside team-states dir, got %s", got)
	}
}

func TestTeamStatePath_DifferentHostsNeverCollide(t *testing.T) {
	path1 := config.TeamStatePath("git@gitlab.com:acme/state.git")
	path2 := config.TeamStatePath("git@github.com:other/state.git")
	if path1 == path2 {
		t.Errorf("paths should differ for different hosts: both resolved to %s", path1)
	}
}

func TestHostFromRemote(t *testing.T) {
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

// --- Tests for ResolveTeamForProject (new multi-team model) ---

func TestResolveTeamForProject_NilTeamID(t *testing.T) {
	cfg := &config.Config{
		Teams: []config.TeamConfig{
			{ID: "acme", Enabled: true, StateRepo: "git@gitlab.com:acme/ts.git", MemberID: "alice"},
		},
	}
	project := &domain.Project{ID: "proj1", TeamID: nil}

	got := config.ResolveTeamForProject(cfg, project)
	if got.Enabled {
		t.Fatal("nil TeamID should mean solo project → disabled")
	}
}

func TestResolveTeamForProject_ValidTeamID(t *testing.T) {
	cfg := &config.Config{
		Teams: []config.TeamConfig{
			{ID: "acme", Enabled: true, StateRepo: "git@gitlab.com:acme/ts.git", StatePath: "/path/acme", MemberID: "alice"},
			{ID: "beta", Enabled: true, StateRepo: "git@github.com:beta/ts.git", StatePath: "/path/beta", MemberID: "bob"},
		},
	}
	teamID := "beta"
	project := &domain.Project{ID: "proj1", TeamID: &teamID}

	got := config.ResolveTeamForProject(cfg, project)
	if !got.Enabled {
		t.Fatal("expected enabled")
	}
	if got.TeamID != "beta" {
		t.Errorf("expected TeamID 'beta', got %q", got.TeamID)
	}
	if got.MemberID != "bob" {
		t.Errorf("expected MemberID 'bob', got %q", got.MemberID)
	}
	if got.StatePath != "/path/beta" {
		t.Errorf("expected StatePath /path/beta, got %q", got.StatePath)
	}
}

func TestResolveTeamForProject_UnknownTeamID(t *testing.T) {
	cfg := &config.Config{
		Teams: []config.TeamConfig{
			{ID: "acme", Enabled: true, StateRepo: "git@gitlab.com:acme/ts.git", MemberID: "alice"},
		},
	}
	teamID := "nonexistent"
	project := &domain.Project{ID: "proj1", TeamID: &teamID}

	got := config.ResolveTeamForProject(cfg, project)
	if got.Enabled {
		t.Fatal("unknown TeamID should gracefully disable")
	}
}

func TestResolveTeamForProject_DisabledTeam(t *testing.T) {
	cfg := &config.Config{
		Teams: []config.TeamConfig{
			{ID: "acme", Enabled: false, StateRepo: "git@gitlab.com:acme/ts.git", MemberID: "alice"},
		},
	}
	teamID := "acme"
	project := &domain.Project{ID: "proj1", TeamID: &teamID}

	got := config.ResolveTeamForProject(cfg, project)
	if got.Enabled {
		t.Fatal("disabled team should propagate Enabled=false")
	}
	if got.TeamID != "acme" {
		t.Errorf("TeamID should still be set even if disabled, got %q", got.TeamID)
	}
}

func TestResolveTeamForProject_LegacyFallback(t *testing.T) {
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

func TestResolveTeamForProject_EmptyTeamID(t *testing.T) {
	cfg := &config.Config{
		Teams: []config.TeamConfig{
			{ID: "acme", Enabled: true, StateRepo: "git@gitlab.com:acme/ts.git", MemberID: "alice"},
		},
	}
	emptyID := ""
	project := &domain.Project{ID: "proj1", TeamID: &emptyID}

	got := config.ResolveTeamForProject(cfg, project)
	if got.Enabled {
		t.Fatal("empty string TeamID should be treated as no team")
	}
}
