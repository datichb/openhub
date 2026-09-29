package tracker

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

func boolPtr(b bool) *bool  { return &b }
func intPtrTC(n int) *int { return &n }

// ─── ResolveTrackerConfig (2-level: shared + local) ─────────────────────────

func TestResolveTrackerConfig_NilShared_DefaultsUsed(t *testing.T) {
	local := config.TrackerLocalConfig{}
	eff := ResolveTrackerConfig(nil, local)

	// With nil shared and nil local, defaults kick in.
	if !eff.Enabled {
		t.Error("expected Enabled=true (default) when both shared and local are nil")
	}
	if eff.AutoSync {
		t.Error("expected AutoSync=false (default)")
	}
	if eff.PushLabels {
		t.Error("expected PushLabels=false (default, and writeEnabled=false)")
	}
	if eff.AutoPlanAssigned {
		t.Error("expected AutoPlanAssigned=false (default)")
	}
	if eff.MaxAutoPlanPerMember != 5 {
		t.Errorf("expected MaxAutoPlanPerMember=5 (default), got %d", eff.MaxAutoPlanPerMember)
	}
	if eff.MaxUnassignedIssues != 0 {
		t.Errorf("expected MaxUnassignedIssues=0 (no shared), got %d", eff.MaxUnassignedIssues)
	}
	if eff.DoneRetentionDays != 7 {
		t.Errorf("expected DoneRetentionDays=7 (default), got %d", eff.DoneRetentionDays)
	}
}

func TestResolveTrackerConfig_SharedValues_Inherited(t *testing.T) {
	shared := &teamstate.TrackerConfig{
		Type:                 "gitlab",
		Enabled:              true,
		AutoSync:             true,
		PushLabels:           true,
		WriteEnabled:         true,
		AutoPlanAssigned:     true,
		MaxAutoPlanPerMember: 10,
		TrackerURL:           "https://gitlab.example.com",
		TrackerTokenKey:      "team-token",
		TrackerProject:       "group/project",
		TicketPattern:        `^PRJ-(\d+)$`,
		SyncIntervalMinutes:  5,
		AutoPlanUnassigned:   true,
		UnassignedLabels:     []string{"backlog"},
		MaxUnassignedIssues:  50,
		StatusMapping:        map[string]string{"In Progress": "in_progress"},
		LabelStatusMapping:   map[string]string{"blocked": "blocked"},
	}
	local := config.TrackerLocalConfig{}
	eff := ResolveTrackerConfig(shared, local)

	if eff.Type != "gitlab" {
		t.Errorf("expected Type=gitlab, got %s", eff.Type)
	}
	if !eff.Enabled {
		t.Error("expected Enabled=true (inherited from shared)")
	}
	if !eff.AutoSync {
		t.Error("expected AutoSync=true (inherited from shared)")
	}
	if !eff.PushLabels {
		t.Error("expected PushLabels=true (shared=true, writeEnabled=true)")
	}
	if !eff.AutoPlanAssigned {
		t.Error("expected AutoPlanAssigned=true (inherited from shared)")
	}
	if eff.MaxAutoPlanPerMember != 10 {
		t.Errorf("expected MaxAutoPlanPerMember=10, got %d", eff.MaxAutoPlanPerMember)
	}
	if eff.TrackerURL != "https://gitlab.example.com" {
		t.Errorf("expected TrackerURL inherited, got %s", eff.TrackerURL)
	}
	if eff.TrackerTokenKey != "team-token" {
		t.Errorf("expected TrackerTokenKey inherited, got %s", eff.TrackerTokenKey)
	}
	if eff.TrackerProject != "group/project" {
		t.Errorf("expected TrackerProject inherited, got %s", eff.TrackerProject)
	}
	if eff.TicketPattern != `^PRJ-(\d+)$` {
		t.Errorf("expected TicketPattern inherited, got %s", eff.TicketPattern)
	}
	if eff.SyncIntervalMinutes != 5 {
		t.Errorf("expected SyncIntervalMinutes=5, got %d", eff.SyncIntervalMinutes)
	}
	if !eff.AutoPlanUnassigned {
		t.Error("expected AutoPlanUnassigned=true (inherited from shared)")
	}
	if len(eff.UnassignedLabels) != 1 || eff.UnassignedLabels[0] != "backlog" {
		t.Errorf("expected UnassignedLabels=[backlog], got %v", eff.UnassignedLabels)
	}
	if eff.MaxUnassignedIssues != 50 {
		t.Errorf("expected MaxUnassignedIssues=50, got %d", eff.MaxUnassignedIssues)
	}
	if eff.StatusMapping["In Progress"] != "in_progress" {
		t.Errorf("expected StatusMapping inherited, got %v", eff.StatusMapping)
	}
	if eff.LabelStatusMapping["blocked"] != "blocked" {
		t.Errorf("expected LabelStatusMapping inherited, got %v", eff.LabelStatusMapping)
	}
}

