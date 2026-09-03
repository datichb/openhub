package views

import (
	"context"
	"fmt"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/viper"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// modelEntry represents a single row in the models view.
type modelEntry struct {
	Level string // "hub" or project name
	Scope string // "default", "family:<name>", "agent:<id>"
	Key   string // display key (e.g., "planning", "dev-senior")
	Value string // model name
}

// ModelsView displays and allows editing of the model configuration cascade.
type ModelsView struct {
	app      *tview.Application
	appCtx   *app.App
	table    *tview.Table
	entries  []modelEntry
	shell    ShellAccess
	commands []ContextCommand
	mountGen uint64
}

var _ View = (*ModelsView)(nil)
var _ CommandProvider = (*ModelsView)(nil)

// NewModelsView creates a new model configuration view.
func NewModelsView(a *app.App) *ModelsView {
	return &ModelsView{appCtx: a}
}

// SetShell provides the shell reference for modal interactions.
func (v *ModelsView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *ModelsView) ID() string { return "models" }

// Title returns the display title.
func (v *ModelsView) Title() string { return "Models" }

// StatusHints returns keybinding hints.
func (v *ModelsView) StatusHints() string {
	return fmt.Sprintf("j/k nav · Enter %s · a %s · d %s · Ctrl+P %s",
		i18n.T("tui.hints.edit"),
		i18n.T("tui.hints.add"),
		i18n.T("tui.hints.delete"),
		i18n.T("tui.hints.commands"),
	)
}

// Mount builds the models cascade table.
func (v *ModelsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.mountGen++
	gen := v.mountGen

	// Show loading placeholder immediately
	loading := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	loading.SetBackgroundColor(theme.BgPanel)
	muted := theme.ColorTag(theme.TextMutedHex)
	loading.SetText(fmt.Sprintf("\n  %sChargement des modèles...%s", muted, theme.TagColor))
	content.AddItem(loading, 0, 1, true)

	// Load entries and populate table asynchronously
	go func() {
		app.QueueUpdateDraw(func() {
			if v.app == nil || v.mountGen != gen {
				return // view was unmounted or re-mounted before the goroutine finished
			}
			v.loadEntries()

			v.table = tview.NewTable().
				SetSelectable(true, false).
				SetFixed(1, 0)
			v.table.SetBackgroundColor(theme.BgPanel)
			v.table.SetBorderPadding(1, 0, 2, 2)
			v.table.SetSelectedStyle(tcell.StyleDefault.
				Background(theme.BgElement).
				Foreground(theme.FgPrimary))

			// Handle Enter to edit
			v.table.SetSelectedFunc(func(row, col int) {
				if row == 0 || row-1 >= len(v.entries) {
					return
				}
				v.editEntry(row - 1)
			})

			v.populateTable()
			v.buildCommands()

			content.RemoveItem(loading)
			content.AddItem(v.table, 0, 1, true)
			app.SetFocus(v.table)
		})
	}()
}

// Unmount cleans up resources.
func (v *ModelsView) Unmount() {
	v.app = nil
	v.table = nil
}

// HandleKey processes models view key events.
func (v *ModelsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'a':
		v.addOverride()
		return nil
	case 'd':
		v.deleteEntry()
		return nil
	}
	return event
}

// Model family names (known set).
var modelFamilies = []string{"planning", "developer", "quality", "auditor", "design", "documentation"}

