package tracker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

// SecretGetter is the minimal interface needed to retrieve a token from a secret
// store (keychain or encrypted file). It is defined here to avoid importing the
// domain package and keep tracker self-contained.
type SecretGetter interface {
	Get(ctx context.Context, key string) (string, error)
}

// CredentialSource holds all the inputs needed to resolve tracker credentials.
// Fields are passed as primitives to avoid coupling with the config package.
// Each tracker type (GitLab / Jira) is fully independent: configuring one
// does not require the other to be present.
type CredentialSource struct {
	// GitLab MCP settings (from hub.toml [mcp.gitlab]).
	// May be zero-value if [mcp.gitlab] is not configured.
	GitLabEnabled      bool
	GitLabTokenKey     string // keychain key name — NOT the secret itself
	GitLabWriteEnabled bool
	// GitLabURL is the resolved base URL for GitLab.
	// Precedence: local MCPServerConfig.URL → shared team-state URL → env GITLAB_URL → "https://gitlab.com"
	// Set via ResolveMCPConfig before building the CredentialSource.
	GitLabURL string

	// Jira MCP settings (from hub.toml [mcp.jira]).
	// May be zero-value if [mcp.jira] is not configured.
	JiraEnabled      bool
	JiraTokenKey     string
	JiraWriteEnabled bool
	// JiraURL is the resolved base URL for Jira (no built-in default — must be explicit).
	JiraURL string

	// TrackerURL overrides the MCP URL for tracker operations when set.
	// This allows the tracker to point to a different instance than the MCP service.
	TrackerURL string
	// TrackerTokenKey is a dedicated keychain key for the tracker token.
	// When set and a token is found, it takes priority over the MCP token key.
	// Falls back to the MCP token key if the tracker-specific token is not found.
	TrackerTokenKey string

	// Secrets is the hub secret store used to read tokens from the keychain.
	// May be nil — env vars still work in that case.
	Secrets SecretGetter
}

// ResolveCredentials returns a fully-resolved tracker Config for the requested type.
//
// Resolution order for the token:
//  1. Environment variable (GITLAB_TOKEN / JIRA_TOKEN)
//  2. Keychain: Secrets.Get(ctx, tokenKey) using the key from the corresponding
//     MCP config field (GitLabTokenKey / JiraTokenKey)
//  3. Error — neither source provided a token
//
// Resolution order for the base URL:
//  1. Environment variable (GITLAB_URL / JIRA_URL)
//  2. Built-in default: "https://gitlab.com" for GitLab
//     (Jira has no default — URL is mandatory)
//
// Each tracker type resolves independently.  Requesting TypeGitLab only reads
// the GitLab* fields; the Jira* fields are ignored, and vice-versa.
// It is valid to have only one of the two configured.
func ResolveCredentials(ctx context.Context, src CredentialSource, t Type) (Config, error) {
	switch t {
	case TypeGitLab:
		return resolveGitLab(ctx, src)
	case TypeJira:
		return resolveJira(ctx, src)
	default:
		return Config{}, fmt.Errorf("tracker: unknown type %q (supported: gitlab, jira)", t)
	}
}

// ── GitLab ────────────────────────────────────────────────────────────────────

func resolveGitLab(ctx context.Context, src CredentialSource) (Config, error) {
	// Base URL priority: TrackerURL override → MCP URL → env var → default
	baseURL := "https://gitlab.com"
	source := "default"
	if src.TrackerURL != "" {
		baseURL = src.TrackerURL
		source = "tracker_url"
	} else if src.GitLabURL != "" {
		baseURL = src.GitLabURL
		source = "mcp"
	} else if u := os.Getenv("GITLAB_URL"); u != "" {
		baseURL = u
		source = "env"
	}
	slog.Debug("tracker.resolve.url", "type", "gitlab", "url", baseURL, "source", source)

	// Token priority: env var → tracker-specific keychain key → MCP keychain key → error
	token, err := resolveTokenWithFallback(ctx,
		"GITLAB_TOKEN",
		src.TrackerTokenKey,
		src.GitLabTokenKey,
		src.Secrets,
		"GitLab",
	)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Type:         TypeGitLab,
		BaseURL:      baseURL,
		Token:        token,
		WriteEnabled: src.GitLabWriteEnabled,
	}, nil
}

