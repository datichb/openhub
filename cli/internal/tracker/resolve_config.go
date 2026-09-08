package tracker

import (
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// EffectiveTrackerConfig is the fully resolved tracker sync configuration,
// merging team-state shared settings with local hub.toml overrides and
// optional per-project overrides.
//
// Factual settings (Type, TrackerProject, TrackerURL, TicketPattern) come
// from team-state and can be overridden per-project via ProjectTrackerConfig.
//
// Behavioural settings (Enabled, AutoSync, PushLabels, AutoPlan) follow
// nil-means-inherit semantics: if the local override is nil, the team-state
// value is used.
type EffectiveTrackerConfig struct {
	// ── Factual (team-state + project override) ──────────────────────────────

	// Type is the tracker backend: "gitlab" or "jira".
	Type string
	// TrackerProject is the resolved external tracker project identifier.
	// Resolution: project.TrackerProject → shared.TrackerProject → ""
	TrackerProject string
	// TrackerURL is the resolved tracker instance URL.
	// Resolution: project.TrackerURL → shared.TrackerURL → MCP URL → env → default
	TrackerURL string
	// TrackerTokenKey is the resolved keychain key for the tracker token.
	// Resolution: project.TrackerTokenKey → shared.TrackerTokenKey → derived default
	TrackerTokenKey string
	// TicketPattern is the resolved regex for extracting the tracker IID.
	// Resolution: project.TicketPattern → shared.TicketPattern → ""
	TicketPattern string
	// DoneRetentionDays is the number of days before done claims are cleaned up.
	DoneRetentionDays int

	// ── Deprecated: backward compat with old map-based config ────────────────
	// These are populated from the old Projects/TicketPatterns maps when
	// the new TrackerProject field is empty. Callers should prefer
	// TrackerProject and TicketPattern.
	Projects       map[string]string
	TicketPatterns map[string]string
	// StatusMapping maps tracker status names to claim statuses.
	// Passed through from the team-state TrackerConfig.
	StatusMapping map[string]string
	// LabelStatusMapping maps tracker labels to claim statuses (ADR-032).
	// Passed through from the team-state TrackerConfig.
	LabelStatusMapping map[string]string

	// ── Behavioural (shared → local → default) ──────────────────────────────

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
	// AutoPlanUnassigned controls whether unassigned tracker issues are fetched
	// and shown as claimable pool tickets on the team board.
	AutoPlanUnassigned bool
	// UnassignedLabels restricts which unassigned issues are fetched (AND filter).
	UnassignedLabels []string
	// MaxUnassignedIssues caps the number of unassigned issues fetched per project.
	MaxUnassignedIssues int
	// SyncIntervalMinutes is the board polling interval (0 = disabled).
	SyncIntervalMinutes int

	// ── Insight fields (for TUI display) ──────────────────────────────────────

	// SharedPushLabels is the raw team recommendation before write_enabled gating.
	SharedPushLabels bool
	// LocalOverrides is the set of fields where the local config differs from
	// the shared recommendation.
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
	return ResolveFullTrackerConfig(shared, local, writeEnabled, nil)
}

// ResolveFullTrackerConfig merges team-state, local, and per-project overrides.
// projectCfg may be nil (no per-project override → inherit team defaults).
//
// Resolution cascade for factual fields:
//
//	project.TrackerProject → shared.TrackerProject → shared.Projects[projectID] → ""
//	project.TrackerURL → shared.TrackerURL → "" (MCP URL resolved separately)
//	project.TicketPattern → shared.TicketPattern → shared.TicketPatterns[projectID] → ""
func ResolveFullTrackerConfig(
	shared *teamstate.TrackerConfig,
	local config.TrackerLocalConfig,
	writeEnabled bool,
	projectCfg *domain.ProjectTrackerConfig,
) EffectiveTrackerConfig {
	eff := EffectiveTrackerConfig{}

	// ── Factual settings (team-state + project override) ─────────────────────

	if shared != nil {
		eff.Type = shared.Type
		eff.TrackerProject = shared.TrackerProject
		eff.TrackerURL = shared.TrackerURL
		eff.TrackerTokenKey = shared.TrackerTokenKey
		eff.TicketPattern = shared.TicketPattern
		eff.SyncIntervalMinutes = shared.SyncIntervalMinutes
		eff.StatusMapping = shared.StatusMapping
		eff.LabelStatusMapping = shared.LabelStatusMapping
		// Backward compat: keep old maps
		eff.Projects = shared.Projects
		eff.TicketPatterns = shared.TicketPatterns
	}

	// Per-project overrides (most specific wins)
	if projectCfg != nil {
		if projectCfg.TrackerProject != "" {
			eff.TrackerProject = projectCfg.TrackerProject
		}
		if projectCfg.TrackerURL != "" {
			eff.TrackerURL = projectCfg.TrackerURL
		}
		if projectCfg.TrackerTokenKey != "" {
			eff.TrackerTokenKey = projectCfg.TrackerTokenKey
		}
		if projectCfg.TicketPattern != "" {
			eff.TicketPattern = projectCfg.TicketPattern
		}
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
		// Pool (unassigned) settings — no local override, always from shared.
		eff.AutoPlanUnassigned = shared.AutoPlanUnassigned
		eff.UnassignedLabels = shared.UnassignedLabels
		eff.MaxUnassignedIssues = shared.MaxUnassignedIssues
		if eff.MaxUnassignedIssues <= 0 {
			eff.MaxUnassignedIssues = 20
		}
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
