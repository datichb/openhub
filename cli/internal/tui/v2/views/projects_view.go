package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ProjectItem represents a project entry in the list.
type ProjectItem struct {
	ID           string
	Name         string
	Path         string
	Language     string
	Provider     string
	Model        string
	Agents       []string
	Status       string
	MCPOverrides map[string]string // service → "inherit"|"enabled"|"disabled"
}

// ProjectConfigUpdate holds updated configuration values from the TUI configure flow.
type ProjectConfigUpdate struct {
	Language     string
	Provider     string
	Model        string
	Agents       []string
	MCPOverrides map[string]string // service → "inherit"|"enabled"|"disabled"
}

// ProjectsViewConfig configures the projects list view.
type ProjectsViewConfig struct {
	Projects         []ProjectItem
	AvailableAgents  []string // agents discovered from hub/agents/*.md
	KnownMCPServices []string // MCP service names known by hub (e.g. figma, gitlab, gslides)
}

// ProjectsView displays the list of registered projects with full CRUD.
type ProjectsView struct {
	cfg              ProjectsViewConfig
	content          *tview.Flex
	list             *tview.List
	detail           *tview.TextView
	app              *tview.Application
	shell            ShellAccess
	mountGen         uint64
	onAdd            func(name, path string)
	onRemove         func(id string)
	onConfigure      func(id string, cfg ProjectConfigUpdate)
	onRename         func(id, newName string)
	onMove           func(id, newPath string)
	onEnterProject   func(project *ActiveProject)
	onInitBeads      func(id, name, path string)
}

var _ View = (*ProjectsView)(nil)

// NewProjectsView creates a new projects list view.
func NewProjectsView(cfg ProjectsViewConfig) *ProjectsView {
	return &ProjectsView{cfg: cfg}
}

// SetShell provides the shell reference for modal interactions.
func (v *ProjectsView) SetShell(s ShellAccess) { v.shell = s }

// SetOnAdd sets the callback for adding a project.
func (v *ProjectsView) SetOnAdd(fn func(name, path string)) { v.onAdd = fn }

// SetOnRemove sets the callback for removing a project.
func (v *ProjectsView) SetOnRemove(fn func(id string)) { v.onRemove = fn }

// SetOnConfigure sets the callback for updating project configuration.
func (v *ProjectsView) SetOnConfigure(fn func(id string, cfg ProjectConfigUpdate)) {
	v.onConfigure = fn
}

// SetOnRename sets the callback for renaming a project.
func (v *ProjectsView) SetOnRename(fn func(id, newName string)) { v.onRename = fn }

// SetOnMove sets the callback for moving a project path.
func (v *ProjectsView) SetOnMove(fn func(id, newPath string)) { v.onMove = fn }

// SetOnEnterProject sets the callback triggered when the user enters project mode.
func (v *ProjectsView) SetOnEnterProject(fn func(project *ActiveProject)) {
	v.onEnterProject = fn
}

// SetOnInitBeads sets the callback triggered when the user requests beads init for a project.
func (v *ProjectsView) SetOnInitBeads(fn func(id, name, path string)) {
	v.onInitBeads = fn
}

// SetAvailableAgents sets the list of discovered agents for configuration.
func (v *ProjectsView) SetAvailableAgents(agents []string) {
	v.cfg.AvailableAgents = agents
}

// ID returns the view identifier.
func (v *ProjectsView) ID() string { return "projects.list" }

// Title returns the display title.
func (v *ProjectsView) Title() string { return "Projets" }

// StatusHints returns keybinding hints.
func (v *ProjectsView) StatusHints() string {
	return fmt.Sprintf("j/k %s · p %s · b %s · c %s · r %s · m %s · a %s · d %s",
		i18n.T("tui.hints.navigate"),
		i18n.T("tui.hints.project_mode"),
		i18n.T("tui.hints.init_board"),
		i18n.T("tui.hints.configure"),
		i18n.T("tui.hints.rename"),
		i18n.T("tui.hints.move"),
		i18n.T("tui.hints.add"),
		i18n.T("tui.hints.delete"),
	)
}

