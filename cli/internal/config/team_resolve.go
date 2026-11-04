package config

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitremote"
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
	// GitLabURL is the GitLab server of the team-state repository when
	// StateRepo goes through an SSH alias (TeamConfig.GitLabURL); empty = the
	// host of StateRepo.
	GitLabURL string
	// MemberID is the current user's member identifier in the team-state repo.
	MemberID string
	// Solo marks a solo team-state (workflows only). Enabled is then false so
	// that team features (MCP team server, claims, board, events) stay off;
	// TeamID, StatePath and MemberID are still filled for the workflows.
	Solo bool
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
			Enabled:   team.Enabled && !team.Solo,
			TeamID:    team.ID,
			StateRepo: team.StateRepo,
			StatePath: ResolveStatePath(team.StateRepo, team.StatePath),
			GitLabURL: team.GitLabURL,
			MemberID:  team.MemberID,
			Solo:      team.Solo && team.Enabled,
		}
	}

	// Legacy compat: if project still uses old TeamConfig, fall through to old logic
	if project.TeamConfig != nil {
		return ResolveTeamConfig(cfg.ActiveTeam(), project.TeamConfig)
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
			StatePath: ResolveStatePath(hub.StateRepo, hub.StatePath),
			GitLabURL: hub.GitLabURL,
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

	return ResolvedTeamConfig{
		Enabled:   true,
		StateRepo: project.StateRepo,
		StatePath: ResolveStatePath(project.StateRepo, project.StatePath),
		MemberID:  memberID,
	}
}

// ResolveStatePath is the one rule that gives the local folder of a team-state
// clone, the folder oh reads and writes: hub.toml is normalised with it when
// it is read (readDisk), so that nothing downstream has a fallback of its own.
//
//   - state_path set: that folder, with "~" expanded to the home directory; a
//     relative path is under the hub folder (HubDir), never under the working
//     directory;
//   - state_path empty: TeamStatePath(stateRepo);
//   - state_path and state_repo empty: "" (no team-state to speak of: the
//     callers refuse an empty path, teamstate.NewRepo included).
func ResolveStatePath(stateRepo, statePath string) string {
	statePath = strings.TrimSpace(statePath)
	if statePath == "" {
		if strings.TrimSpace(stateRepo) == "" {
			return ""
		}
		return TeamStatePath(stateRepo)
	}
	if statePath == "~" || strings.HasPrefix(statePath, "~/") || strings.HasPrefix(statePath, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, statePath[1:])
		}
	}
	if !filepath.IsAbs(statePath) && !strings.HasPrefix(statePath, "/") && !strings.HasPrefix(statePath, `\`) {
		return filepath.Join(HubDir(), statePath)
	}
	return statePath
}

// TeamStatePath derives the local clone path of a team-state from its Git
// remote URL: <hub>/team-states/<host>/<path of the repository>, e.g.
//
//	"git@gitlab.com:acme/team-state.git"        → ~/.oh/team-states/gitlab.com/acme/team-state
//	"https://gitlab.com/beta/team-state.git"    → ~/.oh/team-states/gitlab.com/beta/team-state
//	"https://github.com/acme/my-team.git"       → ~/.oh/team-states/github.com/acme/my-team
//
// The whole path of the repository is used, not its name: two teams whose
// repositories are both called "team-state" (the name the documentation
// recommends) never share a folder. A local remote, which has no host, goes
// under "local/<name>-<hash of its path>".
//
// An existing clone of this very repository at a former location is kept, not
// cloned a second time: <host>/<name> (until 5.3.2), <name> (before 5.3) and
// ~/.oh/team-state (the first default). A folder there that holds another
// repository is never used: the new path is returned, whose own clone
// (teamstate.Repo.EnsureReady) refuses a folder of another repository.
func TeamStatePath(remoteURL string) string {
	base := filepath.Join(HubDir(), "team-states")
	newPath := filepath.Join(append([]string{base}, stateFolderParts(remoteURL)...)...)
	if gitremote.IsCloneOf(newPath, remoteURL) {
		return newPath
	}
	name := cleanSegment(repoNameFromRemote(remoteURL))
	former := []string{
		filepath.Join(base, hostFromRemote(remoteURL), name),
		filepath.Join(base, name),
		DefaultTeamStatePath(),
	}
	for _, p := range former {
		if p != newPath && gitremote.IsCloneOf(p, remoteURL) {
			return p
		}
	}
	return newPath
}

// stateFolderParts are the folders under team-states of the clone of remote:
// the host, then the segments of the path of the repository.
func stateFolderParts(remote string) []string {
	host, path, ok := gitremote.Split(remote)
	if !ok {
		sum := sha1.Sum([]byte(filepath.Clean(strings.TrimSpace(remote))))
		return []string{"local", cleanSegment(repoNameFromRemote(remote)) + "-" + hex.EncodeToString(sum[:4])}
	}
	parts := []string{cleanSegment(host)}
	for _, seg := range strings.Split(path, "/") {
		if seg != "" {
			parts = append(parts, cleanSegment(seg))
		}
	}
	if len(parts) == 1 {
		parts = append(parts, cleanSegment(repoNameFromRemote(remote)))
	}
	return parts
}

// cleanSegment makes s a folder name on every system: no separator, no
// character Windows refuses, no trailing dot or space, never "." or "..".
func cleanSegment(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < ' ' || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, s)
	s = strings.TrimRight(s, ". ")
	if s == "" {
		return "_"
	}
	return s
}

// HostFromRemote is the exported version of hostFromRemote.
// It extracts the hostname from a Git remote URL.
func HostFromRemote(remote string) string {
	return hostFromRemote(remote)
}

// hostFromRemote extracts the hostname from a Git remote URL.
// Supports SCP-style (git@host:path) and standard URLs (https://host/path).
func hostFromRemote(remote string) string {
	// SCP-style: git@gitlab.com:org/repo.git → gitlab.com
	if idx := strings.Index(remote, "@"); idx != -1 {
		rest := remote[idx+1:]
		if colonIdx := strings.Index(rest, ":"); colonIdx != -1 {
			return rest[:colonIdx]
		}
	}
	// HTTP(S) or SSH URL
	if u, err := url.Parse(remote); err == nil && u.Host != "" {
		return u.Hostname()
	}
	return "local"
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

	// Local path on Windows (C:\...\repo.git): url.Parse reads the drive as
	// a scheme, so take the base name as for a URL path.
	if filepath.VolumeName(remote) != "" {
		return strings.TrimSuffix(filepath.Base(remote), ".git")
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
