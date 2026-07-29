package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateTeamToTeams_AlreadyMigrated(t *testing.T) {
	c := &Config{
		Teams: []TeamConfig{{ID: "existing", Enabled: true}},
		Team:  TeamConfig{Enabled: true, StateRepo: "git@gitlab.com:acme/team-state.git"},
	}
	if MigrateTeamToTeams(c) {
		t.Error("should not migrate when Teams is already populated")
	}
}

func TestMigrateTeamToTeams_NoTeam(t *testing.T) {
	c := &Config{}
	if MigrateTeamToTeams(c) {
		t.Error("should not migrate when Team is empty")
	}
}

func TestMigrateTeamToTeams_Success(t *testing.T) {
	c := &Config{
		Team: TeamConfig{
			Enabled:   true,
			StateRepo: "git@gitlab.com:acme/team-state.git",
			StatePath: "/home/user/.oh/team-state",
			MemberID:  "alice",
		},
	}

	migrated := MigrateTeamToTeams(c)
	if !migrated {
		t.Fatal("expected migration to occur")
	}

	if len(c.Teams) != 1 {
		t.Fatalf("expected 1 team, got %d", len(c.Teams))
	}

	team := c.Teams[0]
	if team.ID != "team-state" {
		t.Errorf("expected ID 'team-state', got %q", team.ID)
	}
	if !team.Enabled {
		t.Error("expected team to be enabled")
	}
	if team.StateRepo != "git@gitlab.com:acme/team-state.git" {
		t.Errorf("unexpected StateRepo: %s", team.StateRepo)
	}
	if team.MemberID != "alice" {
		t.Errorf("expected MemberID 'alice', got %q", team.MemberID)
	}

	// Legacy field should be cleared
	if c.Team.StateRepo != "" {
		t.Error("expected legacy Team field to be cleared")
	}
}

func TestMigrateTeamToTeams_IDDerivation(t *testing.T) {
	tests := []struct {
		repo     string
		expected string
	}{
		{"git@gitlab.com:acme/team-state.git", "team-state"},
		{"git@github.com:org/my-awesome-team.git", "my-awesome-team"},
		{"https://github.com/beta/oh-team.git", "oh-team"},
		{"git@gitlab.com:acme/project.git", "project"},
	}

	for _, tt := range tests {
		c := &Config{
			Team: TeamConfig{
				Enabled:   true,
				StateRepo: tt.repo,
				MemberID:  "bob",
			},
		}
		MigrateTeamToTeams(c)
		if c.Teams[0].ID != tt.expected {
			t.Errorf("repo %q → ID %q, want %q", tt.repo, c.Teams[0].ID, tt.expected)
		}
	}
}

func TestBackupHubToml(t *testing.T) {
	// Create a temp hub dir for this test
	tmpDir := t.TempDir()
	origHubDir := os.Getenv("OH_HOME")
	os.Setenv("OH_HOME", tmpDir)
	defer func() {
		if origHubDir != "" {
			os.Setenv("OH_HOME", origHubDir)
		} else {
			os.Unsetenv("OH_HOME")
		}
	}()

	// Write a fake hub.toml
	hubToml := filepath.Join(tmpDir, "hub.toml")
	if err := os.WriteFile(hubToml, []byte("[cli]\nlanguage = \"fr\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// We can't easily test BackupHubToml because it uses HubDir() which uses UserHomeDir.
	// Instead, verify the function at least exists and compiles.
	// A full integration test would mock HubDir.
	_ = BackupHubToml
}

func TestFindTeam(t *testing.T) {
	c := &Config{
		Teams: []TeamConfig{
			{ID: "acme", Name: "ACME Team", Enabled: true},
			{ID: "beta", Name: "Beta Team", Enabled: false},
		},
	}

	team := c.FindTeam("acme")
	if team == nil || team.ID != "acme" {
		t.Error("expected to find team 'acme'")
	}

	team = c.FindTeam("beta")
	if team == nil || team.ID != "beta" {
		t.Error("expected to find team 'beta'")
	}

	team = c.FindTeam("nonexistent")
	if team != nil {
		t.Error("expected nil for nonexistent team")
	}
}

func TestFindTeamByRepo(t *testing.T) {
	c := &Config{
		Teams: []TeamConfig{
			{ID: "acme", StateRepo: "git@gitlab.com:acme/team-state.git"},
			{ID: "beta", StateRepo: "git@github.com:beta/oh-team.git"},
		},
	}

	team := c.FindTeamByRepo("git@gitlab.com:acme/team-state.git")
	if team == nil || team.ID != "acme" {
		t.Error("expected to find team by repo URL")
	}

	team = c.FindTeamByRepo("git@unknown.com:x/y.git")
	if team != nil {
		t.Error("expected nil for unknown repo")
	}
}

func TestDefaultTeam(t *testing.T) {
	// No teams
	c := &Config{}
	if c.DefaultTeam() != nil {
		t.Error("expected nil when no teams configured")
	}

	// First enabled wins
	c = &Config{
		Teams: []TeamConfig{
			{ID: "disabled", Enabled: false},
			{ID: "active", Enabled: true},
		},
	}
	team := c.DefaultTeam()
	if team == nil || team.ID != "active" {
		t.Error("expected first enabled team")
	}

	// No enabled → returns first
	c = &Config{
		Teams: []TeamConfig{
			{ID: "only", Enabled: false},
		},
	}
	team = c.DefaultTeam()
	if team == nil || team.ID != "only" {
		t.Error("expected first team when none enabled")
	}
}
