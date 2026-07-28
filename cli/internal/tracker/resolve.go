package tracker

import (
	"context"
	"fmt"
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
	// Base URL priority: local override in CredentialSource → env var → default
	baseURL := "https://gitlab.com"
	if src.GitLabURL != "" {
		baseURL = src.GitLabURL
	} else if u := os.Getenv("GITLAB_URL"); u != "" {
		baseURL = u
	}

	token, err := resolveToken(ctx,
		"GITLAB_TOKEN",
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
	// Base URL priority: local override in CredentialSource → env var → error
	baseURL := src.JiraURL
	if baseURL == "" {
		baseURL = os.Getenv("JIRA_URL")
	}
	if baseURL == "" {
		return Config{}, fmt.Errorf(
			"tracker: Jira non configuré — ajoutez [mcp.jira] dans hub.toml ou exportez JIRA_URL + JIRA_TOKEN",
		)
	}

	token, err := resolveToken(ctx,
		"JIRA_TOKEN",
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

// ── Shared helper ─────────────────────────────────────────────────────────────

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