func TestResolveTrackerConfig_LocalOverrides_TakePrecedence(t *testing.T) {
	shared := &teamstate.TrackerConfig{
		Enabled:              true,
		AutoSync:             true,
		PushLabels:           true,
		WriteEnabled:         true,
		AutoPlanAssigned:     true,
		MaxAutoPlanPerMember: 10,
	}
	local := config.TrackerLocalConfig{
		Enabled:              boolPtr(false),
		AutoSync:             boolPtr(false),
		PushLabels:           boolPtr(false),
		AutoPlanAssigned:     boolPtr(false),
		MaxAutoPlanPerMember: intPtrTC(3),
	}
	eff := ResolveTrackerConfig(shared, local)

	if eff.Enabled {
		t.Error("expected Enabled=false (local override)")
	}
	if eff.AutoSync {
		t.Error("expected AutoSync=false (local override)")
	}
	if eff.PushLabels {
		t.Error("expected PushLabels=false (local override, even with writeEnabled=true)")
	}
	if eff.AutoPlanAssigned {
		t.Error("expected AutoPlanAssigned=false (local override)")
	}
	if eff.MaxAutoPlanPerMember != 3 {
		t.Errorf("expected MaxAutoPlanPerMember=3 (local override), got %d", eff.MaxAutoPlanPerMember)
	}

	// Verify overrides are flagged
	if !eff.LocalOverrides.Enabled {
		t.Error("expected LocalOverrides.Enabled=true")
	}
	if !eff.LocalOverrides.AutoSync {
		t.Error("expected LocalOverrides.AutoSync=true")
	}
	if !eff.LocalOverrides.PushLabels {
		t.Error("expected LocalOverrides.PushLabels=true")
	}
	if !eff.LocalOverrides.AutoPlanAssigned {
		t.Error("expected LocalOverrides.AutoPlanAssigned=true")
	}
	if !eff.LocalOverrides.MaxAutoPlanPerMember {
		t.Error("expected LocalOverrides.MaxAutoPlanPerMember=true")
	}
}

func TestResolveTrackerConfig_LocalSameAsShared_NotFlaggedAsOverride(t *testing.T) {
	shared := &teamstate.TrackerConfig{
		Enabled:  true,
		AutoSync: false,
	}
	local := config.TrackerLocalConfig{
		Enabled:  boolPtr(true),  // same as shared
		AutoSync: boolPtr(false), // same as shared
	}
	eff := ResolveTrackerConfig(shared, local)

	if eff.LocalOverrides.Enabled {
		t.Error("expected LocalOverrides.Enabled=false (local == shared)")
	}
	if eff.LocalOverrides.AutoSync {
		t.Error("expected LocalOverrides.AutoSync=false (local == shared)")
	}
}

func TestResolveTrackerConfig_PushLabels_GatedByWriteEnabled(t *testing.T) {
	shared := &teamstate.TrackerConfig{
		PushLabels: true,
	}
	local := config.TrackerLocalConfig{}

	// WriteEnabled=false (default) → PushLabels should be false
	eff := ResolveTrackerConfig(shared, local)
	if eff.PushLabels {
		t.Error("expected PushLabels=false when WriteEnabled=false")
	}
	if !eff.SharedPushLabels {
		t.Error("expected SharedPushLabels=true (raw team recommendation)")
	}

	// WriteEnabled=true on shared → PushLabels should be true
	shared.WriteEnabled = true
	eff = ResolveTrackerConfig(shared, local)
	if !eff.PushLabels {
		t.Error("expected PushLabels=true when WriteEnabled=true")
	}
}

func TestResolveTrackerConfig_PushLabels_Enforced(t *testing.T) {
	shared := &teamstate.TrackerConfig{
		PushLabels:         true,
		PushLabelsEnforced: boolPtr(true),
		WriteEnabled:       true,
	}

	// Local override should be ignored when enforced
	local := config.TrackerLocalConfig{
		PushLabels: boolPtr(false),
	}

	eff := ResolveTrackerConfig(shared, local)
	if !eff.PushLabels {
		t.Error("expected PushLabels=true (enforced by team, WriteEnabled=true)")
	}
	if !eff.PushLabelsEnforced {
		t.Error("expected PushLabelsEnforced=true")
	}
	if eff.LocalOverrides.PushLabels {
		t.Error("expected LocalOverrides.PushLabels=false (enforced → cannot override)")
	}

	// Even enforced, gated by WriteEnabled
	shared.WriteEnabled = false
	eff = ResolveTrackerConfig(shared, local)
	if eff.PushLabels {
		t.Error("expected PushLabels=false (enforced=true but WriteEnabled=false)")
	}
}

