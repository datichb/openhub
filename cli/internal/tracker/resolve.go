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
// The tracker has its own independent URL, token, and write permission —
// these are NOT derived from MCP config.
type CredentialSource struct {
	// URL is the base URL of the tracker instance.
	// Resolved from: project.TrackerURL → hub.Tracker.TrackerURL → team.Tracker.TrackerURL → ""
	// An empty URL uses the env var fallback (GITLAB_URL / JIRA_URL) or the built-in default.
	URL string
	// TeamID and MCPTokenKey feed the GitLab token cascade (see
	// ResolveGitLabToken): the team whose GitLab token may be used, and the
	// key set in [mcp.gitlab]. Both are optional.
	TeamID      string
	MCPTokenKey string
	// TokenKey is the keychain key for the tracker token (not the secret itself).
	// Resolved from: project.TrackerTokenKey → hub.Tracker.TrackerTokenKey → team.Tracker.TrackerTokenKey → derived default.
	TokenKey string
	// WriteEnabled controls whether write operations (push labels, assign issues) are allowed.
	WriteEnabled bool

	// Secrets is the hub secret store used to read tokens from the keychain.
	// May be nil — env vars still work in that case.
	Secrets SecretGetter
}

// NewCredentialSource builds a CredentialSource from a resolved EffectiveTrackerConfig.
// This is the standard way to construct a CredentialSource — it avoids manual
// field-by-field construction at call sites.
func NewCredentialSource(eff EffectiveTrackerConfig, secrets SecretGetter) CredentialSource {
	return CredentialSource{
		URL:          eff.TrackerURL,
		TokenKey:     eff.TrackerTokenKey,
		WriteEnabled: eff.WriteEnabled,
		Secrets:      secrets,
	}
}

// ResolveCredentials returns a fully-resolved tracker Config for the requested type.
//
// Resolution order for the token:
//   - GitLab: the cascade of ResolveGitLabToken (GITLAB_TOKEN, tracker key,
//     team, MCP, former tracker key)
//   - Jira: JIRA_TOKEN, then Secrets.Get(ctx, tokenKey)
//   - Error — no source provided a token
//
// Resolution order for the base URL:
//  1. CredentialSource.URL (from config cascade)
//  2. Environment variable (GITLAB_URL / JIRA_URL)
//  3. Built-in default: "https://gitlab.com" for GitLab
//     (Jira has no default — URL is mandatory)
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
	// Base URL priority: config → env var → default
	baseURL := "https://gitlab.com"
	source := "default"
	if src.URL != "" {
		baseURL = src.URL
		source = "config"
	} else if u := os.Getenv("GITLAB_URL"); u != "" {
		baseURL = u
		source = "env"
	}
	slog.Debug("tracker.resolve.url", "type", "gitlab", "url", baseURL, "source", source)

	token, _ := ResolveGitLabToken(ctx, GitLabTokenQuery{
		TrackerKey: src.TokenKey,
		TeamID:     src.TeamID,
		MCPKey:     src.MCPTokenKey,
		Secrets:    src.Secrets,
	})
	if token == "" {
		return Config{}, missingTokenError(GitLabTokenEnv, src.TokenKey, "GitLab")
	}

	return Config{
		Type:         TypeGitLab,
		BaseURL:      baseURL,
		Token:        token,
		WriteEnabled: src.WriteEnabled,
	}, nil
}

// ── Jira ──────────────────────────────────────────────────────────────────────

func resolveJira(ctx context.Context, src CredentialSource) (Config, error) {
	// Base URL priority: config → env var → error (no default for Jira)
	baseURL := ""
	source := ""
	if src.URL != "" {
		baseURL = src.URL
		source = "config"
	} else if u := os.Getenv("JIRA_URL"); u != "" {
		baseURL = u
		source = "env"
	}
	if baseURL == "" {
		return Config{}, fmt.Errorf(
			"tracker: Jira non configuré — ajoutez tracker_url dans [tracker] (hub.toml) ou dans la config équipe, ou exportez JIRA_URL + JIRA_TOKEN",
		)
	}
	slog.Debug("tracker.resolve.url", "type", "jira", "url", baseURL, "source", source)

	token, err := resolveToken(ctx, "JIRA_TOKEN", src.TokenKey, src.Secrets, "Jira")
	if err != nil {
		return Config{}, err
	}

	return Config{
		Type:         TypeJira,
		BaseURL:      baseURL,
		Token:        token,
		WriteEnabled: src.WriteEnabled,
	}, nil
}

// ── Shared helpers ────────────────────────────────────────────────────────────

// resolveToken resolves a token from an env var, then from a keychain key.
func resolveToken(ctx context.Context, envVar, tokenKey string, secrets SecretGetter, displayName string) (string, error) {
	// 1. Env var — always takes priority (CI, shell export, tests)
	if tok := os.Getenv(envVar); tok != "" {
		slog.Debug("tracker.resolve.token", "type", displayName, "source", "env")
		return tok, nil
	}

	// 2. Keychain key
	if tokenKey != "" && secrets != nil {
		tok, err := secrets.Get(ctx, tokenKey)
		if err == nil && tok != "" {
			slog.Debug("tracker.resolve.token", "type", displayName, "source", "keychain", "key", tokenKey)
			return tok, nil
		}
		if err != nil {
			slog.Debug("tracker.resolve.token.miss", "type", displayName, "key", tokenKey, "error", err)
		}
	}

	return "", missingTokenError(envVar, tokenKey, displayName)
}

// missingTokenError is the actionable error returned when no token was found.
func missingTokenError(envVar, tokenKey, displayName string) error {
	hint := fmt.Sprintf("exportez %s ou configurez [tracker] tracker_token_key dans hub.toml", envVar)
	if tokenKey != "" {
		hint = fmt.Sprintf("exportez %s ou stockez le token via: oh secrets set %s", envVar, tokenKey)
	}
	return fmt.Errorf("tracker: token %s non disponible — %s", displayName, hint)
}
