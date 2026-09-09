package views

import (
	"context"
	"fmt"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Config
// ─────────────────────────────────────────────────────────────────────────────

// TeamModelsViewConfig holds the external dependencies for TeamModelsView.
type TeamModelsViewConfig struct {
	// ResolveTeam returns the effective team configuration for the active project.
	ResolveTeam ResolveTeamFunc
	// SaveTeamConfig persists the team config to the team-state repo (commit+push).
	SaveTeamConfig func(ctx context.Context, cfg *teamstate.TeamConfig) error
}

// ─────────────────────────────────────────────────────────────────────────────
// Ref type
// ─────────────────────────────────────────────────────────────────────────────

// teamModelsRef identifies a field in the team models view.
type teamModelsRef struct {
	section string // "general", "families", "agents"
	key     string // map key (empty for the default model field)
}

// ─────────────────────────────────────────────────────────────────────────────
// View
// ─────────────────────────────────────────────────────────────────────────────

// TeamModelsView provides an interactive editor for team-level model
// recommendations (default model, per-family overrides, per-agent overrides).
// It extracts the Models section from TeamDetailView into a standalone sub-page.
type TeamModelsView struct {
	app      *tview.Application
	list     *widgets.SectionedList
	shell    ShellAccess
	cfg      TeamModelsViewConfig
	mountGen uint64

	teamCfg   *teamstate.TeamConfig
	dirty     bool
	undoStack *widgets.UndoStack[teamstate.TeamModelsConfig]
}

var _ View = (*TeamModelsView)(nil)
var _ CommandProvider = (*TeamModelsView)(nil)

// NewTeamModelsView creates the team models view.
func NewTeamModelsView(cfg TeamModelsViewConfig) *TeamModelsView {
	return &TeamModelsView{
		cfg:       cfg,
		undoStack: widgets.NewUndoStack[teamstate.TeamModelsConfig](10),
	}
}

func (v *TeamModelsView) SetShell(s ShellAccess) { v.shell = s }
func (v *TeamModelsView) ID() string             { return "team.models" }
func (v *TeamModelsView) Title() string           { return "Modèles" }

func (v *TeamModelsView) StatusHints() string {
	return fmt.Sprintf("%s  %s  %s  %s  %s",
		i18n.T("tui.hints.enter"),
		i18n.T("tui.hints.add"),
		i18n.T("tui.hints.del"),
		i18n.T("tui.hints.save"),
		i18n.T("tui.hints.undo"),
	)
}

// SupportedModes implements ModeAware — this view is only relevant in team mode.
func (v *TeamModelsView) SupportedModes() []Mode {
	return []Mode{ModeTeam}
}

// ─────────────────────────────────────────────────────────────────────────────
// Lifecycle
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamModelsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.mountGen++
	gen := v.mountGen
	v.dirty = false
	v.undoStack.Clear()

	// Loading placeholder
	loading := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	loading.SetBackgroundColor(theme.BgPanel)
	muted := theme.ColorTag(theme.TextMutedHex)
	loading.SetText(fmt.Sprintf("\n  %sChargement des modèles...%s", muted, theme.TagColor))
	content.AddItem(loading, 0, 1, true)

	// Async pull team-state then build the list
	tc := v.cfg.ResolveTeam()
	if tc.Enabled {
		repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
		syncAsync(v.app, repo, v.shell, func(_ error) {
			if v.app == nil || v.mountGen != gen {
				return
			}
			v.loadData()
			v.buildList(content, loading, app, gen)
		})
	} else {
		// No team — build immediately with empty config
		go func() {
			app.QueueUpdateDraw(func() {
				if v.app == nil || v.mountGen != gen {
					return
				}
				v.loadData()
				v.buildList(content, loading, app, gen)
			})
		}()
	}
}

// buildList creates the SectionedList widget and swaps it in for the loading placeholder.
func (v *TeamModelsView) buildList(content *tview.Flex, loading tview.Primitive, app *tview.Application, gen uint64) {
	v.list = widgets.NewSectionedList()
	v.list.SetApp(app)
	v.list.SetBorderPadding(1, 0, 2, 2)

	v.list.SetItemSelectedFunc(func(index int, item widgets.SectionItem) {
		v.editByItem(item)
	})

	v.renderList()
	content.RemoveItem(loading)
	content.AddItem(v.list, 0, 1, true)
	app.SetFocus(v.list)
}

func (v *TeamModelsView) Unmount() {
	if v.dirty && v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.project.unsaved"), false)
	}
	v.undoStack.Clear()
	v.app = nil
	v.list = nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Data loading
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamModelsView) loadData() {
	tc := v.cfg.ResolveTeam()
	if tc.Enabled {
		repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
		if repo.IsCloned() {
			cfg, err := repo.LoadConfig()
			if err == nil {
				v.teamCfg = cfg
			}
		}
	}
	if v.teamCfg == nil {
		v.teamCfg = &teamstate.TeamConfig{}
	}
	v.ensureMaps()
}

