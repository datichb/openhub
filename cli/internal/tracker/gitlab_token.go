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

// GitLabServerEnv declares the server GITLAB_TOKEN is valid for. When it is
// not set, GITLAB_URL (the variable the GitLab MCP server and the tracker
// already read) plays that role.
const GitLabServerEnv = "OH_GITLAB_URL"

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

// TokenOutcome tells what happened to a source of the cascade.
type TokenOutcome string

// The outcomes of a source.
const (
	// OutcomeUsed: the token is for the server called and was taken.
	OutcomeUsed TokenOutcome = "used"
	// OutcomeEmpty: nothing is stored under the key (the variable is unset).
	OutcomeEmpty TokenOutcome = "empty"
	// OutcomeOtherServer: the key is declared for another server; it was not read.
	OutcomeOtherServer TokenOutcome = "other-server"
	// OutcomeNoServer: a token is stored but no server is declared for its
	// key; it is never sent anywhere.
	OutcomeNoServer TokenOutcome = "no-server"
)

// TokenLook is one source of the cascade that was examined.
type TokenLook struct {
	Source TokenSource
	// Key is the keychain key, or the name of the environment variable.
	Key string
	// Server is the server the key is declared for ("" when it is not).
	Server  string
	Outcome TokenOutcome
}

// TokenOrigin says which token a request is authenticated with: enough to
// tell the user what to fix when the server refuses it.
type TokenOrigin struct {
	Source TokenSource
	// Key is the keychain key, or the name of the environment variable.
	Key string
	// Server is the server the token was sent to.
	Server string
}

// GitLabTokenQuery holds what the GitLab token cascade can use. Every field
// is optional: an empty one skips its step.
type GitLabTokenQuery struct {
	// Server is the URL of the GitLab server about to be called. A token is
	// only returned when the server its source is declared for is this one
	// (ServerOf: host and non-default port, whatever the scheme or the case).
	Server string
	// AnyServer lifts that rule. It only serves to ask "is there a token the
	// MCP server could be given", never to authenticate a request.
	AnyServer bool

	// TrackerKey is the keychain key set by tracker_token_key; TrackerServer
	// the server declared for it (tracker_token_url). The former tracker key
	// uses the same declaration.
	TrackerKey    string
	TrackerServer string
	// ProjectID is the hub project whose own MCP token is looked for;
	// ProjectServer the server of that project's GitLab MCP.
	ProjectID     string
	ProjectServer string
	// TeamID is the team whose GitLab token is looked for; TeamServer the
	// server of its team-state repository, where that token was issued.
	TeamID     string
	TeamServer string
	// MCPKey is the key set in [mcp.gitlab] (token), if any; MCPServer the
	// server of the GitLab MCP (its url, from the project, the hub or the
	// team recommendation).
	MCPKey    string
	MCPServer string
	// Secrets is the keychain; nil leaves only the environment variable.
	Secrets SecretGetter
}

// GitLabTokenResult is the answer of the cascade.
type GitLabTokenResult struct {
	// Token is empty when no source holds a token for the server called.
	Token  string
	Source TokenSource
	// Key and Server are those of the source that answered.
	Key    string
	Server string
	// Called is the server asked for.
	Called string
	// Looks lists the sources examined, in order, up to the one that answered.
	Looks []TokenLook
}

// Found reports whether a token was found.
func (r GitLabTokenResult) Found() bool { return r.Token != "" }

// Origin describes where the token comes from and where it goes.
func (r GitLabTokenResult) Origin() TokenOrigin {
	return TokenOrigin{Source: r.Source, Key: r.Key, Server: r.Called}
}

// EnvServer returns the server GITLAB_TOKEN is declared for: OH_GITLAB_URL,
// else GITLAB_URL; "" when neither is set.
func EnvServer() string {
	if u := os.Getenv(GitLabServerEnv); u != "" {
		return u
	}
	return os.Getenv("GITLAB_URL")
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
// Every source belongs to a server, and a source only answers for the server
// called (q.Server): the token of the team repository is never sent to the
// tracker when they are on two GitLab instances. The server of a source is
// declared by the user for the environment variable (OH_GITLAB_URL or
// GITLAB_URL), for tracker_token_key and the former tracker key
// (tracker_token_url), and for the MCP tokens ([mcp.gitlab] url); it is the
// host of the team-state repository for the team token. A source with no
// declared server is never used.
//
// The result holds an empty token when no source can serve the server.
func ResolveGitLabToken(ctx context.Context, q GitLabTokenQuery) GitLabTokenResult {
	res := GitLabTokenResult{Called: q.Server}
	// try examines one source: it reports whether the token is the answer.
	try := func(src TokenSource, key, server string, read func() string) bool {
		look := TokenLook{Source: src, Key: key, Server: server}
		defer func() { res.Looks = append(res.Looks, look) }()
		matches := q.AnyServer || SameServer(server, q.Server)
		if server != "" && !matches {
			look.Outcome = OutcomeOtherServer
			return false
		}
		tok := read()
		switch {
		case tok == "":
			look.Outcome = OutcomeEmpty
		case !matches:
			look.Outcome = OutcomeNoServer
		default:
			look.Outcome = OutcomeUsed
			res.Token, res.Source, res.Key, res.Server = tok, src, key, server
			slog.Debug("tracker.resolve.gitlab_token", "source", string(src), "key", key, "server", ServerOf(server))
			return true
		}
		return false
	}

	if try(TokenSourceEnv, GitLabTokenEnv, EnvServer(), func() string { return os.Getenv(GitLabTokenEnv) }) {
		return res
	}
	if q.Secrets == nil {
		return res
	}
	if q.TrackerKey != "" &&
		try(TokenSourceTrackerKey, q.TrackerKey, q.TrackerServer, func() string { return getSecret(ctx, q.Secrets, q.TrackerKey) }) {
		return res
	}
	if q.ProjectID != "" {
		key := config.MCPProjectTokenKey("gitlab", q.ProjectID)
		if try(TokenSourceProject, key, q.ProjectServer, func() string { return readMCPGitLab(ctx, q.Secrets, key) }) {
			return res
		}
	}
	if q.TeamID != "" {
		key := config.TeamGitLabTokenKey(q.TeamID)
		if try(TokenSourceTeam, key, q.TeamServer, func() string { return getSecret(ctx, q.Secrets, key) }) {
			return res
		}
	}
	if q.MCPKey != "" && q.MCPKey != config.DefaultGitLabTokenKey &&
		try(TokenSourceMCP, q.MCPKey, q.MCPServer, func() string { return readMCPGitLab(ctx, q.Secrets, q.MCPKey) }) {
		return res
	}
	if try(TokenSourceMCP, config.DefaultGitLabTokenKey, q.MCPServer, func() string { return readMCPGitLab(ctx, q.Secrets, "") }) {
		return res
	}
	if q.TrackerKey != DefaultTrackerGitLabTokenKey { // already read by step 2
		try(TokenSourceTracker, DefaultTrackerGitLabTokenKey, q.TrackerServer, func() string { return getSecret(ctx, q.Secrets, DefaultTrackerGitLabTokenKey) })
	}
	return res
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
