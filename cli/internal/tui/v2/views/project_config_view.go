package views

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
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
	// ResolveMCPSource returns the resolution source for an MCP field.
	// Returns (effectiveValue, sourceAnnotation, isLocked).
	// If nil, no resolution annotations are shown.
	ResolveMCPSource func(service, field string) (effective string, source string, locked bool)
}

// ProjectConfigView displays and edits the active project's configuration.
type ProjectConfigView struct {
	app      *tview.Application
	list     *widgets.SectionedList
	shell    ShellAccess
	cfg      ProjectConfigViewConfig
	mountGen uint64

	live      *domain.Project
	fields    []configField
	undoStack *widgets.UndoStack[domain.Project]
	autoSaver *AutoSaver
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
	return fmt.Sprintf("j/k %s · {/} %s · Space %s · Enter %s · u %s · Esc %s",
		i18n.T("tui.hints.nav"),
		i18n.T("tui.hints.navigate"),
		i18n.T("tui.hints.toggle"),
		i18n.T("tui.hints.edit"),
		i18n.T("tui.hints.undo"),
		i18n.T("tui.hints.back"),
	)
}

func (v *ProjectConfigView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.mountGen++
	gen := v.mountGen
	v.undoStack.Clear()
	v.live = v.cfg.GetProject() // synchronous

	// Auto-save: SQLite is fast, 200ms debounce
	v.autoSaver = NewAutoSaver(200*time.Millisecond, app, func() {
		v.doSave()
	})

	// Loading placeholder
	loading := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	loading.SetBackgroundColor(theme.BgPanel)
	muted := theme.ColorTag(theme.TextMutedHex)
	loading.SetText(fmt.Sprintf("\n  %s%s%s", muted, i18n.T("tui.project.loading"), theme.TagColor))
	content.AddItem(loading, 0, 1, true)

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
				v.onItemSelected(index, item)
			})

			v.buildFields()
			v.renderFields()
			content.RemoveItem(loading)
			content.AddItem(v.list, 0, 1, true)
			app.SetFocus(v.list)
		})
	}()
}

func (v *ProjectConfigView) Unmount() {
	if v.autoSaver != nil {
		v.autoSaver.Flush()
	}
	v.undoStack.Clear()
	v.app = nil
	v.list = nil
}