func resolveJira(ctx context.Context, src CredentialSource) (Config, error) {
	// Base URL priority: TrackerURL override → MCP URL → env var → error
	baseURL := ""
	source := ""
	if src.TrackerURL != "" {
		baseURL = src.TrackerURL
		source = "tracker_url"
	} else if src.JiraURL != "" {
		baseURL = src.JiraURL
		source = "mcp"
	} else {
		baseURL = os.Getenv("JIRA_URL")
		source = "env"
	}
	if baseURL == "" {
		return Config{}, fmt.Errorf(
			"tracker: Jira non configuré — ajoutez tracker_url dans la config équipe, [mcp.jira] dans hub.toml ou exportez JIRA_URL + JIRA_TOKEN",
		)
	}
	slog.Debug("tracker.resolve.url", "type", "jira", "url", baseURL, "source", source)

	// Token priority: env var → tracker-specific keychain key → MCP keychain key → error
	token, err := resolveTokenWithFallback(ctx,
		"JIRA_TOKEN",
		src.TrackerTokenKey,
		src.JiraTokenKey,
		src.Secrets,
		"Jira",
	)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Type:         TypeJira,
		BaseURL:      baseURL,
		Token:        token,
		WriteEnabled: src.JiraWriteEnabled,
	}, nil
}

// ── Shared helpers ────────────────────────────────────────────────────────────

// resolveTokenWithFallback resolves a token from an env var, then from a
// tracker-specific keychain key, then from the MCP keychain key.
// This allows the tracker to use a dedicated token when pointing to a different
// instance than the MCP service, while falling back to the MCP token otherwise.
func resolveTokenWithFallback(ctx context.Context, envVar, trackerTokenKey, mcpTokenKey string, secrets SecretGetter, displayName string) (string, error) {
	// 1. Env var — always takes priority (CI, shell export, tests)
	if tok := os.Getenv(envVar); tok != "" {
		slog.Debug("tracker.resolve.token", "type", displayName, "source", "env")
		return tok, nil
	}

	// 2. Tracker-specific keychain key (if different from MCP key)
	if trackerTokenKey != "" && secrets != nil {
		tok, err := secrets.Get(ctx, trackerTokenKey)
		if err == nil && tok != "" {
			slog.Debug("tracker.resolve.token", "type", displayName, "source", "tracker_key", "key", trackerTokenKey)
			return tok, nil
		}
		if err != nil {
			slog.Debug("tracker.resolve.token.miss", "type", displayName, "key", trackerTokenKey, "error", err)
		}
	}

	// 3. MCP keychain key (fallback)
	if mcpTokenKey != "" && mcpTokenKey != trackerTokenKey && secrets != nil {
		tok, err := secrets.Get(ctx, mcpTokenKey)
		if err == nil && tok != "" {
			slog.Debug("tracker.resolve.token", "type", displayName, "source", "mcp_key", "key", mcpTokenKey)
			return tok, nil
		}
		if err != nil {
			slog.Debug("tracker.resolve.token.miss", "type", displayName, "key", mcpTokenKey, "error", err)
		}
	}

	// 4. Nothing found — actionable error message
	key := trackerTokenKey
	if key == "" {
		key = mcpTokenKey
	}
	hint := fmt.Sprintf("exportez %s ou configurez [mcp.%s] token_key dans hub.toml",
		envVar, displayName)
	if key != "" {
		hint = fmt.Sprintf("exportez %s ou stockez le token via: oh secrets set %s",
			envVar, key)
	}
	return "", fmt.Errorf("tracker: token %s non disponible — %s", displayName, hint)
}

// resolveToken resolves a token from an env var, then from the secret store.
// displayName is used in error messages ("GitLab", "Jira").
func resolveToken(ctx context.Context, envVar, tokenKey string, secrets SecretGetter, displayName string) (string, error) {
	// 1. Env var — always takes priority (CI, shell export, tests)
	if tok := os.Getenv(envVar); tok != "" {
		return tok, nil
	}

	// 2. Keychain via token_key from MCP config
	if tokenKey != "" && secrets != nil {
		tok, err := secrets.Get(ctx, tokenKey)
		if err == nil && tok != "" {
			return tok, nil
		}
	}

	// 3. Nothing found — actionable error message
	hint := fmt.Sprintf("exportez %s ou configurez [mcp.%s] token_key dans hub.toml",
		envVar, displayName)
	if tokenKey != "" {
		hint = fmt.Sprintf("exportez %s ou vérifiez la clé %q dans le keychain (oh secrets set %s)",
			envVar, tokenKey, tokenKey)
	}
	return "", fmt.Errorf("tracker: token %s non disponible — %s", displayName, hint)
}
