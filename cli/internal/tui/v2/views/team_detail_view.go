package views

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────────────────────────────────────

// TeamDetailViewConfig holds the external dependencies for TeamDetailView.
type TeamDetailViewConfig struct {
	GetMCPConfig          func() config.MCPConfig
	GetTrackerLocalConfig func() config.TrackerLocalConfig
	ResolveTeam           ResolveTeamFunc
	SaveTeamConfig        func(ctx context.Context, cfg *teamstate.TeamConfig) error
	SaveLocalMCP          func(key, value string) error
	SaveLocalTracker      func(key, value string) error
	GetSecrets            func() tracker.SecretGetter
	GetHubConfig          func() *config.Config
	ListProjects          func(ctx context.Context) []ProjectInfo
	SyncTracker           func(ctx context.Context) (*SyncTrackerResult, error)
	// CheckSecret tests whether a keychain key has a value stored.
	// Returns (present, maskedValue) — masked shows last 4 chars like "****7a3f".
	CheckSecret func(ctx context.Context, key string) (present bool, masked string)
	// SetSecret stores a secret value in the keychain.
	SetSecret func(ctx context.Context, key, value string) error
	// TeamProviderKey returns the keychain key of the team-level LLM
	// credential for a provider ("" when no team is active). Used by 'k'.
	TeamProviderKey func(provider string) string
	// OnDiscoverTracker launches the tracker discovery wizard.
	// Called when the user presses 'y' in the team detail view.
	OnDiscoverTracker func()
}

// ─────────────────────────────────────────────────────────────────────────────
// View
// ─────────────────────────────────────────────────────────────────────────────

// TeamDetailView provides an interactive list for editing team-state config
// (tracker, notifications, collaboration) and local overrides.
// MCP and Models are accessible via sub-page links.
type TeamDetailView struct {
	app   *tview.Application
	list  *widgets.SectionedList
	shell ShellAccess
	cfg   TeamDetailViewConfig

	teamCfg    *teamstate.TeamConfig
	localMCP   config.MCPConfig
	localTrk   config.TrackerLocalConfig
	fields     []configField
	dirtyTeam  bool
	dirtyLocal bool
	undoStack  *widgets.UndoStack[teamDetailSnapshot]
}

// teamDetailSnapshot holds the combined state for undo (team + local).
type teamDetailSnapshot struct {
	teamCfg  teamstate.TeamConfig
	localMCP config.MCPConfig
	localTrk config.TrackerLocalConfig
}

var _ View = (*TeamDetailView)(nil)

func NewTeamDetailView(cfg TeamDetailViewConfig) *TeamDetailView {
	return &TeamDetailView{
		cfg:       cfg,
		undoStack: widgets.NewUndoStack[teamDetailSnapshot](10),
	}
}

func (v *TeamDetailView) SetShell(s ShellAccess) { v.shell = s }
func (v *TeamDetailView) ID() string             { return "team.detail" }
func (v *TeamDetailView) Title() string          { return i18n.T("tui.team.detail") }

func (v *TeamDetailView) StatusHints() string {
	return fmt.Sprintf("j/k %s · Space %s · Enter edit · w %s · s %s · t %s · a %s · d %s · u %s · r %s · y discovery · K %s",
		i18n.T("tui.hints.nav"), i18n.T("tui.hints.toggle"), i18n.T("tui.hints.save"),
		i18n.T("tui.hints.sync"), i18n.T("tui.hints.test"), i18n.T("tui.hints.add"),
		i18n.T("tui.hints.del"), i18n.T("tui.hints.undo"), i18n.T("tui.hints.refresh"),
		i18n.T("tui.team.llm_key.hint"))
}

// ─────────────────────────────────────────────────────────────────────────────
// Lifecycle
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.list = widgets.NewSectionedList()
	v.list.SetApp(app)
	v.list.SetBorderPadding(1, 0, 2, 2)

	v.list.SetItemSelectedFunc(func(_ int, item widgets.SectionItem) {
		v.editSelected()
	})

	// Async pull team-state then load
	tc := v.cfg.ResolveTeam()
	if tc.Enabled {
		repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
		syncAsync(v.app, repo, v.shell, func(_ error) {
			v.loadData()
			v.buildFields()
			v.renderFields()
		})
	} else {
		v.loadData()
		v.buildFields()
		v.renderFields()
	}

	content.AddItem(v.list, 0, 1, true)
}

