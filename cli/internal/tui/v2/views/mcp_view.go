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
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// MCPService represents the state of an MCP service.
type MCPService struct {
	Name         string
	Enabled      bool
	HasToken     bool
	WriteEnabled bool
}

// MCPView displays MCP server management with contextual omnibar commands.
type MCPView struct {
	app      *tview.Application
	appCtx   *app.App
	display  *tview.TextView
	services []MCPService
	shell    ShellAccess
	cursor   int
	commands []ContextCommand
}

var _ View = (*MCPView)(nil)
var _ CommandProvider = (*MCPView)(nil)

// NewMCPView creates a new MCP view.
func NewMCPView(a *app.App) *MCPView {
	return &MCPView{appCtx: a}
}

// SetShell provides the shell reference for interactions.
func (v *MCPView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *MCPView) ID() string { return "mcp" }

// Title returns the display title.
func (v *MCPView) Title() string { return "MCP" }

// StatusHints returns keybinding hints.
func (v *MCPView) StatusHints() string {
	return "j/k naviguer · Space toggle · t token · Ctrl+P commande"
}

// Mount builds the MCP management interface.
func (v *MCPView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.cursor = 0

	v.display = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	v.display.SetBackgroundColor(theme.BgPanel)
	v.display.SetBorderPadding(1, 1, 2, 2)

	v.loadServices()
	v.render()
	v.buildCommands()

	content.AddItem(v.display, 0, 1, true)
}

// Unmount cleans up resources.
func (v *MCPView) Unmount() {
	v.app = nil
	v.display = nil
	v.commands = nil
}

// HandleKey processes MCP view key events.
func (v *MCPView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'j':
		if v.cursor < len(v.services)-1 {
			v.cursor++
			v.render()
		}
		return nil
	case 'k':
		if v.cursor > 0 {
			v.cursor--
			v.render()
		}
		return nil
	case ' ':
		v.toggleCurrent()
		return nil
	case 't':
		v.promptTokenCurrent()
		return nil
	case 'w':
		v.toggleWriteCurrent()
		return nil
	}
	return event
}

