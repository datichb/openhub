package views

import (
	"context"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ProjectConfigViewConfig holds external dependencies for the project config view.
type ProjectConfigViewConfig struct {
	// GetProject returns the currently active project (nil if none).
	GetProject func() *domain.Project
	// SaveProject persists the modified project to the DB.
	SaveProject func(ctx context.Context, p *domain.Project) error
	// ExecHints computes the « Exécution » hints of a project (detected
	// Dockerfile), off the event loop. Nil = no hint.
	ExecHints func(projectPath string) ProjectExecHints
	// WorkflowIDs lists the workflows offered as default (cached catalogue).
	WorkflowIDs func() []string
}

// ProjectConfigView displays and edits the active project's configuration.
type ProjectConfigView struct {
	app   *tview.Application
	list  *widgets.SectionedList
	page  *widgets.ListDetailPage
	shell ShellAccess
	cfg   ProjectConfigViewConfig
	loads loadSlot // the read of the hints: only the latest is applied

	live      *domain.Project
	fields    []configField
	undoStack *widgets.UndoStack[domain.Project]
	autoSaver *AutoSaver
	badge     saveBadge
	execHints ProjectExecHints
}

var _ View = (*ProjectConfigView)(nil)
var _ CommandProvider = (*ProjectConfigView)(nil)

// NewProjectConfigView creates the project config view.
func NewProjectConfigView(cfg ProjectConfigViewConfig) *ProjectConfigView {
	return &ProjectConfigView{
		cfg:       cfg,
		undoStack: widgets.NewUndoStack[domain.Project](10),
	}
}

func (v *ProjectConfigView) SetShell(s ShellAccess) { v.shell = s }

func (v *ProjectConfigView) ID() string    { return "project.config" }
func (v *ProjectConfigView) Title() string { return i18n.T("tui.project.title") }
func (v *ProjectConfigView) StatusHints() string {
	return keys.Hints(keys.Navigate.HintAs(i18n.T("tui.hints.nav")), "{/} "+i18n.T("tui.hints.navigate"),
		toggleHint(), keys.Open.HintAs(i18n.T("tui.hints.edit")), cfgUndoKey.Hint(), keys.Back.Hint())
}

func (v *ProjectConfigView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.undoStack.Clear()
	v.live = v.cfg.GetProject() // synchronous

	// Auto-save after SaveDelay, the same as the other pages
	v.autoSaver = NewAutoSaver(SaveDelay, app, func() {
		v.doSave()
	})

	// Loading placeholder
	loading := stateView(stateLoading, i18n.T("tui.project.loading"))
	content.AddItem(loading, 0, 1, true)

	// The hints (a Dockerfile to detect) are read off the event loop, then the
	// list is built on it.
	live := v.live
	load(app, shellContext(v.shell), &v.loads, func(context.Context) (ProjectExecHints, error) {
		var hints ProjectExecHints
		if live != nil && v.cfg.ExecHints != nil {
			hints = v.cfg.ExecHints(live.Path)
		}
		return hints, nil
	}, func(hints ProjectExecHints, err error) {
		if v.app == nil {
			return
		}
		if err != nil {
			setState(loading, stateError, err.Error())
			return
		}
		v.execHints = hints

		owned := ownsFocus(app, content)
		v.list = widgets.NewSectionedList()
		v.list.SetApp(app)
		v.page = newListPage(v.pageTitle(), v.list)
		v.badge.attach(v.page, v.pageTitle())

		if v.live == nil {
			v.list.SetItems([]widgets.SectionItem{{MainText: stateText(stateEmpty, i18n.T("tui.project.no_active"))}})
			v.page.SetDetail(emptyDetail())
			v.page.Mount(content)
			focusIf(app, owned, v.list)
			return
		}

		v.list.SetItemSelectedFunc(func(index int, item widgets.SectionItem) {
			v.onItemSelected(index, item)
		})
		v.list.SetItemChangedFunc(func(_ int, item widgets.SectionItem) {
			showFieldItemDetail(v.page, v.fields, item, cfgUndoKey.Keys(), cfgUndoKey.Label())
		})

		v.buildFields()
		v.renderFields()
		v.page.Mount(content)
		focusIf(app, owned, v.list)
	})
}

func (v *ProjectConfigView) Unmount() {
	v.loads.Invalidate() // drop the load in flight
	if v.autoSaver != nil {
		v.autoSaver.Flush()
	}
	v.undoStack.Clear()
	v.badge.detach()
	v.app = nil
	v.list = nil
	v.page = nil
}

// pageTitle is the title of the list frame (the active project).
func (v *ProjectConfigView) pageTitle() string {
	if v.live == nil {
		return v.Title()
	}
	return v.Title() + " · " + v.live.Name
}

func (v *ProjectConfigView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.live == nil || v.list == nil {
		return event
	}
	switch {
	case keys.Open.Matches(event), keys.Edit.Matches(event):
		if idx, item, ok := v.list.CurrentItem(); ok {
			v.onItemSelected(idx, item)
		}
		return nil
	case cfgToggleKey.Matches(event):
		v.onToggleSelected()
		return nil
	case cfgUndoKey.Matches(event):
		v.undo()
		return nil
	}
	return event
}

func (v *ProjectConfigView) undo() {
	prev, ok := v.undoStack.Pop()
	if !ok {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.config.nothing_to_undo"), true)
		}
		return
	}
	*v.live = prev
	if v.autoSaver != nil {
		v.autoSaver.Cancel()
	}
	v.doSave()
	v.renderFields()
	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.config.undone"), true)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Field definitions
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectConfigView) buildFields() {
	languageOptions := []SelectOption{
		{Label: "Français", Value: "fr"},
		{Label: "English", Value: "en"},
		{Label: fmt.Sprintf("(%s)", i18n.T("tui.config.inherited")), Value: ""},
	}
	providerOptionsFunc := func() []SelectOption {
		opts := []SelectOption{{Label: fmt.Sprintf("(%s)", i18n.T("tui.config.inherited")), Value: ""}}
		for _, p := range provider.AllProviders() {
			opts = append(opts, SelectOption{Label: string(p), Value: string(p)})
		}
		return opts
	}
	statusOptions := []SelectOption{
		{Label: "active", Value: "active"},
		{Label: "archived", Value: "archived"},
	}

	v.fields = []configField{
		// ── General ─────────────────────────────────────────────────────────
		{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.config.section.general")},
		{Key: "id", Kind: CfgFieldReadonly, Label: i18n.T("tui.config.field.project_id.label"),
			Description: i18n.T("tui.config.field.project_id.desc"),
			Get:         func() string { return v.live.ID }},
		{Key: "name", Kind: CfgFieldString, Label: i18n.T("tui.config.field.project_name.label"),
			Description: i18n.T("tui.config.field.project_name.desc"),
			Get:         func() string { return v.live.Name },
			Set:         func(val string) { v.live.Name = val }},
		{Key: "path", Kind: CfgFieldString, Label: i18n.T("tui.config.field.project_path.label"),
			Description: i18n.T("tui.config.field.project_path.desc"),
			Get:         func() string { return v.live.Path },
			Set:         func(val string) { v.live.Path = val }},
		{Key: "language", Kind: CfgFieldSelect, Label: i18n.T("tui.config.field.project_language.label"),
			Description: i18n.T("tui.config.field.project_language.desc"),
			Options:     languageOptions,
			Get:         func() string { return v.live.Language },
			Set:         func(val string) { v.live.Language = val }},
		{Key: "provider", Kind: CfgFieldSelect, Label: i18n.T("tui.config.field.project_provider.label"),
			Description: i18n.T("tui.config.field.project_provider.desc"),
			OptionsFunc: providerOptionsFunc,
			Get:         func() string { return v.live.Provider },
			Set:         func(val string) { v.live.Provider = val }},
		{Key: "model", Kind: CfgFieldString, Label: i18n.T("tui.config.field.project_model.label"),
			Description: i18n.T("tui.config.field.project_model.desc"),
			Get:         func() string { return v.live.Model },
			Set:         func(val string) { v.live.Model = val }},
		{Key: "status", Kind: CfgFieldSelect, Label: i18n.T("tui.config.field.project_status.label"),
			Description: i18n.T("tui.config.field.project_status.desc"),
			Options:     statusOptions,
			Get:         func() string { return string(v.live.Status) },
			Set:         func(val string) { v.live.Status = domain.ProjectStatus(val) }},
		{Key: "models", Kind: CfgFieldLink, Label: i18n.T("tui.config.link.models"), LinkTarget: "project.models",
			Get: func() string { return "" }},
		{Key: "bricks", Kind: CfgFieldLink, Label: i18n.T("tui.bricks.link"), LinkTarget: "project.agents",
			Get: func() string { return "" }},

		// ── Team ────────────────────────────────────────────────────────────
		{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.config.section.team")},
		{Key: "team_id", Kind: CfgFieldString, Label: i18n.T("tui.config.field.team_id.label"),
			Description: i18n.T("tui.config.field.team_id.desc"),
			Get: func() string {
				if v.live.TeamID == nil {
					return ""
				}
				return *v.live.TeamID
			},
			Set: func(val string) {
				if val == "" {
					v.live.TeamID = nil
				} else {
					v.live.TeamID = &val
				}
			}},

		// ── Tracker Overrides ───────────────────────────────────────────────
		{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.config.section.tracker")},
		{Key: "tracker_project", Kind: CfgFieldString, Label: i18n.T("tui.config.field.tracker_project.label"),
			Description: i18n.T("tui.config.field.tracker_project.desc"),
			Placeholder: i18n.T("tui.config.field.tracker_project.placeholder"),
			Get: func() string {
				if v.live.TrackerConfig == nil {
					return ""
				}
				return v.live.TrackerConfig.TrackerProject
			},
			Set: func(val string) {
				if v.live.TrackerConfig == nil {
					v.live.TrackerConfig = &domain.ProjectTrackerConfig{}
				}
				v.live.TrackerConfig.TrackerProject = val
			}},
		{Key: "tracker_url", Kind: CfgFieldString, Label: i18n.T("tui.config.field.tracker_url.label"),
			Description: i18n.T("tui.config.field.tracker_url.desc"),
			Placeholder: i18n.T("tui.config.field.tracker_url.placeholder"),
			Get: func() string {
				if v.live.TrackerConfig == nil {
					return ""
				}
				return v.live.TrackerConfig.TrackerURL
			},
			Set: func(val string) {
				if v.live.TrackerConfig == nil {
					v.live.TrackerConfig = &domain.ProjectTrackerConfig{}
				}
				v.live.TrackerConfig.TrackerURL = val
			}},
		{Key: "ticket_pattern", Kind: CfgFieldString, Label: i18n.T("tui.config.field.ticket_pattern.label"),
			Description: i18n.T("tui.config.field.ticket_pattern.desc"),
			Placeholder: i18n.T("tui.config.field.ticket_pattern.placeholder"),
			Get: func() string {
				if v.live.TrackerConfig == nil {
					return ""
				}
				return v.live.TrackerConfig.TicketPattern
			},
			Set: func(val string) {
				if v.live.TrackerConfig == nil {
					v.live.TrackerConfig = &domain.ProjectTrackerConfig{}
				}
				v.live.TrackerConfig.TicketPattern = val
			}},
		{Key: "write_enabled", Kind: CfgFieldTriBool, Label: i18n.T("tui.config.field.tracker_write_enabled.label"),
			Description: i18n.T("tui.config.field.tracker_write_enabled.desc"),
			Scope:       ScopeProject,
			Get: func() string {
				if v.live.TrackerConfig == nil || v.live.TrackerConfig.WriteEnabled == nil {
					return ""
				}
				return boolStr(*v.live.TrackerConfig.WriteEnabled)
			},
			Set: func(val string) {
				if v.live.TrackerConfig == nil {
					v.live.TrackerConfig = &domain.ProjectTrackerConfig{}
				}
				if val == "" {
					v.live.TrackerConfig.WriteEnabled = nil
					return
				}
				b := val == "true"
				v.live.TrackerConfig.WriteEnabled = &b
			}},
	}
	v.fields = append(v.fields, v.execFields()...)
	v.fields = append(v.fields, []configField{
		// ── Links ───────────────────────────────────────────────────────────
		{Key: "mcp_services", Kind: CfgFieldLink, Label: i18n.T("tui.config.link.mcp"), LinkTarget: "project.mcp",
			Get: func() string { return "" }},
		{Key: "workflow", Kind: CfgFieldLink, Label: i18n.T("tui.config.link.workflow"), LinkTarget: "workflows",
			Get: func() string { return "" }},
	}...)
}

// ─────────────────────────────────────────────────────────────────────────────
// MCP helpers (used by project_mcp_view.go too)
// ─────────────────────────────────────────────────────────────────────────────

func mcpServiceEnabled(p *domain.Project, name string) string {
	if p.MCPConfig == nil {
		return ""
	}
	for _, svc := range p.MCPConfig.Services {
		if svc.Name == name && svc.Enabled != nil {
			return boolStr(*svc.Enabled)
		}
	}
	return ""
}

func mcpServiceWriteEnabled(p *domain.Project, name string) string {
	if p.MCPConfig == nil {
		return ""
	}
	for _, svc := range p.MCPConfig.Services {
		if svc.Name == name && svc.WriteEnabled != nil {
			return boolStr(*svc.WriteEnabled)
		}
	}
	return ""
}

func mcpServiceURL(p *domain.Project, name string) string {
	if p.MCPConfig == nil {
		return ""
	}
	for _, svc := range p.MCPConfig.Services {
		if svc.Name == name && svc.URL != "" {
			return svc.URL
		}
	}
	return ""
}

func mcpServiceToken(p *domain.Project, name string) string {
	if p.MCPConfig == nil {
		return ""
	}
	for _, svc := range p.MCPConfig.Services {
		if svc.Name == name && svc.TokenKey != "" {
			return svc.TokenKey
		}
	}
	return ""
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectConfigView) renderFields() {
	if v.list == nil || v.live == nil {
		return
	}
	savedIdx := v.list.GetCurrentItem()

	v.list.SetItems(configItems(v.fields, 28))
	if savedIdx >= 0 {
		v.list.SelectIndex(savedIdx)
	}
	showCurrentFieldDetail(v.page, v.list, v.fields, cfgUndoKey.Keys(), cfgUndoKey.Label())
}

// ─────────────────────────────────────────────────────────────────────────────
// Editing
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectConfigView) onItemSelected(_ int, item widgets.SectionItem) {
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.fields) {
		return
	}
	f := &v.fields[ref]
	if !isEditable(f.Kind) || f.Get == nil {
		return
	}
	if v.shell == nil {
		return
	}

	// Check lock (enforced by team)
	if f.Locked != nil && f.Locked() {
		v.shell.ShowToastMsg(i18n.T("tui.config.enforced_toast"), false)
		return
	}

	v.pushUndo()
	editConfigField(v.shell, f, func() {
		v.scheduleAutoSave()
		v.renderFields()
	})
}

func (v *ProjectConfigView) onToggleSelected() {
	if v.list == nil {
		return
	}
	_, item, ok := v.list.CurrentItem()
	if !ok {
		return
	}
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.fields) {
		return
	}
	f := &v.fields[ref]
	if !isToggleable(f.Kind) {
		return
	}
	if f.Locked != nil && f.Locked() {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.config.enforced_toast"), false)
		}
		return
	}
	v.pushUndo()
	toggleConfigField(f)
	v.scheduleAutoSave()
	v.renderFields()
}

func (v *ProjectConfigView) pushUndo() {
	snapshot := deepCopyProject(v.live)
	v.undoStack.Push(snapshot)
}

func (v *ProjectConfigView) scheduleAutoSave() {
	v.badge.set(saveStatusPending)
	if v.autoSaver != nil {
		v.autoSaver.Schedule()
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Save (auto-save via AutoSaver)
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectConfigView) doSave() {
	if v.live == nil {
		return
	}
	ctx := context.Background()
	if err := v.cfg.SaveProject(ctx, v.live); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.project.save_error")+": "+err.Error(), false)
		}
		v.badge.set(saveStatusFailed)
		return
	}
	v.badge.set(saveStatusSaved)
}

// ContextCommands implements CommandProvider.
func (v *ProjectConfigView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{ID: "project.undo", Label: i18n.T("tui.hints.undo"), Aliases: []string{"undo", "annuler"}, Description: i18n.T("tui.settings.cmd_undo"), Category: i18n.T("tui.project.category"), Action: func() { v.undo() }},
	}
}
