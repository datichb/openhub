package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// teamRejoinParams holds the parameters for the core team rejoin logic.
type teamRejoinParams struct {
	StateRepo string // Git remote URL of the team-state repo
	StatePath string // local clone path (empty = use default)
	MemberID  string // ID of the existing member to rejoin as
}

// teamRejoinResult holds the output of a successful rejoin operation.
type teamRejoinResult struct {
	Member       teamstate.Member
	TeamID       string
	TeamName     string
	EventCount   int  // number of session.complete events found for this member
	SessionCount int  // number of local sessions retro-tagged
	StaleData    bool // true when pull failed and local (possibly outdated) content was used
}

// teamRejoinCore performs the core rejoin operations:
//  1. Clone/pull the team-state repo
//  2. Verify that the member exists
//  3. Validate identity via GitLab token (if available)
//  4. Write hub.toml with the team config
//
// It does NOT display any UI — callers handle presentation.
func teamRejoinCore(ctx context.Context, a *app.App, p teamRejoinParams) (*teamRejoinResult, error) {
	statePath := p.StatePath
	if statePath == "" {
		statePath = config.TeamStatePath(p.StateRepo)
	}

	repo := teamstate.NewRepo(p.StateRepo, statePath)

	// Ensure repo is ready (clone if needed, pull if already cloned).
	// A PullWarning means the repo is cloned but pull failed (e.g. offline);
	// proceed with local content rather than aborting.
	var staleData bool
	if err := repo.EnsureReady(ctx); err != nil {
		if !teamstate.IsPullWarning(err) {
			return nil, fmt.Errorf("cloning team-state: %w", err)
		}
		slog.Warn("team-state sync warning (using local content)", "error", err)
		staleData = true
	}

	// Verify that the member exists in the repo
	member, err := repo.GetMember(p.MemberID)
	if err != nil {
		return nil, fmt.Errorf("member %q not found in team-state repository", p.MemberID)
	}

	// Validate identity via GitLab token (if available)
	if err := validateGitLabIdentity(ctx, a, repo, member); err != nil {
		return nil, err
	}

	// Persist team config to hub.toml
	teamID := config.RepoNameFromRemote(p.StateRepo)
	newTeam := config.TeamConfig{
		ID:        teamID,
		Enabled:   true,
		StateRepo: p.StateRepo,
		StatePath: statePath,
		MemberID:  p.MemberID,
	}

	// Check if this team already exists in the config (update it) or add new
	found := false
	for i, t := range a.Config.Teams {
		if t.StateRepo == p.StateRepo || t.ID == teamID {
			a.Config.Teams[i] = newTeam
			found = true
			break
		}
	}
	if !found {
		a.Config.Teams = append(a.Config.Teams, newTeam)
	}
	// Clear legacy field
	a.Config.Team = config.TeamConfig{}

	if err := config.Save(a.Config); err != nil {
		return nil, fmt.Errorf("writing hub.toml: %w", err)
	}

	// Count team events for this member (for summary display)
	eventCount := 0
	events, err := repo.ListEvents("", time.Time{})
	if err == nil {
		for _, e := range events {
			if e.Actor == member.ID && e.Type == teamstate.EventSessionComplete {
				eventCount++
			}
		}
	}

	return &teamRejoinResult{
		Member:     *member,
		TeamID:     teamID,
		TeamName:   newTeam.DisplayName(),
		EventCount: eventCount,
		StaleData:  staleData,
	}, nil
}

// validateGitLabIdentity verifies that the current GitLab token belongs to the
// member being rejoined. This prevents identity spoofing.
// Returns nil if validation succeeds or if no GitLab token is available (with warning).
func validateGitLabIdentity(ctx context.Context, a *app.App, repo *teamstate.Repo, member *teamstate.Member) error {
	if member.GitLabUsername == "" {
		// Member has no GitLab username configured — skip validation
		return nil
	}

	// Build credential source from the team-state config (shared MCP + tracker)
	// so that the correct GitLab instance URL and token are resolved.
	var sharedMCP map[string]teamstate.SharedMCPConfig
	var trackerCfg *teamstate.TrackerConfig
	if teamCfg, err := repo.LoadConfig(); err == nil && teamCfg != nil {
		sharedMCP = teamCfg.MCP
		trackerCfg = &teamCfg.Tracker
	}
	src := buildCredentialSource(a, sharedMCP, trackerCfg)

	cfg, err := tracker.ResolveCredentials(ctx, src, tracker.TypeGitLab)
	if err != nil {
		// No token available — cannot validate, allow proceeding with a warning
		slog.Warn("GitLab identity validation skipped: no token available",
			"member", member.ID,
			"gitlab_username", member.GitLabUsername,
			"error", err,
		)
		return nil
	}

	// Call GitLab API to get the authenticated user
	t, err := tracker.New(cfg)
	if err != nil {
		return fmt.Errorf("creating GitLab client: %w", err)
	}
	username, err := t.TestConnection(ctx)
	if err != nil {
		return fmt.Errorf("GitLab authentication failed: %w", err)
	}

	// Compare usernames (case-insensitive)
	if !strings.EqualFold(username, member.GitLabUsername) {
		return fmt.Errorf(
			"identité GitLab invalide : le token appartient à %q mais le membre sélectionné (%s) a le username GitLab %q",
			username, member.ID, member.GitLabUsername,
		)
	}

	return nil
}

// listTeamMembers clones/pulls the team-state repo and returns the list of members.
// Used by the rejoin wizard to display the member selection list.
// The stale return value is true when the pull failed and local (possibly outdated) content was used.
func listTeamMembers(ctx context.Context, stateRepo, statePath string) (*teamstate.Repo, []teamstate.Member, bool, error) {
	if statePath == "" {
		statePath = config.TeamStatePath(stateRepo)
	}

	repo := teamstate.NewRepo(stateRepo, statePath)
	var stale bool
	if err := repo.EnsureReady(ctx); err != nil {
		if !teamstate.IsPullWarning(err) {
			return nil, nil, false, fmt.Errorf("cloning team-state: %w", err)
		}
		slog.Warn("team-state sync warning (using local content)", "error", err)
		stale = true
	}

	members, err := repo.ListMembers()
	if err != nil {
		return nil, nil, false, fmt.Errorf("listing members: %w", err)
	}

	return repo, members, stale, nil
}

// retroTagSessions updates existing sessions and agent_events that have no member_id
// with the given member ID. Returns the number of sessions updated.
func retroTagSessions(ctx context.Context, memberID string) (int, error) {
	if store == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	db := store.DB()

	// Update sessions
	result, err := db.ExecContext(ctx, `UPDATE sessions SET member_id = ? WHERE member_id IS NULL`, memberID)
	if err != nil {
		return 0, fmt.Errorf("retro-tagging sessions: %w", err)
	}
	sessionsUpdated, _ := result.RowsAffected()

	// Update agent_events
	if _, err := db.ExecContext(ctx, `UPDATE agent_events SET member_id = ? WHERE member_id IS NULL`, memberID); err != nil {
		return int(sessionsUpdated), fmt.Errorf("retro-tagging agent events: %w", err)
	}

	return int(sessionsUpdated), nil
}
