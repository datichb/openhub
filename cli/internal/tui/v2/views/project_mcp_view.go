package views

import (
	"context"
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ProjectMCPViewConfig holds external dependencies for the project MCP view.
type ProjectMCPViewConfig struct {
	// GetProject returns a copy of the currently active project.
	GetProject func() *domain.Project
	// SaveProject persists the modified project to the DB.
	SaveProject func(ctx context.Context, p *domain.Project) error
	// ResolveMCPSource returns the resolution source for an MCP field.
	// Returns (effectiveValue, sourceAnnotation, isLocked).
	// If nil, no resolution annotations are shown.
	ResolveMCPSource func(service, field string) (effective string, source string, locked bool)
}

// ProjectMCPView displays and edits per-project MCP service overrides.
type ProjectMCPView struct {
	app      *tview.Application
	list     *widgets.SectionedList
	shell    ShellAccess
	cfg      ProjectMCPViewConfig
	mountGen uint64

	live      *domain.Project
	fields    []configField
	undoStack *widgets.UndoStack[domain.Project]
	autoSaver *AutoSaver
}

var _ View = (*ProjectMCPView)(nil)
var _ CommandProvider = (*ProjectMCPView)(nil)

// NewProjectMCPView creates the project MCP view.
func NewProjectMCPView(cfg ProjectMCPViewConfig) *ProjectMCPView {
	return &ProjectMCPView{
		cfg:       cfg,
		undoStack: widgets.NewUndoStack[domain.Project](10),
	}
}

func (v *ProjectMCPView) SetShell(s ShellAccess) { v.shell = s }
func (v *ProjectMCPView) ID() string             { return "project.mcp" }
func (v *ProjectMCPView) Title() string           { return i18n.T("tui.config.section.mcp_services") }
func (v *ProjectMCPView) StatusHints() string {
	return fmt.Sprintf("j/k %s · {/} %s · Enter %s · Space %s · u %s · Esc %s",
		i18n.T("tui.hints.nav"),
		i18n.T("tui.hints.navigate"),
		i18n.T("tui.hints.edit"),
		i18n.T("tui.hints.toggle"),
		i18n.T("tui.hints.undo"),
		i18n.T("tui.hints.back"),
	)
}

func (v *ProjectMCPView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.mountGen++
	gen := v.mountGen
	v.undoStack.Clear()
	v.live = v.cfg.GetProject()

	v.autoSaver = NewAutoSaver(200*time.Millisecond, app, func() {
		v.doSave()
	})

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

func (v *ProjectMCPView) Unmount() {
	if v.autoSaver != nil {
		v.autoSaver.Flush()
	}
	v.undoStack.Clear()
	v.app = nil
	v.list = nil
}

func (v *ProjectMCPView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
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
	case ' ':
		v.onToggleSelected()
		return nil
	case 'u':
		v.undo()
		return nil
	}
	return event
}

func (v *ProjectMCPView) undo() {
	prev, ok := v.undoStack.Pop()
	if !ok {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.config.nothing_to_undo"), true)
		}
		return
	}
	if v.live == nil {
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

// ContextCommands implements CommandProvider.
func (v *ProjectMCPView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{ID: "project.mcp.undo", Label: i18n.T("tui.hints.undo"), Aliases: []string{"undo", "annuler"}, Description: i18n.T("tui.settings.cmd_undo"), Category: "Projet", Action: func() { v.undo() }},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Field definitions
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectMCPView) buildFields() {
	type svcDef struct {
		name   string
		label  string
		fields []struct {
			key   string
			kind  FieldKind
			label string
			desc  string
		}
	}

	services := []svcDef{
		{name: "gitlab", label: "GitLab", fields: []struct {
			key   string
			kind  FieldKind
			label string
			desc  string
		}{
			{"enabled", CfgFieldTriBool, i18n.T("tui.config.field.mcp_enabled.label"), i18n.T("tui.config.field.mcp_enabled.desc")},
			{"url", CfgFieldString, i18n.T("tui.config.field.mcp_url.label"), i18n.T("tui.config.field.mcp_url.desc")},
			{"token_key", CfgFieldString, i18n.T("tui.config.field.mcp_token.label"), i18n.T("tui.config.field.mcp_token.desc")},
			{"write_enabled", CfgFieldTriBool, i18n.T("tui.config.field.mcp_write.label"), i18n.T("tui.config.field.mcp_write.desc")},
		}},
		{name: "jira", label: "Jira", fields: []struct {
			key   string
			kind  FieldKind
			label string
			desc  string
		}{
			{"enabled", CfgFieldTriBool, i18n.T("tui.config.field.mcp_enabled.label"), i18n.T("tui.config.field.mcp_enabled.desc")},
			{"url", CfgFieldString, i18n.T("tui.config.field.mcp_url.label"), i18n.T("tui.config.field.mcp_url.desc")},
		}},
		{name: "figma", label: "Figma", fields: []struct {
			key   string
			kind  FieldKind
			label string
			desc  string
		}{
			{"enabled", CfgFieldTriBool, i18n.T("tui.config.field.mcp_enabled.label"), i18n.T("tui.config.field.mcp_enabled.desc")},
		}},
		{name: "gslides", label: "Google Slides", fields: []struct {
			key   string
			kind  FieldKind
			label string
			desc  string
		}{
			{"enabled", CfgFieldTriBool, i18n.T("tui.config.field.mcp_enabled.label"), i18n.T("tui.config.field.mcp_enabled.desc")},
		}},
		{name: "team", label: "Team", fields: []struct {
			key   string
			kind  FieldKind
			label string
			desc  string
		}{
			{"enabled", CfgFieldTriBool, i18n.T("tui.config.field.mcp_enabled.label"), i18n.T("tui.config.field.mcp_enabled.desc")},
		}},
	}

	v.fields = nil
	for _, s := range services {
		svc := s // capture
		// Section header for each service
		v.fields = append(v.fields, configField{
			Kind:  CfgFieldSectionHeader,
			Label: svc.label,
		})

		for _, f := range svc.fields {
			fld := f // capture
			svcName := svc.name
			fieldKey := fld.key

			cf := configField{
				Section:     svcName,
				Key:         fieldKey,
				Kind:        fld.kind,
				Label:       fld.label,
				Description: fld.desc,
				Scope:       ScopeProject,
				Get:         mcpClosureGetter(v, svcName, fieldKey),
				Set:         mcpClosureSetter(v, svcName, fieldKey),
			}

			// Source and locked from resolution
			if v.cfg.ResolveMCPSource != nil {
				cf.Source = func() string {
					_, src, _ := v.cfg.ResolveMCPSource(svcName, fieldKey)
					return src
				}
				cf.Locked = func() bool {
					_, _, locked := v.cfg.ResolveMCPSource(svcName, fieldKey)
					return locked
				}
			}

			v.fields = append(v.fields, cf)
		}
	}
}

// mcpClosureGetter returns a zero-arg getter closure for an MCP service field.
func mcpClosureGetter(v *ProjectMCPView, service, field string) func() string {
	switch field {
	case "enabled":
		return func() string { return mcpServiceEnabled(v.live, service) }
	case "url":
		return func() string { return mcpServiceURL(v.live, service) }
	case "token_key":
		return func() string { return mcpServiceToken(v.live, service) }
	case "write_enabled":
		return func() string { return mcpServiceWriteEnabled(v.live, service) }
	default:
		return func() string { return "" }
	}
}

// mcpClosureSetter returns a zero-arg setter closure for an MCP service field.
func mcpClosureSetter(v *ProjectMCPView, service, field string) func(string) {
	switch field {
	case "enabled":
		return func(val string) { setMCPEnabled(v.live, service, val) }
	case "url":
		return func(val string) { setMCPURL(v.live, service, val) }
	case "token_key":
		return func(val string) { setMCPToken(v.live, service, val) }
	case "write_enabled":
		return func(val string) { setMCPWriteEnabled(v.live, service, val) }
	default:
		return nil
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectMCPView) renderFields() {
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

func (v *ProjectMCPView) onItemSelected(_ int, item widgets.SectionItem) {
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.fields) {
		return
	}
	f := &v.fields[ref]
	if !isEditable(f.Kind) || f.Get == nil || v.shell == nil {
		return
	}
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

func (v *ProjectMCPView) onToggleSelected() {
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

func (v *ProjectMCPView) pushUndo() {
	if v.live != nil {
		snapshot := deepCopyProject(v.live)
		v.undoStack.Push(snapshot)
	}
}

func (v *ProjectMCPView) scheduleAutoSave() {
	if v.autoSaver != nil {
		v.autoSaver.Schedule()
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Save (auto-save via AutoSaver)
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectMCPView) doSave() {
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

// ─────────────────────────────────────────────────────────────────────────────
// MCP data helpers (setters operate on live project)
// ─────────────────────────────────────────────────────────────────────────────

// mcpFieldGetter returns a typed getter for the legacy mcpFieldDef approach.
// Retained for backward compatibility during migration.
func mcpFieldGetter(service, field string) func(p *domain.Project) string {
	switch field {
	case "enabled":
		return func(p *domain.Project) string { return mcpServiceEnabled(p, service) }
	case "url":
		return func(p *domain.Project) string { return mcpServiceURL(p, service) }
	case "token_key":
		return func(p *domain.Project) string { return mcpServiceToken(p, service) }
	case "write_enabled":
		return func(p *domain.Project) string { return mcpServiceWriteEnabled(p, service) }
	default:
		return func(_ *domain.Project) string { return "" }
	}
}

func mcpFieldSetter(service, field string) func(p *domain.Project, val string) {
	switch field {
	case "enabled":
		return func(p *domain.Project, val string) { setMCPEnabled(p, service, val) }
	case "url":
		return func(p *domain.Project, val string) { setMCPURL(p, service, val) }
	case "token_key":
		return func(p *domain.Project, val string) { setMCPToken(p, service, val) }
	case "write_enabled":
		return func(p *domain.Project, val string) { setMCPWriteEnabled(p, service, val) }
	default:
		return nil
	}
}

func setMCPEnabled(p *domain.Project, name, val string) {
	if p.MCPConfig == nil {
		p.MCPConfig = &domain.ProjectMCPConfig{}
	}
	for i, svc := range p.MCPConfig.Services {
		if svc.Name == name {
			if val == "" {
				p.MCPConfig.Services[i].Enabled = nil
			} else {
				b := val == "true"
				p.MCPConfig.Services[i].Enabled = &b
			}
			return
		}
	}
	if val == "" {
		return
	}
	b := val == "true"
	p.MCPConfig.Services = append(p.MCPConfig.Services, domain.ProjectMCPService{Name: name, Enabled: &b})
}

func setMCPURL(p *domain.Project, name, val string) {
	if p.MCPConfig == nil {
		p.MCPConfig = &domain.ProjectMCPConfig{}
	}
	for i, svc := range p.MCPConfig.Services {
		if svc.Name == name {
			p.MCPConfig.Services[i].URL = val
			return
		}
	}
	if val == "" {
		return
	}
	p.MCPConfig.Services = append(p.MCPConfig.Services, domain.ProjectMCPService{Name: name, URL: val})
}

func setMCPToken(p *domain.Project, name, val string) {
	if p.MCPConfig == nil {
		p.MCPConfig = &domain.ProjectMCPConfig{}
	}
	for i, svc := range p.MCPConfig.Services {
		if svc.Name == name {
			p.MCPConfig.Services[i].TokenKey = val
			return
		}
	}
	if val == "" {
		return
	}
	p.MCPConfig.Services = append(p.MCPConfig.Services, domain.ProjectMCPService{Name: name, TokenKey: val})
}

func setMCPWriteEnabled(p *domain.Project, name, val string) {
	if p.MCPConfig == nil {
		p.MCPConfig = &domain.ProjectMCPConfig{}
	}
	for i, svc := range p.MCPConfig.Services {
		if svc.Name == name {
			if val == "" {
				p.MCPConfig.Services[i].WriteEnabled = nil
			} else {
				b := val == "true"
				p.MCPConfig.Services[i].WriteEnabled = &b
			}
			return
		}
	}
	if val == "" {
		return
	}
	b := val == "true"
	p.MCPConfig.Services = append(p.MCPConfig.Services, domain.ProjectMCPService{Name: name, WriteEnabled: &b})
}
