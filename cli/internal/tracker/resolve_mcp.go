package tracker

import (
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// EffectiveMCPConfig is the resolved MCP configuration for a single service,
// merging team-state recommendations with local hub.toml overrides.
//
// Token and WriteEnabled are always local (never shared) — only Enabled and URL
// can come from team-state recommendations or enforcements.
type EffectiveMCPConfig struct {
	// Enabled is the resolved "is this service active?" flag.
	Enabled bool
	// EnabledEnforced is true when Enabled is imposed by the team (cannot be overridden).
	EnabledEnforced bool
	// URL is the resolved base URL for the service.
	// Precedence: team enforced → local → team recommended → built-in default.
	URL string
	// URLEnforced is true when URL is imposed by the team.
	URLEnforced bool
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
// Resolution cascade (ADR-030):
//  1. Team ENFORCED → imposed value, cannot be overridden
//  2. Local hub.toml → member's personal choice
//  3. Team RECOMMENDED → fallback when no local preference
//
// Merge rules:
//   - Enabled:          enforced → local (if explicit) → shared recommended
//   - URL:              enforced → local.URL → shared.URL → ""
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

	// WriteRecommended: from shared only (informational)
	if shared != nil {
		eff.WriteRecommended = shared.WriteRecommended
	}

	// --- URL resolution ---
	// Step 1: Team enforced URL?
	if shared != nil && shared.IsURLEnforced() && shared.URL != "" {
		eff.URL = shared.URL
		eff.URLEnforced = true
	} else {
		// Step 2: Local URL override → Step 3: Team recommended URL
		switch {
		case local.URL != "":
			eff.URL = local.URL
		case shared != nil && shared.URL != "":
			eff.URL = shared.URL
		}
	}

	// --- Enabled resolution ---
	// Step 1: Team enforced Enabled?
	if shared != nil && shared.IsEnabledEnforced() && shared.Enabled != nil {
		eff.Enabled = *shared.Enabled
		eff.EnabledEnforced = true
	} else {
		// Step 2: Local explicit setting
		// A local MCPServerConfig.Enabled = true|false is always an explicit local choice.
		// We treat the local config as "explicit" when the token_key is set or when
		// the local Enabled flag is true, because having a token key means the member
		// intentionally configured this service.
		localExplicit := local.Enabled || local.Token != ""
		if localExplicit {
			eff.Enabled = local.Enabled
			eff.LocalOverridesEnabled = true
		} else if shared != nil && shared.Enabled != nil {
			// Step 3: Team recommended
			eff.Enabled = *shared.Enabled
		}
	}

	return eff
}

// ResolveFullMCPConfig resolves MCP configuration across all 3 levels:
// team-state (enforced/recommended) → hub (personal preference) → project (override).
//
// Resolution per field:
//   - Enabled:      team_enforced → project (if non-nil) → hub_explicit → team_recommended → false
//   - URL:          team_enforced → project.URL → hub.URL → team_recommended.URL → ""
//   - TokenKey:     project.TokenKey → hub.Token (always personal, never team)
//   - WriteEnabled: project.WriteEnabled → hub.WriteEnabled (always personal, never team)
//
// The teamID parameter is used for source annotations only.
func ResolveFullMCPConfig(
	shared *teamstate.SharedMCPConfig,
	hub config.MCPServerConfig,
	project *domain.ProjectMCPService,
	teamID string,
) EffectiveMCPConfig {
	eff := EffectiveMCPConfig{}

	// ─── TOKEN (always personal: project > hub, never team) ──────────
	if project != nil && project.TokenKey != "" {
		eff.TokenKey = project.TokenKey
	} else {
		eff.TokenKey = hub.Token
	}

	// ─── WRITE ENABLED (always personal: project > hub, never team) ──
	if project != nil && project.WriteEnabled != nil {
		eff.WriteEnabled = *project.WriteEnabled
	} else {
		eff.WriteEnabled = hub.WriteEnabled
	}
	if shared != nil {
		eff.WriteRecommended = shared.WriteRecommended
	}

	// ─── URL (5-step cascade) ────────────────────────────────────────
	// 1. Team enforced
	if shared != nil && shared.IsURLEnforced() && shared.URL != "" {
		eff.URL = shared.URL
		eff.URLEnforced = true
	} else {
		// 2. Project override
		if project != nil && project.URL != "" {
			eff.URL = project.URL
		} else if hub.URL != "" {
			// 3. Hub value
			eff.URL = hub.URL
		} else if shared != nil && shared.URL != "" {
			// 4. Team recommended
			eff.URL = shared.URL
		}
		// 5. Empty = built-in default (caller handles)
	}

	// ─── ENABLED (5-step cascade) ────────────────────────────────────
	// 1. Team enforced
	if shared != nil && shared.IsEnabledEnforced() && shared.Enabled != nil {
		eff.Enabled = *shared.Enabled
		eff.EnabledEnforced = true
	} else {
		// 2. Project override
		if project != nil && project.Enabled != nil {
			eff.Enabled = *project.Enabled
			eff.LocalOverridesEnabled = true
		} else {
			// 3. Hub explicit (token presence = explicit)
			hubExplicit := hub.Enabled || hub.Token != ""
			if hubExplicit {
				eff.Enabled = hub.Enabled
				eff.LocalOverridesEnabled = true
			} else if shared != nil && shared.Enabled != nil {
				// 4. Team recommended
				eff.Enabled = *shared.Enabled
			}
			// 5. Default = false (zero value)
		}
	}

	return eff
}
