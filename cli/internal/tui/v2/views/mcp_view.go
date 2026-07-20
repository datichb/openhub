package views

import (
	"context"
	"fmt"

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

// MCPView displays MCP server management with toggle actions.
type MCPView struct {
	app      *tview.Application
	appCtx   *app.App
	table    *tview.Table
	services []MCPService
	shell    ShellAccess
}

var _ View = (*MCPView)(nil)

// NewMCPView creates a new MCP view.
func NewMCPView(a *app.App) *MCPView {
	return &MCPView{appCtx: a}
}

// SetShell provides the shell reference for modal interactions.
func (v *MCPView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *MCPView) ID() string { return "mcp" }

// Title returns the display title.
func (v *MCPView) Title() string { return "MCP" }

// StatusHints returns keybinding hints.
func (v *MCPView) StatusHints() string {
	return "j/k nav · e enable · d disable · t token · w écriture · S setup · R reset"
}

// Mount builds the MCP management interface.
func (v *MCPView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.table = tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	v.table.SetBackgroundColor(theme.BgPanel)
	v.table.SetBorderPadding(1, 0, 2, 2)
	v.table.SetSelectedStyle(tcell.StyleDefault.
		Background(theme.BgElement).
		Foreground(theme.FgPrimary))

	v.loadServices()
	v.populateTable()

	content.AddItem(v.table, 0, 1, true)
}

// Unmount cleans up resources.
func (v *MCPView) Unmount() {
	v.app = nil
	v.table = nil
}

// HandleKey processes MCP view key events.
func (v *MCPView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	row, _ := v.table.GetSelection()
	idx := row - 1 // account for header
	if idx < 0 || idx >= len(v.services) {
		return event
	}

	switch event.Rune() {
	case 'e':
		v.toggleService(idx, true)
		return nil
	case 'd':
		v.toggleService(idx, false)
		return nil
	case 't':
		v.promptToken(idx)
		return nil
	case 'w':
		v.toggleWriteEnabled(idx)
		return nil
	case 'R':
		v.resetService(idx)
		return nil
	case 'S':
		v.setupWizard()
		return nil
	}
	return event
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

func (v *MCPView) populateTable() {
	v.table.Clear()

	// Header
	headerStyle := tcell.StyleDefault.Foreground(theme.Accent).Bold(true)
	v.table.SetCell(0, 0, tview.NewTableCell("  Service").SetStyle(headerStyle).SetSelectable(false))
	v.table.SetCell(0, 1, tview.NewTableCell("État").SetStyle(headerStyle).SetSelectable(false))
	v.table.SetCell(0, 2, tview.NewTableCell("Écriture").SetStyle(headerStyle).SetSelectable(false))
	v.table.SetCell(0, 3, tview.NewTableCell("Token").SetStyle(headerStyle).SetSelectable(false))

	for i, svc := range v.services {
		// Service name
		nameCell := tview.NewTableCell("  " + svc.Name).
			SetTextColor(theme.FgPrimary).
			SetExpansion(1)

		// Enabled state
		var stateText string
		var stateColor tcell.Color
		if svc.Enabled {
			stateText = "activé"
			stateColor = theme.Success
		} else {
			stateText = "désactivé"
			stateColor = theme.FgMuted
		}
		stateCell := tview.NewTableCell(stateText).
			SetTextColor(stateColor).
			SetExpansion(1)

		// Write enabled state
		var writeText string
		var writeColor tcell.Color
		if svc.Name == "team" || svc.Name == "figma" || svc.Name == "gslides" {
			writeText = "—"
			writeColor = theme.FgMuted
		} else if svc.WriteEnabled {
			writeText = "activé"
			writeColor = theme.Success
		} else {
			writeText = "désactivé"
			writeColor = theme.FgMuted
		}
		writeCell := tview.NewTableCell(writeText).
			SetTextColor(writeColor).
			SetExpansion(1)

		// Token state
		var tokenText string
		var tokenColor tcell.Color
		if svc.Name == "team" {
			tokenText = "—"
			tokenColor = theme.FgMuted
		} else if svc.HasToken {
			tokenText = theme.IconSuccess + " configuré"
			tokenColor = theme.Success
		} else {
			tokenText = theme.IconError + " manquant"
			tokenColor = theme.Error
		}
		tokenCell := tview.NewTableCell(tokenText).
			SetTextColor(tokenColor).
			SetExpansion(1)

		v.table.SetCell(i+1, 0, nameCell)
		v.table.SetCell(i+1, 1, stateCell)
		v.table.SetCell(i+1, 2, writeCell)
		v.table.SetCell(i+1, 3, tokenCell)
	}
}

func (v *MCPView) toggleService(idx int, enable bool) {
	svc := v.services[idx]
	vip := mcpConfigViper()
	vip.Set(fmt.Sprintf("mcp.%s.enabled", svc.Name), enable)
	_ = vip.WriteConfigAs(config.ConfigPath())

	v.services[idx].Enabled = enable
	v.populateTable()
}

func (v *MCPView) promptToken(idx int) {
	svc := v.services[idx]
	if svc.Name == "team" {
		return // team doesn't need a token
	}

	if v.shell != nil {
		v.shell.ShowInputModal("Token "+svc.Name, "", func(token string) {
			if token != "" && v.appCtx != nil && v.appCtx.Secrets != nil {
				key := fmt.Sprintf("%s-token", svc.Name)
				_ = v.appCtx.Secrets.Set(context.Background(), key, token)
				v.services[idx].HasToken = true
				v.populateTable()
				v.shell.ShowToastMsg("Token enregistré", true)
			}
		})
	}
}

func (v *MCPView) toggleWriteEnabled(idx int) {
	svc := v.services[idx]
	// Only gitlab supports write mode
	if svc.Name != "gitlab" {
		if v.shell != nil {
			v.shell.ShowToastMsg("Écriture non applicable pour "+svc.Name, false)
		}
		return
	}

	newVal := !svc.WriteEnabled
	vip := mcpConfigViper()
	vip.Set(fmt.Sprintf("mcp.%s.write_enabled", svc.Name), newVal)
	_ = vip.WriteConfigAs(config.ConfigPath())

	v.services[idx].WriteEnabled = newVal
	v.populateTable()

	if v.shell != nil {
		if newVal {
			v.shell.ShowToastMsg("Écriture activée pour "+svc.Name, true)
		} else {
			v.shell.ShowToastMsg("Écriture désactivée pour "+svc.Name, true)
		}
	}
}

func (v *MCPView) resetService(idx int) {
	svc := v.services[idx]
	vip := mcpConfigViper()

	// Disable service
	vip.Set(fmt.Sprintf("mcp.%s.enabled", svc.Name), false)
	vip.Set(fmt.Sprintf("mcp.%s.write_enabled", svc.Name), false)
	_ = vip.WriteConfigAs(config.ConfigPath())

	// Clear token from keychain
	if svc.Name != "team" && v.appCtx != nil && v.appCtx.Secrets != nil {
		key := fmt.Sprintf("%s-token", svc.Name)
		_ = v.appCtx.Secrets.Delete(context.Background(), key)
	}

	v.services[idx].Enabled = false
	v.services[idx].WriteEnabled = false
	v.services[idx].HasToken = false
	v.populateTable()

	if v.shell != nil {
		v.shell.ShowToastMsg("Service réinitialisé: "+svc.Name, true)
	}
}

var mcpSetupServiceOptions = []SelectOption{
	{Label: "Figma", Value: "figma"},
	{Label: "GitLab", Value: "gitlab"},
	{Label: "Google Slides", Value: "gslides"},
}

func (v *MCPView) setupWizard() {
	if v.shell == nil {
		return
	}

	// Step 1: Service selection
	v.shell.ShowSelectModal("Service à configurer", mcpSetupServiceOptions, "", func(service string) {
		// Step 2: Token
		v.shell.ShowPasswordModal("Token "+service, func(token string) {
			if token == "" {
				return
			}

			// Store token
			if v.appCtx != nil && v.appCtx.Secrets != nil {
				key := fmt.Sprintf("%s-token", service)
				_ = v.appCtx.Secrets.Set(context.Background(), key, token)
			}

			// Enable service
			vip := mcpConfigViper()
			vip.Set(fmt.Sprintf("mcp.%s.enabled", service), true)

			// Step 3 (conditional): write mode for gitlab
			if service == "gitlab" {
				writeOptions := []SelectOption{
					{Label: "Activé (créer MR, commenter)", Value: "true"},
					{Label: "Désactivé (lecture seule)", Value: "false"},
				}
				v.shell.ShowSelectModal("Mode écriture GitLab", writeOptions, "false", func(writeVal string) {
					vip.Set("mcp.gitlab.write_enabled", writeVal == "true")
					_ = vip.WriteConfigAs(config.ConfigPath())
					v.loadServices()
					v.populateTable()
					v.shell.ShowToastMsg("GitLab configuré et activé", true)
				})
			} else {
				_ = vip.WriteConfigAs(config.ConfigPath())
				v.loadServices()
				v.populateTable()
				v.shell.ShowToastMsg(service+" configuré et activé", true)
			}
		})
	})
}

func (v *MCPView) checkToken(serviceName string) bool {
	if v.appCtx == nil || v.appCtx.Secrets == nil {
		return false
	}
	if serviceName == "team" {
		return true // team doesn't need a token
	}
	key := fmt.Sprintf("%s-token", serviceName)
	val, err := v.appCtx.Secrets.Get(context.Background(), key)
	return err == nil && val != ""
}

func mcpConfigViper() *viper.Viper {
	return hubViper()
}
