package views

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ProjectConfigViewConfig holds external dependencies for the project config view.
type ProjectConfigViewConfig struct {
	// GetProject returns the currently active project (nil if none).
	GetProject func() *domain.Project
	// SaveProject persists the modified project to the DB.
	SaveProject func(ctx context.Context, p *domain.Project) error
	// Deploy re-deploys the project (updates opencode.json).
	Deploy func(ctx context.Context, p *domain.Project) error
	// AllAgents returns the list of all available agent IDs.
	AllAgents func() []string
}

// projectConfigLine is a single editable row in the project config view.
type projectConfigLine struct {
	section string
	key     string
	kind    string // "string", "bool", "agents", "section-header", "readonly"
	get     func(p *domain.Project) string
	set     func(p *domain.Project, v string)
}

// ProjectConfigView displays and edits the active project's configuration.
type ProjectConfigView struct {
	app   *tview.Application
	list  *tview.List
	shell ShellAccess
	cfg   ProjectConfigViewConfig

	live       *domain.Project
	dirty      bool
	mcpChanged bool // track if MCP fields changed (triggers redeploy prompt)
	lines      []projectConfigLine
}

var _ View = (*ProjectConfigView)(nil)
var _ CommandProvider = (*ProjectConfigView)(nil)

// NewProjectConfigView creates the project config view.
func NewProjectConfigView(cfg ProjectConfigViewConfig) *ProjectConfigView {
	return &ProjectConfigView{cfg: cfg}
}

func (v *ProjectConfigView) SetShell(s ShellAccess) { v.shell = s }

func (v *ProjectConfigView) ID() string    { return "project.config" }
func (v *ProjectConfigView) Title() string { return "Config Projet" }
func (v *ProjectConfigView) StatusHints() string {
	return "j/k nav · Space toggle · e éditer · w sauvegarder · Esc retour"
}

func (v *ProjectConfigView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.live = v.cfg.GetProject()
	v.dirty = false
	v.mcpChanged = false

	v.list = tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary)
	v.list.SetBackgroundColor(theme.BgPanel)
	v.list.SetBorderPadding(1, 0, 2, 2)

	if v.live == nil {
		v.list.AddItem("  Aucun projet actif — sélectionnez un projet via l'omnibar", "", 0, nil)
		content.AddItem(v.list, 0, 1, true)
		app.SetFocus(v.list)
		return
	}

	v.buildLines()
	v.renderLines()
	content.AddItem(v.list, 0, 1, true)
	app.SetFocus(v.list)
}

func (v *ProjectConfigView) Unmount() {
	if v.dirty && v.shell != nil {
		v.shell.ShowToastMsg("⚠ Modifications non sauvegardées — w pour sauvegarder", false)
	}
	v.app = nil
	v.list = nil
}

func (v *ProjectConfigView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.live == nil {
		return event
	}
	switch event.Key() {
	case tcell.KeyEnter:
		v.editSelected()
		return nil
	}
	switch event.Rune() {
	case 'e':
		v.editSelected()
		return nil
	case ' ':
		v.toggleSelected()
		return nil
	case 'w':
		v.save()
		return nil
	}
	return event
}

