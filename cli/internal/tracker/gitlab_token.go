package tracker

import (
	"context"
	"log/slog"
	"os"

	"github.com/datichb/openhub/cli/internal/config"
)

// GitLabTokenEnv is the environment variable that overrides every stored
// GitLab token.
const GitLabTokenEnv = "GITLAB_TOKEN"

// DefaultTrackerGitLabTokenKey is the keychain key the tracker used before the
// token cascade was unified; it stays the last place looked at.
const DefaultTrackerGitLabTokenKey = "openhub.tracker.gitlab.token"

// TokenSource names where a GitLab token was found.
type TokenSource string

// The sources of the GitLab token, in the order of the cascade.
const (
	TokenSourceNone       TokenSource = ""
	TokenSourceEnv        TokenSource = "env"         // GITLAB_TOKEN
	TokenSourceTrackerKey TokenSource = "tracker-key" // tracker_token_key of the tracker config
	TokenSourceProject    TokenSource = "project"     // openhub.mcp.gitlab.token.<project>
	TokenSourceTeam       TokenSource = "team"        // openhub.team.<team>.gitlab.token
	TokenSourceMCP        TokenSource = "mcp"         // [mcp.gitlab] token key, then openhub.mcp.gitlab.token
	TokenSourceTracker    TokenSource = "tracker"     // openhub.tracker.gitlab.token
)

// GitLabTokenQuery holds what the GitLab token cascade can use. Every field
// is optional: an empty one skips its step.
type GitLabTokenQuery struct {
	// TrackerKey is the keychain key set by tracker_token_key.
	TrackerKey string
	// ProjectID is the hub project whose own MCP token is looked for.
	ProjectID string
	// TeamID is the team whose GitLab token is looked for.
	TeamID string
	// MCPKey is the key set in [mcp.gitlab] (token), if any.
	MCPKey string
	// Secrets is the keychain; nil leaves only the environment variable.
	Secrets SecretGetter
}

// ResolveGitLabToken is the single place that decides which GitLab token is
// used, whatever the command (tracker sync, merge requests, identity check,
// MCP). The cascade, first hit wins:
//
//  1. environment variable GITLAB_TOKEN
//  2. the key set by tracker_token_key
//  3. the token of the project (openhub.mcp.gitlab.token.<project>)
//  4. the token of the team (openhub.team.<team>.gitlab.token)
//  5. the hub MCP token: the key set in [mcp.gitlab], then
//     openhub.mcp.gitlab.token (and its former name gitlab-token)
//  6. the former tracker key openhub.tracker.gitlab.token
//
// It returns an empty token and TokenSourceNone when nothing was found.
func ResolveGitLabToken(ctx context.Context, q GitLabTokenQuery) (string, TokenSource) {
	if tok := os.Getenv(GitLabTokenEnv); tok != "" {
		return tok, TokenSourceEnv
	}
	if q.Secrets == nil {
		return "", TokenSourceNone
	}
	found := func(src TokenSource, tok string) (string, TokenSource) {
		slog.Debug("tracker.resolve.gitlab_token", "source", string(src))
		return tok, src
	}
	if q.TrackerKey != "" {
		if tok := getSecret(ctx, q.Secrets, q.TrackerKey); tok != "" {
			return found(TokenSourceTrackerKey, tok)
		}
	}
	if q.ProjectID != "" {
		if tok := readMCPGitLab(ctx, q.Secrets, config.MCPProjectTokenKey("gitlab", q.ProjectID)); tok != "" {
			return found(TokenSourceProject, tok)
		}
	}
	if q.TeamID != "" {
		if tok := getSecret(ctx, q.Secrets, config.TeamGitLabTokenKey(q.TeamID)); tok != "" {
			return found(TokenSourceTeam, tok)
		}
	}
	if q.MCPKey != "" && q.MCPKey != config.DefaultGitLabTokenKey {
		if tok := readMCPGitLab(ctx, q.Secrets, q.MCPKey); tok != "" {
			return found(TokenSourceMCP, tok)
		}
	}
	if tok := readMCPGitLab(ctx, q.Secrets, ""); tok != "" {
		return found(TokenSourceMCP, tok)
	}
	if tok := getSecret(ctx, q.Secrets, DefaultTrackerGitLabTokenKey); tok != "" {
		return found(TokenSourceTracker, tok)
	}
	return "", TokenSourceNone
}

// getSecret reads one key; a missing key or a failing keychain is a miss.
func getSecret(ctx context.Context, secrets SecretGetter, key string) string {
	tok, err := secrets.Get(ctx, key)
	if err != nil {
		slog.Debug("tracker.resolve.token.miss", "key", key, "error", err)
		return ""
	}
	return tok
}

// readMCPGitLab reads an MCP token of GitLab under its current name, then
// under its former one. When the store can be written, a token found under
// the former name is copied under the current one (config.ReadMCPToken).
func readMCPGitLab(ctx context.Context, secrets SecretGetter, key string) string {
	if rw, ok := secrets.(config.SecretReadWriter); ok {
		tok, _ := config.ReadMCPToken(ctx, rw, "gitlab", key)
		return tok
	}
	for _, k := range config.MCPTokenKeyCandidates("gitlab", key) {
		if tok := getSecret(ctx, secrets, k); tok != "" {
			return tok
		}
	}
	return ""
}
