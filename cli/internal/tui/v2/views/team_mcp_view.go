package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────────────────────────────────────

// TeamMCPViewConfig holds external dependencies for TeamMCPView.
type TeamMCPViewConfig struct {
	// ResolveTeam returns the effective team resolution (state repo path, etc.).
	ResolveTeam ResolveTeamFunc
	// GetMCPConfig returns the local hub.toml MCP configuration.
	GetMCPConfig func() config.MCPConfig
	// SaveTeamConfig persists the team-state config to the team-state repo.
	SaveTeamConfig func(ctx context.Context, cfg *teamstate.TeamConfig) error
	// SaveLocalMCP persists a local hub.toml MCP override key/value.
	SaveLocalMCP func(key, value string) error
	// GetHubConfig returns the live hub config pointer for local save.
	GetHubConfig func() *config.Config
	// CheckSecret tests whether a keychain key has a stored value.
	// Returns (true, masked) if present, (false, "") if absent.
	CheckSecret func(ctx context.Context, key string) (present bool, masked string)
	// SetSecret stores a token value in the system keychain.
	SetSecret func(ctx context.Context, key, value string) error
}

// ─────────────────────────────────────────────────────────────────────────────
// View
// ─────────────────────────────────────────────────────────────────────────────

// TeamMCPView provides a standalone sub-page for editing team-level and local
// MCP service configuration, extracted from TeamDetailView.
type TeamMCPView struct {
	app      *tview.Application
	list     *widgets.SectionedList
	shell    ShellAccess
	cfg      TeamMCPViewConfig
	mountGen uint64

	teamCfg    *teamstate.TeamConfig
	localMCP   config.MCPConfig
	fields     []configField
	dirtyTeam  bool
	dirtyLocal bool
	undoStack  *widgets.UndoStack[teamMCPSnapshot]
}

// teamMCPSnapshot holds combined state for undo.
type teamMCPSnapshot struct {
	teamCfg  teamstate.TeamConfig
	localMCP config.MCPConfig
}

var _ View = (*TeamMCPView)(nil)
var _ CommandProvider = (*TeamMCPView)(nil)

// NewTeamMCPView creates the MCP services configuration view.
func NewTeamMCPView(cfg TeamMCPViewConfig) *TeamMCPView {
	return &TeamMCPView{
		cfg:       cfg,
		undoStack: widgets.NewUndoStack[teamMCPSnapshot](10),
	}
}

// SetShell provides shell access for modals/toasts.
func (v *TeamMCPView) SetShell(s ShellAccess) { v.shell = s }

func (v *TeamMCPView) ID() string    { return "team.mcp" }
func (v *TeamMCPView) Title() string { return "MCP Services" }
func (v *TeamMCPView) StatusHints() string {
	return fmt.Sprintf("j/k %s · {/} %s · Space %s · Enter %s · w %s · u %s · t %s",
		i18n.T("tui.hints.nav"),
		i18n.T("tui.hints.navigate"),
		i18n.T("tui.hints.toggle"),
		i18n.T("tui.hints.edit"),
		i18n.T("tui.hints.save"),
		i18n.T("tui.hints.undo"),
		"test",
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// Mount / Unmount
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.mountGen++
	gen := v.mountGen
	v.dirtyTeam = false
	v.dirtyLocal = false

	// Show loading placeholder immediately
	loading := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	loading.SetBackgroundColor(theme.BgPanel)
	muted := theme.ColorTag(theme.TextMutedHex)
	loading.SetText(fmt.Sprintf("\n  %s%s%s", muted, i18n.T("tui.settings.loading"), theme.TagColor))
	content.AddItem(loading, 0, 1, true)

	// buildList creates the widget and swaps it in for the loading placeholder.
	// MUST be called from inside a QueueUpdateDraw callback (or the tview event
	// loop). Does NOT call QueueUpdateDraw itself — avoids the nested deadlock
	// that would occur if QueueUpdateDraw were called from inside a running
	// QueueUpdateDraw callback (the unbuffered done-channel would block forever).
	buildList := func() {
		if v.app == nil || v.mountGen != gen {
			return
		}

		v.list = widgets.NewSectionedList()
		v.list.SetApp(app)
		v.list.SetBorderPadding(1, 0, 2, 2)

		v.list.SetItemSelectedFunc(func(index int, item widgets.SectionItem) {
			v.onItemSelected(index, item)
		})

		v.buildFields()
		v.renderFields()

		content.RemoveItem(loading)
		content.AddItem(v.list, 0, 1, true)
		app.SetFocus(v.list)
	}

	// Async: pull team-state then load data
	tc := v.cfg.ResolveTeam()
	if tc.Enabled {
		repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
		// syncAsync's onDone is already called inside QueueUpdateDraw (sync.go),
		// so we do UI work directly here — no nested QueueUpdateDraw.
		syncAsync(app, repo, v.shell, func(_ error) {
			v.loadData()
			buildList()
		})
	} else {
		// No team — wrap in goroutine so QueueUpdateDraw is not called from
		// the event loop (Mount runs on the event loop via router.mountLocked).
		go func() {
			app.QueueUpdateDraw(func() {
				if v.app == nil || v.mountGen != gen {
					return
				}
				v.loadData()
				buildList()
			})
		}()
	}
}

func (v *TeamMCPView) Unmount() {
	v.mountGen++ // invalidate in-flight async goroutine
	if (v.dirtyTeam || v.dirtyLocal) && v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.settings.unsaved"), false)
	}
	v.app = nil
	v.list = nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Key handling
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.list == nil {
		return event
	}
	if event.Key() == tcell.KeyEnter {
		if idx, item, ok := v.list.CurrentItem(); ok {
			v.onItemSelected(idx, item)
		}
		return nil
	}
	switch event.Rune() {
	case ' ':
		v.toggleSelected()
		return nil
	case 'w':
		v.save()
		return nil
	case 'u':
		v.undo()
		return nil
	case 't':
		v.testConnection()
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
	}
	return event
}