// ─────────────────────────────────────────────────────────────────────────────
// Line definitions
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectConfigView) buildLines() {
	v.lines = []projectConfigLine{
		// ── Général ──────────────────────────────────────────────────────────
		{kind: "section-header", section: "Général"},
		{section: "Général", key: "id", kind: "readonly",
			get: func(p *domain.Project) string { return p.ID }},
		{section: "Général", key: "name", kind: "string",
			get: func(p *domain.Project) string { return p.Name },
			set: func(p *domain.Project, v string) { p.Name = v }},
		{section: "Général", key: "path", kind: "string",
			get: func(p *domain.Project) string { return p.Path },
			set: func(p *domain.Project, v string) { p.Path = v }},
		{section: "Général", key: "language", kind: "string",
			get: func(p *domain.Project) string { return p.Language },
			set: func(p *domain.Project, v string) { p.Language = v }},
		{section: "Général", key: "provider", kind: "string",
			get: func(p *domain.Project) string { return p.Provider },
			set: func(p *domain.Project, v string) { p.Provider = v }},
		{section: "Général", key: "model", kind: "string",
			get: func(p *domain.Project) string { return p.Model },
			set: func(p *domain.Project, v string) { p.Model = v }},
		{section: "Général", key: "status", kind: "string",
			get: func(p *domain.Project) string { return string(p.Status) },
			set: func(p *domain.Project, v string) { p.Status = domain.ProjectStatus(v) }},

		// ── Team ─────────────────────────────────────────────────────────────
		{kind: "section-header", section: "Team"},
		{section: "Team", key: "mode", kind: "string",
			get: func(p *domain.Project) string {
				if p.TeamConfig == nil {
					return "(inherit)"
				}
				return p.TeamConfig.Mode
			},
			set: func(p *domain.Project, val string) {
				if p.TeamConfig == nil {
					p.TeamConfig = &domain.ProjectTeamConfig{}
				}
				p.TeamConfig.Mode = val
			}},
		{section: "Team", key: "member_id", kind: "string",
			get: func(p *domain.Project) string {
				if p.TeamConfig == nil {
					return ""
				}
				return p.TeamConfig.MemberID
			},
			set: func(p *domain.Project, val string) {
				if p.TeamConfig == nil {
					p.TeamConfig = &domain.ProjectTeamConfig{}
				}
				p.TeamConfig.MemberID = val
			}},
		{section: "Team", key: "state_repo", kind: "string",
			get: func(p *domain.Project) string {
				if p.TeamConfig == nil {
					return ""
				}
				return p.TeamConfig.StateRepo
			},
			set: func(p *domain.Project, val string) {
				if p.TeamConfig == nil {
					p.TeamConfig = &domain.ProjectTeamConfig{}
				}
				p.TeamConfig.StateRepo = val
			}},
		{section: "Team", key: "state_path", kind: "string",
			get: func(p *domain.Project) string {
				if p.TeamConfig == nil {
					return ""
				}
				return p.TeamConfig.StatePath
			},
			set: func(p *domain.Project, val string) {
				if p.TeamConfig == nil {
					p.TeamConfig = &domain.ProjectTeamConfig{}
				}
				p.TeamConfig.StatePath = val
			}},

		// ── MCP Overrides ────────────────────────────────────────────────────
		{kind: "section-header", section: "MCP Overrides"},
		{section: "MCP", key: "gitlab (enabled)", kind: "bool",
			get:  func(p *domain.Project) string { return mcpServiceEnabled(p, "gitlab") },
			set:  v.mcpSetEnabled("gitlab")},
		{section: "MCP", key: "gitlab (write_enabled)", kind: "bool",
			get:  func(p *domain.Project) string { return mcpServiceWriteEnabled(p, "gitlab") },
			set:  v.mcpSetWriteEnabled("gitlab")},
		{section: "MCP", key: "jira (enabled)", kind: "bool",
			get:  func(p *domain.Project) string { return mcpServiceEnabled(p, "jira") },
			set:  v.mcpSetEnabled("jira")},
		{section: "MCP", key: "figma (enabled)", kind: "bool",
			get:  func(p *domain.Project) string { return mcpServiceEnabled(p, "figma") },
			set:  v.mcpSetEnabled("figma")},
		{section: "MCP", key: "gslides (enabled)", kind: "bool",
			get:  func(p *domain.Project) string { return mcpServiceEnabled(p, "gslides") },
			set:  v.mcpSetEnabled("gslides")},
		{section: "MCP", key: "team (enabled)", kind: "bool",
			get:  func(p *domain.Project) string { return mcpServiceEnabled(p, "team") },
			set:  v.mcpSetEnabled("team")},

		// ── Agents ───────────────────────────────────────────────────────────
		{kind: "section-header", section: "Agents"},
		{section: "Agents", key: "agents", kind: "agents",
			get: func(p *domain.Project) string { return strings.Join(p.Agents, ", ") },
			set: func(p *domain.Project, val string) {
				if val == "" {
					p.Agents = nil
					return
				}
				parts := strings.Split(val, ",")
				p.Agents = p.Agents[:0]
				for _, a := range parts {
					if s := strings.TrimSpace(a); s != "" {
						p.Agents = append(p.Agents, s)
					}
				}
			}},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// MCP helpers (operate on ProjectMCPConfig)
// ─────────────────────────────────────────────────────────────────────────────

func mcpServiceEnabled(p *domain.Project, name string) string {
	if p.MCPConfig == nil {
		return "(inherit)"
	}
	for _, svc := range p.MCPConfig.Services {
		if svc.Name == name && svc.Enabled != nil {
			return boolStr(*svc.Enabled)
		}
	}
	return "(inherit)"
}

func mcpServiceWriteEnabled(p *domain.Project, name string) string {
	if p.MCPConfig == nil {
		return "(inherit)"
	}
	for _, svc := range p.MCPConfig.Services {
		if svc.Name == name && svc.WriteEnabled != nil {
			return boolStr(*svc.WriteEnabled)
		}
	}
	return "(inherit)"
}

func (v *ProjectConfigView) mcpSetEnabled(name string) func(p *domain.Project, val string) {
	return func(p *domain.Project, val string) {
		if p.MCPConfig == nil {
			p.MCPConfig = &domain.ProjectMCPConfig{}
		}
		for i, svc := range p.MCPConfig.Services {
			if svc.Name == name {
				if val == "(inherit)" || val == "" {
					p.MCPConfig.Services[i].Enabled = nil
				} else {
					b := val == "true"
					p.MCPConfig.Services[i].Enabled = &b
				}
				v.mcpChanged = true
				return
			}
		}
		// Service not in list yet — add it
		if val == "(inherit)" || val == "" {
			return
		}
		b := val == "true"
		p.MCPConfig.Services = append(p.MCPConfig.Services, domain.ProjectMCPService{
			Name:    name,
			Enabled: &b,
		})
		v.mcpChanged = true
	}
}

func (v *ProjectConfigView) mcpSetWriteEnabled(name string) func(p *domain.Project, val string) {
	return func(p *domain.Project, val string) {
		if p.MCPConfig == nil {
			p.MCPConfig = &domain.ProjectMCPConfig{}
		}
		for i, svc := range p.MCPConfig.Services {
			if svc.Name == name {
				if val == "(inherit)" || val == "" {
					p.MCPConfig.Services[i].WriteEnabled = nil
				} else {
					b := val == "true"
					p.MCPConfig.Services[i].WriteEnabled = &b
				}
				v.mcpChanged = true
				return
			}
		}
		if val == "(inherit)" || val == "" {
			return
		}
		b := val == "true"
		p.MCPConfig.Services = append(p.MCPConfig.Services, domain.ProjectMCPService{
			Name:         name,
			WriteEnabled: &b,
		})
		v.mcpChanged = true
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectConfigView) renderLines() {
	if v.list == nil || v.live == nil {
		return
	}
	savedIdx := v.list.GetCurrentItem()
	v.list.Clear()
	title := v.live.Name
	if v.dirty {
		title += "  " + theme.ColorTag(theme.AccentHex) + "● modifié" + theme.TagColor
	}
	v.list.AddItem(
		fmt.Sprintf("  [::b]Projet: %s%s", title, theme.TagReset),
		"", 0, nil)

	for _, line := range v.lines {
		if line.kind == "section-header" {
			v.list.AddItem(
				fmt.Sprintf("  %s─── %s ──────────────────%s", theme.ColorTag(theme.AccentHex), line.section, theme.TagColor),
				"", 0, nil)
			continue
		}

		val := ""
		if line.get != nil {
			val = line.get(v.live)
		}

		readonlyMarker := ""
		if line.kind == "readonly" {
			readonlyMarker = fmt.Sprintf("  %s(lecture seule)%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor)
		}

		valDisplay := formatProjectValue(val, line.kind)
		main := fmt.Sprintf("  %-28s %s%s", line.key+":", valDisplay, readonlyMarker)
		v.list.AddItem(main, "", 0, nil)
	}
	if savedIdx >= 0 && savedIdx < v.list.GetItemCount() {
		v.list.SetCurrentItem(savedIdx)
	}
}

func formatProjectValue(val, kind string) string {
	switch kind {
	case "bool":
		switch val {
		case "true":
			return fmt.Sprintf("%s✓ true%s", theme.ColorTag("#4CAF50"), theme.TagColor)
		case "false":
			return fmt.Sprintf("%s✗ false%s", theme.ColorTag("#FF5252"), theme.TagColor)
		default: // "(inherit)"
			return fmt.Sprintf("%s%s%s", theme.ColorTag(theme.TextMutedHex), val, theme.TagColor)
		}
	case "agents":
		if val == "" {
			return fmt.Sprintf("%s(aucun)%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor)
		}
		// Show count instead of full list
		agents := strings.Split(val, ",")
		return fmt.Sprintf("%d agents", len(agents))
	default:
		if val == "" || val == "(inherit)" {
			return fmt.Sprintf("%s%s%s", theme.ColorTag(theme.TextMutedHex), val, theme.TagColor)
		}
		return val
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Editing
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectConfigView) selectedLine() (projectConfigLine, bool) {
	if v.list == nil || v.live == nil || v.list.GetItemCount() == 0 {
		return projectConfigLine{}, false
	}
	// The list has an extra title item at index 0 that is not in v.lines.
	rawIdx := v.list.GetCurrentItem()
	idx := rawIdx - 1 // offset for the title item
	if idx < 0 || idx >= len(v.lines) {
		return projectConfigLine{}, false
	}
	line := v.lines[idx]
	if line.kind == "section-header" || line.kind == "readonly" || line.get == nil {
		return projectConfigLine{}, false
	}
	return line, true
}

func (v *ProjectConfigView) toggleSelected() {
	line, ok := v.selectedLine()
	if !ok || line.kind != "bool" {
		return
	}
	cur := line.get(v.live)
	var newVal string
	switch cur {
	case "true":
		newVal = "false"
	case "false":
		newVal = "(inherit)"
	default: // "(inherit)"
		newVal = "true"
	}
	line.set(v.live, newVal)
	v.dirty = true
	v.renderLines()
}

func (v *ProjectConfigView) editSelected() {
	line, ok := v.selectedLine()
	if !ok || v.shell == nil {
		return
	}

	switch line.kind {
	case "bool":
		v.toggleSelected()

	case "agents":
		v.editAgents(line)

	default: // string
		cur := line.get(v.live)
		v.shell.ShowInputModal(line.key, cur, func(newVal string) {
			if line.set != nil {
				line.set(v.live, newVal)
				v.dirty = true
				v.renderLines()
			}
		})
	}
}

func (v *ProjectConfigView) editAgents(line projectConfigLine) {
	if v.shell == nil {
		return
	}

	allAgents := v.cfg.AllAgents()
	if len(allAgents) == 0 {
		v.shell.ShowToastMsg("Aucun agent disponible", false)
		return
	}
	sort.Strings(allAgents)

	// Build multi-select options with current selection
	currentSet := make(map[string]bool)
	for _, a := range v.live.Agents {
		currentSet[a] = true
	}

	opts := make([]SelectOption, len(allAgents))
	for i, a := range allAgents {
		label := a
		if currentSet[a] {
			label = "✓ " + a
		} else {
			label = "  " + a
		}
		opts[i] = SelectOption{Label: label, Value: a}
	}

	v.shell.ShowSelectModal("Agents actifs (sélectionner pour toggle)", opts, "", func(chosen string) {
		if chosen == "" {
			return
		}
		// Toggle the chosen agent
		if currentSet[chosen] {
			// Remove
			newAgents := make([]string, 0, len(v.live.Agents))
			for _, a := range v.live.Agents {
				if a != chosen {
					newAgents = append(newAgents, a)
				}
			}
			v.live.Agents = newAgents
		} else {
			// Add
			v.live.Agents = append(v.live.Agents, chosen)
			sort.Strings(v.live.Agents)
		}
		v.dirty = true
		v.renderLines()
		// Re-open the modal to allow toggling multiple agents
		v.editAgents(line)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Save
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectConfigView) save() {
	if v.live == nil || v.shell == nil {
		return
	}
	ctx := context.Background()
	if err := v.cfg.SaveProject(ctx, v.live); err != nil {
		v.shell.ShowToastMsg("Erreur de sauvegarde: "+err.Error(), false)
		return
	}
	v.dirty = false
	v.shell.ShowToastMsg("Projet sauvegardé", true)

	// Propose redeploy if MCP changed
	if v.mcpChanged && v.cfg.Deploy != nil {
		v.shell.ShowSelectModal(
			"Configuration MCP modifiée",
			[]SelectOption{
				{Label: "Redéployer maintenant (oh deploy)", Value: "deploy"},
				{Label: "Plus tard", Value: "later"},
			},
			"",
			func(choice string) {
				if choice == "deploy" {
					go func() {
						if err := v.cfg.Deploy(ctx, v.live); err != nil {
							if v.app != nil {
								v.app.QueueUpdateDraw(func() {
									v.shell.ShowToastMsg("Deploy échoué: "+err.Error(), false)
								})
							}
							return
						}
						if v.app != nil {
							v.app.QueueUpdateDraw(func() {
								v.shell.ShowToastMsg("Projet redéployé", true)
							})
						}
					}()
				} else {
					v.shell.ShowToastMsg("Pensez à 'oh deploy' pour appliquer les changements MCP", true)
				}
				v.mcpChanged = false
			})
	}
}

// ContextCommands implements CommandProvider.
func (v *ProjectConfigView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{ID: "project.save", Label: "Sauvegarder", Aliases: []string{"save", "write"}, Description: "Sauvegarder les modifications projet", Category: "Projet", Action: func() { v.save() }},
	}
}
