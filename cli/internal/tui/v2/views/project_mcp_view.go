package views

import (
	"context"
	"fmt"

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

// mcpFieldDef describes a single editable field within an MCP service section.
type mcpFieldDef struct {
	service string // "gitlab", "jira", "figma", "gslides", "team"
	key     string // display key
	kind    string // "tri-bool", "string"
	get     func(p *domain.Project) string
	set     func(p *domain.Project, val string)
}

// ProjectMCPView displays and edits per-project MCP service overrides.
type ProjectMCPView struct {
	app      *tview.Application
	list     *widgets.SectionedList
	shell    ShellAccess
	cfg      ProjectMCPViewConfig
	mountGen uint64

	live      *domain.Project
	dirty     bool
	fields    []mcpFieldDef
	undoStack *widgets.UndoStack[domain.Project]
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

// SetShell provides the shell reference for modal interactions.
func (v *ProjectMCPView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *ProjectMCPView) ID() string { return "project.mcp" }

// Title returns the display title.
func (v *ProjectMCPView) Title() string { return "MCP Services" }

// StatusHints returns keybinding hints for the omnibar.
func (v *ProjectMCPView) StatusHints() string {
	return fmt.Sprintf("j/k %s · Enter %s · Space %s · w %s · u %s · Esc %s",
		i18n.T("tui.hints.nav"),
		i18n.T("tui.hints.edit"),
		i18n.T("tui.hints.toggle"),
		i18n.T("tui.hints.save"),
		i18n.T("tui.hints.undo"),
		i18n.T("tui.hints.back"),
	)
}

// Mount builds the MCP configuration list.
func (v *ProjectMCPView) Mount(content *tview.Flex, app *tview.Application) {
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
	loading.SetText(fmt.Sprintf("\n  %s%s%s", muted, i18n.T("tui.project.loading"), theme.TagColor))
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
				v.editByIndex(index, item)
			})

			v.buildFields()
			v.renderList()
			content.RemoveItem(loading)
			content.AddItem(v.list, 0, 1, true)
			app.SetFocus(v.list)
		})
	}()
}

// Unmount cleans up resources.
func (v *ProjectMCPView) Unmount() {
	if v.dirty && v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.project.unsaved"), false)
	}
	v.undoStack.Clear()
	v.app = nil
	v.list = nil
}

