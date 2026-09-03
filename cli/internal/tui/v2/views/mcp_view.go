package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/viper"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// MCPService represents the hub-level state of an MCP service.
type MCPService struct {
	Name         string
	Enabled      bool
	HasToken     bool
	WriteEnabled bool
}

// MCPViewConfig holds configuration for the MCP view.
type MCPViewConfig struct{}

// MCPView displays MCP server management with a single tview.List panel
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
	content *tview.Flex
	hubList *tview.List

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
	return fmt.Sprintf("j/k nav · Space %s · t %s · w %s · r %s",
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
	loading.SetText(fmt.Sprintf("\n  %sChargement des services MCP...%s", muted, theme.TagColor))
	content.AddItem(loading, 0, 1, true)

	// Load data and build UI asynchronously
	go func() {
		app.QueueUpdateDraw(func() {
			if v.app == nil || v.mountGen != gen {
				return // view was unmounted or re-mounted before the goroutine finished
			}
			v.loadServices()

			// ── Hub label ──────────────────────────────────────────────────────
			hubLabel := tview.NewTextView().
				SetDynamicColors(true).
				SetText(fmt.Sprintf("  %s─── Hub (global) ──────────────────────────────────────%s",
					theme.ColorTag(theme.AccentHex), theme.TagColor))
			hubLabel.SetBackgroundColor(theme.BgPanel)

			// ── Hub list ───────────────────────────────────────────────────────
			v.hubList = tview.NewList().
				ShowSecondaryText(true).
				SetHighlightFullLine(true).
				SetMainTextColor(theme.FgPrimary).
				SetSecondaryTextColor(theme.FgSecondary).
				SetSelectedTextColor(theme.FgPrimary).
				SetSelectedBackgroundColor(theme.Accent)
			v.hubList.SetBackgroundColor(theme.BgPanel)
			v.hubList.SetBorderPadding(0, 0, 2, 2)
			v.populateHubList()

			// ── Hints ─────────────────────────────────────────────────────────
			hints := tview.NewTextView().
				SetDynamicColors(true).
				SetText(fmt.Sprintf("  %sSpace toggle · t token · w écriture · r refresh%s",
					theme.ColorTag(theme.TextMutedHex), theme.TagColor))
			hints.SetBackgroundColor(theme.BgPanel)

			// ── Layout ────────────────────────────────────────────────────────
			v.content = tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(hubLabel, 1, 0, false).
				AddItem(v.hubList, len(v.services)+1, 0, true).
				AddItem(hints, 2, 0, false)
			v.content.SetBackgroundColor(theme.BgPanel)

			// ── Key handlers on hub list ─────────────────────────────────────
			v.hubList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
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
					v.populateHubList()
					return nil
				}
				return event
			})

			v.buildCommands()
			content.RemoveItem(loading)
			content.AddItem(v.content, 0, 1, true)
			app.SetFocus(v.hubList)
		})
	}()
}

// Unmount cleans up resources.
func (v *MCPView) Unmount() {
	v.app = nil
	v.content = nil
	v.hubList = nil
	v.commands = nil
}

// HandleKey delegates to the focused list; Tab is handled by the lists themselves.
func (v *MCPView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	// All other navigation is handled natively by the focused tview.List.
	return event
}

// ContextCommands returns contextual commands for the omnibar.
func (v *MCPView) ContextCommands() []ContextCommand {
	return v.commands
}

// ─────────────────────────────────────────────────────────────────────────────
// Hub list management
// ─────────────────────────────────────────────────────────────────────────────

func (v *MCPView) populateHubList() {
	v.hubList.Clear()
	for _, svc := range v.services {
		main := v.hubItemText(svc)
		secondary := v.hubItemSubtext(svc)
		v.hubList.AddItem(main, secondary, 0, nil)
	}
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
	return fmt.Sprintf("  %s%s%s  %s%-12s%s",
		theme.ColorTag(statusColor), statusIcon, theme.TagColor,
		theme.ColorTag(theme.TextPrimaryHex), svc.Name, theme.TagColor)
}

