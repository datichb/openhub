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
	section string
	key     string
	kind    string // "bool", "string", "select", "tri-state", "password", "section-header", "sub-header"
	options []SelectOption
	scope   configScope
	dynamic bool         // can be added/deleted (mappings, families, agents)
	grayed  func() bool  // returns true if field is grayed-out (enforced elsewhere)
	hint    string       // help text shown when value is empty
	get     func() string
	set     func(val string)
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
// (MCP, tracker, notifications, collaboration, models) and local overrides.
type TeamDetailView struct {
	app   *tview.Application
	list  *tview.List
	shell ShellAccess
	cfg   TeamDetailViewConfig

	teamCfg    *teamstate.TeamConfig
	localMCP   config.MCPConfig
	localTrk   config.TrackerLocalConfig
	lines      []teamConfigLine
	lineMap    []int // lineMap[listIdx] = index in v.lines (-1 for spacer)
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
	return "j/k nav · Space toggle · Enter edit · w save · s sync · t test · a add · d del · r refresh"
}

// ─────────────────────────────────────────────────────────────────────────────
// Lifecycle
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.list = tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSelectedTextColor(theme.FgPrimary).
		SetSelectedBackgroundColor(theme.BgElement)
	v.list.SetBackgroundColor(theme.BgPanel)
	v.list.SetBorderPadding(1, 0, 2, 2)

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
// Key handling
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamDetailView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEnter:
		v.editSelected()
		return nil
	case tcell.KeyDown:
		v.moveDown()
		return nil
	case tcell.KeyUp:
		v.moveUp()
		return nil
	}
	switch event.Rune() {
	case 'j':
		v.moveDown()
		return nil
	case 'k':
		v.moveUp()
		return nil
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

// moveDown moves cursor to the next selectable (non-header, non-spacer) item.
func (v *TeamDetailView) moveDown() {
	if v.list == nil {
		return
	}
	cur := v.list.GetCurrentItem()
	for i := cur + 1; i < v.list.GetItemCount(); i++ {
		if v.isSelectable(i) {
			v.list.SetCurrentItem(i)
			return
		}
	}
}

// moveUp moves cursor to the previous selectable item.
func (v *TeamDetailView) moveUp() {
	if v.list == nil {
		return
	}
	cur := v.list.GetCurrentItem()
	for i := cur - 1; i >= 0; i-- {
		if v.isSelectable(i) {
			v.list.SetCurrentItem(i)
			return
		}
	}
}

// isSelectable returns true if the list item at idx is an editable field (not a header/spacer).
func (v *TeamDetailView) isSelectable(idx int) bool {
	if idx < 0 || idx >= len(v.lineMap) {
		return false
	}
	lineIdx := v.lineMap[idx]
	if lineIdx < 0 {
		return false // spacer
	}
	line := v.lines[lineIdx]
	return line.kind != "section-header" && line.kind != "sub-header" && line.get != nil
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

	// URL label and hints per service
	urlLabels := map[string]string{
		"gitlab":  "url (issues/MRs)",
		"jira":    "url (issues)",
		"figma":   "url (API)",
		"gslides": "url (API)",
	}
	urlHintsTeam := map[string]string{
		"gitlab":  "URL instance GitLab pour les issues et MRs",
		"jira":    "URL instance Jira (ex: https://jira.company.com)",
		"figma":   "URL API Figma (vide = SaaS public)",
		"gslides": "URL API Google (vide = SaaS public)",
	}
	urlHintsPerso := map[string]string{
		"gitlab":  "Override perso (vide = utilise celle de l'équipe)",
		"jira":    "Override perso (vide = utilise celle de l'équipe)",
		"figma":   "Override perso (vide = SaaS public)",
		"gslides": "Override perso (vide = SaaS public)",
	}

	// ── MCP Services ──
	for _, svc := range []string{"gitlab", "jira", "figma", "gslides"} {
		svc := svc // capture
		v.lines = append(v.lines, teamConfigLine{kind: "section-header", section: "MCP " + strings.Title(svc)})

		// ── Équipe ──
		v.lines = append(v.lines, teamConfigLine{kind: "sub-header", section: "équipe"})
		v.lines = append(v.lines, teamConfigLine{
			section: "MCP", key: "enabled", kind: "bool", scope: scopeTeam,
			get: func() string { return boolPtrToStr(v.teamCfg.MCP[svc].Enabled) },
			set: func(val string) { s := v.teamCfg.MCP[svc]; b := val == "true"; s.Enabled = &b; v.teamCfg.MCP[svc] = s; v.dirtyTeam = true },
		})
		v.lines = append(v.lines, teamConfigLine{
			section: "MCP", key: "enabled_enforced", kind: "bool", scope: scopeTeam,
			get: func() string { return boolPtrToStr(v.teamCfg.MCP[svc].EnabledEnforced) },
			set: func(val string) { s := v.teamCfg.MCP[svc]; b := val == "true"; s.EnabledEnforced = &b; v.teamCfg.MCP[svc] = s; v.dirtyTeam = true },
		})
		v.lines = append(v.lines, teamConfigLine{
			section: "MCP", key: urlLabels[svc], kind: "string", scope: scopeTeam,
			hint: urlHintsTeam[svc],
			get:  func() string { return v.teamCfg.MCP[svc].URL },
			set:  func(val string) { s := v.teamCfg.MCP[svc]; s.URL = val; v.teamCfg.MCP[svc] = s; v.dirtyTeam = true },
		})
		v.lines = append(v.lines, teamConfigLine{
			section: "MCP", key: "url_enforced", kind: "bool", scope: scopeTeam,
			get: func() string { return boolPtrToStr(v.teamCfg.MCP[svc].URLEnforced) },
			set: func(val string) { s := v.teamCfg.MCP[svc]; b := val == "true"; s.URLEnforced = &b; v.teamCfg.MCP[svc] = s; v.dirtyTeam = true },
		})
		v.lines = append(v.lines, teamConfigLine{
			section: "MCP", key: "write_recommended", kind: "bool", scope: scopeTeam,
			get: func() string { return tdBoolToStr(v.teamCfg.MCP[svc].WriteRecommended) },
			set: func(val string) { s := v.teamCfg.MCP[svc]; s.WriteRecommended = val == "true"; v.teamCfg.MCP[svc] = s; v.dirtyTeam = true },
		})

		// ── Personnel ──
		v.lines = append(v.lines, teamConfigLine{kind: "sub-header", section: "personnel"})
		v.lines = append(v.lines, teamConfigLine{
			section: "MCP.perso", key: "enabled", kind: "bool", scope: scopeLocal,
			grayed: func() bool { return v.teamCfg.MCP[svc].IsEnabledEnforced() },
			get:    v.getMCPLocalEnabled(svc),
			set:    v.setMCPLocalEnabled(svc),
		})
		v.lines = append(v.lines, teamConfigLine{
			section: "MCP.perso", key: urlLabels[svc], kind: "string", scope: scopeLocal,
			hint:   urlHintsPerso[svc],
			grayed: func() bool { return v.teamCfg.MCP[svc].IsURLEnforced() },
			get:    v.getMCPLocalURL(svc),
			set:    v.setMCPLocalURL(svc),
		})
		v.lines = append(v.lines, teamConfigLine{
			section: "MCP.perso", key: "token", kind: "password", scope: scopeLocal,
			get: v.getMCPLocalToken(svc),
			set: v.setMCPLocalToken(svc),
		})
		v.lines = append(v.lines, teamConfigLine{
			section: "MCP.perso", key: "write_enabled", kind: "bool", scope: scopeLocal,
			get: v.getMCPLocalWrite(svc),
			set: v.setMCPLocalWrite(svc),
		})
	}

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
		set: func(val string) { n, _ := strconv.Atoi(val); v.teamCfg.Tracker.MaxAutoPlanPerMember = n; v.dirtyTeam = true },
	})
	v.lines = append(v.lines, teamConfigLine{
		section: "Tracker", key: "sync_interval_min", kind: "string", scope: scopeTeam,
		get: func() string { return strconv.Itoa(v.teamCfg.Tracker.SyncIntervalMinutes) },
		set: func(val string) { n, _ := strconv.Atoi(val); v.teamCfg.Tracker.SyncIntervalMinutes = n; v.dirtyTeam = true },
	})

	// ── Mappings (dynamic) ──
	v.lines = append(v.lines, teamConfigLine{kind: "section-header", section: "Mappings projets"})
	for k := range v.teamCfg.Tracker.Projects {
		k := k
		v.lines = append(v.lines, teamConfigLine{
			section: "Mappings", key: k, kind: "string", scope: scopeTeam, dynamic: true,
			get: func() string { return v.teamCfg.Tracker.Projects[k] },
			set: func(val string) { v.teamCfg.Tracker.Projects[k] = val; v.dirtyTeam = true },
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
		set: func(val string) { n, _ := strconv.Atoi(val); v.teamCfg.Parallel.MaxSessions = n; v.dirtyTeam = true },
	})
	v.lines = append(v.lines, teamConfigLine{
		section: "Collaboration", key: "stale_days", kind: "string", scope: scopeTeam,
		get: func() string { return strconv.Itoa(v.teamCfg.Takeover.StaleDays) },
		set: func(val string) { n, _ := strconv.Atoi(val); v.teamCfg.Takeover.StaleDays = n; v.dirtyTeam = true },
	})
	v.lines = append(v.lines, teamConfigLine{
		section: "Collaboration", key: "done_retention_days", kind: "string", scope: scopeTeam,
		get: func() string { return strconv.Itoa(v.teamCfg.Claim.DoneRetentionDays) },
		set: func(val string) { n, _ := strconv.Atoi(val); v.teamCfg.Claim.DoneRetentionDays = n; v.dirtyTeam = true },
	})

	// ── Models (recommandations) ──
	v.lines = append(v.lines, teamConfigLine{kind: "section-header", section: "Models (recommandations)"})
	v.lines = append(v.lines, teamConfigLine{
		section: "Models", key: "default", kind: "string", scope: scopeTeam,
		get: func() string { return v.teamCfg.Models.Default },
		set: func(val string) { v.teamCfg.Models.Default = val; v.dirtyTeam = true },
	})
	v.lines = append(v.lines, teamConfigLine{kind: "sub-header", section: "families"})
	for k := range v.teamCfg.Models.Families {
		k := k
		v.lines = append(v.lines, teamConfigLine{
			section: "Models.families", key: k, kind: "string", scope: scopeTeam, dynamic: true,
			get: func() string { return v.teamCfg.Models.Families[k] },
			set: func(val string) { v.teamCfg.Models.Families[k] = val; v.dirtyTeam = true },
		})
	}
	v.lines = append(v.lines, teamConfigLine{kind: "sub-header", section: "agents"})
	for k := range v.teamCfg.Models.Agents {
		k := k
		v.lines = append(v.lines, teamConfigLine{
			section: "Models.agents", key: k, kind: "string", scope: scopeTeam, dynamic: true,
			get: func() string { return v.teamCfg.Models.Agents[k] },
			set: func(val string) { v.teamCfg.Models.Agents[k] = val; v.dirtyTeam = true },
		})
	}

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
	saved := v.list.GetCurrentItem()
	v.list.Clear()
	v.lineMap = nil

	for i, line := range v.lines {
		switch line.kind {
		case "section-header":
			// Spacer before section (except first)
			if i > 0 {
				v.list.AddItem("", "", 0, nil)
				v.lineMap = append(v.lineMap, -1) // spacer
			}
			v.list.AddItem(
				fmt.Sprintf("  %s─── %s ──────────────────%s", theme.ColorTag(theme.AccentHex), line.section, theme.TagColor),
				"", 0, nil)
			v.lineMap = append(v.lineMap, i)

		case "sub-header":
			v.list.AddItem(
				fmt.Sprintf("      %s── %s ──%s", theme.ColorTag(theme.TextMutedHex), line.section, theme.TagColor),
				"", 0, nil)
			v.lineMap = append(v.lineMap, i)

		default:
			val := ""
			if line.get != nil {
				val = line.get()
			}

			// Handle password kind (token fields)
			var display string
			if line.kind == "password" {
				display = v.formatToken(val)
			} else {
				display = v.formatValueWithHint(val, line.kind, line.hint)
			}

			// Handle grayed-out fields (enforced by team)
			grayedSuffix := ""
			if line.grayed != nil && line.grayed() {
				grayedSuffix = fmt.Sprintf("  %s(enforced par l'équipe)%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor)
				// Show value in muted color but still readable
				rawVal := val
				if rawVal == "" {
					rawVal = "(vide)"
				}
				display = fmt.Sprintf("%s%s%s", theme.ColorTag(theme.TextMutedHex), rawVal, theme.TagColor)
			}

			prefix := "    "
			if line.dynamic {
				prefix = "      "
			}
			main := fmt.Sprintf("%s%-22s %s%s", prefix, line.key, display, grayedSuffix)
			v.list.AddItem(main, "", 0, nil)
			v.lineMap = append(v.lineMap, i)
		}
	}

	// Restore cursor position, ensuring it's on a selectable item
	if saved >= 0 && saved < v.list.GetItemCount() {
		v.list.SetCurrentItem(saved)
	}
	// If current item is not selectable, move to first selectable
	cur := v.list.GetCurrentItem()
	if !v.isSelectable(cur) {
		v.moveDown()
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
			return fmt.Sprintf("%s✓ true%s", theme.ColorTag(theme.SuccessHex), theme.TagColor)
		}
		return fmt.Sprintf("%s✗ false%s", theme.ColorTag("#FF5252"), theme.TagColor)
	case "tri-state":
		switch val {
		case "true":
			return fmt.Sprintf("%s✓ oui%s", theme.ColorTag(theme.SuccessHex), theme.TagColor)
		case "false":
			return fmt.Sprintf("%s✗ non%s", theme.ColorTag("#FF5252"), theme.TagColor)
		default:
			return fmt.Sprintf("%s(hériter)%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor)
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
	if v.list == nil || v.list.GetItemCount() == 0 {
		return teamConfigLine{}, -1, false
	}
	listIdx := v.list.GetCurrentItem()
	if listIdx < 0 || listIdx >= len(v.lineMap) {
		return teamConfigLine{}, -1, false
	}
	lineIdx := v.lineMap[listIdx]
	if lineIdx < 0 {
		return teamConfigLine{}, -1, false // spacer
	}
	if lineIdx >= len(v.lines) {
		return teamConfigLine{}, -1, false
	}
	line := v.lines[lineIdx]
	if line.kind == "section-header" || line.kind == "sub-header" || line.get == nil {
		return teamConfigLine{}, -1, false
	}
	return line, lineIdx, true
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
	case "password":
		tokenKey := line.get()
		if tokenKey == "" {
			// Auto-assign default key name based on section context
			tokenKey = config.DefaultTokenKeyForService(v.currentServiceForLine(line))
			line.set(tokenKey)
		}
		v.shell.ShowPasswordModal("Valeur du token ("+tokenKey+")", func(val string) {
			if val == "" {
				return
			}
			if v.cfg.SetSecret != nil {
				ctx := context.Background()
				_ = v.cfg.SetSecret(ctx, tokenKey, val)
			}
			v.renderLines()
			if v.shell != nil {
				v.shell.ShowToastMsg("✓ Token enregistré", true)
			}
		})
	}
}

// currentServiceForLine determines which MCP service a line belongs to.
func (v *TeamDetailView) currentServiceForLine(line teamConfigLine) string {
	// Walk up from the line to find the section header
	for _, l := range v.lines {
		if l.kind == "section-header" && strings.HasPrefix(l.section, "MCP ") {
			svc := strings.ToLower(strings.TrimPrefix(l.section, "MCP "))
			// Check if this line is under this service section
			// Simple heuristic: return the last seen service before we hit the target line
			_ = svc
		}
	}
	// Fallback: extract from section field
	if strings.HasPrefix(line.section, "MCP") {
		return "gitlab" // default fallback
	}
	return ""
}

func (v *TeamDetailView) addDynamic() {
	if v.shell == nil {
		return
	}

	// Determine which dynamic section we're in by walking up the lineMap
	listIdx := v.list.GetCurrentItem()
	section := ""
	for i := listIdx; i >= 0; i-- {
		if i >= len(v.lineMap) {
			continue
		}
		lineIdx := v.lineMap[i]
		if lineIdx < 0 {
			continue
		}
		l := v.lines[lineIdx]
		if l.kind == "section-header" || l.kind == "sub-header" {
			section = l.section
			break
		}
		if l.section != "" {
			section = l.section
			break
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
				v.teamCfg.Tracker.Projects[key] = val
				v.dirtyTeam = true
				v.buildLines()
				v.renderLines()
			})
		})
	case section == "families":
		v.shell.ShowInputModal("Nom de la famille", "", func(key string) {
			if key == "" {
				return
			}
			v.shell.ShowInputModal("Model recommandé", "", func(val string) {
				if val == "" {
					return
				}
				v.teamCfg.Models.Families[key] = val
				v.dirtyTeam = true
				v.buildLines()
				v.renderLines()
			})
		})
	case section == "agents":
		v.shell.ShowInputModal("Nom de l'agent", "", func(key string) {
			if key == "" {
				return
			}
			v.shell.ShowInputModal("Model recommandé", "", func(val string) {
				if val == "" {
					return
				}
				v.teamCfg.Models.Agents[key] = val
				v.dirtyTeam = true
				v.buildLines()
				v.renderLines()
			})
		})
	default:
		v.shell.ShowToastMsg("'a' disponible dans: Mappings, families, agents", false)
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
	case line.section == "Models.families":
		delete(v.teamCfg.Models.Families, key)
	case line.section == "Models.agents":
		delete(v.teamCfg.Models.Agents, key)
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

	go func() {
		ctx := context.Background()
		result, err := v.cfg.SyncTracker(ctx)

		v.app.QueueUpdateDraw(func() {
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
				_ = setter.Set(ctx, tokenKey, value)
			}
		}

		v.shell.ShowToastMsg("✓ Token configuré — relancez 's' pour synchroniser", true)
		v.loadData()
		v.buildLines()
		v.renderLines()
	})
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

	go func() {
		ctx := context.Background()
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

		trackerType := tracker.Type(v.teamCfg.Tracker.Type)
		cfg, err := tracker.ResolveCredentials(ctx, src, trackerType)
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.shell.ShowToastMsg("✗ Credentials non disponibles: "+err.Error(), false)
			})
			return
		}

		t, err := tracker.New(cfg)
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.shell.ShowToastMsg("✗ Initialisation échouée: "+err.Error(), false)
			})
			return
		}

		username, err := t.TestConnection(ctx)
		v.app.QueueUpdateDraw(func() {
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

// ─── MCP Local (hub.toml) accessor helpers ──────────────────────────────────

func (v *TeamDetailView) getMCPLocalEnabled(svc string) func() string {
	return func() string {
		switch svc {
		case "gitlab":
			return tdBoolToStr(v.localMCP.Gitlab.Enabled)
		case "jira":
			return tdBoolToStr(v.localMCP.Jira.Enabled)
		case "figma":
			return tdBoolToStr(v.localMCP.Figma.Enabled)
		case "gslides":
			return tdBoolToStr(v.localMCP.Gslides.Enabled)
		}
		return "false"
	}
}

func (v *TeamDetailView) setMCPLocalEnabled(svc string) func(string) {
	return func(val string) {
		b := val == "true"
		switch svc {
		case "gitlab":
			v.localMCP.Gitlab.Enabled = b
		case "jira":
			v.localMCP.Jira.Enabled = b
		case "figma":
			v.localMCP.Figma.Enabled = b
		case "gslides":
			v.localMCP.Gslides.Enabled = b
		}
		v.dirtyLocal = true
	}
}

func (v *TeamDetailView) getMCPLocalURL(svc string) func() string {
	return func() string {
		switch svc {
		case "gitlab":
			return v.localMCP.Gitlab.URL
		case "jira":
			return v.localMCP.Jira.URL
		case "figma":
			return v.localMCP.Figma.URL
		case "gslides":
			return v.localMCP.Gslides.URL
		}
		return ""
	}
}

func (v *TeamDetailView) setMCPLocalURL(svc string) func(string) {
	return func(val string) {
		switch svc {
		case "gitlab":
			v.localMCP.Gitlab.URL = val
		case "jira":
			v.localMCP.Jira.URL = val
		case "figma":
			v.localMCP.Figma.URL = val
		case "gslides":
			v.localMCP.Gslides.URL = val
		}
		v.dirtyLocal = true
	}
}

func (v *TeamDetailView) getMCPLocalToken(svc string) func() string {
	return func() string {
		switch svc {
		case "gitlab":
			return v.localMCP.Gitlab.Token
		case "jira":
			return v.localMCP.Jira.Token
		case "figma":
			return v.localMCP.Figma.Token
		case "gslides":
			return v.localMCP.Gslides.Token
		}
		return ""
	}
}

func (v *TeamDetailView) setMCPLocalToken(svc string) func(string) {
	return func(val string) {
		switch svc {
		case "gitlab":
			v.localMCP.Gitlab.Token = val
		case "jira":
			v.localMCP.Jira.Token = val
		case "figma":
			v.localMCP.Figma.Token = val
		case "gslides":
			v.localMCP.Gslides.Token = val
		}
		v.dirtyLocal = true
	}
}

func (v *TeamDetailView) getMCPLocalWrite(svc string) func() string {
	return func() string {
		switch svc {
		case "gitlab":
			return tdBoolToStr(v.localMCP.Gitlab.WriteEnabled)
		case "jira":
			return tdBoolToStr(v.localMCP.Jira.WriteEnabled)
		case "figma":
			return tdBoolToStr(v.localMCP.Figma.WriteEnabled)
		case "gslides":
			return tdBoolToStr(v.localMCP.Gslides.WriteEnabled)
		}
		return "false"
	}
}

func (v *TeamDetailView) setMCPLocalWrite(svc string) func(string) {
	return func(val string) {
		b := val == "true"
		switch svc {
		case "gitlab":
			v.localMCP.Gitlab.WriteEnabled = b
		case "jira":
			v.localMCP.Jira.WriteEnabled = b
		case "figma":
			v.localMCP.Figma.WriteEnabled = b
		case "gslides":
			v.localMCP.Gslides.WriteEnabled = b
		}
		v.dirtyLocal = true
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