// ContextCommands returns contextual commands for the omnibar.
func (v *MCPView) ContextCommands() []ContextCommand {
	return v.commands
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

func (v *MCPView) render() {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("  %sServeurs MCP%s\n\n",
		theme.ColorTag(theme.AccentHex), theme.TagColor))

	for i, svc := range v.services {
		// Status indicator
		var statusIcon, statusColor string
		if svc.Enabled {
			statusIcon = "✓"
			statusColor = theme.SuccessHex
		} else {
			statusIcon = "✗"
			statusColor = theme.TextMutedHex
		}

		// Token indicator
		var tokenIcon, tokenColor string
		if svc.Name == "team" {
			tokenIcon = "—"
			tokenColor = theme.TextMutedHex
		} else if svc.HasToken {
			tokenIcon = "✓"
			tokenColor = theme.SuccessHex
		} else {
			tokenIcon = "!"
			tokenColor = theme.ErrorHex
		}

		// Write indicator
		var writeInfo string
		if svc.Name == "gitlab" {
			if svc.WriteEnabled {
				writeInfo = fmt.Sprintf("  %sécriture ✓%s", theme.ColorTag(theme.SuccessHex), theme.TagColor)
			} else {
				writeInfo = fmt.Sprintf("  %sécriture ✗%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor)
			}
		}

		// Cursor
		if i == v.cursor {
			b.WriteString(fmt.Sprintf("  %s▸%s  %s%-12s%s  %s%s%s  token %s%s%s%s\n",
				theme.ColorTag(theme.ActionHex), theme.TagColor,
				theme.ColorTag(theme.TextPrimaryHex), svc.Name, theme.TagColor,
				theme.ColorTag(statusColor), statusIcon, theme.TagColor,
				theme.ColorTag(tokenColor), tokenIcon, theme.TagColor,
				writeInfo))
		} else {
			b.WriteString(fmt.Sprintf("     %s%-12s%s  %s%s%s  token %s%s%s%s\n",
				theme.ColorTag(theme.TextSecondaryHex), svc.Name, theme.TagColor,
				theme.ColorTag(statusColor), statusIcon, theme.TagColor,
				theme.ColorTag(tokenColor), tokenIcon, theme.TagColor,
				writeInfo))
		}
	}

	b.WriteString(fmt.Sprintf("\n\n  %sSpace toggle · t token · w écriture (gitlab)%s",
		theme.ColorTag(theme.TextMutedHex), theme.TagColor))

	v.display.SetText(b.String())
}

// ─────────────────────────────────────────────────────────────────────────────
// Actions
// ─────────────────────────────────────────────────────────────────────────────

func (v *MCPView) toggleCurrent() {
	if v.cursor < 0 || v.cursor >= len(v.services) {
		return
	}
	svc := &v.services[v.cursor]
	svc.Enabled = !svc.Enabled

	vip := mcpConfigViper()
	vip.Set(fmt.Sprintf("mcp.%s.enabled", svc.Name), svc.Enabled)
	_ = vip.WriteConfigAs(config.ConfigPath())

	v.render()
	v.buildCommands()
}

func (v *MCPView) promptTokenCurrent() {
	if v.cursor < 0 || v.cursor >= len(v.services) {
		return
	}
	svc := v.services[v.cursor]
	if svc.Name == "team" {
		return
	}
	v.promptToken(v.cursor)
}

func (v *MCPView) toggleWriteCurrent() {
	if v.cursor < 0 || v.cursor >= len(v.services) {
		return
	}
	svc := &v.services[v.cursor]
	if svc.Name != "gitlab" {
		if v.shell != nil {
			v.shell.ShowToastMsg("Écriture non applicable pour "+svc.Name, false)
		}
		return
	}

	svc.WriteEnabled = !svc.WriteEnabled
	vip := mcpConfigViper()
	vip.Set(fmt.Sprintf("mcp.%s.write_enabled", svc.Name), svc.WriteEnabled)
	_ = vip.WriteConfigAs(config.ConfigPath())

	v.render()
	v.buildCommands()
	if v.shell != nil {
		if svc.WriteEnabled {
			v.shell.ShowToastMsg("Écriture activée pour "+svc.Name, true)
		} else {
			v.shell.ShowToastMsg("Écriture désactivée pour "+svc.Name, true)
		}
	}
}

func (v *MCPView) promptToken(idx int) {
	svc := v.services[idx]
	if v.shell != nil {
		v.shell.ShowInputModal("Token "+svc.Name, "", func(token string) {
			if token != "" && v.appCtx != nil && v.appCtx.Secrets != nil {
				key := fmt.Sprintf("%s-token", svc.Name)
				_ = v.appCtx.Secrets.Set(context.Background(), key, token)
				v.services[idx].HasToken = true
				v.render()
				v.buildCommands()
				v.shell.ShowToastMsg("Token enregistré", true)
			}
		})
	}
}

func (v *MCPView) toggleService(name string, enable bool) {
	for i := range v.services {
		if v.services[i].Name == name {
			v.services[i].Enabled = enable
			vip := mcpConfigViper()
			vip.Set(fmt.Sprintf("mcp.%s.enabled", name), enable)
			_ = vip.WriteConfigAs(config.ConfigPath())
			v.render()
			v.buildCommands()
			return
		}
	}
}

func (v *MCPView) loadServices() {
	serviceNames := []string{"figma", "gitlab", "gslides", "team"}
	v.services = make([]MCPService, 0, len(serviceNames))

	vip := mcpConfigViper()

	for _, name := range serviceNames {
		enabled := vip.GetBool(fmt.Sprintf("mcp.%s.enabled", name))
		writeEnabled := vip.GetBool(fmt.Sprintf("mcp.%s.write_enabled", name))
		hasToken := v.checkToken(name)
		v.services = append(v.services, MCPService{
			Name:         name,
			Enabled:      enabled,
			HasToken:     hasToken,
			WriteEnabled: writeEnabled,
		})
	}
}

func (v *MCPView) checkToken(serviceName string) bool {
	if v.appCtx == nil || v.appCtx.Secrets == nil {
		return false
	}
	if serviceName == "team" {
		return true
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

		// Toggle command
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
			Action: func() {
				v.toggleService(svc.Name, !svc.Enabled)
			},
		})

		// Token command (except team)
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
							v.promptToken(i)
							return
						}
					}
				},
			})
		}
	}

	// Write toggle for gitlab
	cmds = append(cmds, ContextCommand{
		ID:          "write.gitlab",
		Label:       "gitlab écriture",
		Aliases:     []string{"write gitlab", "gitlab write"},
		Description: "Toggle mode écriture GitLab",
		Category:    "MCP",
		Action: func() {
			for i, s := range v.services {
				if s.Name == "gitlab" {
					v.cursor = i
					v.toggleWriteCurrent()
					return
				}
			}
		},
	})

	v.commands = cmds
}

// ─────────────────────────────────────────────────────────────────────────────
// Config helper (kept for compatibility)
// ─────────────────────────────────────────────────────────────────────────────

var mcpSetupServiceOptions = []SelectOption{
	{Label: "Figma", Value: "figma"},
	{Label: "GitLab", Value: "gitlab"},
	{Label: "Google Slides", Value: "gslides"},
}

func mcpConfigViper() *viper.Viper {
	return hubViper()
}
