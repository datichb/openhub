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
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// MCPService represents the hub-level state of an MCP service.
type MCPService struct {
	Name         string
	Enabled      bool
	HasToken     bool
	WriteEnabled bool
}

// ProjectMCPOverride holds the per-project state for a single MCP service.
type ProjectMCPOverride struct {
	Name      string // service name
	Override  string // "inherit" | "enabled" | "disabled"
	Effective bool   // resolved effective state (hub + override)
}

// MCPViewConfig holds callbacks for project-level MCP persistence.
type MCPViewConfig struct {
	// OnUpdateProjectMCP is called when the user changes a per-project MCP
	// override. state is "inherit", "enabled", or "disabled".
	OnUpdateProjectMCP func(projectID, service, state string)
}

// MCPView displays MCP server management with two tview.List panels:
//   - Hub panel (global config: enable/disable, token, write)
//   - Project panel (per-project overrides: inherit/enabled/disabled)
//
// Tab switches focus between the two panels.
type MCPView struct {
	app    *tview.Application
	appCtx *app.App
	cfg    MCPViewConfig
	shell  ShellAccess

	// Layout
	content     *tview.Flex
	hubList     *tview.List
	projectList *tview.List
	projectLabel *tview.TextView

	// Data
	services         []MCPService
	projectOverrides []ProjectMCPOverride
	projects         []domain.Project
	selectedProjectID string
	selectedProject   string

	commands []ContextCommand
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
	return "j/k nav · Space toggle · t token · w écriture · Tab panel · p projet · Enter override"
}

// Mount builds the MCP management interface.
func (v *MCPView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	// ── Load data ──────────────────────────────────────────────────────
	v.loadServices()
	v.loadProjects()

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

	// ── Project label ─────────────────────────────────────────────────
	v.projectLabel = tview.NewTextView().
		SetDynamicColors(true)
	v.projectLabel.SetBackgroundColor(theme.BgPanel)
	v.updateProjectLabel()

	// ── Project list ──────────────────────────────────────────────────
	v.projectList = tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSelectedTextColor(theme.FgPrimary).
		SetSelectedBackgroundColor(theme.Accent)
	v.projectList.SetBackgroundColor(theme.BgPanel)
	v.projectList.SetBorderPadding(0, 0, 2, 2)
	v.populateProjectList()

	// ── Hints ─────────────────────────────────────────────────────────
	hints := tview.NewTextView().
		SetDynamicColors(true).
		SetText(fmt.Sprintf("  %sHub: Space toggle · t token · w écriture   Projet: Enter override · p choisir   Tab: changer panel%s",
			theme.ColorTag(theme.TextMutedHex), theme.TagColor))
	hints.SetBackgroundColor(theme.BgPanel)

	// ── Layout ────────────────────────────────────────────────────────
	v.content = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(hubLabel, 1, 0, false).
		AddItem(v.hubList, len(v.services)+1, 0, true).
		AddItem(v.projectLabel, 1, 0, false).
		AddItem(v.projectList, v.projectListHeight(), 0, false).
		AddItem(hints, 2, 0, false)
	v.content.SetBackgroundColor(theme.BgPanel)

	// ── Key handlers on lists ─────────────────────────────────────────
	// Hub list: Space, t, w actions
	v.hubList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			if len(v.projectOverrides) > 0 {
				v.app.SetFocus(v.projectList)
			}
			return nil
		}
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
		case 'p':
			v.selectProject()
			return nil
		}
		return event
	})

	// Project list: Enter cycle, p select, Tab back to hub
	v.projectList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			v.cycleProjectOverride()
			return nil
		case tcell.KeyTab:
			v.app.SetFocus(v.hubList)
			return nil
		}
		switch event.Rune() {
		case 'p':
			v.selectProject()
			return nil
		}
		return event
	})

	v.buildCommands()
	content.AddItem(v.content, 0, 1, true)
}

// Unmount cleans up resources.
func (v *MCPView) Unmount() {
	v.app = nil
	v.content = nil
	v.hubList = nil
	v.projectList = nil
	v.projectLabel = nil
	v.commands = nil
}

