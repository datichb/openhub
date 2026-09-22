package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
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
	Member           teamstate.Member
	TeamID           string
	TeamName         string
	EventCount       int                    // number of session.complete events found for this member
	SessionCount     int                    // number of local sessions retro-tagged
	StaleData        bool                   // true when pull failed and local (possibly outdated) content was used
	IdentityMismatch *identityMismatchError // non-nil when GitLab identity verification detected a mismatch
}

// identityMismatchError is returned by validateGitLabIdentity when the
// authenticated GitLab user does not match the expected member username.
type identityMismatchError struct {
	AuthenticatedAs string // username returned by GET /api/v4/user (e.g. "project_1772_bot_...")
	ExpectedUser    string // the member's gitlab_username (e.g. "benjamin.datiche1")
	MemberID        string // the member ID (e.g. "bdatiche")
	IsBot           bool   // true when the token is a Project/Group Access Token
}

func (e *identityMismatchError) Error() string {
	if e.IsBot {
		return fmt.Sprintf(
			"le token appartient à un bot (%s), pas à un compte personnel — la vérification nécessite un Personal Access Token (PAT)",
			e.AuthenticatedAs,
		)
	}
	return fmt.Sprintf(
		"le token appartient à %q mais le membre sélectionné (%s) a le username GitLab %q",
		e.AuthenticatedAs, e.MemberID, e.ExpectedUser,
	)
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
	var identityMismatch *identityMismatchError
	if err := validateGitLabIdentity(ctx, a, repo, member); err != nil {
		if errors.As(err, &identityMismatch) {
			// Identity mismatch — store for caller to handle interactively.
			// Don't block the rejoin; the caller will show a step for the user
			// to provide a PAT or skip verification.
			slog.Warn("GitLab identity mismatch (will prompt user)",
				"member", member.ID,
				"authenticated_as", identityMismatch.AuthenticatedAs,
				"expected", identityMismatch.ExpectedUser,
				"is_bot", identityMismatch.IsBot,
			)
		} else {
			return nil, err
		}
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
		Member:           *member,
		TeamID:           teamID,
		TeamName:         newTeam.DisplayName(),
		EventCount:       eventCount,
		StaleData:        staleData,
		IdentityMismatch: identityMismatch,
	}, nil
}

// validateGitLabIdentity verifies that the current GitLab token belongs to the
// member being rejoined. This prevents identity spoofing.
// The token is tested against the GitLab instance that hosts the team-state repo
// (not the tracker instance, which may be different).
// Returns nil if validation succeeds or if no GitLab token is available (with warning).
func validateGitLabIdentity(ctx context.Context, a *app.App, repo *teamstate.Repo, member *teamstate.Member) error {
	if member.GitLabUsername == "" {
		// Member has no GitLab username configured — skip validation
		return nil
	}

	// Resolve the GitLab base URL from the team-state repo remote.
	// The member's gitlab_username belongs to this instance, not the tracker.
	baseURL := gitlabBaseURLFromRemote(repo.Remote())
	if baseURL == "" {
		slog.Warn("GitLab identity validation skipped: cannot determine GitLab URL from repo remote",
			"member", member.ID,
			"remote", repo.Remote(),
		)
		return nil
	}

	// Resolve the token: env → team-specific key → MCP key → tracker key
	teamID := config.RepoNameFromRemote(repo.Remote())
	token := resolveGitLabTokenForIdentity(ctx, a, teamID)
	if token == "" {
		slog.Warn("GitLab identity validation skipped: no token available",
			"member", member.ID,
			"gitlab_username", member.GitLabUsername,
		)
		return nil
	}

	// Test connection against the team-state repo's GitLab instance
	cfg := tracker.Config{
		Type:    tracker.TypeGitLab,
		BaseURL: baseURL,
		Token:   token,
	}
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
		isBot := strings.HasPrefix(username, "project_") || strings.HasPrefix(username, "group_")
		return &identityMismatchError{
			AuthenticatedAs: username,
			ExpectedUser:    member.GitLabUsername,
			MemberID:        member.ID,
			IsBot:           isBot,
		}
	}

	return nil
}

// gitlabBaseURLFromRemote extracts the base URL (scheme + host) from a git
// remote URL. Supports HTTPS and SCP-style (git@host:path) formats.
// Returns "" if the URL cannot be parsed.
func gitlabBaseURLFromRemote(remote string) string {
	// SCP-style: git@gitlab.octo.tools:org/repo.git → https://gitlab.octo.tools
	if strings.Contains(remote, "@") && strings.Contains(remote, ":") && !strings.Contains(remote, "://") {
		parts := strings.SplitN(remote, "@", 2)
		if len(parts) == 2 {
			hostPart := strings.SplitN(parts[1], ":", 2)
			if len(hostPart) >= 1 && hostPart[0] != "" {
				return "https://" + hostPart[0]
			}
		}
		return ""
	}
	// HTTP(S) URL: https://gitlab.octo.tools/org/repo.git → https://gitlab.octo.tools
	if u, err := url.Parse(remote); err == nil && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	return ""
}

// resolveGitLabTokenForIdentity tries to find a GitLab token suitable for
// identity verification. The priority is:
//  1. GITLAB_TOKEN environment variable
//  2. Team-specific keychain key (openhub.team.<teamID>.gitlab.token)
//  3. MCP GitLab keychain key (openhub.mcp.gitlab.token)
//  4. Tracker GitLab keychain key (openhub.tracker.gitlab.token)
//
// Returns "" if no token is found.
func resolveGitLabTokenForIdentity(ctx context.Context, a *app.App, teamID string) string {
	// 1. Env var — always takes priority
	if tok := os.Getenv("GITLAB_TOKEN"); tok != "" {
		return tok
	}
	if a.Secrets == nil {
		return ""
	}
	// 2. Team-specific key
	if teamID != "" {
		if tok, err := a.Secrets.Get(ctx, config.TeamGitLabTokenKey(teamID)); err == nil && tok != "" {
			return tok
		}
	}
	// 3. MCP GitLab key (might be for the same instance)
	if tok, err := a.Secrets.Get(ctx, config.DefaultGitLabTokenKey); err == nil && tok != "" {
		return tok
	}
	// 4. Tracker GitLab key
	if tok, err := a.Secrets.Get(ctx, "openhub.tracker.gitlab.token"); err == nil && tok != "" {
		return tok
	}
	return ""
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
