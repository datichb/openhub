package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

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

// mcpLine represents a single editable field in the MCP config view.
type mcpLine struct {
	section string // e.g. "MCP Gitlab"
	key     string // e.g. "enabled", "url", "token"
	kind    string // "bool", "string", "password", "section-header", "sub-header"
	scope   configScope
	hint    string      // help text when value is empty
	grayed  func() bool // returns true if field is enforced (non-editable)
	get     func() string
	set     func(string)
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
	lines      []mcpLine
	dirtyTeam  bool
	dirtyLocal bool
}

var _ View = (*TeamMCPView)(nil)
var _ CommandProvider = (*TeamMCPView)(nil)

// NewTeamMCPView creates the MCP services configuration view.
func NewTeamMCPView(cfg TeamMCPViewConfig) *TeamMCPView {
	return &TeamMCPView{cfg: cfg}
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

	// Async: pull team-state then load data
	tc := v.cfg.ResolveTeam()
	onDataReady := func() {
		app.QueueUpdateDraw(func() {
			if v.app == nil || v.mountGen != gen {
				return
			}

			v.list = widgets.NewSectionedList()
			v.list.SetApp(app)
			v.list.SetBorderPadding(1, 0, 2, 2)

			v.list.SetItemSelectedFunc(func(index int, item widgets.SectionItem) {
				v.editByIndex(index, item)
			})

			v.buildLines()
			v.renderLines()

			content.RemoveItem(loading)
			content.AddItem(v.list, 0, 1, true)
			app.SetFocus(v.list)
		})
	}

	if tc.Enabled {
		repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
		syncAsync(app, repo, v.shell, func(_ error) {
			v.loadData()
			onDataReady()
		})
	} else {
		v.loadData()
		onDataReady()
	}
}

