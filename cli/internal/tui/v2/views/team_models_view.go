package views

import (
	"context"
	"fmt"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Config
// ─────────────────────────────────────────────────────────────────────────────

// TeamModelsViewConfig holds the external dependencies for TeamModelsView.
type TeamModelsViewConfig struct {
	// ResolveTeam returns the effective team configuration for the active project.
	ResolveTeam ResolveTeamFunc
	// SaveTeamConfig is called on the event loop and gives the function that
	// persists the team config to the team-state repo (commit+push), which the
	// view runs off the loop. Nil: nothing to save with.
	SaveTeamConfig func() TeamConfigWriter
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
	app   *tview.Application
	list  *widgets.SectionedList
	page  *widgets.ListDetailPage
	shell ShellAccess
	cfg   TeamModelsViewConfig
	loads loadSlot // the reads of the team config: only the latest is applied

	content *tview.Flex // where the page is mounted (nil once unmounted)
	teamCfg *teamstate.TeamConfig
	// baseTeam is teamCfg as it was read or last saved: the save writes the
	// difference, on top of the latest version of the team-state.
	baseTeam *teamstate.TeamConfig
	// cfgErr is why the team config could not be read: the view shows it and
	// nothing can be saved until a read succeeds.
	cfgErr error
	dirty  bool
	badge  saveBadge
	// saving is true while a save runs off the event loop; saveAgain, that an
	// edit asked for another one meanwhile.
	saving, saveAgain bool
	undoStack         *widgets.UndoStack[teamstate.TeamModelsConfig]
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
func (v *TeamModelsView) Title() string          { return i18n.T("tui.config.section.models") }

func (v *TeamModelsView) StatusHints() string {
	if v.cfgErr != nil {
		return keys.Hints(keys.Refresh.Hint(), keys.Back.Hint())
	}
	return keys.Hints(keys.Open.HintAs(i18n.T("tui.hints.edit")), keys.Create.Hint(),
		keys.Delete.Hint(), keys.Save.Hint(), cfgUndoKey.Hint())
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
	v.content = content
	v.dirty = false
	v.undoStack.Clear()
	v.teamCfg, v.baseTeam, v.cfgErr = nil, nil, nil

	// Loading placeholder
	loading := stateView(stateLoading, i18n.T("tui.team_models.loading"))
	content.AddItem(loading, 0, 1, true)
	v.read(content, app)
}

// read pulls the team-state, reads its config off the event loop, then builds
// the list on it (or says why the config could not be read).
func (v *TeamModelsView) read(content *tview.Flex, app *tview.Application) {
	tc := v.cfg.ResolveTeam()
	fetch := func(ctx context.Context) (*teamstate.TeamConfig, error) {
		if !tc.Enabled {
			return nil, nil // no team: an empty config
		}
		return readTeamConfig(ctx, tc)
	}
	apply := func(cfg *teamstate.TeamConfig, err error) {
		if v.app == nil {
			return
		}
		v.setTeamConfig(cfg, err)
		v.buildList(content, app)
	}
	if tc.Enabled {
		syncLoad(app, shellContext(v.shell), &v.loads, teamstate.NewRepo(tc.StateRepo, tc.StatePath), v.shell, fetch, apply)
		return
	}
	load(app, shellContext(v.shell), &v.loads, fetch, apply)
}

// buildList creates the SectionedList widget and mounts it in place of the
// loading placeholder.
func (v *TeamModelsView) buildList(content *tview.Flex, app *tview.Application) {
	owned := ownsFocus(app, content)
	v.list = widgets.NewSectionedList()
	v.list.SetApp(app)
	v.page = newListPage(v.Title(), v.list)
	v.badge.attach(v.page, v.Title())

	v.list.SetItemSelectedFunc(func(index int, item widgets.SectionItem) {
		v.editByItem(item)
	})
	v.list.SetItemChangedFunc(func(_ int, item widgets.SectionItem) { v.showDetail(item) })

	if v.cfgErr != nil {
		v.showConfigError()
	} else {
		v.renderList()
	}
	v.page.Mount(content)
	focusIf(app, owned, v.list)
	refreshHints(v.shell)
}

// showConfigError is the state of a view whose team config could not be read:
// the reason, nothing to edit, nothing to save.
func (v *TeamModelsView) showConfigError() {
	v.dirty = false
	v.undoStack.Clear()
	v.list.SetItems([]widgets.SectionItem{{MainText: stateText(stateError, unreadableConfigText(v.cfgErr))}})
	v.page.SetDetail(unreadableDetail(v.cfgErr))
	v.badge.set(saveStatusFailed)
}

func (v *TeamModelsView) Unmount() {
	v.content = nil
	v.loads.Invalidate() // drop the load in flight
	if v.dirty && v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.save.unpublished_left"), false)
	}
	v.undoStack.Clear()
	v.app = nil
	v.list = nil
	v.page = nil
}

// showDetail fills the detail pane with the override of item.
func (v *TeamModelsView) showDetail(item widgets.SectionItem) {
	if v.page == nil || v.teamCfg == nil {
		return
	}
	extra := append(keys.Pairs(keys.Create, keys.Delete), keys.Save.Keys(), keys.Save.Label(), cfgUndoKey.Keys(), cfgUndoKey.Label())
	ref, ok := item.Reference.(teamModelsRef)
	if !ok {
		v.page.SetDetail(modelOverrideDetail("", "", "", "", extra...))
		return
	}
	val := ""
	switch ref.section {
	case "general":
		val = v.teamCfg.Models.Default
	case "families":
		val = v.teamCfg.Models.Families[ref.key]
	case "agents":
		val = v.teamCfg.Models.Agents[ref.key]
	}
	v.page.SetDetail(modelOverrideDetail(ref.section, ref.key, val, "", extra...))
}

// ─────────────────────────────────────────────────────────────────────────────
// Data loading
// ─────────────────────────────────────────────────────────────────────────────

// setTeamConfig takes the config that was read. One that could not be read is
// not replaced by an empty one: the view keeps the error (cfgErr) and nothing
// of the config, so nothing can be saved. nil without error: the team has no
// state to read, an empty config is shown (and cannot be written, see the
// writer).
func (v *TeamModelsView) setTeamConfig(cfg *teamstate.TeamConfig, err error) {
	v.cfgErr = err
	if err != nil {
		v.teamCfg, v.baseTeam = nil, nil
		return
	}
	if cfg != nil {
		v.teamCfg = cfg
	}
	if v.teamCfg == nil {
		v.teamCfg = &teamstate.TeamConfig{}
	}
	v.ensureMaps()
	v.baseTeam = teamstate.CloneConfig(v.teamCfg)
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

// syncBadge shows whether the changes are published to the team-state.
func (v *TeamModelsView) syncBadge() {
	if v.saving {
		v.badge.set(saveStatusPending) // the write runs: not published yet
		return
	}
	v.badge.settle(v.dirty)
}

func (v *TeamModelsView) renderList() {
	if v.list == nil {
		return
	}
	savedIdx := v.list.GetCurrentItem()
	notConfigured := muted(i18n.T("tui.config.not_configured"))

	v.syncBadge()

	// ── General ──────────────────────────────────────────────────────────
	items := []widgets.SectionItem{{IsHeader: true, MainText: i18n.T("tui.config.section.general")}}

	defaultVal := v.teamCfg.Models.Default
	if defaultVal == "" {
		defaultVal = notConfigured
	}
	items = append(items, widgets.SectionItem{
		MainText:      fmt.Sprintf("%-28s %s", i18n.T("tui.config.field.default_model.label"), defaultVal),
		SecondaryText: muted(i18n.T("tui.config.field.default_model.desc")),
		Reference:     teamModelsRef{section: "general", key: "default"},
	})

	// ── Families ─────────────────────────────────────────────────────────
	items = append(items, widgets.SectionItem{IsHeader: true, MainText: i18n.T("tui.config.section.families")})

	families := v.teamCfg.Models.Families
	if len(families) == 0 {
		items = append(items, widgets.SectionItem{
			MainText: "  " + notConfigured,
		})
	} else {
		fKeys := teamModelsSortedKeys(families)
		for _, k := range fKeys {
			val := families[k]
			if val == "" {
				val = notConfigured
			}
			items = append(items, widgets.SectionItem{
				MainText:  fmt.Sprintf("%-28s %s", k, val),
				Reference: teamModelsRef{section: "families", key: k},
			})
		}
	}

	// ── Agents ───────────────────────────────────────────────────────────
	items = append(items, widgets.SectionItem{IsHeader: true, MainText: i18n.T("tui.config.section.agents")})

	agents := v.teamCfg.Models.Agents
	if len(agents) == 0 {
		items = append(items, widgets.SectionItem{
			MainText: "  " + notConfigured,
		})
	} else {
		aKeys := teamModelsSortedKeys(agents)
		for _, k := range aKeys {
			val := agents[k]
			if val == "" {
				val = notConfigured
			}
			items = append(items, widgets.SectionItem{
				MainText:  fmt.Sprintf("%-28s %s", k, val),
				Reference: teamModelsRef{section: "agents", key: k},
			})
		}
	}

	v.list.SetItems(items)
	if savedIdx >= 0 {
		v.list.SelectIndex(savedIdx)
	}
	_, it, _ := v.list.CurrentItem()
	v.showDetail(it)
}

// ─────────────────────────────────────────────────────────────────────────────
// Key handling
// ─────────────────────────────────────────────────────────────────────────────

func (v *TeamModelsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.cfgErr != nil { // nothing to edit: only a new read gets out of it
		switch {
		case keys.Refresh.Matches(event) && v.content != nil && v.app != nil:
			v.read(v.content, v.app)
			return nil
		case keys.Save.Matches(event):
			v.save() // says why it is refused
			return nil
		}
		return event
	}
	if v.teamCfg == nil {
		return event
	}
	switch {
	case keys.Create.Matches(event):
		v.addEntry()
		return nil
	case keys.Open.Matches(event), keys.Edit.Matches(event):
		if v.list != nil {
			if _, item, ok := v.list.CurrentItem(); ok {
				v.editByItem(item)
			}
		}
		return nil
	case keys.Delete.Matches(event):
		v.deleteEntry()
		return nil
	case keys.Save.Matches(event):
		v.save()
		return nil
	case cfgUndoKey.Matches(event):
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
		v.shell.ShowInputModal(i18n.T("tui.config.field.default_model.label"), cur, func(newVal string) {
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
		{Label: i18n.T("tui.team_models.family"), Value: "families"},
		{Label: i18n.T("tui.team_models.agent"), Value: "agents"},
	}

	v.shell.ShowSelectModal(i18n.T("tui.team.models.recommendation_type"), scopeOptions, "", func(scope string) {
		if scope == "" {
			return
		}
		v.shell.ShowInputModal(i18n.T("tui.project_models.key"), "", func(key string) {
			if key == "" {
				return
			}
			v.shell.ShowInputModal(i18n.T("tui.team.models.add_recommendation"), "", func(model string) {
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
				v.shell.ShowToastMsg(i18n.T("tui.team.models.recommendation_added"), true)
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

	title := i18n.Tf("tui.team_models.delete_confirm", ref.section, ref.key)
	v.shell.Confirm(title, i18n.T("tui.team_models.delete_detail"), true, func() {
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
		v.shell.ShowToastMsg(i18n.Tf("tui.team_models.deleted", ref.key), true)
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
	if v.shell == nil {
		return
	}
	if v.cfgErr != nil || v.teamCfg == nil { // never write what was not read
		v.shell.ShowToastMsg(saveRefusedText(v.cfgErr), false)
		return
	}
	if v.cfg.SaveTeamConfig == nil {
		v.shell.ShowToastMsg(i18n.T("tui.team_models.no_save_func"), false)
		return
	}
	if v.saving {
		v.saveAgain = true
		return
	}
	// The writer is taken on the event loop (it reads the app), the copy of
	// the config too (the user keeps editing); the commit and the push run off it.
	write := v.cfg.SaveTeamConfig()
	if write == nil {
		v.shell.ShowToastMsg(i18n.T("tui.team_models.no_save_func"), false)
		return
	}
	cfg := deepCopyTeamConfig(v.teamCfg)
	base := teamstate.CloneConfig(v.baseTeam)
	edits := v.undoStack.Len()
	v.dirty = false // an edit made during the write sets it again
	v.saving = true
	v.badge.set(saveStatusPending) // the write really runs: saved is said when it ends
	persist(v.app, func(ctx context.Context) error { return write(ctx, base, &cfg) }, func(err error) {
		v.saving = false
		again := v.saveAgain
		v.saveAgain = false
		if err != nil {
			v.dirty = true
			v.shell.ShowToastMsg(i18n.T("tui.team_models.save_error")+err.Error(), false)
			v.badge.set(saveStatusFailed)
			confirmReloadOnConflict(v.shell, err, func() {
				if v.content != nil && v.app != nil {
					v.dirty = false
					v.undoStack.Clear()
					v.read(v.content, v.app)
				}
			})
			return
		}
		v.baseTeam = teamstate.CloneConfig(&cfg)
		if v.undoStack.Len() == edits {
			v.undoStack.Clear()
		}
		v.renderList()
		v.shell.ShowToastMsg(i18n.T("tui.config.saved_team"), true)
		if again {
			v.save()
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Commands
// ─────────────────────────────────────────────────────────────────────────────

// ContextCommands implements CommandProvider.
func (v *TeamModelsView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{
			ID: "team.models.save", Label: i18n.T("tui.hints.save"),
			Aliases:     []string{"save", "write", "sauvegarder"},
			Description: i18n.T("tui.team.models.save_recommendations"),
			Category:    i18n.T("tui.config.section.models"), Action: func() { v.save() },
		},
		{
			ID: "team.models.undo", Label: i18n.T("tui.hints.undo"),
			Aliases:     []string{"undo", "annuler"},
			Description: i18n.T("tui.settings.cmd_undo"),
			Category:    i18n.T("tui.config.section.models"), Action: func() { v.undo() },
		},
		{
			ID: "team.models.add", Label: i18n.T("tui.keys.action.create"),
			Aliases:     []string{"ajouter", "add"},
			Description: i18n.T("tui.team.models.add_recommendation"),
			Category:    i18n.T("tui.config.section.models"), Action: func() { v.addEntry() },
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
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
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
