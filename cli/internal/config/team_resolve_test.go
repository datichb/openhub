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
		remote string
		want   string
	}{
		{"git@github.com:acme/team-state.git", "team-state"},
		{"git@gitlab.com:org/sub/my-team.git", "my-team"},
		{"git@host:repo.git", "repo"},
	}
	for _, c := range cases {
		got := config.TeamStatePath(c.remote)
		if !strings.HasSuffix(got, c.want) {
			t.Errorf("TeamStatePath(%q): want suffix %q, got %q", c.remote, c.want, got)
		}
	}
}

func TestTeamStatePath_HTTPS(t *testing.T) {
	got := config.TeamStatePath("https://github.com/acme/my-team.git")
	if !strings.HasSuffix(got, "my-team") {
		t.Errorf("expected suffix 'my-team', got %s", got)
	}
}

func TestTeamStatePath_IsInsideTeamStatesDir(t *testing.T) {
	got := config.TeamStatePath("git@github.com:acme/team-state.git")
	if !strings.Contains(got, "team-states") {
		t.Errorf("path should be inside team-states dir, got %s", got)
	}
}
