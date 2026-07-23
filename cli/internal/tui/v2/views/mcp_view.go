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

// MCPView displays MCP server management with contextual omnibar commands.
// It shows hub-level config and, when a project is selected, per-project overrides.
type MCPView struct {
	app      *tview.Application
	appCtx   *app.App
	cfg      MCPViewConfig
	display  *tview.TextView
	services []MCPService
	shell    ShellAccess
	commands []ContextCommand

	// Hub section cursor (0 … len(services)-1)
	cursor int

	// Project section
	projects          []domain.Project
	selectedProjectID string
	selectedProject   string // display name
	projectOverrides  []ProjectMCPOverride
	// inProjectSection: true when cursor is in the project overrides section
	inProjectSection bool
	// projectCursor: index within projectOverrides
	projectCursor int
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
	return "j/k nav · Space toggle · t token · Tab section · P projet · Enter toggle override"
}

// Mount builds the MCP management interface.
func (v *MCPView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.cursor = 0
	v.inProjectSection = false
	v.projectCursor = 0

	v.display = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	v.display.SetBackgroundColor(theme.BgPanel)
	v.display.SetBorderPadding(1, 1, 2, 2)

	v.loadServices()
	v.loadProjects()
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
	switch event.Key() {
	case tcell.KeyTab:
		v.toggleSection()
		return nil
	case tcell.KeyEnter:
		if v.inProjectSection {
			v.cycleProjectOverride()
			return nil
		}
	}

	switch event.Rune() {
	case 'j':
		v.moveCursorDown()
		return nil
	case 'k':
		v.moveCursorUp()
		return nil
	case ' ':
		if !v.inProjectSection {
			v.toggleCurrent()
		}
		return nil
	case 't':
		if !v.inProjectSection {
			v.promptTokenCurrent()
		}
		return nil
	case 'w':
		if !v.inProjectSection {
			v.toggleWriteCurrent()
		}
		return nil
	case 'P':
		v.selectProject()
		return nil
	}
	return event
}

// ContextCommands returns contextual commands for the omnibar.
func (v *MCPView) ContextCommands() []ContextCommand {
	return v.commands
}

// ─────────────────────────────────────────────────────────────────────────────
// Navigation
// ─────────────────────────────────────────────────────────────────────────────

func (v *MCPView) toggleSection() {
	if len(v.projectOverrides) == 0 {
		return // no project selected, no project section
	}
	v.inProjectSection = !v.inProjectSection
	v.render()
}

func (v *MCPView) moveCursorDown() {
	if v.inProjectSection {
		if v.projectCursor < len(v.projectOverrides)-1 {
			v.projectCursor++
			v.render()
		}
	} else {
		if v.cursor < len(v.services)-1 {
			v.cursor++
			v.render()
		}
	}
}