func (v *TeamDetailView) Unmount() {
	if (v.dirtyTeam || v.dirtyLocal) && v.shell != nil {
		v.shell.ShowToastMsg("⚠ Modifications non sauvegardées — 'w' pour sauvegarder", false)
	}
	v.app = nil
	v.list = nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Key handling — navigation is handled by SectionedList
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.list == nil {
		return event
	}
	if event.Key() == tcell.KeyEnter {
		v.editSelected()
		return nil
	}
	switch event.Rune() {
	case ' ':
		v.toggleSelected()
		return nil
	case 'w':
		v.save()
		return nil
	case 's':
		v.syncTracker()
		return nil
	case 't':
		v.testConnection()
		return nil
	case 'a':
		v.addDynamic()
		return nil
	case 'd':
		v.deleteDynamic()
		return nil
	case 'u':
		v.undo()
		return nil
	case 'K':
		v.setTeamProviderKey()
		return nil
	case 'r':
		tc := v.cfg.ResolveTeam()
		if tc.Enabled {
			repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
			syncAsync(v.app, repo, v.shell, func(_ error) {
				v.loadData()
				v.buildFields()
				v.renderFields()
				v.dirtyTeam = false
				v.dirtyLocal = false
			})
		}
		return nil
	case 'y':
		if v.cfg.OnDiscoverTracker != nil {
			v.cfg.OnDiscoverTracker()
		}
		return nil
	}
	return event
}

