package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// MCPService represents the hub-level state of an MCP service.
type MCPService struct {
	Name         string
	Enabled      bool
	HasToken     bool
	WriteEnabled bool
}

// MCPViewConfig holds configuration for the MCP view.
type MCPViewConfig struct {
	// GetConfig returns the live hub config pointer.
	GetConfig func() *config.Config
	// SaveConfig persists the modified config to hub.toml (unified mutation path).
	SaveConfig func(c *config.Config) error
}

// MCPView displays MCP server management with a SectionedList panel
// for hub-level configuration (enable/disable, token, write).
//
// Per-project MCP overrides are managed exclusively in ProjectConfigView
// (single mutation path — ADR-031).
type MCPView struct {
	app    *tview.Application
	appCtx *app.App
	cfg    MCPViewConfig
	shell  ShellAccess

	// Layout
	list *widgets.SectionedList

	// Data
	services []MCPService

	commands []ContextCommand
	mountGen uint64
}

var _ View = (*MCPView)(nil)
var _ CommandProvider = (*MCPView)(nil)

// NewMCPView creates a new MCP view.
func NewMCPView(a *app.App, cfg MCPViewConfig) *MCPView {
	return &MCPView{appCtx: a, cfg: cfg}
}

// SetShell provides the shell reference for interactions.
func (v *MCPView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *MCPView) ID() string { return "mcp" }

// Title returns the display title.
func (v *MCPView) Title() string { return "MCP" }

// StatusHints returns keybinding hints.
func (v *MCPView) StatusHints() string {
	return fmt.Sprintf("j/k %s · {/} %s · Space %s · t %s · w %s · r %s",
		i18n.T("tui.hints.nav"),
		i18n.T("tui.hints.navigate"),
		i18n.T("tui.hints.toggle"),
		i18n.T("tui.hints.token"),
		i18n.T("tui.hints.write"),
		i18n.T("tui.hints.refresh"),
	)
}

// Mount builds the MCP management interface.
func (v *MCPView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.mountGen++
	gen := v.mountGen

	// Show loading placeholder immediately
	loading := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	loading.SetBackgroundColor(theme.BgPanel)
	muted := theme.ColorTag(theme.TextMutedHex)
	loading.SetText(fmt.Sprintf("\n  %s%s%s", muted, i18n.T("tui.mcp.loading"), theme.TagColor))
	content.AddItem(loading, 0, 1, true)

	// Load data and build UI asynchronously
	go func() {
		app.QueueUpdateDraw(func() {
			if v.app == nil || v.mountGen != gen {
				return
			}
			v.loadServices()

			v.list = widgets.NewSectionedList()
			v.list.SetApp(app)
			v.list.SetBorderPadding(1, 0, 2, 2)

			v.populateList()
			v.buildCommands()

			content.RemoveItem(loading)
			content.AddItem(v.list, 0, 1, true)
			app.SetFocus(v.list)
		})
	}()
}

// Unmount cleans up resources.
func (v *MCPView) Unmount() {
	v.app = nil
	v.list = nil
	v.commands = nil
}

// HandleKey processes key events.
func (v *MCPView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case ' ':
		v.toggleHubCurrent()
		return nil
	case 't':
		v.promptTokenCurrent()
		return nil
	case 'w':
		v.toggleWriteCurrent()
		return nil
	case 'r':
		v.loadServices()
		v.populateList()
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.mcp.refreshed"), true)
		}
		return nil
	}
	return event
}

// ContextCommands returns contextual commands for the omnibar.
func (v *MCPView) ContextCommands() []ContextCommand {
	return v.commands
}

// ─────────────────────────────────────────────────────────────────────────────
// List management
// ─────────────────────────────────────────────────────────────────────────────

func (v *MCPView) populateList() {
	if v.list == nil {
		return
	}

	items := make([]widgets.SectionItem, 0, len(v.services)+2)
	items = append(items, widgets.SectionItem{
		IsHeader: true,
		MainText: i18n.T("tui.mcp.section_hub"),
	})

	for i, svc := range v.services {
		items = append(items, widgets.SectionItem{
			MainText:      v.hubItemText(svc),
			SecondaryText: v.hubItemSubtext(svc),
			Reference:     i,
		})
	}

	v.list.SetItems(items)
}

