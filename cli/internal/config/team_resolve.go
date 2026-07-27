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
	// StateRepo is the Git remote URL of the team-state repository.
	StateRepo string
	// StatePath is the local filesystem path to the team-state clone.
	StatePath string
	// MemberID is the current user's member identifier in the team-state repo.
	MemberID string
}

// ResolveTeamConfig merges a project-level override with the hub-level TeamConfig.
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