// Mount builds the projects list with footer detail.
func (v *ProjectsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.content = content
	v.mountGen++
	gen := v.mountGen

	// Show loading placeholder immediately
	loading := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	loading.SetBackgroundColor(theme.BgPanel)
	muted := theme.ColorTag(theme.TextMutedHex)
	loading.SetText(fmt.Sprintf("\n  %sChargement des projets...%s", muted, theme.TagColor))
	content.AddItem(loading, 0, 1, true)

	// Build list asynchronously
	go func() {
		// Capture data needed (cfg.Projects is a slice — safe to read)
		projects := v.cfg.Projects

		app.QueueUpdateDraw(func() {
			if v.app == nil || v.mountGen != gen {
				return // view was unmounted or re-mounted before the goroutine finished
			}

			v.list = tview.NewList().
				ShowSecondaryText(true).
				SetHighlightFullLine(true).
				SetMainTextColor(theme.FgPrimary).
				SetSecondaryTextColor(theme.FgSecondary)
			v.list.SetBackgroundColor(theme.BgPanel)
			v.list.SetBorderPadding(1, 0, 2, 2)

			v.detail = tview.NewTextView().
				SetDynamicColors(true).
				SetTextAlign(tview.AlignLeft)
			v.detail.SetBackgroundColor(theme.BgPanel)
			v.detail.SetBorderPadding(0, 0, 2, 2)

			for _, p := range projects {
				v.list.AddItem(p.Name, "    "+p.Path, 0, nil)
			}

			if len(projects) == 0 {
				muted := theme.ColorTag(theme.TextMutedHex)
				v.list.AddItem(fmt.Sprintf("%sAucun projet configuré. Appuyez sur 'a' pour ajouter un projet.%s", muted, theme.TagColor), "", 0, nil)
			}

			v.list.SetChangedFunc(func(index int, _ string, _ string, _ rune) {
				if index >= 0 && index < len(v.cfg.Projects) {
					v.showDetail(v.cfg.Projects[index])
				}
			})

			if len(projects) > 0 {
				v.showDetail(projects[0])
			}

			// Replace loading with real layout
			content.RemoveItem(loading)
			content.AddItem(v.list, 0, 3, true)
			content.AddItem(v.detail, 6, 0, false)
			app.SetFocus(v.list)
		})
	}()
}

// Unmount cleans up resources.
func (v *ProjectsView) Unmount() {
	v.app = nil
	v.list = nil
	v.detail = nil
}

// HandleKey processes view-specific key events.
func (v *ProjectsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'a':
		v.addProject()
		return nil
	case 'd':
		v.removeProject()
		return nil
	case 'c':
		v.configureProject()
		return nil
	case 'r':
		v.renameProject()
		return nil
	case 'm':
		v.moveProject()
		return nil
	case 'p':
		v.enterProjectMode()
		return nil
	case 'b':
		v.initBeads()
		return nil
	}
	if event.Key() == tcell.KeyEnter {
		v.configureProject()
		return nil
	}
	return event
}

// ─────────────────────────────────────────────────────────────────────────────
// Add / Remove
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectsView) addProject() {
	if v.shell == nil {
		return
	}

	v.shell.ShowInlineForm(InlineFormConfig{
		Title: "Ajouter un projet",
		Fields: []FormField{
			{Key: "name", Label: "Nom", Type: FieldText, Required: true},
			{Key: "path", Label: "Chemin", Type: FieldText, Required: true},
		},
		OnSubmit: func(values map[string]string, _ map[string][]string) {
			name := values["name"]
			path := values["path"]
			if name == "" || path == "" {
				return
			}
			if v.onAdd != nil {
				v.onAdd(name, path)
			}
			v.cfg.Projects = append(v.cfg.Projects, ProjectItem{Name: name, Path: path})
			v.list.AddItem(name, "    "+path, 0, nil)
			v.shell.ShowToastMsg("Projet ajouté: "+name, true)
		},
		OnCancel: nil,
	})
}