func (v *ProjectConfigView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.live == nil || v.list == nil {
		return event
	}
	if event.Key() == tcell.KeyEnter {
		if idx, item, ok := v.list.CurrentItem(); ok {
			v.onItemSelected(idx, item)
		}
		return nil
	}
	switch event.Rune() {
	case 'e':
		if idx, item, ok := v.list.CurrentItem(); ok {
			v.onItemSelected(idx, item)
		}
		return nil
	case ' ':
		v.onToggleSelected()
		return nil
	case 'u':
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
		// ── Shortcuts (first section) ───────────────────────────────────────
		{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.config.section.shortcuts")},
		{Key: "mcp_services", Kind: CfgFieldLink, Label: i18n.T("tui.config.link.mcp"), LinkTarget: "project.mcp",
			Get: func() string { return "" }},
		{Key: "agents", Kind: CfgFieldLink, Label: i18n.T("tui.config.link.agents"), LinkTarget: "project.agents",
			Get: func() string { return "" }},
		{Key: "models", Kind: CfgFieldLink, Label: i18n.T("tui.config.link.models"), LinkTarget: "project.models",
			Get: func() string { return "" }},
		{Key: "workflow", Kind: CfgFieldLink, Label: i18n.T("tui.config.link.workflow"), LinkTarget: "workflow",
			Get: func() string { return "" }},

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
	}
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

// formatProjectValue is the legacy value formatter for project config views.
// It is still used by project_mcp_view.go until that view is migrated.
func formatProjectValue(val, kind string) string {
	switch kind {
	case "bool":
		switch val {
		case "true":
			return fmt.Sprintf("%s✓ %s%s", theme.ColorTag(theme.SuccessHex), i18n.T("tui.settings.enabled"), theme.TagColor)
		case "false":
			return fmt.Sprintf("%s✗ %s%s", theme.ColorTag(theme.ErrorHex), i18n.T("tui.settings.disabled"), theme.TagColor)
		default:
			return fmt.Sprintf("%s%s%s", theme.ColorTag(theme.TextMutedHex), val, theme.TagColor)
		}
	case "tri-bool":
		switch val {
		case "true":
			return fmt.Sprintf("%s✓ %s%s", theme.ColorTag(theme.SuccessHex), i18n.T("tui.settings.enabled"), theme.TagColor)
		case "false":
			return fmt.Sprintf("%s✗ %s%s", theme.ColorTag(theme.ErrorHex), i18n.T("tui.settings.disabled"), theme.TagColor)
		default:
			return fmt.Sprintf("%s↩ %s%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.settings.inherited"), theme.TagColor)
		}
	case "agents":
		if val == "" {
			return fmt.Sprintf("%s(%s)%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.project.no_agents"), theme.TagColor)
		}
		agents := strings.Split(val, ",")
		return fmt.Sprintf("%d agents", len(agents))
	case "select":
		if val == "" {
			return fmt.Sprintf("%s(%s)%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.settings.inherited"), theme.TagColor)
		}
		return val
	case "link":
		return fmt.Sprintf("%s→ %s%s", theme.ColorTag(theme.AccentHex), val, theme.TagColor)
	default:
		if val == "" || val == "(inherit)" {
			return fmt.Sprintf("%s%s%s", theme.ColorTag(theme.TextMutedHex), val, theme.TagColor)
		}
		return val
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectConfigView) renderFields() {
	if v.list == nil || v.live == nil {
		return
	}
	savedIdx := v.list.GetCurrentItem()

	items := make([]widgets.SectionItem, 0, len(v.fields))
	for i, f := range v.fields {
		item := renderConfigItem(f, 28)
		item.Reference = i
		items = append(items, item)
	}

	v.list.SetItems(items)
	if savedIdx >= 0 {
		v.list.SelectIndex(savedIdx)
	}
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

	// Agent editing is custom
	if f.Kind == CfgFieldAgents {
		v.editAgents()
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

func (v *ProjectConfigView) editAgents() {
	if v.shell == nil {
		return
	}
	allAgents := v.cfg.AllAgents()
	if len(allAgents) == 0 {
		v.shell.ShowToastMsg(i18n.T("tui.project.no_agents_available"), false)
		return
	}
	sort.Strings(allAgents)

	currentSet := make(map[string]bool)
	for _, a := range v.live.Agents {
		currentSet[a] = true
	}

	opts := make([]SelectOption, len(allAgents))
	for i, a := range allAgents {
		var label string
		if currentSet[a] {
			label = "✓ " + a
		} else {
			label = "  " + a
		}
		opts[i] = SelectOption{Label: label, Value: a}
	}

	v.shell.ShowSelectModal(i18n.T("tui.project.agents_select"), opts, "", func(chosen string) {
		if chosen == "" {
			return
		}
		v.pushUndo()
		if currentSet[chosen] {
			newAgents := make([]string, 0, len(v.live.Agents))
			for _, a := range v.live.Agents {
				if a != chosen {
					newAgents = append(newAgents, a)
				}
			}
			v.live.Agents = newAgents
		} else {
			v.live.Agents = append(v.live.Agents, chosen)
			sort.Strings(v.live.Agents)
		}
		v.scheduleAutoSave()
		v.renderFields()
		v.editAgents() // Re-open for multi-toggle
	})
}

func (v *ProjectConfigView) pushUndo() {
	snapshot := deepCopyProject(v.live)
	v.undoStack.Push(snapshot)
}

func (v *ProjectConfigView) scheduleAutoSave() {
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
		return
	}
	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.config.autosaved"), true)
	}
}

// ContextCommands implements CommandProvider.
func (v *ProjectConfigView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{ID: "project.undo", Label: i18n.T("tui.hints.undo"), Aliases: []string{"undo", "annuler"}, Description: i18n.T("tui.settings.cmd_undo"), Category: "Projet", Action: func() { v.undo() }},
	}
}
