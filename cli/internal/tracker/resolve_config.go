package tracker

import (
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// EffectiveTrackerConfig is the fully resolved tracker sync configuration,
// merging team-state shared settings with local hub.toml overrides.
//
// Factual settings (Type, Projects, TicketPatterns, DoneRetentionDays) always
// come from team-state and cannot be overridden locally — they describe objective
// facts about the team's project setup.
//
// Behavioural settings (Enabled, AutoSync, PushLabels, AutoPlan) follow
// nil-means-inherit semantics: if the local override is nil, the team-state
// value is used.
type EffectiveTrackerConfig struct {
	// ── Factual (team-state only) ─────────────────────────────────────────────

	// Type is the tracker backend: "gitlab" or "jira".
	Type string
	// Projects maps hub project IDs to tracker project identifiers.
	Projects map[string]string
	// TicketPatterns maps hub project IDs to a regex for extracting the IID.
	TicketPatterns map[string]string
	// DoneRetentionDays is the number of days before done claims are cleaned up.
	DoneRetentionDays int

	// ── Behavioural (shared ?? local ?? default) ──────────────────────────────

	// Enabled controls whether tracker sync runs for this member.
	Enabled bool
	// AutoSync controls whether sync runs automatically on team view open.
	AutoSync bool
	// PushLabels controls whether hub labels are pushed back to the tracker.
	// This is the RESOLVED value: it already accounts for write_enabled.
	PushLabels bool
	// AutoPlanAssigned controls whether assigned tracker issues without a claim
	// automatically get a "planned" claim created.
	AutoPlanAssigned bool
	// MaxAutoPlanPerMember limits the number of auto-planned claims per member.
	MaxAutoPlanPerMember int
	// SyncIntervalMinutes is the board polling interval (0 = disabled).
	SyncIntervalMinutes int

	// ── Insight fields (for TUI display) ──────────────────────────────────────

	// SharedPushLabels is the raw team recommendation before write_enabled gating.
	// Used to display the ℹ insight when the local override differs.
	SharedPushLabels bool
	// LocalOverrides is the set of fields where the local config differs from
	// the shared recommendation (used to render ℹ markers in the TUI).
	LocalOverrides OverrideSet
}

// OverrideSet tracks which fields are locally overridden vs inherited from team-state.
type OverrideSet struct {
	Enabled              bool
	AutoSync             bool
	PushLabels           bool
	AutoPlanAssigned     bool
	MaxAutoPlanPerMember bool
}

// ResolveTrackerConfig merges the team-state TrackerConfig with the member's
// local TrackerLocalConfig overrides and the effective write permission.
//
// If shared is nil (no team-state or tracker not configured), only local values
// and defaults are used — graceful degradation to hub.toml-only mode.
func ResolveTrackerConfig(
	shared *teamstate.TrackerConfig,
	local config.TrackerLocalConfig,
	writeEnabled bool,
) EffectiveTrackerConfig {
	eff := EffectiveTrackerConfig{}

	// ── Factual settings (team-state only) ────────────────────────────────────

	if shared != nil {
		eff.Type = shared.Type
		eff.Projects = shared.Projects
		eff.TicketPatterns = shared.TicketPatterns
		eff.SyncIntervalMinutes = shared.SyncIntervalMinutes
	}

	// DoneRetentionDays comes from ClaimConfig — use a sensible default.
	eff.DoneRetentionDays = 7

	// ── Behavioural: shared ?? local ?? default ────────────────────────────────

	// Helper: boolOrDefault returns the local override if non-nil, else shared,
	// else the provided default.
	boolResolve := func(localPtr *bool, sharedVal bool, dflt bool) (effective bool, overridden bool) {
		if localPtr != nil {
			return *localPtr, *localPtr != sharedVal
		}
		if shared != nil {
			return sharedVal, false
		}
		return dflt, false
	}

	intResolve := func(localPtr *int, sharedVal int, dflt int) (effective int, overridden bool) {
		if localPtr != nil {
			return *localPtr, *localPtr != sharedVal
		}
		if shared != nil {
			return sharedVal, false
		}
		return dflt, false
	}

	var sharedEnabled, sharedAutoSync, sharedAutoPlan bool
	var sharedMaxAutoPlan, sharedPushLabels bool
	var sharedMaxAutoPlanVal int

	if shared != nil {
		sharedEnabled = shared.Enabled
		sharedAutoSync = shared.AutoSync
		sharedAutoPlan = shared.AutoPlanAssigned
		sharedMaxAutoPlanVal = shared.MaxAutoPlanPerMember
		sharedPushLabels = shared.PushLabels
	}

	eff.Enabled, eff.LocalOverrides.Enabled = boolResolve(local.Enabled, sharedEnabled, true)
	eff.AutoSync, eff.LocalOverrides.AutoSync = boolResolve(local.AutoSync, sharedAutoSync, false)
	eff.AutoPlanAssigned, eff.LocalOverrides.AutoPlanAssigned = boolResolve(local.AutoPlanAssigned, sharedAutoPlan, false)
	eff.MaxAutoPlanPerMember, eff.LocalOverrides.MaxAutoPlanPerMember = intResolve(local.MaxAutoPlanPerMember, sharedMaxAutoPlanVal, 5)

	// push_labels: resolve recommendation first, then gate by write_enabled.
	rawPushLabels, pushOverridden := boolResolve(local.PushLabels, sharedPushLabels, false)
	eff.SharedPushLabels = sharedPushLabels
	eff.PushLabels = rawPushLabels && writeEnabled // always requires write permission
	eff.LocalOverrides.PushLabels = pushOverridden

	_ = sharedMaxAutoPlan // suppress unused warning

	return eff
}