func (v *ProjectsView) removeProject() {
	if v.list == nil {
		return
	}
	idx := v.list.GetCurrentItem()
	if idx < 0 || idx >= len(v.cfg.Projects) {
		return
	}
	project := v.cfg.Projects[idx]
	if v.shell == nil {
		return
	}
	v.shell.ShowSelectModal(fmt.Sprintf("Supprimer le projet %q ?", project.Name), []SelectOption{
		{Label: "Annuler", Value: ""},
		{Label: "Confirmer la suppression", Value: "yes"},
	}, "", func(choice string) {
		if choice != "yes" {
			return
		}
		if v.onRemove != nil {
			v.onRemove(project.ID)
		}
		// Remove from local list
		v.cfg.Projects = append(v.cfg.Projects[:idx], v.cfg.Projects[idx+1:]...)
		v.list.RemoveItem(idx)
		v.shell.ShowToastMsg("Projet supprimé: "+project.Name, true)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Configure (language → provider → model → agents)
// ─────────────────────────────────────────────────────────────────────────────

// Language options for project configuration.
var projectLanguageOptions = []SelectOption{
	{Label: "Go", Value: "go"},
	{Label: "TypeScript", Value: "typescript"},
	{Label: "Python", Value: "python"},
	{Label: "Rust", Value: "rust"},
	{Label: "Java", Value: "java"},
	{Label: "Autre", Value: "other"},
}

// Provider options for project configuration.
var projectProviderOptions = []SelectOption{
	{Label: "Amazon Bedrock", Value: "bedrock"},
	{Label: "Anthropic", Value: "anthropic"},
	{Label: "OpenRouter", Value: "openrouter"},
	{Label: "GitHub Copilot", Value: "github-copilot"},
	{Label: "Hub (défaut)", Value: ""},
}

func (v *ProjectsView) configureProject() {
	if v.shell == nil || v.list == nil {
		return
	}
	idx := v.list.GetCurrentItem()
	if idx < 0 || idx >= len(v.cfg.Projects) {
		return
	}
	project := v.cfg.Projects[idx]

	// Build agent options
	agentOptions := make([]SelectOption, len(v.cfg.AvailableAgents))
	for i, ag := range v.cfg.AvailableAgents {
		agentOptions[i] = SelectOption{Label: ag, Value: ag}
	}

	fields := []FormField{
		{Key: "language", Label: "Langage", Type: FieldSelect, Options: projectLanguageOptions, Default: project.Language},
		{Key: "provider", Label: "Provider", Type: FieldSelect, Options: projectProviderOptions, Default: project.Provider},
		{Key: "model", Label: "Modèle", Type: FieldText, Default: project.Model},
	}
	if len(agentOptions) > 0 {
		fields = append(fields, FormField{
			Key: "agents", Label: "Agents", Type: FieldMultiSelect,
			Options: agentOptions, DefaultMulti: project.Agents,
		})
	}

	// MCP per-project overrides (tri-state: inherit / enabled / disabled)
	mcpStateOptions := []SelectOption{
		{Label: "Hérite hub", Value: "inherit"},
		{Label: "Activer", Value: "enabled"},
		{Label: "Désactiver", Value: "disabled"},
	}
	for _, svc := range v.cfg.KnownMCPServices {
		current := "inherit"
		if project.MCPOverrides != nil {
			if val, ok := project.MCPOverrides[svc]; ok {
				current = val
			}
		}
		fields = append(fields, FormField{
			Key:     "mcp." + svc,
			Label:   "MCP " + svc,
			Type:    FieldSelect,
			Options: mcpStateOptions,
			Default: current,
		})
	}

	v.shell.ShowInlineForm(InlineFormConfig{
		Title:  "Configurer: " + project.Name,
		Fields: fields,
		OnSubmit: func(values map[string]string, multi map[string][]string) {
			mcpOverrides := make(map[string]string)
			for _, svc := range v.cfg.KnownMCPServices {
				if val, ok := values["mcp."+svc]; ok {
					mcpOverrides[svc] = val
				}
			}
			update := ProjectConfigUpdate{
				Language:     values["language"],
				Provider:     values["provider"],
				Model:        values["model"],
				Agents:       multi["agents"],
				MCPOverrides: mcpOverrides,
			}
			v.applyConfiguration(idx, update)
		},
		OnCancel: nil,
	})
}

func (v *ProjectsView) applyConfiguration(idx int, update ProjectConfigUpdate) {
	if idx < 0 || idx >= len(v.cfg.Projects) {
		return
	}
	project := &v.cfg.Projects[idx]

	// Update local state
	project.Language = update.Language
	project.Provider = update.Provider
	project.Model = update.Model
	project.Agents = update.Agents
	project.MCPOverrides = update.MCPOverrides

	// Persist via callback
	if v.onConfigure != nil {
		v.onConfigure(project.ID, update)
	}

	// Refresh display
	v.showDetail(*project)
	if v.shell != nil {
		v.shell.ShowToastMsg("Configuration mise à jour: "+project.Name, true)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Rename
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectsView) renameProject() {
	if v.shell == nil || v.list == nil {
		return
	}
	idx := v.list.GetCurrentItem()
	if idx < 0 || idx >= len(v.cfg.Projects) {
		return
	}
	project := &v.cfg.Projects[idx]

	v.shell.ShowInputModal("Nouveau nom", project.Name, func(newName string) {
		if newName == "" || newName == project.Name {
			return
		}
		oldName := project.Name
		project.Name = newName

		// Update list display
		v.list.SetItemText(idx, newName, "    "+project.Path)

		// Persist via callback
		if v.onRename != nil {
			v.onRename(project.ID, newName)
		}

		v.showDetail(*project)
		v.shell.ShowToastMsg(fmt.Sprintf("Renommé: %s → %s", oldName, newName), true)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Move
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectsView) moveProject() {
	if v.shell == nil || v.list == nil {
		return
	}
	idx := v.list.GetCurrentItem()
	if idx < 0 || idx >= len(v.cfg.Projects) {
		return
	}
	project := &v.cfg.Projects[idx]

	v.shell.ShowInputModal("Nouveau chemin", project.Path, func(newPath string) {
		if newPath == "" || newPath == project.Path {
			return
		}
		project.Path = newPath

		// Update list display
		v.list.SetItemText(idx, project.Name, "    "+newPath)

		// Persist via callback
		if v.onMove != nil {
			v.onMove(project.ID, newPath)
		}

		v.showDetail(*project)
		v.shell.ShowToastMsg("Chemin mis à jour: "+project.Name, true)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Detail footer
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectsView) showDetail(p ProjectItem) {
	if v.detail == nil {
		return
	}

	sep := theme.ColorTag(theme.TextMutedHex) + "─────────────────────────────────────────────────────────────────" + theme.TagColor

	// Line 1: ID and path
	line1 := fmt.Sprintf("  %sID:%s %s  %s·%s  %sChemin:%s %s",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, p.ID,
		theme.ColorTag(theme.TextMutedHex), theme.TagColor,
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, p.Path,
	)

	// Line 2: Language, Provider, Model
	lang := displayOrPlaceholder(p.Language, "-")
	prov := displayOrPlaceholder(p.Provider, "hub default")
	model := displayOrPlaceholder(p.Model, "hub default")
	line2 := fmt.Sprintf("  %sLangage:%s %s  %s·%s  %sProvider:%s %s  %s·%s  %sModèle:%s %s",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, lang,
		theme.ColorTag(theme.TextMutedHex), theme.TagColor,
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, prov,
		theme.ColorTag(theme.TextMutedHex), theme.TagColor,
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, model,
	)

	// Line 3: Agents
	var agentDisplay string
	if len(p.Agents) == 0 {
		agentDisplay = "-"
	} else if len(p.Agents) <= 5 {
		agentDisplay = strings.Join(p.Agents, ", ")
	} else {
		agentDisplay = fmt.Sprintf("%s (+%d)", strings.Join(p.Agents[:5], ", "), len(p.Agents)-5)
	}
	line3 := fmt.Sprintf("  %sAgents:%s %s (%d)",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, agentDisplay, len(p.Agents),
	)

	v.detail.SetText(fmt.Sprintf("%s\n%s\n%s\n%s", sep, line1, line2, line3))
}

// displayOrPlaceholder returns val if non-empty, otherwise the placeholder.
func displayOrPlaceholder(val, placeholder string) string {
	if val == "" {
		return placeholder
	}
	return val
}

// enterProjectMode activates project mode for the currently selected project.
func (v *ProjectsView) enterProjectMode() {
	if v.list == nil {
		return
	}
	idx := v.list.GetCurrentItem()
	if idx < 0 || idx >= len(v.cfg.Projects) {
		return
	}
	p := v.cfg.Projects[idx]
	if v.onEnterProject != nil {
		v.onEnterProject(&ActiveProject{
			ID:   p.ID,
			Name: p.Name,
			Path: p.Path,
		})
	}
}

// initBeads triggers beads initialization for the currently selected project.
func (v *ProjectsView) initBeads() {
	if v.list == nil || v.shell == nil {
		return
	}
	idx := v.list.GetCurrentItem()
	if idx < 0 || idx >= len(v.cfg.Projects) {
		return
	}
	p := v.cfg.Projects[idx]
	if v.onInitBeads != nil {
		v.onInitBeads(p.ID, p.Name, p.Path)
	}
}