func (v *ModelsView) loadEntries() {
	v.entries = nil
	vip := modelsConfigViper()

	// Hub default
	hubDefault := vip.GetString("models.default")
	if hubDefault != "" {
		v.entries = append(v.entries, modelEntry{Level: "hub", Scope: "default", Key: "défaut", Value: hubDefault})
	} else {
		v.entries = append(v.entries, modelEntry{Level: "hub", Scope: "default", Key: "défaut", Value: "(non défini)"})
	}

	// Hub families
	hubFamilies := vip.GetStringMapString("models.families")
	familyKeys := make([]string, 0, len(hubFamilies))
	for k := range hubFamilies {
		familyKeys = append(familyKeys, k)
	}
	sort.Strings(familyKeys)
	for _, k := range familyKeys {
		v.entries = append(v.entries, modelEntry{Level: "hub", Scope: "family:" + k, Key: k, Value: hubFamilies[k]})
	}

	// Hub agents
	hubAgents := vip.GetStringMapString("models.agents")
	agentKeys := make([]string, 0, len(hubAgents))
	for k := range hubAgents {
		agentKeys = append(agentKeys, k)
	}
	sort.Strings(agentKeys)
	for _, k := range agentKeys {
		v.entries = append(v.entries, modelEntry{Level: "hub", Scope: "agent:" + k, Key: k, Value: hubAgents[k]})
	}

	// Project-level overrides (first project for simplicity)
	if v.appCtx != nil && v.appCtx.Projects != nil {
		projects, _ := v.appCtx.Projects.List(context.Background(), "")
		for _, p := range projects {
			prefix := p.Name
			if p.Model != "" {
				v.entries = append(v.entries, modelEntry{Level: prefix, Scope: "default", Key: "défaut", Value: p.Model})
			}
			if p.ModelOverrides != nil {
				fKeys := make([]string, 0, len(p.ModelOverrides.Families))
				for k := range p.ModelOverrides.Families {
					fKeys = append(fKeys, k)
				}
				sort.Strings(fKeys)
				for _, k := range fKeys {
					v.entries = append(v.entries, modelEntry{Level: prefix, Scope: "family:" + k, Key: k, Value: p.ModelOverrides.Families[k]})
				}
				aKeys := make([]string, 0, len(p.ModelOverrides.Agents))
				for k := range p.ModelOverrides.Agents {
					aKeys = append(aKeys, k)
				}
				sort.Strings(aKeys)
				for _, k := range aKeys {
					v.entries = append(v.entries, modelEntry{Level: prefix, Scope: "agent:" + k, Key: k, Value: p.ModelOverrides.Agents[k]})
				}
			}
		}
	}
}

func (v *ModelsView) populateTable() {
	v.table.Clear()

	// Header
	headerStyle := tcell.StyleDefault.Foreground(theme.Accent).Bold(true)
	v.table.SetCell(0, 0, tview.NewTableCell("  Niveau").SetStyle(headerStyle).SetSelectable(false))
	v.table.SetCell(0, 1, tview.NewTableCell("Type").SetStyle(headerStyle).SetSelectable(false))
	v.table.SetCell(0, 2, tview.NewTableCell("Clé").SetStyle(headerStyle).SetSelectable(false))
	v.table.SetCell(0, 3, tview.NewTableCell("Modèle").SetStyle(headerStyle).SetSelectable(false))

	for i, e := range v.entries {
		levelColor := theme.FgSecondary
		if e.Level != "hub" {
			levelColor = theme.Accent
		}

		scopeDisplay := "default"
		if len(e.Scope) > 7 && e.Scope[:7] == "family:" {
			scopeDisplay = "family"
		} else if len(e.Scope) > 6 && e.Scope[:6] == "agent:" {
			scopeDisplay = "agent"
		}

		v.table.SetCell(i+1, 0, tview.NewTableCell("  "+e.Level).SetTextColor(levelColor).SetExpansion(1))
		v.table.SetCell(i+1, 1, tview.NewTableCell(scopeDisplay).SetTextColor(theme.FgMuted).SetExpansion(1))
		v.table.SetCell(i+1, 2, tview.NewTableCell(e.Key).SetTextColor(theme.FgPrimary).SetExpansion(1))
		v.table.SetCell(i+1, 3, tview.NewTableCell(e.Value).SetTextColor(theme.FgPrimary).SetExpansion(2))
	}
}

