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
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────────────────────────────────────

type configScope string

const (
	scopeTeam  configScope = "team"
	scopeLocal configScope = "local"
)

type teamConfigLine struct {
	section    string
	key        string
	kind       string // "bool", "string", "select", "tri-state", "password", "section-header", "sub-header", "link", "placeholder"
	options    []SelectOption
	scope      configScope
	dynamic    bool         // can be added/deleted (mappings)
	grayed     func() bool  // returns true if field is grayed-out (enforced elsewhere)
	hint       string       // help text shown when value is empty
	linkTarget string       // view ID to navigate to for "link" kind
	get        func() string
	set        func(val string)
}

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
	lines      []teamConfigLine
	dirtyTeam  bool
	dirtyLocal bool
}

var _ View = (*TeamDetailView)(nil)

func NewTeamDetailView(cfg TeamDetailViewConfig) *TeamDetailView {
	return &TeamDetailView{cfg: cfg}
}

func (v *TeamDetailView) SetShell(s ShellAccess) { v.shell = s }
func (v *TeamDetailView) ID() string             { return "team.detail" }
func (v *TeamDetailView) Title() string          { return i18n.T("tui.team.detail") }

func (v *TeamDetailView) StatusHints() string {
	return fmt.Sprintf("j/k %s · Space %s · Enter edit · w %s · s %s · t %s · a %s · d %s · u %s · r %s",
		i18n.T("tui.hints.nav"), i18n.T("tui.hints.toggle"), i18n.T("tui.hints.save"),
		i18n.T("tui.hints.sync"), i18n.T("tui.hints.test"), i18n.T("tui.hints.add"),
		i18n.T("tui.hints.del"), i18n.T("tui.hints.undo"), i18n.T("tui.hints.refresh"))
}