// HandleKey delegates to the focused list; Tab is handled by the lists themselves.
func (v *MCPView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	// p is handled by list InputCapture; ensure it doesn't fall through to omnibar.
	if event.Rune() == 'p' {
		v.selectProject()
		return nil
	}
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
// Project list management
// ─────────────────────────────────────────────────────────────────────────────

func (v *MCPView) populateProjectList() {
	v.projectList.Clear()
	if len(v.projectOverrides) == 0 {
		v.projectList.AddItem(
			fmt.Sprintf("  %sAucun projet sélectionné — appuyez p pour choisir%s",
				theme.ColorTag(theme.TextMutedHex), theme.TagColor),
			"", 0, nil)
		return
	}
	for _, ov := range v.projectOverrides {
		v.projectList.AddItem(v.projectItemText(ov), "", 0, nil)
	}
}

func (v *MCPView) projectItemText(ov ProjectMCPOverride) string {
	var stateLabel, stateColor string
	switch ov.Override {
	case "enabled":
		stateLabel = "activer   "
		stateColor = theme.SuccessHex
	case "disabled":
		stateLabel = "désactiver"
		stateColor = theme.ErrorHex
	default:
		stateLabel = "hérite hub"
		stateColor = theme.TextMutedHex
	}

	var effLabel, effColor string
	if ov.Effective {
		effLabel = "actif"
		effColor = theme.SuccessHex
	} else {
		effLabel = "inactif"
		effColor = theme.TextMutedHex
	}

	return fmt.Sprintf("  %s%-12s%s  [%s%s%s]  → %s%s%s",
		theme.ColorTag(theme.TextPrimaryHex), ov.Name, theme.TagColor,
		theme.ColorTag(stateColor), stateLabel, theme.TagColor,
		theme.ColorTag(effColor), effLabel, theme.TagColor)
}

func (v *MCPView) updateProjectLabel() {
	if v.selectedProjectID == "" {
		v.projectLabel.SetText(fmt.Sprintf("  %s─── Projet (aucun sélectionné) ──────────────────────%s",
			theme.ColorTag(theme.AccentHex), theme.TagColor))
	} else {
		v.projectLabel.SetText(fmt.Sprintf("  %s─── Projet : %s ──────────────────────────────────────%s",
			theme.ColorTag(theme.AccentHex), v.selectedProject, theme.TagColor))
	}
}

func (v *MCPView) projectListHeight() int {
	if len(v.projectOverrides) == 0 {
		return 2
	}
	return len(v.projectOverrides) + 1
}

// resizeProjectList updates the project list height in the layout after data change.
func (v *MCPView) resizeProjectList() {
	if v.content == nil {
		return
	}
	// Rebuild layout to update heights (tview.Flex doesn't support dynamic resize directly)
	// Simpler: just update the item fixed size via ResizeItem
	v.content.ResizeItem(v.projectList, v.projectListHeight(), 0)
}

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
	v.recomputeEffective()
	v.populateProjectList()

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
		v.shell.ShowInputModal("Token "+svc.Name, "", func(token string) {
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
			v.recomputeEffective()
			v.populateProjectList()
			v.buildCommands()
			return
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Project-level actions
// ─────────────────────────────────────────────────────────────────────────────

func (v *MCPView) selectProject() {
	if v.shell == nil {
		return
	}
	if len(v.projects) == 0 {
		v.shell.ShowToastMsg("Aucun projet enregistré", false)
		return
	}
	options := make([]SelectOption, len(v.projects))
	for i, p := range v.projects {
		options[i] = SelectOption{Label: p.Name, Value: p.ID}
	}
	v.shell.ShowSelectModal("Choisir un projet", options, v.selectedProjectID, func(projectID string) {
		for _, p := range v.projects {
			if p.ID == projectID {
				v.selectedProjectID = p.ID
				v.selectedProject = p.Name
				v.rebuildProjectOverrides(p)
				v.updateProjectLabel()
				v.populateProjectList()
				v.resizeProjectList()
				// Switch focus to project list
				if v.app != nil {
					v.app.SetFocus(v.projectList)
				}
				v.buildCommands()
				return
			}
		}
	})
}

func (v *MCPView) cycleProjectOverride() {
	idx := v.projectList.GetCurrentItem()
	if idx < 0 || idx >= len(v.projectOverrides) {
		return
	}
	ov := &v.projectOverrides[idx]

	next := map[string]string{
		"inherit":  "enabled",
		"enabled":  "disabled",
		"disabled": "inherit",
	}
	ov.Override = next[ov.Override]
	ov.Effective = v.resolveEffective(ov.Name, ov.Override)

	// Persist
	if v.cfg.OnUpdateProjectMCP != nil {
		v.cfg.OnUpdateProjectMCP(v.selectedProjectID, ov.Name, ov.Override)
	}

	// Update list item in place
	v.projectList.SetItemText(idx, v.projectItemText(*ov), "")

	if v.shell != nil {
		labels := map[string]string{
			"inherit":  "hérite hub",
			"enabled":  "activé",
			"disabled": "désactivé",
		}
		v.shell.ShowToastMsg(fmt.Sprintf("%s : %s", ov.Name, labels[ov.Override]), true)
	}

	v.buildCommands()
}

func (v *MCPView) rebuildProjectOverrides(p domain.Project) {
	overrideMap := make(map[string]string)
	if p.MCPConfig != nil {
		for _, svc := range p.MCPConfig.Services {
			if svc.Enabled == nil {
				overrideMap[svc.Name] = "inherit"
			} else if *svc.Enabled {
				overrideMap[svc.Name] = "enabled"
			} else {
				overrideMap[svc.Name] = "disabled"
			}
		}
	}
	v.projectOverrides = v.projectOverrides[:0]
	for _, svc := range v.services {
		if svc.Name == "team" {
			continue
		}
		override := "inherit"
		if val, ok := overrideMap[svc.Name]; ok {
			override = val
		}
		v.projectOverrides = append(v.projectOverrides, ProjectMCPOverride{
			Name:      svc.Name,
			Override:  override,
			Effective: v.resolveEffective(svc.Name, override),
		})
	}
}

func (v *MCPView) recomputeEffective() {
	for i := range v.projectOverrides {
		v.projectOverrides[i].Effective = v.resolveEffective(
			v.projectOverrides[i].Name,
			v.projectOverrides[i].Override,
		)
	}
}

func (v *MCPView) resolveEffective(serviceName, override string) bool {
	hubEnabled := false
	for _, svc := range v.services {
		if svc.Name == serviceName {
			hubEnabled = svc.Enabled
			break
		}
	}
	switch override {
	case "enabled":
		return true
	case "disabled":
		return false
	default:
		return hubEnabled
	}
}

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

func (v *MCPView) loadProjects() {
	if v.appCtx == nil || v.appCtx.Projects == nil {
		return
	}
	projects, err := v.appCtx.Projects.List(context.Background(), "")
	if err != nil {
		return
	}
	v.projects = projects
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

	if v.selectedProjectID != "" {
		for _, ov := range v.projectOverrides {
			ov := ov
			cmds = append(cmds, ContextCommand{
				ID:          "proj.mcp." + ov.Name,
				Label:       "projet " + ov.Name,
				Aliases:     []string{ov.Name + " projet"},
				Description: fmt.Sprintf("Override %s : %s → cycle", ov.Name, ov.Override),
				Category:    "MCP Projet",
				Action: func() {
					for i, o := range v.projectOverrides {
						if o.Name == ov.Name {
							v.projectList.SetCurrentItem(i)
							if v.app != nil {
								v.app.SetFocus(v.projectList)
							}
							v.cycleProjectOverride()
							return
						}
					}
				},
			})
		}
		cmds = append(cmds, ContextCommand{
			ID:          "proj.select",
			Label:       "changer projet MCP",
			Aliases:     []string{"select project mcp"},
			Description: "Choisir un autre projet",
			Category:    "MCP Projet",
			Action:      v.selectProject,
		})
	}

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