func (v *MCPView) hubItemText(svc MCPService) string {
	var statusIcon, statusColor string
	if svc.Enabled {
		statusIcon = "✓"
		statusColor = theme.SuccessHex
	} else {
		statusIcon = "✗"
		statusColor = theme.TextMutedHex
	}
	return fmt.Sprintf("%s%s%s  %s%-12s%s",
		theme.ColorTag(statusColor), statusIcon, theme.TagColor,
		theme.ColorTag(theme.TextPrimaryHex), svc.Name, theme.TagColor)
}

func (v *MCPView) hubItemSubtext(svc MCPService) string {
	var parts []string

	// Token indicator
	if svc.Name == "team" {
		parts = append(parts, fmt.Sprintf("%s(%s)%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.mcp.no_token"), theme.TagColor))
	} else if svc.HasToken {
		parts = append(parts, fmt.Sprintf("%s%s ✓%s", theme.ColorTag(theme.SuccessHex), i18n.T("tui.hints.token"), theme.TagColor))
	} else {
		parts = append(parts, fmt.Sprintf("%s%s !%s", theme.ColorTag(theme.ErrorHex), i18n.T("tui.mcp.token_missing"), theme.TagColor))
	}

	// Write mode (gitlab only)
	if svc.Name == "gitlab" {
		if svc.WriteEnabled {
			parts = append(parts, fmt.Sprintf("%s%s ✓%s", theme.ColorTag(theme.SuccessHex), i18n.T("tui.hints.write"), theme.TagColor))
		} else {
			parts = append(parts, fmt.Sprintf("%s%s ✗%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.hints.write"), theme.TagColor))
		}
	}

	return strings.Join(parts, "  ")
}

// ─────────────────────────────────────────────────────────────────────────────
// Hub-level actions — unified mutation via config.Save()
// ─────────────────────────────────────────────────────────────────────────────

func (v *MCPView) currentServiceIndex() int {
	if v.list == nil {
		return -1
	}
	_, item, ok := v.list.CurrentItem()
	if !ok {
		return -1
	}
	idx, ok := item.Reference.(int)
	if !ok || idx < 0 || idx >= len(v.services) {
		return -1
	}
	return idx
}

func (v *MCPView) toggleHubCurrent() {
	idx := v.currentServiceIndex()
	if idx < 0 {
		return
	}
	svc := &v.services[idx]
	svc.Enabled = !svc.Enabled

	// Mutate live config and save
	cfg := v.cfg.GetConfig()
	v.setMCPEnabled(cfg, svc.Name, svc.Enabled)
	if err := v.cfg.SaveConfig(cfg); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.mcp.save_error")+": "+err.Error(), false)
		}
		return
	}

	v.populateList()
	v.buildCommands()

	if v.shell != nil {
		if svc.Enabled {
			v.shell.ShowToastMsg(svc.Name+" "+i18n.T("tui.mcp.enabled"), true)
		} else {
			v.shell.ShowToastMsg(svc.Name+" "+i18n.T("tui.mcp.disabled"), true)
		}
	}
}

func (v *MCPView) promptTokenCurrent() {
	idx := v.currentServiceIndex()
	if idx < 0 {
		return
	}
	svc := v.services[idx]
	if svc.Name == "team" {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.mcp.team_no_token"), false)
		}
		return
	}
	if v.shell != nil {
		v.shell.ShowPasswordModal("Token "+svc.Name, func(token string) {
			if token != "" && v.appCtx != nil && v.appCtx.Secrets != nil {
				key := fmt.Sprintf("%s-token", svc.Name)
				_ = v.appCtx.Secrets.Set(context.Background(), key, token)
				v.services[idx].HasToken = true
				v.populateList()
				v.buildCommands()
				v.shell.ShowToastMsg(i18n.Tf("tui.mcp.token_saved", svc.Name), true)
			}
		})
	}
}

func (v *MCPView) toggleWriteCurrent() {
	idx := v.currentServiceIndex()
	if idx < 0 {
		return
	}
	svc := &v.services[idx]
	if svc.Name != "gitlab" {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.mcp.write_gitlab_only"), false)
		}
		return
	}
	svc.WriteEnabled = !svc.WriteEnabled

	cfg := v.cfg.GetConfig()
	cfg.MCP.Gitlab.WriteEnabled = svc.WriteEnabled
	if err := v.cfg.SaveConfig(cfg); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.mcp.save_error")+": "+err.Error(), false)
		}
		return
	}

	v.populateList()
	v.buildCommands()

	if v.shell != nil {
		if svc.WriteEnabled {
			v.shell.ShowToastMsg(i18n.T("tui.mcp.write_enabled"), true)
		} else {
			v.shell.ShowToastMsg(i18n.T("tui.mcp.write_disabled"), true)
		}
	}
}