func (v *MCPView) moveCursorUp() {
	if v.inProjectSection {
		if v.projectCursor > 0 {
			v.projectCursor--
			v.render()
		}
	} else {
		if v.cursor > 0 {
			v.cursor--
			v.render()
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

func (v *MCPView) render() {
	var b strings.Builder

	// ── Hub section ──
	b.WriteString(fmt.Sprintf("  %s─── Hub (global) ───%s\n\n",
		theme.ColorTag(theme.AccentHex), theme.TagColor))

	for i, svc := range v.services {
		var statusIcon, statusColor string
		if svc.Enabled {
			statusIcon = "✓"
			statusColor = theme.SuccessHex
		} else {
			statusIcon = "✗"
			statusColor = theme.TextMutedHex
		}

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

		var writeInfo string
		if svc.Name == "gitlab" {
			if svc.WriteEnabled {
				writeInfo = fmt.Sprintf("  %sécriture ✓%s", theme.ColorTag(theme.SuccessHex), theme.TagColor)
			} else {
				writeInfo = fmt.Sprintf("  %sécriture ✗%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor)
			}
		}

		if !v.inProjectSection && i == v.cursor {
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

	b.WriteString(fmt.Sprintf("\n  %sSpace toggle · t token · w écriture · Tab section projet%s\n",
		theme.ColorTag(theme.TextMutedHex), theme.TagColor))

	// ── Project section ──
	b.WriteString("\n")

	if v.selectedProjectID == "" {
		b.WriteString(fmt.Sprintf("  %s─── Projet ───%s\n", theme.ColorTag(theme.AccentHex), theme.TagColor))
		b.WriteString(fmt.Sprintf("  %sAucun projet sélectionné — appuyez P pour choisir%s\n",
			theme.ColorTag(theme.TextMutedHex), theme.TagColor))
	} else {
		b.WriteString(fmt.Sprintf("  %s─── Projet : %s ───%s\n\n",
			theme.ColorTag(theme.AccentHex), v.selectedProject, theme.TagColor))

		for i, ov := range v.projectOverrides {
			// State label
			var stateLabel, stateColor string
			switch ov.Override {
			case "enabled":
				stateLabel = "activer       "
				stateColor = theme.SuccessHex
			case "disabled":
				stateLabel = "désactiver    "
				stateColor = theme.ErrorHex
			default:
				stateLabel = "hérite hub    "
				stateColor = theme.TextMutedHex
			}

			// Effective indicator
			var effLabel, effColor string
			if ov.Effective {
				effLabel = "actif"
				effColor = theme.SuccessHex
			} else {
				effLabel = "inactif"
				effColor = theme.TextMutedHex
			}

			if v.inProjectSection && i == v.projectCursor {
				b.WriteString(fmt.Sprintf("  %s▸%s  %s%-12s%s  [%s%s%s]  → %s%s%s\n",
					theme.ColorTag(theme.ActionHex), theme.TagColor,
					theme.ColorTag(theme.TextPrimaryHex), ov.Name, theme.TagColor,
					theme.ColorTag(stateColor), stateLabel, theme.TagColor,
					theme.ColorTag(effColor), effLabel, theme.TagColor))
			} else {
				b.WriteString(fmt.Sprintf("     %s%-12s%s  [%s%s%s]  → %s%s%s\n",
					theme.ColorTag(theme.TextSecondaryHex), ov.Name, theme.TagColor,
					theme.ColorTag(stateColor), stateLabel, theme.TagColor,
					theme.ColorTag(effColor), effLabel, theme.TagColor))
			}
		}
		b.WriteString(fmt.Sprintf("\n  %sEnter toggle · P changer projet · Tab section hub%s\n",
			theme.ColorTag(theme.TextMutedHex), theme.TagColor))
	}

	v.display.SetText(b.String())
}

// ─────────────────────────────────────────────────────────────────────────────
// Hub-level actions
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

// ─────────────────────────────────────────────────────────────────────────────
// Project-level actions
// ─────────────────────────────────────────────────────────────────────────────

// selectProject opens a modal to choose the active project for override display.
func (v *MCPView) selectProject() {
	if v.shell == nil || len(v.projects) == 0 {
		if v.shell != nil {
			v.shell.ShowToastMsg("Aucun projet enregistré", false)
		}
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
				v.inProjectSection = true
				v.projectCursor = 0
				v.render()
				v.buildCommands()
				return
			}
		}
	})
}

// cycleProjectOverride cycles the selected service through inherit → enabled → disabled → inherit.
func (v *MCPView) cycleProjectOverride() {
	if v.projectCursor < 0 || v.projectCursor >= len(v.projectOverrides) {
		return
	}
	ov := &v.projectOverrides[v.projectCursor]

	next := map[string]string{
		"inherit":  "enabled",
		"enabled":  "disabled",
		"disabled": "inherit",
	}
	ov.Override = next[ov.Override]

	// Recompute effective state
	ov.Effective = v.resolveEffective(ov.Name, ov.Override)

	// Persist via callback
	if v.cfg.OnUpdateProjectMCP != nil {
		v.cfg.OnUpdateProjectMCP(v.selectedProjectID, ov.Name, ov.Override)
	}

	v.render()
	v.buildCommands()

	if v.shell != nil {
		labels := map[string]string{
			"inherit":  "hérite hub",
			"enabled":  "activé",
			"disabled": "désactivé",
		}
		v.shell.ShowToastMsg(fmt.Sprintf("%s : %s", ov.Name, labels[ov.Override]), true)
	}
}

// rebuildProjectOverrides rebuilds the projectOverrides slice from project MCPConfig.
func (v *MCPView) rebuildProjectOverrides(p domain.Project) {
	// Collect per-project overrides
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

	// One row per hub service (excluding "team" which has no project override)
	v.projectOverrides = make([]ProjectMCPOverride, 0, len(v.services))
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

// resolveEffective computes the effective MCP state (hub enabled + override).
func (v *MCPView) resolveEffective(serviceName, override string) bool {
	// Find hub-level enabled state
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
	default: // "inherit"
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

	// Hub-level commands
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
			Action: func() {
				v.toggleService(svc.Name, !svc.Enabled)
			},
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

	// Project-level commands (if a project is selected)
	if v.selectedProjectID != "" {
		for _, ov := range v.projectOverrides {
			ov := ov
			cmds = append(cmds, ContextCommand{
				ID:          "proj.mcp." + ov.Name,
				Label:       "projet " + ov.Name,
				Aliases:     []string{ov.Name + " projet", "mcp " + ov.Name + " projet"},
				Description: fmt.Sprintf("Override %s: %s → cycle", ov.Name, ov.Override),
				Category:    "MCP Projet",
				Action: func() {
					for i, o := range v.projectOverrides {
						if o.Name == ov.Name {
							v.inProjectSection = true
							v.projectCursor = i
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
