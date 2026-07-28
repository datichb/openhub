package tracker

import (
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// EffectiveMCPConfig is the resolved MCP configuration for a single service,
// merging team-state recommendations with local hub.toml overrides.
//
// Token and WriteEnabled are always local (never shared) — only Enabled and URL
// can come from team-state recommendations.
type EffectiveMCPConfig struct {
	// Enabled is the resolved "is this service active?" flag.
	Enabled bool
	// URL is the resolved base URL for the service.
	// Precedence: local MCPServerConfig.URL → shared SharedMCPConfig.URL → built-in default.
	URL string
	// TokenKey is the keychain key name. Always from the local config (never shared).
	TokenKey string
	// WriteEnabled is the local write permission. Always from the local config.
	WriteEnabled bool
	// WriteRecommended is the team recommendation for write operations.
	// Informational only — does not override WriteEnabled.
	WriteRecommended bool
	// LocalOverridesEnabled is true when the local config explicitly sets Enabled,
	// overriding the team recommendation.
	LocalOverridesEnabled bool
}

// ResolveMCPConfig merges a team-state MCP recommendation with a local hub.toml
// MCP server config for a single service (e.g. "gitlab", "figma").
//
// Merge rules:
//   - Enabled:          local.Enabled (if the service is referenced in hub.toml)
//                       OR shared.Enabled (if not locally set)
//   - URL:              local.URL → shared.URL → ""  (caller provides the built-in default)
//   - TokenKey:         always from local (secrets are never shared)
//   - WriteEnabled:     always from local (permissions are personal)
//   - WriteRecommended: always from shared (informational)
//
// If shared is nil (no team-state config available), only local values are used —
// the function degrades gracefully to hub.toml-only mode.
func ResolveMCPConfig(shared *teamstate.SharedMCPConfig, local config.MCPServerConfig) EffectiveMCPConfig {
	eff := EffectiveMCPConfig{
		TokenKey:     local.Token,
		WriteEnabled: local.WriteEnabled,
	}

	// URL: local override → shared recommendation → empty (caller uses built-in default)
	switch {
	case local.URL != "":
		eff.URL = local.URL
	case shared != nil && shared.URL != "":
		eff.URL = shared.URL
	}

	// WriteRecommended: from shared only (informational)
	if shared != nil {
		eff.WriteRecommended = shared.WriteRecommended
	}

	// Enabled: local takes priority; fall back to shared recommendation.
	// A local MCPServerConfig.Enabled = true|false is always an explicit local choice
	// (the field is a plain bool — false means "not enabled locally").
	// We treat the local config as "explicit" when the token_key is set or when
	// the local Enabled flag is true, because having a token key means the member
	// intentionally configured this service.
	localExplicit := local.Enabled || local.Token != ""
	if localExplicit {
		eff.Enabled = local.Enabled
		eff.LocalOverridesEnabled = true
	} else if shared != nil && shared.Enabled != nil {
		eff.Enabled = *shared.Enabled
	}

	return eff
}