// ─────────────────────────────────────────────────────────────────────────────
// Context commands (omnibar)
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{
			ID: "team.mcp.save", Label: i18n.T("tui.hints.save"),
			Aliases:     []string{"save", "write", "sauvegarder"},
			Description: i18n.T("tui.team_mcp.cmd_save_desc"), Category: "MCP",
			Action: v.save,
		},
		{
			ID: "team.mcp.reload", Label: i18n.T("tui.hints.refresh"),
			Aliases:     []string{"refresh", "reload", "recharger"},
			Description: i18n.T("tui.team_mcp.cmd_reload_desc"), Category: "MCP",
			Action: func() {
				v.loadData()
				v.buildFields()
				v.renderFields()
				v.dirtyTeam = false
				v.dirtyLocal = false
			},
		},
		{
			ID: "team.mcp.test", Label: i18n.T("tui.team_mcp.cmd_test_label"),
			Aliases:     []string{"test", "ping", "check"},
			Description: i18n.T("tui.team_mcp.cmd_test_desc"), Category: "MCP",
			Action: v.testConnection,
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Data loading
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) loadData() {
	v.localMCP = v.cfg.GetMCPConfig()

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
}

// ─────────────────────────────────────────────────────────────────────────────
// Build config lines
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) buildFields() {
	v.fields = nil

	for _, svc := range []string{"gitlab", "jira", "figma", "gslides"} {
		svc := svc // capture
		svcTitle := cases.Title(language.Und).String(svc)

		// ── Section header: service name ──
		v.fields = append(v.fields, configField{Kind: CfgFieldSectionHeader, Label: "MCP " + svcTitle})

		// ── Sub-section: Team (shared config.toml) ──
		v.fields = append(v.fields, configField{Kind: CfgFieldSubHeader, Label: i18n.T("tui.config.section.team_shared")})
		v.fields = append(v.fields, configField{
			Key: "enabled", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.mcp_enabled.label"),
			Description: i18n.T("tui.config.field.mcp_enabled.desc"), Scope: ScopeTeamShared, Section: svc,
			Get: func() string { return mcpBoolPtrToStr(v.teamCfg.MCP[svc].Enabled) },
			Set: func(val string) {
				s := v.teamCfg.MCP[svc]
				b := val == "true"
				s.Enabled = &b
				v.teamCfg.MCP[svc] = s
				v.dirtyTeam = true
			},
		})
		v.fields = append(v.fields, configField{
			Key: "enabled_enforced", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.mcp_enforced.label"),
			Description: i18n.T("tui.config.field.mcp_enforced.desc"), Scope: ScopeTeamShared, Section: svc,
			Get: func() string { return mcpBoolPtrToStr(v.teamCfg.MCP[svc].EnabledEnforced) },
			Set: func(val string) {
				s := v.teamCfg.MCP[svc]
				b := val == "true"
				s.EnabledEnforced = &b
				v.teamCfg.MCP[svc] = s
				v.dirtyTeam = true
			},
		})
		v.fields = append(v.fields, configField{
			Key: "url", Kind: CfgFieldString, Label: i18n.T("tui.config.field.mcp_url.label"),
			Description: i18n.T("tui.config.field.mcp_url.desc"), Scope: ScopeTeamShared, Section: svc,
			Placeholder: i18n.T("tui.config.field.mcp_url.placeholder"),
			Get:         func() string { return v.teamCfg.MCP[svc].URL },
			Set: func(val string) {
				s := v.teamCfg.MCP[svc]
				s.URL = val
				v.teamCfg.MCP[svc] = s
				v.dirtyTeam = true
			},
		})
		v.fields = append(v.fields, configField{
			Key: "url_enforced", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.mcp_url_enforced.label"),
			Description: i18n.T("tui.config.field.mcp_url_enforced.desc"), Scope: ScopeTeamShared, Section: svc,
			Get: func() string { return mcpBoolPtrToStr(v.teamCfg.MCP[svc].URLEnforced) },
			Set: func(val string) {
				s := v.teamCfg.MCP[svc]
				b := val == "true"
				s.URLEnforced = &b
				v.teamCfg.MCP[svc] = s
				v.dirtyTeam = true
			},
		})
		v.fields = append(v.fields, configField{
			Key: "write_recommended", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.mcp_write_recommended.label"),
			Description: i18n.T("tui.config.field.mcp_write_recommended.desc"), Scope: ScopeTeamShared, Section: svc,
			Get: func() string { return mcpBoolStr(v.teamCfg.MCP[svc].WriteRecommended) },
			Set: func(val string) {
				s := v.teamCfg.MCP[svc]
				s.WriteRecommended = val == "true"
				v.teamCfg.MCP[svc] = s
				v.dirtyTeam = true
			},
		})

		// ── Sub-section: Personal (local hub.toml) ──
		v.fields = append(v.fields, configField{Kind: CfgFieldSubHeader, Label: i18n.T("tui.config.section.personal")})
		v.fields = append(v.fields, configField{
			Key: "enabled", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.mcp_enabled.label"),
			Description: i18n.T("tui.config.field.mcp_enabled.desc"), Scope: ScopeTeamLocal, Section: svc + ".perso",
			Locked: func() bool { return v.teamCfg.MCP[svc].IsEnabledEnforced() },
			Get:    v.getMCPLocalEnabled(svc),
			Set:    v.setMCPLocalEnabled(svc),
		})
		v.fields = append(v.fields, configField{
			Key: "url", Kind: CfgFieldString, Label: i18n.T("tui.config.field.mcp_url.label"),
			Description: i18n.T("tui.config.field.mcp_url.desc"), Scope: ScopeTeamLocal, Section: svc + ".perso",
			Locked: func() bool { return v.teamCfg.MCP[svc].IsURLEnforced() },
			Get:    v.getMCPLocalURL(svc),
			Set:    v.setMCPLocalURL(svc),
		})
		v.fields = append(v.fields, configField{
			Key: "token", Kind: CfgFieldPassword, Label: i18n.T("tui.config.field.mcp_token.label"),
			Description: i18n.T("tui.config.field.mcp_token.desc"), Scope: ScopeTeamLocal, Section: svc + ".perso",
			Get: v.getMCPLocalToken(svc),
			Set: v.setMCPLocalToken(svc),
		})
		v.fields = append(v.fields, configField{
			Key: "write_enabled", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.mcp_write.label"),
			Description: i18n.T("tui.config.field.mcp_write.desc"), Scope: ScopeTeamLocal, Section: svc + ".perso",
			Get: v.getMCPLocalWrite(svc),
			Set: v.setMCPLocalWrite(svc),
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) renderFields() {
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

func (v *TeamMCPView) onItemSelected(_ int, item widgets.SectionItem) {
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.fields) {
		return
	}
	f := &v.fields[ref]
	if !isEditable(f.Kind) || f.Get == nil || v.shell == nil {
		return
	}
	if f.Locked != nil && f.Locked() {
		v.shell.ShowToastMsg(i18n.T("tui.config.enforced_toast"), false)
		return
	}

	// Password fields: token setup flow
	if f.Kind == CfgFieldPassword {
		tokenKey := f.Get()
		if tokenKey == "" {
			// Auto-assign default key name based on service section
			svc := f.Section
			if idx := strings.Index(svc, "."); idx >= 0 {
				svc = svc[:idx]
			}
			tokenKey = config.DefaultTokenKeyForService(svc)
			f.Set(tokenKey)
		}
		v.shell.ShowPasswordModal(i18n.T("tui.config.field.mcp_token.label")+" ("+tokenKey+")", func(val string) {
			if val == "" {
				return
			}
			if v.cfg.SetSecret != nil {
				ctx := context.Background()
				if err := v.cfg.SetSecret(ctx, tokenKey, val); err != nil {
					v.shell.ShowToastMsg(i18n.T("tui.settings.error")+": "+err.Error(), false)
					return
				}
			}
			v.renderFields()
			v.shell.ShowToastMsg(i18n.T("tui.settings.secret_updated"), true)
		})
		return
	}

	v.pushUndo()
	editConfigField(v.shell, f, func() {
		v.renderFields()
	})
}

func (v *TeamMCPView) toggleSelected() {
	if v.list == nil {
		return
	}
	_, item, ok := v.list.CurrentItem()
	if !ok {
		return
	}
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.fields) {
		return
	}
	f := &v.fields[ref]
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

// ─────────────────────────────────────────────────────────────────────────────
// Test connection
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) testConnection() {
	if v.list == nil || v.shell == nil {
		return
	}
	_, item, ok := v.list.CurrentItem()
	if !ok {
		return
	}
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.fields) {
		return
	}

	// Determine service from field's Section (e.g. "gitlab", "gitlab.perso")
	svc := v.fields[ref].Section
	if idx := strings.Index(svc, "."); idx >= 0 {
		svc = svc[:idx]
	}
	if svc == "" {
		v.shell.ShowToastMsg(i18n.T("tui.team_mcp.select_field"), false)
		return
	}

	// Check that we have a token configured
	token := v.getMCPLocalToken(svc)()
	if token == "" {
		v.shell.ShowToastMsg(i18n.Tf("tui.team_mcp.token_missing", svc), false)
		return
	}

	v.shell.ShowToastMsg(i18n.Tf("tui.team_mcp.testing", svc), true)
}

// ─────────────────────────────────────────────────────────────────────────────
// Undo
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) pushUndo() {
	if v.teamCfg == nil {
		return
	}
	v.undoStack.Push(teamMCPSnapshot{
		teamCfg:  deepCopyTeamConfig(v.teamCfg),
		localMCP: v.localMCP,
	})
}