// ensureMaps guarantees all map fields are non-nil for safe read/write.
func (v *TeamModelsView) ensureMaps() {
	if v.teamCfg.Models.Families == nil {
		v.teamCfg.Models.Families = make(map[string]string)
	}
	if v.teamCfg.Models.Agents == nil {
		v.teamCfg.Models.Agents = make(map[string]string)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamModelsView) renderList() {
	if v.list == nil {
		return
	}
	savedIdx := v.list.GetCurrentItem()

	title := "Modèles d'équipe"
	if v.dirty {
		title += "  " + theme.ColorTag(theme.AccentHex) + "● " + i18n.T("tui.settings.modified") + theme.TagColor
	}

	items := []widgets.SectionItem{
		{IsHeader: true, MainText: title},
	}

	// ── Général ──────────────────────────────────────────────────────────
	items = append(items, widgets.SectionItem{IsHeader: true, MainText: "Général"})

	defaultVal := v.teamCfg.Models.Default
	if defaultVal == "" {
		defaultVal = fmt.Sprintf("%s(vide)%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor)
	}
	items = append(items, widgets.SectionItem{
		MainText:  fmt.Sprintf("%-28s %s", "default:", defaultVal),
		Reference: teamModelsRef{section: "general", key: "default"},
	})

	// ── Familles ─────────────────────────────────────────────────────────
	items = append(items, widgets.SectionItem{IsHeader: true, MainText: "Familles"})

	families := v.teamCfg.Models.Families
	if len(families) == 0 {
		items = append(items, widgets.SectionItem{
			MainText: fmt.Sprintf("  %s(vide)%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor),
		})
	} else {
		fKeys := teamModelsSortedKeys(families)
		for _, k := range fKeys {
			val := families[k]
			if val == "" {
				val = fmt.Sprintf("%s(vide)%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor)
			}
			items = append(items, widgets.SectionItem{
				MainText:  fmt.Sprintf("%-28s %s", k+":", val),
				Reference: teamModelsRef{section: "families", key: k},
			})
		}
	}

	// ── Agents ───────────────────────────────────────────────────────────
	items = append(items, widgets.SectionItem{IsHeader: true, MainText: "Agents"})

	agents := v.teamCfg.Models.Agents
	if len(agents) == 0 {
		items = append(items, widgets.SectionItem{
			MainText: fmt.Sprintf("  %s(vide)%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor),
		})
	} else {
		aKeys := teamModelsSortedKeys(agents)
		for _, k := range aKeys {
			val := agents[k]
			if val == "" {
				val = fmt.Sprintf("%s(vide)%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor)
			}
			items = append(items, widgets.SectionItem{
				MainText:  fmt.Sprintf("%-28s %s", k+":", val),
				Reference: teamModelsRef{section: "agents", key: k},
			})
		}
	}

	v.list.SetItems(items)
	if savedIdx >= 0 {
		v.list.SelectIndex(savedIdx)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Key handling
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamModelsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.teamCfg == nil {
		return event
	}
	switch event.Key() {
	case tcell.KeyEnter:
		if v.list != nil {
			if _, item, ok := v.list.CurrentItem(); ok {
				v.editByItem(item)
			}
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
// Editing
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamModelsView) editByItem(item widgets.SectionItem) {
	ref, ok := item.Reference.(teamModelsRef)
	if !ok || v.shell == nil {
		return
	}

	switch ref.section {
	case "general":
		cur := v.teamCfg.Models.Default
		v.shell.ShowInputModal("default", cur, func(newVal string) {
			v.pushUndo()
			v.teamCfg.Models.Default = newVal
			v.dirty = true
			v.renderList()
		})

	case "families":
		cur := v.teamCfg.Models.Families[ref.key]
		v.shell.ShowInputModal(ref.key, cur, func(newVal string) {
			v.pushUndo()
			v.ensureMaps()
			v.teamCfg.Models.Families[ref.key] = newVal
			v.dirty = true
			v.renderList()
		})

	case "agents":
		cur := v.teamCfg.Models.Agents[ref.key]
		v.shell.ShowInputModal(ref.key, cur, func(newVal string) {
			v.pushUndo()
			v.ensureMaps()
			v.teamCfg.Models.Agents[ref.key] = newVal
			v.dirty = true
			v.renderList()
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Add / Delete dynamic entries
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamModelsView) addEntry() {
	if v.shell == nil || v.teamCfg == nil {
		return
	}

	scopeOptions := []SelectOption{
		{Label: "Famille", Value: "families"},
		{Label: "Agent", Value: "agents"},
	}

	v.shell.ShowSelectModal("Type de recommandation", scopeOptions, "", func(scope string) {
		if scope == "" {
			return
		}
		v.shell.ShowInputModal("Clé", "", func(key string) {
			if key == "" {
				return
			}
			v.shell.ShowInputModal("Modèle recommandé", "", func(model string) {
				if model == "" {
					return
				}
				v.pushUndo()
				v.ensureMaps()
				switch scope {
				case "families":
					v.teamCfg.Models.Families[key] = model
				case "agents":
					v.teamCfg.Models.Agents[key] = model
				}
				v.dirty = true
				v.renderList()
				v.shell.ShowToastMsg("Recommandation ajoutée", true)
			})
		})
	})
}

func (v *TeamModelsView) deleteEntry() {
	if v.list == nil || v.shell == nil {
		return
	}
	_, item, ok := v.list.CurrentItem()
	if !ok {
		return
	}
	ref, ok := item.Reference.(teamModelsRef)
	if !ok || ref.section == "general" {
		// Cannot delete the default model field — only clear it via edit.
		return
	}

	label := fmt.Sprintf("Supprimer %s:%s ?", ref.section, ref.key)
	v.shell.ShowSelectModal(label, []SelectOption{
		{Label: "Annuler", Value: ""},
		{Label: "Confirmer la suppression", Value: "yes"},
	}, "", func(choice string) {
		if choice != "yes" {
			return
		}
		v.pushUndo()
		v.ensureMaps()
		switch ref.section {
		case "families":
			delete(v.teamCfg.Models.Families, ref.key)
		case "agents":
			delete(v.teamCfg.Models.Agents, ref.key)
		}
		v.dirty = true
		v.renderList()
		v.shell.ShowToastMsg("Supprimé: "+ref.key, true)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Undo / Save
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamModelsView) pushUndo() {
	snapshot := deepCopyTeamModels(v.teamCfg.Models)
	v.undoStack.Push(snapshot)
}

func (v *TeamModelsView) undo() {
	prev, ok := v.undoStack.Pop()
	if !ok {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.settings.nothing_to_undo"), true)
		}
		return
	}
	v.teamCfg.Models = prev
	v.ensureMaps()
	v.dirty = v.undoStack.Len() > 0
	v.renderList()
	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.settings.undone"), true)
	}
}

func (v *TeamModelsView) save() {
	if v.shell == nil || v.teamCfg == nil {
		return
	}
	ctx := context.Background()
	if v.cfg.SaveTeamConfig == nil {
		v.shell.ShowToastMsg("SaveTeamConfig non configuré", false)
		return
	}
	if err := v.cfg.SaveTeamConfig(ctx, v.teamCfg); err != nil {
		v.shell.ShowToastMsg("Erreur sauvegarde équipe: "+err.Error(), false)
		return
	}
	v.dirty = false
	v.undoStack.Clear()
	v.renderList()
	v.shell.ShowToastMsg("Modèles sauvegardés", true)
}

// ─────────────────────────────────────────────────────────────────────────────
// Commands
// ─────────────────────────────────────────────────────────────────────────────

// ContextCommands implements CommandProvider.
func (v *TeamModelsView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{
			ID: "team.models.save", Label: i18n.T("tui.hints.save"),
			Aliases: []string{"save", "write", "sauvegarder"},
			Description: "Sauvegarder les recommandations de modèles",
			Category: "Modèles", Action: func() { v.save() },
		},
		{
			ID: "team.models.undo", Label: i18n.T("tui.hints.undo"),
			Aliases: []string{"undo", "annuler"},
			Description: i18n.T("tui.settings.cmd_undo"),
			Category: "Modèles", Action: func() { v.undo() },
		},
		{
			ID: "team.models.add", Label: i18n.T("tui.hints.add"),
			Aliases: []string{"ajouter", "add"},
			Description: "Ajouter une recommandation de modèle",
			Category: "Modèles", Action: func() { v.addEntry() },
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// teamModelsSortedKeys returns the keys of a string map in sorted order.
// Named distinctly to avoid collisions with the package-level sortedKeys used
// by ProjectModelsView.
func teamModelsSortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// deepCopyTeamModels creates a value-copy of TeamModelsConfig, including deep
// copies of the Families and Agents maps, suitable for undo snapshots.
func deepCopyTeamModels(src teamstate.TeamModelsConfig) teamstate.TeamModelsConfig {
	dst := teamstate.TeamModelsConfig{
		Default: src.Default,
	}
	if src.Families != nil {
		dst.Families = make(map[string]string, len(src.Families))
		for k, v := range src.Families {
			dst.Families[k] = v
		}
	}
	if src.Agents != nil {
		dst.Agents = make(map[string]string, len(src.Agents))
		for k, v := range src.Agents {
			dst.Agents[k] = v
		}
	}
	return dst
}