// ─────────────────────────────────────────────────────────────────────────────
// Lifecycle
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.list = widgets.NewSectionedList()
	v.list.SetApp(app)
	v.list.ShowSecondaryText(false)
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
			v.buildLines()
			v.renderLines()
		})
	} else {
		v.loadData()
		v.buildLines()
		v.renderLines()
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
	switch event.Key() {
	case tcell.KeyEnter:
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
		v.loadData()
		v.buildLines()
		v.renderLines()
		v.dirtyTeam = false
		v.dirtyLocal = false
		if v.shell != nil {
			v.shell.ShowToastMsg("↩ Rechargé", true)
		}
		return nil
	case 'r':
		tc := v.cfg.ResolveTeam()
		if tc.Enabled {
			repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
			syncAsync(v.app, repo, v.shell, func(_ error) {
				v.loadData()
				v.buildLines()
				v.renderLines()
				v.dirtyTeam = false
				v.dirtyLocal = false
			})
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
	if v.teamCfg.Tracker.Projects == nil {
		v.teamCfg.Tracker.Projects = make(map[string]string)
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

func (v *TeamDetailView) buildLines() {
	v.lines = nil

	// ── Raccourcis ──
	v.lines = append(v.lines, teamConfigLine{kind: "section-header", section: "Raccourcis"})
	v.lines = append(v.lines, teamConfigLine{
		kind: "link", key: "mcp", linkTarget: "team.mcp",
		get: func() string { return "MCP Services..." },
	})
	v.lines = append(v.lines, teamConfigLine{
		kind: "link", key: "models", linkTarget: "team.models",
		get: func() string { return "Modèles..." },
	})

	// ── Tracker ──
	v.lines = append(v.lines, teamConfigLine{kind: "section-header", section: "Tracker"})
	v.lines = append(v.lines, teamConfigLine{
		section: "Tracker", key: "type", kind: "select", scope: scopeTeam,
		hint:    "Type de tracker externe pour la sync issues",
		options: []SelectOption{{Label: "GitLab", Value: "gitlab"}, {Label: "Jira", Value: "jira"}},
		get:     func() string { return v.teamCfg.Tracker.Type },
		set:     func(val string) { v.teamCfg.Tracker.Type = val; v.dirtyTeam = true },
	})
	v.lines = append(v.lines, teamConfigLine{
		section: "Tracker", key: "tracker_url", kind: "string", scope: scopeTeam,
		hint: "URL de l'instance GitLab/Jira (ex: https://gitlab.example.com)",
		get:  func() string { return v.teamCfg.Tracker.TrackerURL },
		set:  func(val string) { v.teamCfg.Tracker.TrackerURL = val; v.dirtyTeam = true },
	})
	v.lines = append(v.lines, teamConfigLine{
		section: "Tracker", key: "tracker_token", kind: "password", scope: scopeLocal,
		hint: "Token d'accès pour le tracker (stocké dans le keychain)",
		get:  func() string { return v.resolveTrackerTokenKey() },
		set: func(val string) {
			v.teamCfg.Tracker.TrackerTokenKey = val
			v.dirtyTeam = true
		},
	})
	v.lines = append(v.lines, teamConfigLine{
		section: "Tracker", key: "tracker_project", kind: "string", scope: scopeTeam,
		hint: "ID ou path du projet sur le tracker (ex: group/project ou 42)",
		get:  func() string { return v.teamCfg.Tracker.TrackerProject },
		set:  func(val string) { v.teamCfg.Tracker.TrackerProject = val; v.dirtyTeam = true },
	})
	v.lines = append(v.lines, teamConfigLine{
		section: "Tracker", key: "enabled", kind: "bool", scope: scopeTeam,
		get: func() string { return tdBoolToStr(v.teamCfg.Tracker.Enabled) },
		set: func(val string) { v.teamCfg.Tracker.Enabled = val == "true"; v.dirtyTeam = true },
	})
	v.lines = append(v.lines, teamConfigLine{
		section: "Tracker", key: "auto_sync", kind: "bool", scope: scopeTeam,
		get: func() string { return tdBoolToStr(v.teamCfg.Tracker.AutoSync) },
		set: func(val string) { v.teamCfg.Tracker.AutoSync = val == "true"; v.dirtyTeam = true },
	})
	v.lines = append(v.lines, teamConfigLine{
		section: "Tracker", key: "push_labels", kind: "bool", scope: scopeTeam,
		get: func() string { return tdBoolToStr(v.teamCfg.Tracker.PushLabels) },
		set: func(val string) { v.teamCfg.Tracker.PushLabels = val == "true"; v.dirtyTeam = true },
	})
	v.lines = append(v.lines, teamConfigLine{
		section: "Tracker", key: "auto_plan_assigned", kind: "bool", scope: scopeTeam,
		get: func() string { return tdBoolToStr(v.teamCfg.Tracker.AutoPlanAssigned) },
		set: func(val string) { v.teamCfg.Tracker.AutoPlanAssigned = val == "true"; v.dirtyTeam = true },
	})
	v.lines = append(v.lines, teamConfigLine{
		section: "Tracker", key: "max_auto_plan", kind: "string", scope: scopeTeam,
		get: func() string { return strconv.Itoa(v.teamCfg.Tracker.MaxAutoPlanPerMember) },
		set: func(val string) {
			n, err := strconv.Atoi(val)
			if err != nil {
				return
			}
			v.teamCfg.Tracker.MaxAutoPlanPerMember = n
			v.dirtyTeam = true
		},
	})
	v.lines = append(v.lines, teamConfigLine{
		section: "Tracker", key: "sync_interval_min", kind: "string", scope: scopeTeam,
		get: func() string { return strconv.Itoa(v.teamCfg.Tracker.SyncIntervalMinutes) },
		set: func(val string) {
			n, err := strconv.Atoi(val)
			if err != nil {
				return
			}
			v.teamCfg.Tracker.SyncIntervalMinutes = n
			v.dirtyTeam = true
		},
	})

	// ── Label → Status Mapping (ADR-032) ──
	v.lines = append(v.lines, teamConfigLine{kind: "sub-header", section: "label_status_mapping",
		hint: "Mappe les labels du tracker vers les colonnes du board. Premier label qui matche gagne."})
	for k := range v.teamCfg.Tracker.LabelStatusMapping {
		k := k
		v.lines = append(v.lines, teamConfigLine{
			section: "label_status_mapping", key: k, kind: "select", scope: scopeTeam, dynamic: true,
			hint:    "Statut du board pour le label '" + k + "'",
			options: labelStatusOptions(),
			get:     func() string { return v.teamCfg.Tracker.LabelStatusMapping[k] },
			set:     func(val string) { v.teamCfg.Tracker.LabelStatusMapping[k] = val; v.dirtyTeam = true },
		})
	}
	if len(v.teamCfg.Tracker.LabelStatusMapping) == 0 {
		v.lines = append(v.lines, teamConfigLine{
			section: "label_status_mapping", key: "(vide)", kind: "placeholder",
			hint: "a pour ajouter un mapping label → statut",
			get:  func() string { return "" },
		})
	}

	// ── Notifications ──
	v.lines = append(v.lines, teamConfigLine{kind: "section-header", section: "Notifications"})
	v.lines = append(v.lines, teamConfigLine{
		section: "Notifications", key: "type", kind: "select", scope: scopeTeam,
		options: []SelectOption{
			{Label: "Mattermost", Value: "mattermost"},
			{Label: "Slack", Value: "slack"},
			{Label: "Discord", Value: "discord"},
			{Label: "Teams", Value: "teams"},
		},
		get: func() string { return v.teamCfg.Notification.Type },
		set: func(val string) { v.teamCfg.Notification.Type = val; v.dirtyTeam = true },
	})
	v.lines = append(v.lines, teamConfigLine{
		section: "Notifications", key: "webhook_url", kind: "string", scope: scopeTeam,
		hint: "URL du webhook (Intégrations > Webhooks entrants)",
		get:  func() string { return v.teamCfg.Notification.WebhookURL },
		set:  func(val string) { v.teamCfg.Notification.WebhookURL = val; v.dirtyTeam = true },
	})
	v.lines = append(v.lines, teamConfigLine{
		section: "Notifications", key: "channel", kind: "string", scope: scopeTeam,
		get: func() string { return v.teamCfg.Notification.Channel },
		set: func(val string) { v.teamCfg.Notification.Channel = val; v.dirtyTeam = true },
	})
	v.lines = append(v.lines, teamConfigLine{
		section: "Notifications", key: "bot_name", kind: "string", scope: scopeTeam,
		get: func() string { return v.teamCfg.Notification.BotName },
		set: func(val string) { v.teamCfg.Notification.BotName = val; v.dirtyTeam = true },
	})

	// ── Collaboration ──
	v.lines = append(v.lines, teamConfigLine{kind: "section-header", section: "Collaboration"})
	v.lines = append(v.lines, teamConfigLine{
		section: "Collaboration", key: "max_sessions", kind: "string", scope: scopeTeam,
		get: func() string { return strconv.Itoa(v.teamCfg.Parallel.MaxSessions) },
		set: func(val string) {
			n, err := strconv.Atoi(val)
			if err != nil {
				return
			}
			v.teamCfg.Parallel.MaxSessions = n
			v.dirtyTeam = true
		},
	})
	v.lines = append(v.lines, teamConfigLine{
		section: "Collaboration", key: "stale_days", kind: "string", scope: scopeTeam,
		get: func() string { return strconv.Itoa(v.teamCfg.Takeover.StaleDays) },
		set: func(val string) {
			n, err := strconv.Atoi(val)
			if err != nil {
				return
			}
			v.teamCfg.Takeover.StaleDays = n
			v.dirtyTeam = true
		},
	})
	v.lines = append(v.lines, teamConfigLine{
		section: "Collaboration", key: "done_retention_days", kind: "string", scope: scopeTeam,
		get: func() string { return strconv.Itoa(v.teamCfg.Claim.DoneRetentionDays) },
		set: func(val string) {
			n, err := strconv.Atoi(val)
			if err != nil {
				return
			}
			v.teamCfg.Claim.DoneRetentionDays = n
			v.dirtyTeam = true
		},
	})

	// ── Overrides locaux ──
	v.lines = append(v.lines, teamConfigLine{kind: "section-header", section: "Overrides locaux"})
	v.lines = append(v.lines, teamConfigLine{
		section: "Local", key: "tracker enabled", kind: "tri-state", scope: scopeLocal,
		get: func() string { return ptrBoolToTriState(v.localTrk.Enabled) },
		set: func(val string) { v.localTrk.Enabled = triStateToPtrBool(val); v.dirtyLocal = true },
	})
	v.lines = append(v.lines, teamConfigLine{
		section: "Local", key: "auto_sync", kind: "tri-state", scope: scopeLocal,
		get: func() string { return ptrBoolToTriState(v.localTrk.AutoSync) },
		set: func(val string) { v.localTrk.AutoSync = triStateToPtrBool(val); v.dirtyLocal = true },
	})
	v.lines = append(v.lines, teamConfigLine{
		section: "Local", key: "push_labels", kind: "tri-state", scope: scopeLocal,
		get: func() string { return ptrBoolToTriState(v.localTrk.PushLabels) },
		set: func(val string) { v.localTrk.PushLabels = triStateToPtrBool(val); v.dirtyLocal = true },
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) renderLines() {
	if v.list == nil {
		return
	}
	savedIdx := v.list.GetCurrentItem()

	var items []widgets.SectionItem
	for i, line := range v.lines {
		if line.kind == "section-header" || line.kind == "sub-header" {
			items = append(items, widgets.SectionItem{
				IsHeader: true,
				MainText: line.section,
			})
			continue
		}

		val := ""
		if line.get != nil {
			val = line.get()
		}

		var display string
		switch line.kind {
		case "link":
			display = fmt.Sprintf("%s→ %s%s", theme.ColorTag(theme.AccentHex), val, theme.TagColor)
		case "password":
			display = v.formatToken(val)
		default:
			display = v.formatValueWithHint(val, line.kind, line.hint)
		}

		grayedSuffix := ""
		if line.grayed != nil && line.grayed() {
			grayedSuffix = fmt.Sprintf("  %s(enforced par l'équipe)%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor)
			rawVal := val
			if rawVal == "" {
				rawVal = "(vide)"
			}
			display = fmt.Sprintf("%s%s%s", theme.ColorTag(theme.TextMutedHex), rawVal, theme.TagColor)
		}

		prefix := ""
		if line.dynamic {
			prefix = "  "
		}

		mainText := fmt.Sprintf("%s%-24s %s%s", prefix, line.key+":", display, grayedSuffix)
		items = append(items, widgets.SectionItem{
			MainText:  mainText,
			Reference: i,
		})
	}

	v.list.SetItems(items)
	if savedIdx >= 0 {
		v.list.SelectIndex(savedIdx)
	}
}

func (v *TeamDetailView) formatToken(tokenKey string) string {
	if tokenKey == "" {
		return fmt.Sprintf("%s(non configuré)%s  %s[✗ non configuré]%s",
			theme.ColorTag(theme.TextMutedHex), theme.TagColor,
			theme.ColorTag("#FF5252"), theme.TagColor)
	}
	if v.cfg.CheckSecret != nil {
		ctx := context.Background()
		present, masked := v.cfg.CheckSecret(ctx, tokenKey)
		if present {
			return fmt.Sprintf("%s  %s[✓ configuré]%s", masked, theme.ColorTag(theme.SuccessHex), theme.TagColor)
		}
	}
	return fmt.Sprintf("%s  %s[✗ non configuré]%s", tokenKey, theme.ColorTag("#FF5252"), theme.TagColor)
}

func (v *TeamDetailView) formatValue(val, kind string) string {
	switch kind {
	case "bool":
		if val == "true" {
			return fmt.Sprintf("%s✓ %s%s", theme.ColorTag(theme.SuccessHex), i18n.T("tui.settings.enabled"), theme.TagColor)
		}
		return fmt.Sprintf("%s✗ %s%s", theme.ColorTag("#FF5252"), i18n.T("tui.settings.disabled"), theme.TagColor)
	case "tri-state":
		switch val {
		case "true":
			return fmt.Sprintf("%s✓ %s%s", theme.ColorTag(theme.SuccessHex), i18n.T("tui.settings.enabled"), theme.TagColor)
		case "false":
			return fmt.Sprintf("%s✗ %s%s", theme.ColorTag("#FF5252"), i18n.T("tui.settings.disabled"), theme.TagColor)
		default:
			return fmt.Sprintf("%s↩ %s%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.settings.inherited"), theme.TagColor)
		}
	default:
		if val == "" {
			return fmt.Sprintf("%s(vide)%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor)
		}
		return val
	}
}

// formatValueWithHint adds a hint annotation when the value is empty.
func (v *TeamDetailView) formatValueWithHint(val, kind, hint string) string {
	if val == "" && hint != "" {
		return fmt.Sprintf("%s(vide) ← %s%s", theme.ColorTag(theme.TextMutedHex), hint, theme.TagColor)
	}
	return v.formatValue(val, kind)
}

// ─────────────────────────────────────────────────────────────────────────────
// Editing
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) selectedLine() (teamConfigLine, int, bool) {
	_, item, ok := v.list.CurrentItem()
	if !ok {
		return teamConfigLine{}, -1, false
	}
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.lines) {
		return teamConfigLine{}, -1, false
	}
	line := v.lines[ref]
	if line.kind == "section-header" || line.kind == "sub-header" || line.get == nil {
		return teamConfigLine{}, -1, false
	}
	return line, ref, true
}

func (v *TeamDetailView) toggleSelected() {
	line, _, ok := v.selectedLine()
	if !ok {
		return
	}
	// Check grayed (enforced by team)
	if line.grayed != nil && line.grayed() {
		if v.shell != nil {
			v.shell.ShowToastMsg("Imposé par l'équipe (non-modifiable)", false)
		}
		return
	}
	switch line.kind {
	case "bool":
		cur := line.get()
		if cur == "true" {
			line.set("false")
		} else {
			line.set("true")
		}
	case "tri-state":
		cur := line.get()
		switch cur {
		case "inherit":
			line.set("true")
		case "true":
			line.set("false")
		case "false":
			line.set("inherit")
		}
	default:
		return
	}
	v.renderLines()
}

func (v *TeamDetailView) editSelected() {
	line, _, ok := v.selectedLine()
	if !ok || v.shell == nil {
		return
	}
	// Check grayed (enforced by team)
	if line.grayed != nil && line.grayed() {
		v.shell.ShowToastMsg("Imposé par l'équipe (non-modifiable)", false)
		return
	}

	switch line.kind {
	case "link":
		if line.linkTarget != "" && v.shell != nil {
			v.shell.NavigateTo(line.linkTarget)
		}
	case "bool", "tri-state":
		v.toggleSelected()
	case "select":
		v.shell.ShowSelectModal(line.key, line.options, line.get(), func(val string) {
			line.set(val)
			v.renderLines()
		})
	case "string":
		v.shell.ShowInputModal(line.key, line.get(), func(val string) {
			line.set(val)
			v.renderLines()
		})
	case "placeholder":
		v.addDynamic()
	case "password":
		tokenKey := line.get()
		if tokenKey == "" {
			// For tracker token, derive default key from tracker type
			if v.teamCfg.Tracker.Type != "" {
				tokenKey = config.DefaultTokenKeyForService(v.teamCfg.Tracker.Type)
			} else {
				tokenKey = config.DefaultTokenKeyForService("gitlab")
			}
			line.set(tokenKey)
		}
		v.shell.ShowPasswordModal("Valeur du token ("+tokenKey+")", func(val string) {
			if val == "" {
				return
			}
			if v.cfg.SetSecret != nil {
				ctx := context.Background()
				if err := v.cfg.SetSecret(ctx, tokenKey, val); err != nil {
					if v.shell != nil {
						v.shell.ShowToastMsg("Erreur sauvegarde token: "+err.Error(), false)
					}
					return
				}
			}
			v.renderLines()
			if v.shell != nil {
				v.shell.ShowToastMsg("✓ Token enregistré", true)
			}
		})
	}
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
		if refOk && ref >= 0 && ref < len(v.lines) {
			// Walk back from current line to find section
			for i := ref; i >= 0; i-- {
				l := v.lines[i]
				if l.kind == "section-header" || l.kind == "sub-header" {
					section = l.section
					break
				}
				if l.section != "" {
					section = l.section
					break
				}
			}
		}
	}

	switch {
	case section == "Mappings" || section == "Mappings projets":
		v.shell.ShowInputModal("Hub project ID", "", func(key string) {
			if key == "" {
				return
			}
			v.shell.ShowInputModal("Tracker project ID/path", "", func(val string) {
				if val == "" {
					return
				}
				if v.teamCfg.Tracker.Projects == nil {
					v.teamCfg.Tracker.Projects = make(map[string]string)
				}
				v.teamCfg.Tracker.Projects[key] = val
				v.dirtyTeam = true
				v.buildLines()
				v.renderLines()
			})
		})
	case section == "label_status_mapping":
		v.shell.ShowInputModal("Label du tracker (ex: Bloqué, TO REVIEW...)", "", func(key string) {
			if key == "" {
				return
			}
			v.shell.ShowSelectModal("Statut du board", labelStatusOptions(), "", func(val string) {
				if val == "" {
					return
				}
				if v.teamCfg.Tracker.LabelStatusMapping == nil {
					v.teamCfg.Tracker.LabelStatusMapping = make(map[string]string)
				}
				v.teamCfg.Tracker.LabelStatusMapping[key] = val
				v.dirtyTeam = true
				v.buildLines()
				v.renderLines()
			})
		})
	default:
		v.shell.ShowToastMsg("'a' disponible dans: Mappings, label_status_mapping", false)
	}
}

func (v *TeamDetailView) deleteDynamic() {
	line, _, ok := v.selectedLine()
	if !ok || !line.dynamic || v.shell == nil {
		if v.shell != nil {
			v.shell.ShowToastMsg("'d' disponible uniquement sur les entrées dynamiques", false)
		}
		return
	}

	key := line.key
	switch {
	case line.section == "Mappings":
		delete(v.teamCfg.Tracker.Projects, key)
	case line.section == "label_status_mapping":
		delete(v.teamCfg.Tracker.LabelStatusMapping, key)
	default:
		return
	}

	v.dirtyTeam = true
	v.buildLines()
	v.renderLines()
	if v.shell != nil {
		v.shell.ShowToastMsg("Supprimé: "+key, true)
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
					v.buildLines()
					v.renderLines()
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
		_ = config.Save(hubCfg)
	}

	if secrets := v.cfg.GetSecrets(); secrets != nil {
		ctx := context.Background()
		if setter, ok := secrets.(interface{ Set(ctx context.Context, key, value string) error }); ok {
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
		v.buildLines()
		v.renderLines()
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
		mcp := v.cfg.GetMCPConfig()
		var sharedGitLab, sharedJira *teamstate.SharedMCPConfig
		if v.teamCfg.MCP != nil {
			if g, ok := v.teamCfg.MCP["gitlab"]; ok {
				sharedGitLab = &g
			}
			if j, ok := v.teamCfg.MCP["jira"]; ok {
				sharedJira = &j
			}
		}
		effGL := tracker.ResolveMCPConfig(sharedGitLab, mcp.Gitlab)
		effJira := tracker.ResolveMCPConfig(sharedJira, mcp.Jira)
		src := tracker.CredentialSource{
			GitLabEnabled:      effGL.Enabled,
			GitLabTokenKey:     effGL.TokenKey,
			GitLabWriteEnabled: effGL.WriteEnabled,
			GitLabURL:          effGL.URL,
			JiraEnabled:        effJira.Enabled,
			JiraTokenKey:       effJira.TokenKey,
			JiraWriteEnabled:   effJira.WriteEnabled,
			JiraURL:            effJira.URL,
			Secrets:            v.cfg.GetSecrets(),
		}
		// Apply tracker-specific overrides
		if v.teamCfg.Tracker.TrackerURL != "" {
			src.TrackerURL = v.teamCfg.Tracker.TrackerURL
		}
		tokenKey := v.teamCfg.Tracker.TrackerTokenKey
		if tokenKey == "" && v.teamCfg.Tracker.TrackerURL != "" && v.teamCfg.Tracker.Type != "" {
			tokenKey = "openhub.tracker." + v.teamCfg.Tracker.Type + ".token"
		}
		if tokenKey != "" {
			src.TrackerTokenKey = tokenKey
		}

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

func boolPtrToStr(b *bool) string {
	if b == nil {
		return "false"
	}
	if *b {
		return "true"
	}
	return "false"
}

// labelStatusOptions returns the valid board statuses for the label_status_mapping select.
func labelStatusOptions() []SelectOption {
	return []SelectOption{
		{Label: "planned (TODO)", Value: "planned"},
		{Label: "in_progress (IN PROGRESS)", Value: "in_progress"},
		{Label: "review (REVIEW)", Value: "review"},
		{Label: "validation (VALIDATION)", Value: "validation"},
		{Label: "blocked (BLOCKED)", Value: "blocked"},
		{Label: "done (DONE)", Value: "done"},
	}
}

func tdBoolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func ptrBoolToTriState(p *bool) string {
	if p == nil {
		return "inherit"
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
		return nil
	}
}

func formatSyncTrackerResultView(r *SyncTrackerResult) string {
	if r == nil {
		return "Aucun résultat"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Claims créés:      %d\n", r.ClaimsCreated))
	sb.WriteString(fmt.Sprintf("Claims mis à jour: %d\n", r.ClaimsUpdated))
	sb.WriteString(fmt.Sprintf("Labels poussés:    %d\n", r.LabelsPushed))
	if len(r.Projects) > 0 {
		sb.WriteString("\nProjets:\n")
		for _, p := range r.Projects {
			sb.WriteString(fmt.Sprintf("  %s\n", p))
		}
	}
	if len(r.Warnings) > 0 {
		sb.WriteString("\nWarnings:\n")
		for _, w := range r.Warnings {
			sb.WriteString(fmt.Sprintf("  ⚠ %s\n", w))
		}
	}
	if len(r.Errors) > 0 {
		sb.WriteString("\nErreurs:\n")
		for _, e := range r.Errors {
			sb.WriteString(fmt.Sprintf("  ✗ %s\n", e))
		}
	}
	return sb.String()
}