func (v *TeamMCPView) undo() {
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

func (v *TeamMCPView) save() {
	if v.shell == nil {
		return
	}
	ctx := context.Background()
	var saved []string

	if v.dirtyTeam && v.cfg.SaveTeamConfig != nil {
		if err := v.cfg.SaveTeamConfig(ctx, v.teamCfg); err != nil {
			v.shell.ShowToastMsg(i18n.T("tui.team_mcp.save_error_team")+err.Error(), false)
			return
		}
		saved = append(saved, i18n.T("tui.team_mcp.scope_team"))
		v.dirtyTeam = false
	}

	if v.dirtyLocal {
		hubCfg := v.cfg.GetHubConfig()
		if hubCfg != nil {
			hubCfg.MCP = v.localMCP
			if err := config.Save(hubCfg); err != nil {
				v.shell.ShowToastMsg(i18n.T("tui.team_mcp.save_error_local")+err.Error(), false)
				return
			}
		}
		saved = append(saved, i18n.T("tui.team_mcp.scope_local"))
		v.dirtyLocal = false
	}

	if len(saved) == 0 {
		v.shell.ShowToastMsg(i18n.T("tui.team_mcp.no_changes"), true)
		return
	}
	v.shell.ShowToastMsg(i18n.Tf("tui.team_mcp.saved", strings.Join(saved, " + ")), true)
	v.renderFields()
}

// ─────────────────────────────────────────────────────────────────────────────
// MCP Local (hub.toml) accessor helpers
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) getMCPLocalEnabled(svc string) func() string {
	return func() string {
		switch svc {
		case "gitlab":
			return mcpBoolStr(v.localMCP.Gitlab.Enabled)
		case "jira":
			return mcpBoolStr(v.localMCP.Jira.Enabled)
		case "figma":
			return mcpBoolStr(v.localMCP.Figma.Enabled)
		case "gslides":
			return mcpBoolStr(v.localMCP.Gslides.Enabled)
		}
		return "false"
	}
}