func (v *MCPView) hubItemSubtext(svc MCPService) string {
	var parts []string

	// Token indicator
	if svc.Name == "team" {
		parts = append(parts, fmt.Sprintf("     %s(sans token)%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor))
	} else if svc.HasToken {
		parts = append(parts, fmt.Sprintf("     %stoken ✓%s", theme.ColorTag(theme.SuccessHex), theme.TagColor))
	} else {
		parts = append(parts, fmt.Sprintf("     %stoken manquant !%s", theme.ColorTag(theme.ErrorHex), theme.TagColor))
	}

	// Write mode (gitlab only)
	if svc.Name == "gitlab" {
		if svc.WriteEnabled {
			parts = append(parts, fmt.Sprintf("%sécriture ✓%s", theme.ColorTag(theme.SuccessHex), theme.TagColor))
		} else {
			parts = append(parts, fmt.Sprintf("%sécriture ✗%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor))
		}
	}

	return strings.Join(parts, "  ")
}

// ─────────────────────────────────────────────────────────────────────────────

// ─────────────────────────────────────────────────────────────────────────────
// Hub-level actions
// ─────────────────────────────────────────────────────────────────────────────

func (v *MCPView) toggleHubCurrent() {
	idx := v.hubList.GetCurrentItem()
	if idx < 0 || idx >= len(v.services) {
		return
	}
	svc := &v.services[idx]
	svc.Enabled = !svc.Enabled

	vip := mcpConfigViper()
	vip.Set(fmt.Sprintf("mcp.%s.enabled", svc.Name), svc.Enabled)
	_ = vip.WriteConfigAs(config.ConfigPath())

	// Update list item in place
	v.hubList.SetItemText(idx, v.hubItemText(*svc), v.hubItemSubtext(*svc))

	// Recompute effective states in project overrides

	v.buildCommands()

	if v.shell != nil {
		if svc.Enabled {
			v.shell.ShowToastMsg(svc.Name+" activé", true)
		} else {
			v.shell.ShowToastMsg(svc.Name+" désactivé", true)
		}
	}
}

func (v *MCPView) promptTokenCurrent() {
	idx := v.hubList.GetCurrentItem()
	if idx < 0 || idx >= len(v.services) {
		return
	}
	svc := v.services[idx]
	if svc.Name == "team" {
		if v.shell != nil {
			v.shell.ShowToastMsg("team n'utilise pas de token", false)
		}
		return
	}
	if v.shell != nil {
		v.shell.ShowPasswordModal("Token "+svc.Name, func(token string) {
			if token != "" && v.appCtx != nil && v.appCtx.Secrets != nil {
				key := fmt.Sprintf("%s-token", svc.Name)
				_ = v.appCtx.Secrets.Set(context.Background(), key, token)
				v.services[idx].HasToken = true
				v.hubList.SetItemText(idx, v.hubItemText(v.services[idx]), v.hubItemSubtext(v.services[idx]))
				v.buildCommands()
				v.shell.ShowToastMsg("Token enregistré pour "+svc.Name, true)
			}
		})
	}
}

func (v *MCPView) toggleWriteCurrent() {
	idx := v.hubList.GetCurrentItem()
	if idx < 0 || idx >= len(v.services) {
		return
	}
	svc := &v.services[idx]
	if svc.Name != "gitlab" {
		if v.shell != nil {
			v.shell.ShowToastMsg("Écriture applicable uniquement à gitlab", false)
		}
		return
	}
	svc.WriteEnabled = !svc.WriteEnabled
	vip := mcpConfigViper()
	vip.Set(fmt.Sprintf("mcp.%s.write_enabled", svc.Name), svc.WriteEnabled)
	_ = vip.WriteConfigAs(config.ConfigPath())

	v.hubList.SetItemText(idx, v.hubItemText(*svc), v.hubItemSubtext(*svc))
	v.buildCommands()

	if v.shell != nil {
		if svc.WriteEnabled {
			v.shell.ShowToastMsg("Écriture gitlab activée", true)
		} else {
			v.shell.ShowToastMsg("Écriture gitlab désactivée", true)
		}
	}
}

func (v *MCPView) toggleService(name string, enable bool) {
	for i := range v.services {
		if v.services[i].Name == name {
			v.services[i].Enabled = enable
			vip := mcpConfigViper()
			vip.Set(fmt.Sprintf("mcp.%s.enabled", name), enable)
			_ = vip.WriteConfigAs(config.ConfigPath())
			v.hubList.SetItemText(i, v.hubItemText(v.services[i]), v.hubItemSubtext(v.services[i]))
			v.buildCommands()
			return
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Project-level actions
// ─────────────────────────────────────────────────────────────────────────────



// ─────────────────────────────────────────────────────────────────────────────
// Data loading
// ─────────────────────────────────────────────────────────────────────────────

func (v *MCPView) loadServices() {
	serviceNames := []string{"figma", "gitlab", "gslides", "team"}
	v.services = make([]MCPService, 0, len(serviceNames))
	vip := mcpConfigViper()
	for _, name := range serviceNames {
		v.services = append(v.services, MCPService{
			Name:         name,
			Enabled:      vip.GetBool(fmt.Sprintf("mcp.%s.enabled", name)),
			HasToken:     v.checkToken(name),
			WriteEnabled: vip.GetBool(fmt.Sprintf("mcp.%s.write_enabled", name)),
		})
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
			desc = "✓ → désactiver"
		} else {
			desc = "✗ → activer"
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
				Description: "Configurer le token",
				Category:    "MCP",
				Action: func() {
					for i, s := range v.services {
						if s.Name == svc.Name {
							v.hubList.SetCurrentItem(i)
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
		Label:       "gitlab écriture",
		Aliases:     []string{"write gitlab"},
		Description: "Toggle mode écriture GitLab",
		Category:    "MCP",
		Action: func() {
			for i, s := range v.services {
				if s.Name == "gitlab" {
					v.hubList.SetCurrentItem(i)
					v.toggleWriteCurrent()
					return
				}
			}
		},
	})

	v.commands = cmds
}

// ─────────────────────────────────────────────────────────────────────────────
// Config helper
// ─────────────────────────────────────────────────────────────────────────────

var mcpSetupServiceOptions = []SelectOption{
	{Label: "Figma", Value: "figma"},
	{Label: "GitLab", Value: "gitlab"},
	{Label: "Google Slides", Value: "gslides"},
}

func mcpConfigViper() *viper.Viper {
	return hubViper()
}