func (v *MCPView) toggleService(name string, enable bool) {
	for i := range v.services {
		if v.services[i].Name == name {
			v.services[i].Enabled = enable
			cfg := v.cfg.GetConfig()
			v.setMCPEnabled(cfg, name, enable)
			_ = v.cfg.SaveConfig(cfg)
			v.populateList()
			v.buildCommands()
			return
		}
	}
}

// setMCPEnabled sets the enabled field on the corresponding MCPServerConfig.
func (v *MCPView) setMCPEnabled(cfg *config.Config, name string, enabled bool) {
	switch name {
	case "figma":
		cfg.MCP.Figma.Enabled = enabled
	case "gitlab":
		cfg.MCP.Gitlab.Enabled = enabled
	case "jira":
		cfg.MCP.Jira.Enabled = enabled
	case "gslides":
		cfg.MCP.Gslides.Enabled = enabled
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Data loading
// ─────────────────────────────────────────────────────────────────────────────

func (v *MCPView) loadServices() {
	cfg := v.cfg.GetConfig()
	v.services = []MCPService{
		{Name: "figma", Enabled: cfg.MCP.Figma.Enabled, HasToken: v.checkToken("figma"), WriteEnabled: false},
		{Name: "gitlab", Enabled: cfg.MCP.Gitlab.Enabled, HasToken: v.checkToken("gitlab"), WriteEnabled: cfg.MCP.Gitlab.WriteEnabled},
		{Name: "gslides", Enabled: cfg.MCP.Gslides.Enabled, HasToken: v.checkToken("gslides"), WriteEnabled: false},
		{Name: "jira", Enabled: cfg.MCP.Jira.Enabled, HasToken: v.checkToken("jira"), WriteEnabled: false},
		{Name: "team", Enabled: true, HasToken: true, WriteEnabled: false},
	}
}

func (v *MCPView) checkToken(serviceName string) bool {
	if serviceName == "team" {
		return true
	}
	if v.appCtx == nil || v.appCtx.Secrets == nil {
		return false
	}
	key := fmt.Sprintf("%s-token", serviceName)
	val, err := v.appCtx.Secrets.Get(context.Background(), key)
	return err == nil && val != ""
}

// ─────────────────────────────────────────────────────────────────────────────
// Contextual commands
// ─────────────────────────────────────────────────────────────────────────────

func (v *MCPView) buildCommands() {
	var cmds []ContextCommand

	for _, svc := range v.services {
		svc := svc
		var desc string
		if svc.Enabled {
			desc = "✓ → " + i18n.T("tui.mcp.cmd_disable")
		} else {
			desc = "✗ → " + i18n.T("tui.mcp.cmd_enable")
		}
		cmds = append(cmds, ContextCommand{
			ID:          "toggle." + svc.Name,
			Label:       svc.Name,
			Aliases:     []string{"toggle " + svc.Name},
			Description: desc,
			Category:    "MCP",
			Action:      func() { v.toggleService(svc.Name, !svc.Enabled) },
		})
		if svc.Name != "team" {
			cmds = append(cmds, ContextCommand{
				ID:          "token." + svc.Name,
				Label:       "token " + svc.Name,
				Aliases:     []string{svc.Name + " token"},
				Description: i18n.T("tui.mcp.cmd_token"),
				Category:    "MCP",
				Action: func() {
					for i, s := range v.services {
						if s.Name == svc.Name {
							v.list.SelectIndex(i + 1) // +1 for header
							v.promptTokenCurrent()
							return
						}
					}
				},
			})
		}
	}

	cmds = append(cmds, ContextCommand{
		ID:          "write.gitlab",
		Label:       "gitlab " + i18n.T("tui.hints.write"),
		Aliases:     []string{"write gitlab"},
		Description: i18n.T("tui.mcp.cmd_write_toggle"),
		Category:    "MCP",
		Action: func() {
			for i, s := range v.services {
				if s.Name == "gitlab" {
					v.list.SelectIndex(i + 1) // +1 for header
					v.toggleWriteCurrent()
					return
				}
			}
		},
	})

	v.commands = cmds
}

// ─────────────────────────────────────────────────────────────────────────────
// Config helper — mcpSetupServiceOptions preserved for omnibar
// ─────────────────────────────────────────────────────────────────────────────

var mcpSetupServiceOptions = []SelectOption{
	{Label: "Figma", Value: "figma"},
	{Label: "GitLab", Value: "gitlab"},
	{Label: "Google Slides", Value: "gslides"},
}