func (v *TeamMCPView) setMCPLocalEnabled(svc string) func(string) {
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

func (v *TeamMCPView) getMCPLocalURL(svc string) func() string {
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

func (v *TeamMCPView) setMCPLocalURL(svc string) func(string) {
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

func (v *TeamMCPView) getMCPLocalToken(svc string) func() string {
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

func (v *TeamMCPView) setMCPLocalToken(svc string) func(string) {
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

func (v *TeamMCPView) getMCPLocalWrite(svc string) func() string {
	return func() string {
		switch svc {
		case "gitlab":
			return mcpBoolStr(v.localMCP.Gitlab.WriteEnabled)
		case "jira":
			return mcpBoolStr(v.localMCP.Jira.WriteEnabled)
		case "figma":
			return mcpBoolStr(v.localMCP.Figma.WriteEnabled)
		case "gslides":
			return mcpBoolStr(v.localMCP.Gslides.WriteEnabled)
		}
		return "false"
	}
}

func (v *TeamMCPView) setMCPLocalWrite(svc string) func(string) {
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

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// mcpBoolPtrToStr converts a *bool to "true"/"false" (nil → "" = not configured).
func mcpBoolPtrToStr(b *bool) string {
	if b == nil {
		return "" // not configured
	}
	if *b {
		return "true"
	}
	return "false"
}

// mcpBoolStr converts a plain bool to "true"/"false".
func mcpBoolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