// HandleKey processes view-specific key events.
func (v *ProjectMCPView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.live == nil {
		return event
	}
	switch event.Key() {
	case tcell.KeyEnter:
		if idx, item, ok := v.list.CurrentItem(); ok {
			v.editByIndex(idx, item)
		}
		return nil
	}
	switch event.Rune() {
	case ' ':
		v.toggleSelected()
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

// ContextCommands implements CommandProvider.
func (v *ProjectMCPView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{ID: "project.mcp.save", Label: i18n.T("tui.hints.save"), Aliases: []string{"save", "write", "sauvegarder"}, Description: i18n.T("tui.project.cmd_save"), Category: "Projet", Action: func() { v.save() }},
		{ID: "project.mcp.undo", Label: i18n.T("tui.hints.undo"), Aliases: []string{"undo", "annuler"}, Description: i18n.T("tui.settings.cmd_undo"), Category: "Projet", Action: func() { v.undo() }},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Field definitions
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectMCPView) buildFields() {
	type svc struct {
		name   string
		fields []struct {
			key  string
			kind string
		}
	}

	services := []svc{
		{name: "gitlab", fields: []struct{ key, kind string }{
			{"enabled", "tri-bool"},
			{"url", "string"},
			{"token_key", "string"},
			{"write_enabled", "tri-bool"},
		}},
		{name: "jira", fields: []struct{ key, kind string }{
			{"enabled", "tri-bool"},
			{"url", "string"},
		}},
		{name: "figma", fields: []struct{ key, kind string }{
			{"enabled", "tri-bool"},
		}},
		{name: "gslides", fields: []struct{ key, kind string }{
			{"enabled", "tri-bool"},
		}},
		{name: "team", fields: []struct{ key, kind string }{
			{"enabled", "tri-bool"},
		}},
	}

	v.fields = nil
	for _, s := range services {
		for _, f := range s.fields {
			fd := mcpFieldDef{service: s.name, key: f.key, kind: f.kind}
			fd.get = mcpFieldGetter(s.name, f.key)
			fd.set = mcpFieldSetter(s.name, f.key)
			v.fields = append(v.fields, fd)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectMCPView) renderList() {
	if v.list == nil || v.live == nil {
		return
	}
	savedIdx := v.list.GetCurrentItem()

	items := make([]widgets.SectionItem, 0, len(v.fields)+8)

	// Title header
	title := v.live.Name
	if v.dirty {
		title += "  " + theme.ColorTag(theme.AccentHex) + "● " + i18n.T("tui.settings.modified") + theme.TagColor
	}
	items = append(items, widgets.SectionItem{
		IsHeader: true,
		MainText: fmt.Sprintf("MCP Services: %s", title),
	})

	currentSection := ""
	for i, fd := range v.fields {
		// Section header when service changes
		if fd.service != currentSection {
			currentSection = fd.service
			items = append(items, widgets.SectionItem{
				IsHeader: true,
				MainText: mcpSectionLabel(fd.service),
			})
		}

		val := fd.get(v.live)
		valDisplay := formatProjectValue(val, fd.kind)

		// Source annotation
		sourceAnnotation := ""
		if v.cfg.ResolveMCPSource != nil {
			if _, src, _ := v.cfg.ResolveMCPSource(fd.service, fd.key); src != "" {
				sourceAnnotation = fmt.Sprintf("  %s%s%s", theme.ColorTag(theme.TextMutedHex), src, theme.TagColor)
			}
		}

		mainText := fmt.Sprintf("%-28s %s%s", fd.key+":", valDisplay, sourceAnnotation)

		locked := false
		if v.cfg.ResolveMCPSource != nil {
			_, _, locked = v.cfg.ResolveMCPSource(fd.service, fd.key)
		}

		items = append(items, widgets.SectionItem{
			MainText:  mainText,
			Locked:    locked,
			Reference: i,
		})
	}

	v.list.SetItems(items)
	if savedIdx >= 0 {
		v.list.SelectIndex(savedIdx)
	}
}

func mcpSectionLabel(service string) string {
	switch service {
	case "gitlab":
		return "GitLab"
	case "jira":
		return "Jira"
	case "figma":
		return "Figma"
	case "gslides":
		return "Google Slides"
	case "team":
		return "Team"
	default:
		return service
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Editing
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectMCPView) editByIndex(_ int, item widgets.SectionItem) {
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.fields) {
		return
	}
	fd := v.fields[ref]
	if v.shell == nil || fd.get == nil {
		return
	}

	// Check lock
	if v.cfg.ResolveMCPSource != nil {
		if _, _, locked := v.cfg.ResolveMCPSource(fd.service, fd.key); locked {
			v.shell.ShowToastMsg(i18n.T("tui.project.locked"), false)
			return
		}
	}

	switch fd.kind {
	case "tri-bool":
		opts := []SelectOption{
			{Label: "↩ " + i18n.T("tui.settings.inherited"), Value: "(inherit)"},
			{Label: "✓ " + i18n.T("tui.settings.enabled"), Value: "true"},
			{Label: "✗ " + i18n.T("tui.settings.disabled"), Value: "false"},
		}
		cur := fd.get(v.live)
		v.shell.ShowSelectModal(fd.key, opts, cur, func(newVal string) {
			if fd.set != nil {
				v.pushUndo()
				fd.set(v.live, newVal)
				v.dirty = true
				v.renderList()
			}
		})

	default: // string
		cur := fd.get(v.live)
		v.shell.ShowInputModal(fd.key, cur, func(newVal string) {
			if fd.set != nil {
				v.pushUndo()
				fd.set(v.live, newVal)
				v.dirty = true
				v.renderList()
			}
		})
	}
}

func (v *ProjectMCPView) toggleSelected() {
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
	fd := v.fields[ref]

	// Check lock
	if v.cfg.ResolveMCPSource != nil {
		if _, _, locked := v.cfg.ResolveMCPSource(fd.service, fd.key); locked {
			if v.shell != nil {
				v.shell.ShowToastMsg(i18n.T("tui.project.locked"), false)
			}
			return
		}
	}

	if fd.kind != "tri-bool" || fd.set == nil {
		return
	}

	cur := fd.get(v.live)
	var newVal string
	switch cur {
	case "true":
		newVal = "false"
	case "false":
		newVal = "(inherit)"
	default:
		newVal = "true"
	}
	v.pushUndo()
	fd.set(v.live, newVal)
	v.dirty = true
	v.renderList()
}

func (v *ProjectMCPView) pushUndo() {
	snapshot := deepCopyProject(v.live)
	v.undoStack.Push(snapshot)
}

// ─────────────────────────────────────────────────────────────────────────────
// Save / Undo
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectMCPView) save() {
	if v.shell == nil || v.live == nil {
		return
	}
	ctx := v.shell.Context()
	if err := v.cfg.SaveProject(ctx, v.live); err != nil {
		v.shell.ShowToastMsg(i18n.T("tui.project.save_error")+": "+err.Error(), false)
		return
	}
	v.dirty = false
	v.undoStack.Clear()
	v.renderList()
	v.shell.ShowToastMsg(i18n.T("tui.project.saved"), true)
}

func (v *ProjectMCPView) undo() {
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

// ─────────────────────────────────────────────────────────────────────────────
// Standalone MCP data helpers
// ─────────────────────────────────────────────────────────────────────────────

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

// setMCPEnabled sets the Enabled field for the given service.
func setMCPEnabled(p *domain.Project, name, val string) {
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
			return
		}
	}
	if val == "(inherit)" || val == "" {
		return
	}
	b := val == "true"
	p.MCPConfig.Services = append(p.MCPConfig.Services, domain.ProjectMCPService{Name: name, Enabled: &b})
}

// setMCPURL sets the URL field for the given service.
func setMCPURL(p *domain.Project, name, val string) {
	if p.MCPConfig == nil {
		p.MCPConfig = &domain.ProjectMCPConfig{}
	}
	for i, svc := range p.MCPConfig.Services {
		if svc.Name == name {
			if val == "(inherit)" || val == "" {
				p.MCPConfig.Services[i].URL = ""
			} else {
				p.MCPConfig.Services[i].URL = val
			}
			return
		}
	}
	if val == "(inherit)" || val == "" {
		return
	}
	p.MCPConfig.Services = append(p.MCPConfig.Services, domain.ProjectMCPService{Name: name, URL: val})
}

// setMCPToken sets the TokenKey field for the given service.
func setMCPToken(p *domain.Project, name, val string) {
	if p.MCPConfig == nil {
		p.MCPConfig = &domain.ProjectMCPConfig{}
	}
	for i, svc := range p.MCPConfig.Services {
		if svc.Name == name {
			if val == "(inherit)" || val == "" {
				p.MCPConfig.Services[i].TokenKey = ""
			} else {
				p.MCPConfig.Services[i].TokenKey = val
			}
			return
		}
	}
	if val == "(inherit)" || val == "" {
		return
	}
	p.MCPConfig.Services = append(p.MCPConfig.Services, domain.ProjectMCPService{Name: name, TokenKey: val})
}

// setMCPWriteEnabled sets the WriteEnabled field for the given service.
func setMCPWriteEnabled(p *domain.Project, name, val string) {
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
			return
		}
	}
	if val == "(inherit)" || val == "" {
		return
	}
	b := val == "true"
	p.MCPConfig.Services = append(p.MCPConfig.Services, domain.ProjectMCPService{Name: name, WriteEnabled: &b})
}