func TestResolveTrackerConfig_MaxUnassignedIssues_DefaultWhenZero(t *testing.T) {
	shared := &teamstate.TrackerConfig{
		MaxUnassignedIssues: 0, // zero → should default to 20
	}
	local := config.TrackerLocalConfig{}
	eff := ResolveTrackerConfig(shared, local)

	if eff.MaxUnassignedIssues != 20 {
		t.Errorf("expected MaxUnassignedIssues=20 (default when 0), got %d", eff.MaxUnassignedIssues)
	}
}

// ─── ResolveFullTrackerConfig (3-level: shared + local + project) ───────────

func TestResolveFullTrackerConfig_ProjectOverrides(t *testing.T) {
	shared := &teamstate.TrackerConfig{
		Type:            "gitlab",
		TrackerURL:      "https://gitlab.team.com",
		TrackerTokenKey: "team-token",
		TrackerProject:  "team/default",
		TicketPattern:   `^TEAM-(\d+)$`,
	}
	local := config.TrackerLocalConfig{}
	project := &domain.ProjectTrackerConfig{
		TrackerProject:  "team/specific-project",
		TrackerURL:      "https://gitlab.project.com",
		TrackerTokenKey: "project-token",
		TicketPattern:   `^PROJ-(\d+)$`,
	}

	eff := ResolveFullTrackerConfig(shared, local, project)

	if eff.TrackerProject != "team/specific-project" {
		t.Errorf("expected project override for TrackerProject, got %s", eff.TrackerProject)
	}
	if eff.TrackerURL != "https://gitlab.project.com" {
		t.Errorf("expected project override for TrackerURL, got %s", eff.TrackerURL)
	}
	if eff.TrackerTokenKey != "project-token" {
		t.Errorf("expected project override for TrackerTokenKey, got %s", eff.TrackerTokenKey)
	}
	if eff.TicketPattern != `^PROJ-(\d+)$` {
		t.Errorf("expected project override for TicketPattern, got %s", eff.TicketPattern)
	}
	// Type is NOT overridable per-project — always comes from shared
	if eff.Type != "gitlab" {
		t.Errorf("expected Type=gitlab (from shared, not project), got %s", eff.Type)
	}
}

func TestResolveFullTrackerConfig_EmptyProjectFields_InheritShared(t *testing.T) {
	shared := &teamstate.TrackerConfig{
		TrackerURL:      "https://gitlab.team.com",
		TrackerTokenKey: "team-token",
		TrackerProject:  "team/default",
		TicketPattern:   `^TEAM-(\d+)$`,
	}
	local := config.TrackerLocalConfig{}
	project := &domain.ProjectTrackerConfig{
		// All empty — should inherit from shared
	}

	eff := ResolveFullTrackerConfig(shared, local, project)

	if eff.TrackerURL != "https://gitlab.team.com" {
		t.Errorf("expected shared TrackerURL when project is empty, got %s", eff.TrackerURL)
	}
	if eff.TrackerTokenKey != "team-token" {
		t.Errorf("expected shared TrackerTokenKey when project is empty, got %s", eff.TrackerTokenKey)
	}
	if eff.TrackerProject != "team/default" {
		t.Errorf("expected shared TrackerProject when project is empty, got %s", eff.TrackerProject)
	}
}

func TestResolveFullTrackerConfig_NilProject_InheritShared(t *testing.T) {
	shared := &teamstate.TrackerConfig{
		TrackerURL: "https://gitlab.team.com",
	}
	local := config.TrackerLocalConfig{}

	eff := ResolveFullTrackerConfig(shared, local, nil)

	if eff.TrackerURL != "https://gitlab.team.com" {
		t.Errorf("expected shared TrackerURL when project is nil, got %s", eff.TrackerURL)
	}
}

func TestResolveFullTrackerConfig_NilShared_NilProject(t *testing.T) {
	local := config.TrackerLocalConfig{
		Enabled: boolPtr(true),
	}

	eff := ResolveFullTrackerConfig(nil, local, nil)

	if !eff.Enabled {
		t.Error("expected Enabled=true from local override")
	}
	if eff.Type != "" {
		t.Errorf("expected empty Type when no shared, got %s", eff.Type)
	}
	if eff.TrackerURL != "" {
		t.Errorf("expected empty TrackerURL when no shared, got %s", eff.TrackerURL)
	}
}