func (v *ModelsView) editEntry(idx int) {
	if v.shell == nil || idx < 0 || idx >= len(v.entries) {
		return
	}
	entry := v.entries[idx]

	v.shell.ShowInputModal("Modèle pour "+entry.Key, entry.Value, func(newModel string) {
		if newModel == "" || newModel == entry.Value {
			return
		}

		if entry.Level == "hub" {
			v.setHubModel(entry.Scope, newModel)
		} else {
			v.setProjectModel(entry.Level, entry.Scope, newModel)
		}

		v.loadEntries()
		v.populateTable()
		if v.shell != nil {
			v.shell.ShowToastMsg("Modèle mis à jour", true)
		}
	})
}

func (v *ModelsView) addOverride() {
	if v.shell == nil {
		return
	}

	scopeOptions := []SelectOption{
		{Label: "Famille", Value: "family"},
		{Label: "Agent", Value: "agent"},
	}

	familyOptions := make([]SelectOption, len(modelFamilies))
	for i, f := range modelFamilies {
		familyOptions[i] = SelectOption{Label: f, Value: f}
	}

	v.shell.ShowInlineForm(InlineFormConfig{
		Title: "Ajouter un override de modèle",
		Fields: []FormField{
			{Key: "scope", Label: "Type", Type: FieldSelect, Options: scopeOptions, Default: "family", Required: true},
			{Key: "family", Label: "Famille", Type: FieldSelect, Options: familyOptions, Default: modelFamilies[0],
				Conditional: func(vals map[string]string) bool { return vals["scope"] == "family" }},
			{Key: "agent_id", Label: "Agent ID", Type: FieldText,
				Conditional: func(vals map[string]string) bool { return vals["scope"] == "agent" }},
			{Key: "model", Label: "Modèle", Type: FieldText, Required: true},
		},
		OnSubmit: func(values map[string]string, _ map[string][]string) {
			scope := values["scope"]
			model := values["model"]
			if model == "" {
				return
			}
			switch scope {
			case "family":
				if family := values["family"]; family != "" {
					v.setHubModel("family:"+family, model)
				}
			case "agent":
				if agentID := values["agent_id"]; agentID != "" {
					v.setHubModel("agent:"+agentID, model)
				}
			}
			v.loadEntries()
			v.buildCommands()
			v.populateTable()
			v.shell.ShowToastMsg("Override ajouté", true)
		},
		OnCancel: nil,
	})
}

func (v *ModelsView) deleteEntry() {
	if v.table == nil {
		return
	}
	row, _ := v.table.GetSelection()
	idx := row - 1
	if idx < 0 || idx >= len(v.entries) {
		return
	}
	entry := v.entries[idx]

	if v.shell == nil {
		return
	}
	label := fmt.Sprintf("Supprimer l'override %s (%s) ?", entry.Scope, entry.Level)
	v.shell.ShowSelectModal(label, []SelectOption{
		{Label: "Annuler", Value: ""},
		{Label: "Confirmer la suppression", Value: "yes"},
	}, "", func(choice string) {
		if choice != "yes" {
			return
		}
		if entry.Level == "hub" {
			vip := modelsConfigViper()
			switch {
			case entry.Scope == "default":
				vip.Set("models.default", "")
			case len(entry.Scope) > 7 && entry.Scope[:7] == "family:":
				families := vip.GetStringMapString("models.families")
				delete(families, entry.Scope[7:])
				vip.Set("models.families", families)
			case len(entry.Scope) > 6 && entry.Scope[:6] == "agent:":
				agents := vip.GetStringMapString("models.agents")
				delete(agents, entry.Scope[6:])
				vip.Set("models.agents", agents)
			}
			_ = vip.WriteConfigAs(config.ConfigPath())
		} else {
			v.deleteProjectModel(entry.Level, entry.Scope)
		}

		v.loadEntries()
		v.populateTable()
		v.shell.ShowToastMsg("Override supprimé", true)
	})
}

