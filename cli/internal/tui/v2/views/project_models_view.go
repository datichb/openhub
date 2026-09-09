package views

import (
	"context"
	"fmt"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ProjectModelsViewConfig holds external dependencies for the project models view.
type ProjectModelsViewConfig struct {
	// GetProject returns the currently active project (copy).
	GetProject func() *domain.Project
	// SaveProject persists the modified project to the DB.
	SaveProject func(ctx context.Context, p *domain.Project) error
}

// projectModelsRef identifies a dynamic map entry in the models view.
type projectModelsRef struct {
	section string // "general", "families", "agents"
	key     string // map key (empty for the default model field)
}

// ProjectModelsView displays and edits per-project model overrides.
type ProjectModelsView struct {
	app      *tview.Application
	list     *widgets.SectionedList
	shell    ShellAccess
	cfg      ProjectModelsViewConfig
	mountGen uint64

	live      *domain.Project
	dirty     bool
	undoStack *widgets.UndoStack[domain.Project]
}

var _ View = (*ProjectModelsView)(nil)
var _ CommandProvider = (*ProjectModelsView)(nil)

// NewProjectModelsView creates the project models view.
func NewProjectModelsView(cfg ProjectModelsViewConfig) *ProjectModelsView {
	return &ProjectModelsView{
		cfg:       cfg,
		undoStack: widgets.NewUndoStack[domain.Project](10),
	}
}

func (v *ProjectModelsView) SetShell(s ShellAccess) { v.shell = s }

func (v *ProjectModelsView) ID() string    { return "project.models" }
func (v *ProjectModelsView) Title() string { return "Modèles" }
func (v *ProjectModelsView) StatusHints() string {
	return fmt.Sprintf("j/k %s · Enter %s · a %s · d %s · w %s · u %s · Esc %s",
		i18n.T("tui.hints.nav"),
		i18n.T("tui.hints.edit"),
		i18n.T("tui.hints.add"),
		i18n.T("tui.hints.delete"),
		i18n.T("tui.hints.save"),
		i18n.T("tui.hints.undo"),
		i18n.T("tui.hints.back"),
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// Lifecycle
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectModelsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.mountGen++
	gen := v.mountGen
	v.dirty = false
	v.undoStack.Clear()
	v.live = v.cfg.GetProject()

	// Show loading placeholder immediately
	loading := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	loading.SetBackgroundColor(theme.BgPanel)
	muted := theme.ColorTag(theme.TextMutedHex)
	loading.SetText(fmt.Sprintf("\n  %sChargement des modèles...%s", muted, theme.TagColor))
	content.AddItem(loading, 0, 1, true)

	// Build UI asynchronously
	go func() {
		app.QueueUpdateDraw(func() {
			if v.app == nil || v.mountGen != gen {
				return
			}

			v.list = widgets.NewSectionedList()
			v.list.SetApp(app)
			v.list.SetBorderPadding(1, 0, 2, 2)

			if v.live == nil {
				items := []widgets.SectionItem{{
					MainText: i18n.T("tui.project.no_active"),
				}}
				v.list.SetItems(items)
				content.RemoveItem(loading)
				content.AddItem(v.list, 0, 1, true)
				app.SetFocus(v.list)
				return
			}

			v.list.SetItemSelectedFunc(func(index int, item widgets.SectionItem) {
				v.editByItem(item)
			})

			v.renderList()
			content.RemoveItem(loading)
			content.AddItem(v.list, 0, 1, true)
			app.SetFocus(v.list)
		})
	}()
}

func (v *ProjectModelsView) Unmount() {
	if v.dirty && v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.project.unsaved"), false)
	}
	v.undoStack.Clear()
	v.app = nil
	v.list = nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Key handling
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectModelsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.live == nil {
		return event
	}
	switch event.Key() {
	case tcell.KeyEnter:
		if _, item, ok := v.list.CurrentItem(); ok {
			v.editByItem(item)
		}
		return nil
	}
	switch event.Rune() {
	case 'a':
		v.addEntry()
		return nil
	case 'd':
		v.deleteEntry()
		return nil
	case 'w':
		v.save()
		return nil
	case 'u':
		v.undo()
		return nil
	}
	return event
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectModelsView) renderList() {
	if v.list == nil || v.live == nil {
		return
	}
	savedIdx := v.list.GetCurrentItem()

	title := v.live.Name + " · Modèles"
	if v.dirty {
		title += "  " + theme.ColorTag(theme.AccentHex) + "● " + i18n.T("tui.settings.modified") + theme.TagColor
	}

	items := []widgets.SectionItem{
		{IsHeader: true, MainText: title},
	}

	// ── Général ──────────────────────────────────────────────────────────
	items = append(items, widgets.SectionItem{IsHeader: true, MainText: "Général"})
	modelVal := v.live.Model
	if modelVal == "" {
		modelVal = fmt.Sprintf("%s(%s)%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.settings.inherited"), theme.TagColor)
	}
	items = append(items, widgets.SectionItem{
		MainText:  fmt.Sprintf("%-28s %s", "model:", modelVal),
		Reference: projectModelsRef{section: "general", key: "model"},
	})

	// ── Familles ─────────────────────────────────────────────────────────
	items = append(items, widgets.SectionItem{IsHeader: true, MainText: "Familles"})
	families := v.modelOverrides().Families
	if len(families) == 0 {
		items = append(items, widgets.SectionItem{
			MainText: fmt.Sprintf("  %s(vide)%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor),
		})
	} else {
		fKeys := sortedKeys(families)
		for _, k := range fKeys {
			items = append(items, widgets.SectionItem{
				MainText:  fmt.Sprintf("%-28s %s", k+":", families[k]),
				Reference: projectModelsRef{section: "families", key: k},
			})
		}
	}

	// ── Agents ───────────────────────────────────────────────────────────
	items = append(items, widgets.SectionItem{IsHeader: true, MainText: "Agents"})
	agents := v.modelOverrides().Agents
	if len(agents) == 0 {
		items = append(items, widgets.SectionItem{
			MainText: fmt.Sprintf("  %s(vide)%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor),
		})
	} else {
		aKeys := sortedKeys(agents)
		for _, k := range aKeys {
			items = append(items, widgets.SectionItem{
				MainText:  fmt.Sprintf("%-28s %s", k+":", agents[k]),
				Reference: projectModelsRef{section: "agents", key: k},
			})
		}
	}

	v.list.SetItems(items)
	if savedIdx >= 0 {
		v.list.SelectIndex(savedIdx)
	}
}

// modelOverrides returns a non-nil ProjectModelOverrides (reads from live, never mutates).
func (v *ProjectModelsView) modelOverrides() domain.ProjectModelOverrides {
	if v.live.ModelOverrides == nil {
		return domain.ProjectModelOverrides{}
	}
	return *v.live.ModelOverrides
}

// ensureOverrides initialises ModelOverrides on live if nil.
func (v *ProjectModelsView) ensureOverrides() {
	if v.live.ModelOverrides == nil {
		v.live.ModelOverrides = &domain.ProjectModelOverrides{}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Editing
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectModelsView) editByItem(item widgets.SectionItem) {
	ref, ok := item.Reference.(projectModelsRef)
	if !ok || v.shell == nil {
		return
	}

	switch ref.section {
	case "general":
		cur := v.live.Model
		v.shell.ShowInputModal("model", cur, func(newVal string) {
			v.pushUndo()
			v.live.Model = newVal
			v.dirty = true
			v.renderList()
		})

	case "families":
		cur := v.modelOverrides().Families[ref.key]
		v.shell.ShowInputModal(ref.key, cur, func(newVal string) {
			v.pushUndo()
			v.ensureOverrides()
			if v.live.ModelOverrides.Families == nil {
				v.live.ModelOverrides.Families = make(map[string]string)
			}
			v.live.ModelOverrides.Families[ref.key] = newVal
			v.dirty = true
			v.renderList()
		})

	case "agents":
		cur := v.modelOverrides().Agents[ref.key]
		v.shell.ShowInputModal(ref.key, cur, func(newVal string) {
			v.pushUndo()
			v.ensureOverrides()
			if v.live.ModelOverrides.Agents == nil {
				v.live.ModelOverrides.Agents = make(map[string]string)
			}
			v.live.ModelOverrides.Agents[ref.key] = newVal
			v.dirty = true
			v.renderList()
		})
	}
}

func (v *ProjectModelsView) addEntry() {
	if v.shell == nil || v.live == nil {
		return
	}

	scopeOptions := []SelectOption{
		{Label: "Famille", Value: "families"},
		{Label: "Agent", Value: "agents"},
	}

	v.shell.ShowSelectModal("Type d'override", scopeOptions, "", func(scope string) {
		if scope == "" {
			return
		}
		v.shell.ShowInputModal("Clé", "", func(key string) {
			if key == "" {
				return
			}
			v.shell.ShowInputModal("Modèle", "", func(model string) {
				if model == "" {
					return
				}
				v.pushUndo()
				v.ensureOverrides()
				switch scope {
				case "families":
					if v.live.ModelOverrides.Families == nil {
						v.live.ModelOverrides.Families = make(map[string]string)
					}
					v.live.ModelOverrides.Families[key] = model
				case "agents":
					if v.live.ModelOverrides.Agents == nil {
						v.live.ModelOverrides.Agents = make(map[string]string)
					}
					v.live.ModelOverrides.Agents[key] = model
				}
				v.dirty = true
				v.renderList()
				v.shell.ShowToastMsg("Override ajouté", true)
			})
		})
	})
}

func (v *ProjectModelsView) deleteEntry() {
	if v.list == nil || v.shell == nil {
		return
	}
	_, item, ok := v.list.CurrentItem()
	if !ok {
		return
	}
	ref, ok := item.Reference.(projectModelsRef)
	if !ok || ref.section == "general" {
		return // cannot delete the default model field
	}

	label := fmt.Sprintf("Supprimer l'override %s:%s ?", ref.section, ref.key)
	v.shell.ShowSelectModal(label, []SelectOption{
		{Label: "Annuler", Value: ""},
		{Label: "Confirmer la suppression", Value: "yes"},
	}, "", func(choice string) {
		if choice != "yes" {
			return
		}
		v.pushUndo()
		v.ensureOverrides()
		switch ref.section {
		case "families":
			delete(v.live.ModelOverrides.Families, ref.key)
		case "agents":
			delete(v.live.ModelOverrides.Agents, ref.key)
		}
		v.dirty = true
		v.renderList()
		v.shell.ShowToastMsg("Override supprimé", true)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Undo / Save
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectModelsView) pushUndo() {
	snapshot := deepCopyProject(v.live)
	v.undoStack.Push(snapshot)
}

func (v *ProjectModelsView) undo() {
	prev, ok := v.undoStack.Pop()
	if !ok {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.settings.nothing_to_undo"), true)
		}
		return
	}
	*v.live = prev
	v.dirty = v.undoStack.Len() > 0
	v.renderList()
	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.settings.undone"), true)
	}
}

func (v *ProjectModelsView) save() {
	if v.shell == nil || v.live == nil {
		return
	}
	ctx := context.Background()
	if err := v.cfg.SaveProject(ctx, v.live); err != nil {
		v.shell.ShowToastMsg(i18n.T("tui.project.save_error")+": "+err.Error(), false)
		return
	}
	v.dirty = false
	v.undoStack.Clear()
	v.renderList()
	v.shell.ShowToastMsg(i18n.T("tui.project.saved"), true)
}

// ─────────────────────────────────────────────────────────────────────────────
// Commands
// ─────────────────────────────────────────────────────────────────────────────

// ContextCommands implements CommandProvider.
func (v *ProjectModelsView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{ID: "project.models.save", Label: i18n.T("tui.hints.save"), Aliases: []string{"save", "write", "sauvegarder"}, Description: "Sauvegarder les overrides de modèles", Category: "Modèles", Action: func() { v.save() }},
		{ID: "project.models.undo", Label: i18n.T("tui.hints.undo"), Aliases: []string{"undo", "annuler"}, Description: i18n.T("tui.settings.cmd_undo"), Category: "Modèles", Action: func() { v.undo() }},
		{ID: "project.models.add", Label: i18n.T("tui.hints.add"), Aliases: []string{"ajouter", "add override"}, Description: "Ajouter un override de modèle", Category: "Modèles", Action: func() { v.addEntry() }},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// sortedKeys returns the keys of a string map in sorted order.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