func (v *TeamMCPView) Unmount() {
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
	switch event.Key() {
	case tcell.KeyEnter:
		if idx, item, ok := v.list.CurrentItem(); ok {
			v.editByIndex(idx, item)
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
		v.loadData()
		v.buildLines()
		v.renderLines()
		v.dirtyTeam = false
		v.dirtyLocal = false
		if v.shell != nil {
			v.shell.ShowToastMsg("↩ Rechargé", true)
		}
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
// Context commands (omnibar)
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{
			ID: "team.mcp.save", Label: i18n.T("tui.hints.save"),
			Aliases:     []string{"save", "write", "sauvegarder"},
			Description: "Sauvegarder la configuration MCP", Category: "MCP",
			Action: v.save,
		},
		{
			ID: "team.mcp.reload", Label: i18n.T("tui.hints.refresh"),
			Aliases:     []string{"refresh", "reload", "recharger"},
			Description: "Recharger depuis le disque", Category: "MCP",
			Action: func() {
				v.loadData()
				v.buildLines()
				v.renderLines()
				v.dirtyTeam = false
				v.dirtyLocal = false
			},
		},
		{
			ID: "team.mcp.test", Label: "Test connexion",
			Aliases:     []string{"test", "ping", "check"},
			Description: "Tester la connexion au service MCP sélectionné", Category: "MCP",
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

func (v *TeamMCPView) buildLines() {
	v.lines = nil

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

	for _, svc := range []string{"gitlab", "jira", "figma", "gslides"} {
		svc := svc // capture

		// ── Section header: service name ──
		v.lines = append(v.lines, mcpLine{
			kind:    "section-header",
			section: "MCP " + strings.Title(svc),
		})

		// ── Sub-section: Équipe (team-state config.toml) ──
		v.lines = append(v.lines, mcpLine{kind: "sub-header", section: "Équipe"})
		v.lines = append(v.lines, mcpLine{
			section: "MCP", key: "enabled", kind: "bool", scope: scopeTeam,
			get: func() string { return mcpBoolPtrToStr(v.teamCfg.MCP[svc].Enabled) },
			set: func(val string) {
				s := v.teamCfg.MCP[svc]
				b := val == "true"
				s.Enabled = &b
				v.teamCfg.MCP[svc] = s
				v.dirtyTeam = true
			},
		})
		v.lines = append(v.lines, mcpLine{
			section: "MCP", key: "enabled_enforced", kind: "bool", scope: scopeTeam,
			get: func() string { return mcpBoolPtrToStr(v.teamCfg.MCP[svc].EnabledEnforced) },
			set: func(val string) {
				s := v.teamCfg.MCP[svc]
				b := val == "true"
				s.EnabledEnforced = &b
				v.teamCfg.MCP[svc] = s
				v.dirtyTeam = true
			},
		})
		v.lines = append(v.lines, mcpLine{
			section: "MCP", key: urlLabels[svc], kind: "string", scope: scopeTeam,
			hint: urlHintsTeam[svc],
			get:  func() string { return v.teamCfg.MCP[svc].URL },
			set: func(val string) {
				s := v.teamCfg.MCP[svc]
				s.URL = val
				v.teamCfg.MCP[svc] = s
				v.dirtyTeam = true
			},
		})
		v.lines = append(v.lines, mcpLine{
			section: "MCP", key: "url_enforced", kind: "bool", scope: scopeTeam,
			get: func() string { return mcpBoolPtrToStr(v.teamCfg.MCP[svc].URLEnforced) },
			set: func(val string) {
				s := v.teamCfg.MCP[svc]
				b := val == "true"
				s.URLEnforced = &b
				v.teamCfg.MCP[svc] = s
				v.dirtyTeam = true
			},
		})
		v.lines = append(v.lines, mcpLine{
			section: "MCP", key: "write_recommended", kind: "bool", scope: scopeTeam,
			get: func() string { return mcpBoolStr(v.teamCfg.MCP[svc].WriteRecommended) },
			set: func(val string) {
				s := v.teamCfg.MCP[svc]
				s.WriteRecommended = val == "true"
				v.teamCfg.MCP[svc] = s
				v.dirtyTeam = true
			},
		})

		// ── Sub-section: Personnel (local hub.toml) ──
		v.lines = append(v.lines, mcpLine{kind: "sub-header", section: "Personnel"})
		v.lines = append(v.lines, mcpLine{
			section: "MCP.perso", key: "enabled", kind: "bool", scope: scopeLocal,
			grayed: func() bool { return v.teamCfg.MCP[svc].IsEnabledEnforced() },
			get:    v.getMCPLocalEnabled(svc),
			set:    v.setMCPLocalEnabled(svc),
		})
		v.lines = append(v.lines, mcpLine{
			section: "MCP.perso", key: urlLabels[svc], kind: "string", scope: scopeLocal,
			hint:   urlHintsPerso[svc],
			grayed: func() bool { return v.teamCfg.MCP[svc].IsURLEnforced() },
			get:    v.getMCPLocalURL(svc),
			set:    v.setMCPLocalURL(svc),
		})
		v.lines = append(v.lines, mcpLine{
			section: "MCP.perso", key: "token", kind: "password", scope: scopeLocal,
			get: v.getMCPLocalToken(svc),
			set: v.setMCPLocalToken(svc),
		})
		v.lines = append(v.lines, mcpLine{
			section: "MCP.perso", key: "write_enabled", kind: "bool", scope: scopeLocal,
			get: v.getMCPLocalWrite(svc),
			set: v.setMCPLocalWrite(svc),
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) renderLines() {
	if v.list == nil {
		return
	}
	savedIdx := v.list.GetCurrentItem()
	ctx := context.Background()

	items := make([]widgets.SectionItem, 0, len(v.lines))
	for i, line := range v.lines {
		switch line.kind {
		case "section-header":
			title := line.section
			if i == 0 && (v.dirtyTeam || v.dirtyLocal) {
				title += "  " + theme.ColorTag(theme.AccentHex) + "● " + i18n.T("tui.settings.modified") + theme.TagColor
			}
			items = append(items, widgets.SectionItem{
				IsHeader: true,
				MainText: title,
			})

		case "sub-header":
			items = append(items, widgets.SectionItem{
				IsHeader: true,
				MainText: line.section,
			})

		default:
			val := ""
			if line.get != nil {
				val = line.get()
			}

			display := v.formatValue(val, line, ctx)

			// Handle grayed-out fields (enforced by team)
			grayedSuffix := ""
			if line.grayed != nil && line.grayed() {
				grayedSuffix = fmt.Sprintf("  %s(enforced par l'équipe)%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor)
				rawVal := val
				if rawVal == "" {
					rawVal = "(vide)"
				}
				display = fmt.Sprintf("%s%s%s", theme.ColorTag(theme.TextMutedHex), rawVal, theme.TagColor)
			}

			mainText := fmt.Sprintf("%-22s %s%s", line.key+":", display, grayedSuffix)
			items = append(items, widgets.SectionItem{
				MainText:  mainText,
				Reference: i,
				Locked:    line.grayed != nil && line.grayed(),
			})
		}
	}

	v.list.SetItems(items)
	if savedIdx >= 0 {
		v.list.SelectIndex(savedIdx)
	}

	_ = ctx
}

func (v *TeamMCPView) formatValue(val string, line mcpLine, ctx context.Context) string {
	switch line.kind {
	case "bool":
		if val == "true" {
			return fmt.Sprintf("%s✓ %s%s", theme.ColorTag(theme.SuccessHex), i18n.T("tui.settings.enabled"), theme.TagColor)
		}
		return fmt.Sprintf("%s✗ %s%s", theme.ColorTag("#FF5252"), i18n.T("tui.settings.disabled"), theme.TagColor)

	case "password":
		return v.formatToken(val, ctx)

	default: // string
		if val == "" {
			if line.hint != "" {
				return fmt.Sprintf("%s(vide) ← %s%s", theme.ColorTag(theme.TextMutedHex), line.hint, theme.TagColor)
			}
			return fmt.Sprintf("%s(vide)%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor)
		}
		return val
	}
}

func (v *TeamMCPView) formatToken(tokenKey string, ctx context.Context) string {
	if tokenKey == "" {
		return fmt.Sprintf("%s(non configuré)%s  %s[✗ non configuré]%s",
			theme.ColorTag(theme.TextMutedHex), theme.TagColor,
			theme.ColorTag("#FF5252"), theme.TagColor)
	}
	if v.cfg.CheckSecret != nil {
		present, masked := v.cfg.CheckSecret(ctx, tokenKey)
		if present {
			return fmt.Sprintf("%s  %s[✓ configuré]%s", masked, theme.ColorTag(theme.SuccessHex), theme.TagColor)
		}
	}
	return fmt.Sprintf("%s  %s[✗ non configuré]%s", tokenKey, theme.ColorTag("#FF5252"), theme.TagColor)
}

// ─────────────────────────────────────────────────────────────────────────────
// Editing
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamMCPView) editByIndex(index int, item widgets.SectionItem) {
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.lines) {
		return
	}
	line := v.lines[ref]
	if line.kind == "section-header" || line.kind == "sub-header" || line.get == nil {
		return
	}
	if v.shell == nil {
		return
	}

	// Check grayed (enforced by team)
	if line.grayed != nil && line.grayed() {
		v.shell.ShowToastMsg("Imposé par l'équipe (non-modifiable)", false)
		return
	}

	switch line.kind {
	case "bool":
		v.toggleByRef(ref)

	case "string":
		cur := line.get()
		v.shell.ShowInputModal(line.key, cur, func(newVal string) {
			if line.set != nil {
				line.set(newVal)
				v.renderLines()
			}
		})

	case "password":
		tokenKey := line.get()
		if tokenKey == "" {
			// Auto-assign default key name based on service
			svc := v.serviceForLine(ref)
			tokenKey = config.DefaultTokenKeyForService(svc)
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

func (v *TeamMCPView) toggleSelected() {
	if v.list == nil {
		return
	}
	_, item, ok := v.list.CurrentItem()
	if !ok {
		return
	}
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.lines) {
		return
	}
	line := v.lines[ref]
	if line.grayed != nil && line.grayed() {
		if v.shell != nil {
			v.shell.ShowToastMsg("Imposé par l'équipe (non-modifiable)", false)
		}
		return
	}
	if line.kind == "bool" {
		v.toggleByRef(ref)
	}
}

func (v *TeamMCPView) toggleByRef(ref int) {
	if ref < 0 || ref >= len(v.lines) {
		return
	}
	line := v.lines[ref]
	if line.kind != "bool" || line.set == nil {
		return
	}
	cur := line.get()
	if cur == "true" {
		line.set("false")
	} else {
		line.set("true")
	}
	v.renderLines()
}

// serviceForLine determines which MCP service a line belongs to
// by walking backward through lines to find the last section header.
func (v *TeamMCPView) serviceForLine(lineIdx int) string {
	for i := lineIdx; i >= 0; i-- {
		if v.lines[i].kind == "section-header" && strings.HasPrefix(v.lines[i].section, "MCP ") {
			return strings.ToLower(strings.TrimPrefix(v.lines[i].section, "MCP "))
		}
	}
	return "gitlab"
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
	if !ok || ref < 0 || ref >= len(v.lines) {
		return
	}

	svc := v.serviceForLine(ref)
	if svc == "" {
		v.shell.ShowToastMsg("Sélectionnez un champ dans une section MCP", false)
		return
	}

	// Check that we have a token configured
	token := v.getMCPLocalToken(svc)()
	if token == "" {
		v.shell.ShowToastMsg(fmt.Sprintf("Token non configuré pour %s", svc), false)
		return
	}

	v.shell.ShowToastMsg(fmt.Sprintf("Test connexion %s...", svc), true)
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
			if err := config.Save(hubCfg); err != nil {
				v.shell.ShowToastMsg("Erreur sauvegarde locale: "+err.Error(), false)
				return
			}
		}
		saved = append(saved, "local")
		v.dirtyLocal = false
	}

	if len(saved) == 0 {
		v.shell.ShowToastMsg("Aucune modification à sauvegarder", true)
		return
	}
	v.shell.ShowToastMsg(fmt.Sprintf("✓ Sauvegardé: %s", strings.Join(saved, " + ")), true)
	v.renderLines()
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

// mcpBoolPtrToStr converts a *bool to "true"/"false" (nil → "false").
func mcpBoolPtrToStr(b *bool) string {
	if b == nil {
		return "false"
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