func (v *ModelsView) setHubModel(scope, model string) {
	vip := modelsConfigViper()
	switch {
	case scope == "default":
		vip.Set("models.default", model)
	case len(scope) > 7 && scope[:7] == "family:":
		key := "models.families." + scope[7:]
		vip.Set(key, model)
	case len(scope) > 6 && scope[:6] == "agent:":
		key := "models.agents." + scope[6:]
		vip.Set(key, model)
	}
	_ = vip.WriteConfigAs(config.ConfigPath())
	if v.shell != nil {
		v.shell.ShowToastMsg("Override sauvegardé", true)
	}
}

func (v *ModelsView) setProjectModel(projectName, scope, model string) {
	if v.appCtx == nil || v.appCtx.Projects == nil {
		return
	}
	ctx := context.Background()
	project, err := v.appCtx.Projects.GetByName(ctx, projectName)
	if err != nil {
		return
	}

	switch {
	case scope == "default":
		project.Model = model
	case len(scope) > 7 && scope[:7] == "family:":
		if project.ModelOverrides == nil {
			project.ModelOverrides = &domain.ProjectModelOverrides{}
		}
		if project.ModelOverrides.Families == nil {
			project.ModelOverrides.Families = make(map[string]string)
		}
		project.ModelOverrides.Families[scope[7:]] = model
	case len(scope) > 6 && scope[:6] == "agent:":
		if project.ModelOverrides == nil {
			project.ModelOverrides = &domain.ProjectModelOverrides{}
		}
		if project.ModelOverrides.Agents == nil {
			project.ModelOverrides.Agents = make(map[string]string)
		}
		project.ModelOverrides.Agents[scope[6:]] = model
	}
	_ = v.appCtx.Projects.Update(ctx, project)
}

func (v *ModelsView) deleteProjectModel(projectName, scope string) {
	if v.appCtx == nil || v.appCtx.Projects == nil {
		return
	}
	ctx := context.Background()
	project, err := v.appCtx.Projects.GetByName(ctx, projectName)
	if err != nil || project.ModelOverrides == nil {
		return
	}

	switch {
	case scope == "default":
		project.Model = ""
	case len(scope) > 7 && scope[:7] == "family:":
		delete(project.ModelOverrides.Families, scope[7:])
	case len(scope) > 6 && scope[:6] == "agent:":
		delete(project.ModelOverrides.Agents, scope[6:])
	}
	_ = v.appCtx.Projects.Update(ctx, project)
}

func modelsConfigViper() *viper.Viper {
	return hubViper()
}

// ContextCommands returns contextual commands for the omnibar.
func (v *ModelsView) ContextCommands() []ContextCommand {
	return v.commands
}

func (v *ModelsView) buildCommands() {
	v.commands = []ContextCommand{
		{
			ID:          "models.add.family",
			Label:       "add family",
			Aliases:     []string{"ajouter famille", "override family"},
			Description: "Ajouter un override de famille",
			Category:    "Models",
			Action:      v.addOverride,
		},
		{
			ID:          "models.delete",
			Label:       "delete",
			Aliases:     []string{"supprimer", "remove"},
			Description: "Supprimer l'override sélectionné",
			Category:    "Models",
			Action:      v.deleteEntry,
		},
	}

	// Add edit commands for existing entries
	for _, e := range v.entries {
		e := e
		v.commands = append(v.commands, ContextCommand{
			ID:          "models.edit." + e.Key,
			Label:       e.Key,
			Aliases:     []string{e.Value, e.Scope},
			Description: e.Level + " → " + e.Value,
			Category:    "Models",
			Action: func() {
				for i, entry := range v.entries {
					if entry.Key == e.Key && entry.Level == e.Level && entry.Scope == e.Scope {
						v.editEntry(i)
						return
					}
				}
			},
		})
	}
}