// ─────────────────────────────────────────────────────────────────────────────
// Data loading
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) loadData() {
	v.localMCP = v.cfg.GetMCPConfig()
	v.localTrk = v.cfg.GetTrackerLocalConfig()

	tc := v.cfg.ResolveTeam()
	if tc.Enabled {
		repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
		if repo.IsCloned() {
			teamCfg, err := repo.LoadConfig()
			if err == nil {
				v.teamCfg = teamCfg
			}
		}
	}
	// Ensure non-nil teamCfg for editing
	if v.teamCfg == nil {
		v.teamCfg = &teamstate.TeamConfig{}
	}
	if v.teamCfg.MCP == nil {
		v.teamCfg.MCP = make(map[string]teamstate.SharedMCPConfig)
	}
	if v.teamCfg.Tracker.Projects == nil { //nolint:staticcheck // backward compat: deprecated field
		v.teamCfg.Tracker.Projects = make(map[string]string) //nolint:staticcheck // backward compat: deprecated field
	}
	if v.teamCfg.Models.Families == nil {
		v.teamCfg.Models.Families = make(map[string]string)
	}
	if v.teamCfg.Models.Agents == nil {
		v.teamCfg.Models.Agents = make(map[string]string)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Build config lines
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) buildFields() {
	v.fields = nil

	// ── Tracker ──────────────────────────────────────────────────────────
	v.fields = append(v.fields, configField{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.config.section.tracker")})
	v.fields = append(v.fields, configField{
		Key: "type", Kind: CfgFieldSelect, Label: i18n.T("tui.config.field.tracker_type.label"),
		Description: i18n.T("tui.config.field.tracker_type.desc"), Scope: ScopeTeamShared,
		Options: []SelectOption{{Label: "GitLab", Value: "gitlab"}, {Label: "Jira", Value: "jira"}},
		Locked:  func() bool { return v.teamCfg.Tracker.IsTypeEnforced() },
		Get:     func() string { return v.teamCfg.Tracker.Type },
		Set:     func(val string) { v.teamCfg.Tracker.Type = val; v.dirtyTeam = true },
	})
	v.fields = append(v.fields, configField{
		Key: "type_enforced", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.type_enforced.label"),
		Description: i18n.T("tui.config.field.type_enforced.desc"), Scope: ScopeTeamShared,
		Get: func() string { return tdBoolToStr(v.teamCfg.Tracker.IsTypeEnforced()) },
		Set: func(val string) {
			b := val == "true"
			v.teamCfg.Tracker.TypeEnforced = &b
			v.dirtyTeam = true
		},
	})
	v.fields = append(v.fields, configField{
		Key: "tracker_url", Kind: CfgFieldString, Label: i18n.T("tui.config.field.tracker_url.label"),
		Description: i18n.T("tui.config.field.tracker_url.desc"), Scope: ScopeTeamShared,
		Placeholder: i18n.T("tui.config.field.tracker_url.placeholder"),
		Get:         func() string { return v.teamCfg.Tracker.TrackerURL },
		Set:         func(val string) { v.teamCfg.Tracker.TrackerURL = val; v.dirtyTeam = true },
	})
	v.fields = append(v.fields, configField{
		Key: "tracker_token", Kind: CfgFieldPassword, Label: i18n.T("tui.config.field.tracker_token.label"),
		Description: i18n.T("tui.config.field.tracker_token.desc"), Scope: ScopeTeamLocal,
		Get: func() string { return v.resolveTrackerTokenKey() },
		Set: func(val string) {
			v.teamCfg.Tracker.TrackerTokenKey = val
			v.dirtyTeam = true
		},
	})
	v.fields = append(v.fields, configField{
		Key: "tracker_project", Kind: CfgFieldString, Label: i18n.T("tui.config.field.tracker_project.label"),
		Description: i18n.T("tui.config.field.tracker_project.desc"), Scope: ScopeTeamShared,
		Placeholder: i18n.T("tui.config.field.tracker_project.placeholder"),
		Get:         func() string { return v.teamCfg.Tracker.TrackerProject },
		Set:         func(val string) { v.teamCfg.Tracker.TrackerProject = val; v.dirtyTeam = true },
	})
	v.fields = append(v.fields, configField{
		Key: "enabled", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.tracker_enabled.label"),
		Description: i18n.T("tui.config.field.tracker_enabled.desc"), Scope: ScopeTeamShared,
		Get: func() string { return tdBoolToStr(v.teamCfg.Tracker.Enabled) },
		Set: func(val string) { v.teamCfg.Tracker.Enabled = val == "true"; v.dirtyTeam = true },
	})
	v.fields = append(v.fields, configField{
		Key: "auto_sync", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.auto_sync.label"),
		Description: i18n.T("tui.config.field.auto_sync.desc"), Scope: ScopeTeamShared,
		Get: func() string { return tdBoolToStr(v.teamCfg.Tracker.AutoSync) },
		Set: func(val string) { v.teamCfg.Tracker.AutoSync = val == "true"; v.dirtyTeam = true },
	})
	v.fields = append(v.fields, configField{
		Key: "push_labels", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.push_labels.label"),
		Description: i18n.T("tui.config.field.push_labels.desc"), Scope: ScopeTeamShared,
		Locked: func() bool { return v.teamCfg.Tracker.IsPushLabelsEnforced() },
		Get:    func() string { return tdBoolToStr(v.teamCfg.Tracker.PushLabels) },
		Set:    func(val string) { v.teamCfg.Tracker.PushLabels = val == "true"; v.dirtyTeam = true },
	})
	v.fields = append(v.fields, configField{
		Key: "push_labels_enforced", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.push_labels_enforced.label"),
		Description: i18n.T("tui.config.field.push_labels_enforced.desc"), Scope: ScopeTeamShared,
		Get: func() string { return tdBoolToStr(v.teamCfg.Tracker.IsPushLabelsEnforced()) },
		Set: func(val string) {
			b := val == "true"
			v.teamCfg.Tracker.PushLabelsEnforced = &b
			v.dirtyTeam = true
		},
	})
	v.fields = append(v.fields, configField{
		Key: "auto_plan_assigned", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.auto_plan_assigned.label"),
		Description: i18n.T("tui.config.field.auto_plan_assigned.desc"), Scope: ScopeTeamShared,
		Get: func() string { return tdBoolToStr(v.teamCfg.Tracker.AutoPlanAssigned) },
		Set: func(val string) { v.teamCfg.Tracker.AutoPlanAssigned = val == "true"; v.dirtyTeam = true },
	})
	v.fields = append(v.fields, configField{
		Key: "max_auto_plan", Kind: CfgFieldInt, Label: i18n.T("tui.config.field.max_auto_plan.label"),
		Description: i18n.T("tui.config.field.max_auto_plan.desc"), Scope: ScopeTeamShared,
		Placeholder: i18n.T("tui.config.field.max_auto_plan.placeholder"),
		Validator:   &FieldValidator{Numeric: true, MinInt: intPtr(0), MaxInt: intPtr(100)},
		Get:         func() string { return strconv.Itoa(v.teamCfg.Tracker.MaxAutoPlanPerMember) },
		Set: func(val string) {
			if n, err := strconv.Atoi(val); err == nil {
				v.teamCfg.Tracker.MaxAutoPlanPerMember = n
				v.dirtyTeam = true
			}
		},
	})
	v.fields = append(v.fields, configField{
		Key: "sync_interval", Kind: CfgFieldInt, Label: i18n.T("tui.config.field.sync_interval.label"),
		Description: i18n.T("tui.config.field.sync_interval.desc"), Scope: ScopeTeamShared,
		Validator: &FieldValidator{Numeric: true, MinInt: intPtr(0), MaxInt: intPtr(60)},
		Get:       func() string { return strconv.Itoa(v.teamCfg.Tracker.SyncIntervalMinutes) },
		Set: func(val string) {
			if n, err := strconv.Atoi(val); err == nil {
				v.teamCfg.Tracker.SyncIntervalMinutes = n
				v.dirtyTeam = true
			}
		},
	})

	// ── Notifications ────────────────────────────────────────────────────
	v.fields = append(v.fields, configField{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.config.section.notifications")})
	v.fields = append(v.fields, configField{
		Key: "type", Kind: CfgFieldSelect, Label: i18n.T("tui.config.field.notif_type.label"),
		Description: i18n.T("tui.config.field.notif_type.desc"), Scope: ScopeTeamShared,
		Options: []SelectOption{
			{Label: "Mattermost", Value: "mattermost"},
			{Label: "Slack", Value: "slack"},
			{Label: "Discord", Value: "discord"},
			{Label: "Teams", Value: "teams"},
		},
		Get: func() string { return v.teamCfg.Notification.Type },
		Set: func(val string) { v.teamCfg.Notification.Type = val; v.dirtyTeam = true },
	})
	v.fields = append(v.fields, configField{
		Key: "webhook_url", Kind: CfgFieldString, Label: i18n.T("tui.config.field.webhook_url.label"),
		Description: i18n.T("tui.config.field.webhook_url.desc"), Scope: ScopeTeamShared,
		Placeholder: i18n.T("tui.config.field.webhook_url.placeholder"),
		Get:         func() string { return v.teamCfg.Notification.WebhookURL },
		Set:         func(val string) { v.teamCfg.Notification.WebhookURL = val; v.dirtyTeam = true },
	})
	v.fields = append(v.fields, configField{
		Key: "channel", Kind: CfgFieldString, Label: i18n.T("tui.config.field.channel.label"),
		Description: i18n.T("tui.config.field.channel.desc"), Scope: ScopeTeamShared,
		Get: func() string { return v.teamCfg.Notification.Channel },
		Set: func(val string) { v.teamCfg.Notification.Channel = val; v.dirtyTeam = true },
	})
	v.fields = append(v.fields, configField{
		Key: "bot_name", Kind: CfgFieldString, Label: i18n.T("tui.config.field.bot_name.label"),
		Description: i18n.T("tui.config.field.bot_name.desc"), Scope: ScopeTeamShared,
		Get: func() string { return v.teamCfg.Notification.BotName },
		Set: func(val string) { v.teamCfg.Notification.BotName = val; v.dirtyTeam = true },
	})

	// ── Collaboration ────────────────────────────────────────────────────
	v.fields = append(v.fields, configField{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.config.section.collaboration")})
	v.fields = append(v.fields, configField{
		Key: "max_sessions", Kind: CfgFieldInt, Label: i18n.T("tui.config.field.max_sessions.label"),
		Description: i18n.T("tui.config.field.max_sessions.desc"), Scope: ScopeTeamShared,
		Placeholder: i18n.T("tui.config.field.max_sessions.placeholder"),
		Validator:   &FieldValidator{Numeric: true, MinInt: intPtr(1), MaxInt: intPtr(20)},
		Get:         func() string { return strconv.Itoa(v.teamCfg.Parallel.MaxSessions) },
		Set: func(val string) {
			if n, err := strconv.Atoi(val); err == nil {
				v.teamCfg.Parallel.MaxSessions = n
				v.dirtyTeam = true
			}
		},
	})
	v.fields = append(v.fields, configField{
		Key: "stale_days", Kind: CfgFieldInt, Label: i18n.T("tui.config.field.stale_days.label"),
		Description: i18n.T("tui.config.field.stale_days.desc"), Scope: ScopeTeamShared,
		Placeholder: i18n.T("tui.config.field.stale_days.placeholder"),
		Validator:   &FieldValidator{Numeric: true, MinInt: intPtr(1), MaxInt: intPtr(30)},
		Get:         func() string { return strconv.Itoa(v.teamCfg.Takeover.StaleDays) },
		Set: func(val string) {
			if n, err := strconv.Atoi(val); err == nil {
				v.teamCfg.Takeover.StaleDays = n
				v.dirtyTeam = true
			}
		},
	})
	v.fields = append(v.fields, configField{
		Key: "done_retention_days", Kind: CfgFieldInt, Label: i18n.T("tui.config.field.done_retention.label"),
		Description: i18n.T("tui.config.field.done_retention.desc"), Scope: ScopeTeamShared,
		Placeholder: i18n.T("tui.config.field.done_retention.placeholder"),
		Validator:   &FieldValidator{Numeric: true, MinInt: intPtr(1), MaxInt: intPtr(90)},
		Get:         func() string { return strconv.Itoa(v.teamCfg.Claim.DoneRetentionDays) },
		Set: func(val string) {
			if n, err := strconv.Atoi(val); err == nil {
				v.teamCfg.Claim.DoneRetentionDays = n
				v.dirtyTeam = true
			}
		},
	})

	// ── Surcharges locales ───────────────────────────────────────────────
	v.fields = append(v.fields, configField{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.config.section.local_overrides")})
	v.fields = append(v.fields, configField{
		Key: "tracker_enabled", Kind: CfgFieldTriBool, Label: i18n.T("tui.config.field.tracker_enabled.label"),
		Description: i18n.T("tui.config.field.tracker_enabled.desc"), Scope: ScopeTeamLocal,
		Get: func() string { return ptrBoolToTriState(v.localTrk.Enabled) },
		Set: func(val string) { v.localTrk.Enabled = triStateToPtrBool(val); v.dirtyLocal = true },
	})
	v.fields = append(v.fields, configField{
		Key: "auto_sync", Kind: CfgFieldTriBool, Label: i18n.T("tui.config.field.auto_sync.label"),
		Description: i18n.T("tui.config.field.auto_sync.desc"), Scope: ScopeTeamLocal,
		Get: func() string { return ptrBoolToTriState(v.localTrk.AutoSync) },
		Set: func(val string) { v.localTrk.AutoSync = triStateToPtrBool(val); v.dirtyLocal = true },
	})
	v.fields = append(v.fields, configField{
		Key: "push_labels", Kind: CfgFieldTriBool, Label: i18n.T("tui.config.field.push_labels.label"),
		Description: i18n.T("tui.config.field.push_labels.desc"), Scope: ScopeTeamLocal,
		Locked: func() bool { return v.teamCfg.Tracker.IsPushLabelsEnforced() },
		Get:    func() string { return ptrBoolToTriState(v.localTrk.PushLabels) },
		Set:    func(val string) { v.localTrk.PushLabels = triStateToPtrBool(val); v.dirtyLocal = true },
	})

	// Links at the bottom
	v.fields = append(v.fields, configField{
		Key: "mcp", Kind: CfgFieldLink, Label: i18n.T("tui.config.link.mcp"), LinkTarget: "team.mcp",
		Get: func() string { return "" }})
	v.fields = append(v.fields, configField{
		Key: "models", Kind: CfgFieldLink, Label: i18n.T("tui.config.link.models"), LinkTarget: "team.models",
		Get: func() string { return "" }})
	v.fields = append(v.fields, configField{
		Key: "workflow", Kind: CfgFieldLink, Label: i18n.T("tui.config.link.workflow"), LinkTarget: "workflow",
		Get: func() string { return "" }})
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) renderFields() {
	if v.list == nil {
		return
	}
	savedIdx := v.list.GetCurrentItem()

	items := make([]widgets.SectionItem, 0, len(v.fields))
	for i, f := range v.fields {
		item := renderConfigItem(f, 28)
		item.Reference = i
		items = append(items, item)
	}

	v.list.SetItems(items)
	if savedIdx >= 0 {
		v.list.SelectIndex(savedIdx)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Editing (delegates to shared editConfigField / toggleConfigField)
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) selectedField() (*configField, int, bool) {
	_, item, ok := v.list.CurrentItem()
	if !ok {
		return nil, -1, false
	}
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.fields) {
		return nil, -1, false
	}
	f := &v.fields[ref]
	if !isSelectable(f.Kind) || f.Get == nil {
		return nil, -1, false
	}
	return f, ref, true
}

func (v *TeamDetailView) toggleSelected() {
	f, _, ok := v.selectedField()
	if !ok {
		return
	}
	if f.Locked != nil && f.Locked() {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.config.enforced_toast"), false)
		}
		return
	}
	if !isToggleable(f.Kind) {
		return
	}
	v.pushUndo()
	toggleConfigField(f)
	v.renderFields()
}

func (v *TeamDetailView) editSelected() {
	f, _, ok := v.selectedField()
	if !ok || v.shell == nil {
		return
	}
	if f.Locked != nil && f.Locked() {
		v.shell.ShowToastMsg(i18n.T("tui.config.enforced_toast"), false)
		return
	}
	if !isEditable(f.Kind) {
		return
	}

	// Special handling for password fields (token setup flow)
	if f.Kind == CfgFieldPassword {
		v.promptTokenSetup()
		return
	}

	// Special handling for links
	if f.Kind == CfgFieldLink {
		editConfigField(v.shell, f, nil)
		return
	}

	v.pushUndo()
	editConfigField(v.shell, f, func() {
		v.renderFields()
	})
}
func (v *TeamDetailView) addDynamic() {
	if v.shell == nil {
		return
	}

	// Determine which dynamic section we're in
	_, item, ok := v.list.CurrentItem()
	section := ""
	if ok {
		ref, refOk := item.Reference.(int)
		if refOk && ref >= 0 && ref < len(v.fields) {
			// Walk back from current field to find section
			for i := ref; i >= 0; i-- {
				f := v.fields[i]
				if f.Kind == CfgFieldSectionHeader || f.Kind == CfgFieldSubHeader {
					section = f.Label
					break
				}
				if f.Section != "" {
					section = f.Section
					break
				}
			}
		}
	}

	switch section {
	case "Mappings", "Mappings projets":
		v.shell.ShowInputModal("Hub project ID", "", func(key string) {
			if key == "" {
				return
			}
			v.shell.ShowInputModal("Tracker project ID/path", "", func(val string) {
				if val == "" {
					return
				}
				if v.teamCfg.Tracker.Projects == nil { //nolint:staticcheck // backward compat: deprecated field
					v.teamCfg.Tracker.Projects = make(map[string]string)
				}
				v.teamCfg.Tracker.Projects[key] = val //nolint:staticcheck // backward compat: deprecated field
				v.dirtyTeam = true
				v.buildFields()
				v.renderFields()
			})
		})
	case "label_status_mapping":
		v.shell.ShowToastMsg("Les mappings label→statut se configurent dans l'éditeur de colonnes (Ctrl+P → Colonnes du board)", false)
	default:
		v.shell.ShowToastMsg("'a' disponible dans: Mappings", false)
	}
}

func (v *TeamDetailView) deleteDynamic() {
	f, _, ok := v.selectedField()
	if !ok || !f.Dynamic || v.shell == nil {
		if v.shell != nil {
			v.shell.ShowToastMsg("'d' disponible uniquement sur les entrées dynamiques", false)
		}
		return
	}

	_ = f.Key
	// NOTE: no sections support deletion yet; early-return above covers all cases.
	// When a section supports deletion, add a switch on f.Section here and
	// set v.dirtyTeam = true, then call v.buildFields() / v.renderFields().
}

// ─────────────────────────────────────────────────────────────────────────────
// Undo
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) pushUndo() {
	if v.teamCfg == nil {
		return
	}
	v.undoStack.Push(teamDetailSnapshot{
		teamCfg:  deepCopyTeamConfig(v.teamCfg),
		localMCP: v.localMCP,
		localTrk: v.localTrk,
	})
}

func (v *TeamDetailView) undo() {
	prev, ok := v.undoStack.Pop()
	if !ok {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.config.nothing_to_undo"), true)
		}
		return
	}
	if v.teamCfg != nil {
		*v.teamCfg = prev.teamCfg
	}
	v.localMCP = prev.localMCP
	v.localTrk = prev.localTrk
	v.dirtyTeam = v.undoStack.Len() > 0
	v.dirtyLocal = v.undoStack.Len() > 0
	v.buildFields()
	v.renderFields()
	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.config.undone"), true)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Persistence
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) save() {
	if v.shell == nil {
		return
	}
	ctx := context.Background()
	var saved []string

	if v.dirtyTeam && v.cfg.SaveTeamConfig != nil {
		if err := v.cfg.SaveTeamConfig(ctx, v.teamCfg); err != nil {
			v.shell.ShowToastMsg("Erreur sauvegarde équipe: "+err.Error(), false)
			return
		}
		saved = append(saved, "équipe")
		v.dirtyTeam = false
	}

	if v.dirtyLocal {
		hubCfg := v.cfg.GetHubConfig()
		if hubCfg != nil {
			hubCfg.MCP = v.localMCP
			hubCfg.Tracker = v.localTrk
			if err := config.Save(hubCfg); err != nil {
				v.shell.ShowToastMsg("Erreur sauvegarde locale: "+err.Error(), false)
				return
			}
		}
		saved = append(saved, "local")
		v.dirtyLocal = false
	}

	if len(saved) == 0 {
		v.shell.ShowToastMsg("Rien à sauvegarder", false)
	} else {
		v.shell.ShowToastMsg("✓ Sauvegardé ("+strings.Join(saved, " + ")+")", true)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Actions (sync + test + token setup)
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) syncTracker() {
	if v.shell == nil || v.app == nil {
		return
	}
	if v.cfg.SyncTracker == nil {
		v.shell.ShowToastMsg("Sync tracker non disponible", false)
		return
	}

	v.shell.ShowToastMsg("Synchronisation en cours...", true)

	app := v.app // capture stable reference before goroutine
	go func() {
		ctx := v.shell.Context()
		select {
		case <-ctx.Done():
			return
		default:
		}
		result, err := v.cfg.SyncTracker(ctx)

		select {
		case <-ctx.Done():
			return
		default:
		}
		app.QueueUpdateDraw(func() {
			if v.shell == nil {
				return
			}
			if err != nil {
				errMsg := err.Error()
				if strings.Contains(errMsg, "credentials manquants") || strings.Contains(errMsg, "token") {
					v.shell.ShowScrollableModal("Token manquant",
						"Le token n'est pas configuré pour ce service.\n\n"+
							"Voulez-vous le saisir maintenant ?\n\n"+
							"Le token sera stocké dans le keychain système (sécurisé).",
						[]ModalAction{
							{Label: "Configurer le token", Callback: func() {
								v.promptTokenSetup()
							}},
							{Label: "Plus tard", Callback: func() {}},
						})
				} else {
					v.shell.ShowToastMsg("✗ Sync échouée: "+errMsg, false)
				}
				return
			}
			content := formatSyncTrackerResultView(result)
			v.shell.ShowScrollableModal("Résultat sync tracker", content, []ModalAction{
				{Label: "OK", Callback: func() {
					v.loadData()
					v.buildFields()
					v.renderFields()
				}},
			})
		})
	}()
}

func (v *TeamDetailView) promptTokenSetup() {
	if v.shell == nil || v.teamCfg == nil {
		return
	}
	trackerType := v.teamCfg.Tracker.Type
	if trackerType == "" {
		trackerType = "gitlab"
	}

	tokenKey := trackerType + "-token"
	hubCfg := v.cfg.GetHubConfig()
	if hubCfg != nil {
		switch trackerType {
		case "gitlab":
			if hubCfg.MCP.Gitlab.Token != "" {
				tokenKey = hubCfg.MCP.Gitlab.Token
			}
		case "jira":
			if hubCfg.MCP.Jira.Token != "" {
				tokenKey = hubCfg.MCP.Jira.Token
			}
		}
	}

	v.shell.ShowPasswordModal("Token "+trackerType+" (clé: "+tokenKey+")", func(value string) {
		if value == "" {
			return
		}
		if hubCfg != nil {
			switch trackerType {
			case "gitlab":
				hubCfg.MCP.Gitlab.Enabled = true
				if hubCfg.MCP.Gitlab.Token == "" {
					hubCfg.MCP.Gitlab.Token = tokenKey
				}
			case "jira":
				hubCfg.MCP.Jira.Enabled = true
				if hubCfg.MCP.Jira.Token == "" {
					hubCfg.MCP.Jira.Token = tokenKey
				}
			}
			if err := config.Save(hubCfg); err != nil {
				if v.shell != nil {
					v.shell.ShowToastMsg("Error: "+err.Error(), false)
				}
			}
		}

		if secrets := v.cfg.GetSecrets(); secrets != nil {
			ctx := context.Background()
			if setter, ok := secrets.(interface {
				Set(ctx context.Context, key, value string) error
			}); ok {
				if err := setter.Set(ctx, tokenKey, value); err != nil {
					if v.shell != nil {
						v.shell.ShowToastMsg("Erreur sauvegarde token: "+err.Error(), false)
					}
					return
				}
			}
		}

		v.shell.ShowToastMsg("✓ Token configuré — relancez 's' pour synchroniser", true)
		v.loadData()
		v.buildFields()
		v.renderFields()
	})
}

// resolveTrackerTokenKey returns the keychain key for the tracker token.
// Uses TrackerTokenKey if set, otherwise derives from "openhub.tracker.<type>.token".
func (v *TeamDetailView) resolveTrackerTokenKey() string {
	if v.teamCfg == nil {
		return ""
	}
	if v.teamCfg.Tracker.TrackerTokenKey != "" {
		return v.teamCfg.Tracker.TrackerTokenKey
	}
	if v.teamCfg.Tracker.Type != "" {
		return "openhub.tracker." + v.teamCfg.Tracker.Type + ".token"
	}
	return ""
}

func (v *TeamDetailView) testConnection() {
	if v.shell == nil || v.app == nil {
		return
	}
	if v.teamCfg == nil || v.teamCfg.Tracker.Type == "" {
		v.shell.ShowToastMsg("Tracker non configuré (configurez le type d'abord)", false)
		return
	}

	v.shell.ShowToastMsg("Test de connexion...", true)

	app := v.app // capture stable reference before goroutine
	go func() {
		ctx := v.shell.Context()
		select {
		case <-ctx.Done():
			return
		default:
		}
		// Resolve effective tracker config and build credential source
		effTracker := tracker.ResolveTrackerConfig(&v.teamCfg.Tracker, v.cfg.GetTrackerLocalConfig())
		src := tracker.NewCredentialSource(effTracker, v.cfg.GetSecrets())

		trackerType := tracker.Type(v.teamCfg.Tracker.Type)
		cfg, err := tracker.ResolveCredentials(ctx, src, trackerType)
		if err != nil {
			app.QueueUpdateDraw(func() {
				if v.shell == nil {
					return
				}
				v.shell.ShowToastMsg("✗ Credentials non disponibles: "+err.Error(), false)
			})
			return
		}

		t, err := tracker.New(cfg)
		if err != nil {
			app.QueueUpdateDraw(func() {
				if v.shell == nil {
					return
				}
				v.shell.ShowToastMsg("✗ Initialisation échouée: "+err.Error(), false)
			})
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
		}
		username, err := t.TestConnection(ctx)
		app.QueueUpdateDraw(func() {
			if v.shell == nil {
				return
			}
			if err != nil {
				v.shell.ShowToastMsg("✗ Connexion échouée: "+err.Error(), false)
			} else {
				v.shell.ShowToastMsg("✓ Connecté en tant que @"+username, true)
			}
		})
	}()
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func tdBoolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func ptrBoolToTriState(p *bool) string {
	if p == nil {
		return "" // not configured
	}
	if *p {
		return "true"
	}
	return "false"
}

func triStateToPtrBool(val string) *bool {
	switch val {
	case "true":
		b := true
		return &b
	case "false":
		b := false
		return &b
	default:
		return nil // not configured
	}
}

func formatSyncTrackerResultView(r *SyncTrackerResult) string {
	if r == nil {
		return "Aucun résultat"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Claims créés:      %d\n", r.ClaimsCreated)
	fmt.Fprintf(&sb, "Claims mis à jour: %d\n", r.ClaimsUpdated)
	fmt.Fprintf(&sb, "Labels poussés:    %d\n", r.LabelsPushed)
	if len(r.Projects) > 0 {
		sb.WriteString("\nProjets:\n")
		for _, p := range r.Projects {
			fmt.Fprintf(&sb, "  %s\n", p)
		}
	}
	if len(r.Warnings) > 0 {
		sb.WriteString("\nWarnings:\n")
		for _, w := range r.Warnings {
			fmt.Fprintf(&sb, "  ⚠ %s\n", w)
		}
	}
	if len(r.Errors) > 0 {
		sb.WriteString("\nErreurs:\n")
		for _, e := range r.Errors {
			fmt.Fprintf(&sb, "  ✗ %s\n", e)
		}
	}
	return sb.String()
}

// setTeamProviderKey stores the team-level LLM credential of a provider in
// the keychain (cascade: project key → team key → hub key).
func (v *TeamDetailView) setTeamProviderKey() {
	if v.shell == nil || v.cfg.TeamProviderKey == nil || v.cfg.SetSecret == nil {
		return
	}
	options := []SelectOption{
		{Label: i18n.T("tui.team.llm_key.bedrock"), Value: "bedrock"},
		{Label: "Anthropic", Value: "anthropic"},
		{Label: "OpenRouter", Value: "openrouter"},
	}
	v.shell.ShowSelectModal(i18n.T("tui.team.llm_key.choose_provider"), options, "bedrock", func(prov string) {
		key := v.cfg.TeamProviderKey(prov)
		if key == "" {
			v.shell.ShowToastMsg(i18n.T("tui.team.llm_key.no_team"), false)
			return
		}
		v.shell.ShowPasswordModal(i18n.Tf("tui.team.llm_key.prompt", prov), func(value string) {
			if value == "" {
				return
			}
			if err := v.cfg.SetSecret(context.Background(), key, value); err != nil {
				v.shell.ShowToastMsg(err.Error(), false)
				return
			}
			v.shell.ShowToastMsg(i18n.Tf("tui.team.llm_key.saved", prov), true)
		})
	})
}
