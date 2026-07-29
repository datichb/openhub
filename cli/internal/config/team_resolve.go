package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
)

// ResolvedTeamConfig is the effective team configuration for a project after
// applying the project-level override on top of the hub-level defaults.
type ResolvedTeamConfig struct {
	// Enabled reports whether team features are active for this project.
	Enabled bool
	// TeamID is the resolved team identifier (from Config.Teams[].ID).
	TeamID string
	// StateRepo is the Git remote URL of the team-state repository.
	StateRepo string
	// StatePath is the local filesystem path to the team-state clone.
	StatePath string
	// MemberID is the current user's member identifier in the team-state repo.
	MemberID string
}

// ResolveTeamForProject resolves the effective team configuration for a project
// using the new TeamID-based model (ADR-029).
//
// Resolution rules:
//   - project.TeamID == nil → solo project, team features disabled
//   - project.TeamID points to a valid teams[].id → use that team config
//   - project.TeamID points to unknown ID → team features disabled (graceful)
func ResolveTeamForProject(cfg *Config, project *domain.Project) ResolvedTeamConfig {
	// New model: TeamID-based lookup
	if project.TeamID != nil && *project.TeamID != "" {
		team := cfg.FindTeam(*project.TeamID)
		if team == nil {
			// Unknown team ID — graceful degradation
			return ResolvedTeamConfig{Enabled: false}
		}
		return ResolvedTeamConfig{
			Enabled:   team.Enabled,
			TeamID:    team.ID,
			StateRepo: team.StateRepo,
			StatePath: team.StatePath,
			MemberID:  team.MemberID,
		}
	}

	// Legacy compat: if project still uses old TeamConfig, fall through to old logic
	if project.TeamConfig != nil {
		return ResolveTeamConfig(cfg.Team, project.TeamConfig)
	}

	// No team affiliation
	return ResolvedTeamConfig{Enabled: false}
}

// ResolveTeamConfig merges a project-level override with the hub-level TeamConfig.
// This is the legacy resolution function kept for backward-compat with projects
// that still have a ProjectTeamConfig (not yet migrated to TeamID).
//
// Resolution rules:
//   - nil or Mode == "inherit" → returns the hub config as-is
//   - Mode == "disabled"       → returns ResolvedTeamConfig{Enabled: false}
//   - Mode == "custom"         → uses the project fields; MemberID falls back to hub
//     when empty; StatePath is auto-derived from StateRepo when empty
func ResolveTeamConfig(hub TeamConfig, project *domain.ProjectTeamConfig) ResolvedTeamConfig {
	if project == nil || project.Mode == "" || project.Mode == domain.ProjectTeamModeInherit {
		return ResolvedTeamConfig{
			Enabled:   hub.Enabled,
			TeamID:    hub.ID,
			StateRepo: hub.StateRepo,
			StatePath: hub.StatePath,
			MemberID:  hub.MemberID,
		}
	}

	if project.Mode == domain.ProjectTeamModeDisabled {
		return ResolvedTeamConfig{Enabled: false}
	}

	// Mode == "custom"
	memberID := project.MemberID
	if memberID == "" {
		memberID = hub.MemberID
	}

	statePath := project.StatePath
	if statePath == "" && project.StateRepo != "" {
		statePath = TeamStatePath(project.StateRepo)
	}

	return ResolvedTeamConfig{
		Enabled:   true,
		StateRepo: project.StateRepo,
		StatePath: statePath,
		MemberID:  memberID,
	}
}

// TeamStatePath derives a local clone path from a Git remote URL.
// The result is always inside ~/.oh/team-states/<repo-name> so that projects
// using different team-state repos each get their own isolated clone.
//
// Examples:
//
//	"git@gitlab.com:acme/team-state.git" → "~/.oh/team-states/team-state"
//	"https://github.com/acme/my-team.git" → "~/.oh/team-states/my-team"
func TeamStatePath(remoteURL string) string {
	name := repoNameFromRemote(remoteURL)
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".oh", "team-states", name)
}

// RepoNameFromRemote is the exported version of repoNameFromRemote.
// It extracts the repository base name from a remote URL, useful for
// deriving team IDs from Git remote URLs.
func RepoNameFromRemote(remote string) string {
	return repoNameFromRemote(remote)
}

// repoNameFromRemote extracts the repository base name from a remote URL.
// Supports SCP-style Git remotes (git@host:path/repo.git) and HTTP(S) URLs.
func repoNameFromRemote(remote string) string {
	// SCP-style: git@github.com:org/repo.git
	if strings.HasPrefix(remote, "git@") {
		if idx := strings.LastIndex(remote, "/"); idx != -1 {
			remote = remote[idx+1:]
		} else if idx := strings.Index(remote, ":"); idx != -1 {
			remote = remote[idx+1:]
		}
		return strings.TrimSuffix(remote, ".git")
	}

	// HTTP(S) URL
	u, err := url.Parse(remote)
	if err == nil && u.Path != "" {
		base := filepath.Base(u.Path)
		return strings.TrimSuffix(base, ".git")
	}

	// Fallback: sanitize the whole string
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, remote)
	if safe == "" {
		safe = "team-state"
	}
	return fmt.Sprintf("team-state-%s", safe)
}
