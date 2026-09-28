package provider

import (
	"context"

	"github.com/datichb/openhub/cli/internal/platform"
)

// SecretStore is the minimal interface for credential retrieval.
// Satisfied by app.App.Secrets (keychain or filecrypt backends).
type SecretStore interface {
	Get(ctx context.Context, key string) (string, error)
}

// ResolveProvider returns the effective provider following the cascade:
//
//	explicit flag → project DB value → hub config default → "bedrock".
//
// Each argument may be empty; the first non-empty value wins.
func ResolveProvider(explicit, projectProvider, hubDefault string) string {
	if explicit != "" {
		return explicit
	}
	if projectProvider != "" {
		return projectProvider
	}
	if hubDefault != "" {
		return hubDefault
	}
	return "bedrock"
}

// ResolveProviderConfig merges per-project provider overrides with hub-level
// defaults. Non-empty project fields take precedence.
func ResolveProviderConfig(projectCfg *ProviderConfig, hubCfg Config) Config {
	result := hubCfg
	if projectCfg != nil {
		if projectCfg.AWSProfile != "" {
			result.AWSProfile = projectCfg.AWSProfile
		}
		if projectCfg.AWSRegion != "" {
			result.AWSRegion = projectCfg.AWSRegion
		}
	}
	return result
}

// ProviderConfig mirrors domain.ProjectProviderConfig without importing the
// domain package, to avoid a circular dependency. Call sites convert from
// domain.ProjectProviderConfig to this type via ToProviderConfig().
type ProviderConfig struct {
	AWSProfile string
	AWSRegion  string
}

// ResolveCredentials extracts provider-specific credentials from the secret store.
// Follows the cascade: project-scoped key → global key.
//
// The returned platform.Credentials is backend-agnostic — it carries whatever
// the provider needs without encoding any runtime-specific env var names.
func ResolveCredentials(ctx context.Context, secrets SecretStore, prov Name, projectID string, cfg *Config) platform.Credentials {
	if secrets == nil {
		return platform.Credentials{}
	}

	var creds platform.Credentials
	switch prov {
	case Bedrock:
		creds.BearerToken, _ = secrets.Get(ctx, KeychainKey(prov, projectID))
		if creds.BearerToken == "" {
			creds.BearerToken, _ = secrets.Get(ctx, KeychainKey(prov, ""))
		}
		if cfg != nil {
			creds.AWSProfile = cfg.AWSProfile
			creds.AWSRegion = cfg.AWSRegion
		}
	case Anthropic:
		creds.APIKey, _ = secrets.Get(ctx, KeychainKey(prov, projectID))
		if creds.APIKey == "" {
			creds.APIKey, _ = secrets.Get(ctx, KeychainKey(prov, ""))
		}
	case OpenRouter:
		creds.APIKey, _ = secrets.Get(ctx, KeychainKey(prov, projectID))
		if creds.APIKey == "" {
			creds.APIKey, _ = secrets.Get(ctx, KeychainKey(prov, ""))
		}
	}
	return creds
}
