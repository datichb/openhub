package views

import (
	"context"
	"fmt"
	"sort"
	"strings"

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

// projectConfigLine is a single editable row in the project config view.
type projectConfigLine struct {
	section string
	key     string
	kind    string // "string", "bool", "agents", "section-header", "readonly", "select", "tri-bool"
	// options holds allowed values for "select" kind fields.
	options []SelectOption
	// optionsFunc returns dynamic allowed values.
	optionsFunc func() []SelectOption
	// validator holds optional validation rules.
	validator *FieldValidator
	get       func(p *domain.Project) string
	set       func(p *domain.Project, v string)
	// source returns the resolution source annotation (e.g., "[hub]", "[equipe: enforced]").
	source func(p *domain.Project) string
	// locked indicates this field is enforced by the team and cannot be edited.
	locked func(p *domain.Project) bool
}

// ProjectConfigView displays and edits the active project's configuration.
type ProjectConfigView struct {
	app      *tview.Application
	list     *widgets.SectionedList
	shell    ShellAccess
	cfg      ProjectConfigViewConfig
	mountGen uint64

	live       *domain.Project
	dirty      bool
	mcpChanged bool // track if MCP fields changed (triggers redeploy prompt)
	lines      []projectConfigLine
	undoStack  *widgets.UndoStack[domain.Project]
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
	return fmt.Sprintf("j/k %s · {/} %s · Space %s · e %s · w %s · u %s · Esc %s",
		i18n.T("tui.hints.nav"),
		i18n.T("tui.hints.navigate"),
		i18n.T("tui.hints.toggle"),
		i18n.T("tui.hints.edit"),
		i18n.T("tui.hints.save"),
		i18n.T("tui.hints.undo"),
		i18n.T("tui.hints.back"),
	)
}

func (v *ProjectConfigView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.mountGen++
	gen := v.mountGen
	v.dirty = false
	v.mcpChanged = false
	v.undoStack.Clear()
	v.live = v.cfg.GetProject() // synchronous: always available for save()

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

			v.buildLines()
			v.renderLines()
			content.RemoveItem(loading)
			content.AddItem(v.list, 0, 1, true)
			app.SetFocus(v.list)
		})
	}()
}

func (v *ProjectConfigView) Unmount() {
	if v.dirty && v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.project.unsaved"), false)
	}
	v.undoStack.Clear()
	v.app = nil
	v.list = nil
}

func (v *ProjectConfigView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
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
	case 'e':
		if idx, item, ok := v.list.CurrentItem(); ok {
			v.editByIndex(idx, item)
		}
		return nil
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

func (v *ProjectConfigView) undo() {
	prev, ok := v.undoStack.Pop()
	if !ok {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.settings.nothing_to_undo"), true)
		}
		return
	}
	*v.live = prev
	v.dirty = v.undoStack.Len() > 0
	v.renderLines()
	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.settings.undone"), true)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Line definitions
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectConfigView) buildLines() {
	languageOptions := []SelectOption{
		{Label: "Français", Value: "fr"},
		{Label: "English", Value: "en"},
		{Label: fmt.Sprintf("(%s)", i18n.T("tui.settings.inherited")), Value: ""},
	}
	providerOptionsFunc := func() []SelectOption {
		opts := []SelectOption{{Label: fmt.Sprintf("(%s)", i18n.T("tui.settings.inherited")), Value: ""}}
		for _, p := range provider.AllProviders() {
			opts = append(opts, SelectOption{Label: string(p), Value: string(p)})
		}
		return opts
	}
	statusOptions := []SelectOption{
		{Label: "active", Value: "active"},
		{Label: "archived", Value: "archived"},
	}
	teamModeOptions := []SelectOption{
		{Label: fmt.Sprintf("(%s)", i18n.T("tui.settings.inherited")), Value: domain.ProjectTeamModeInherit},
		{Label: "custom", Value: domain.ProjectTeamModeCustom},
		{Label: "disabled", Value: domain.ProjectTeamModeDisabled},
	}
	triBoolOptions := []SelectOption{
		{Label: "↩ " + i18n.T("tui.settings.inherited"), Value: "(inherit)"},
		{Label: "✓ " + i18n.T("tui.settings.yes"), Value: "true"},
		{Label: "✗ " + i18n.T("tui.settings.no"), Value: "false"},
	}

	v.lines = []projectConfigLine{
		// ── Général ──────────────────────────────────────────────────────────
		{kind: "section-header", section: i18n.T("tui.settings.section_general")},
		{section: i18n.T("tui.settings.section_general"), key: "id", kind: "readonly",
			get: func(p *domain.Project) string { return p.ID }},
		{section: i18n.T("tui.settings.section_general"), key: "name", kind: "string",
			get: func(p *domain.Project) string { return p.Name },
			set: func(p *domain.Project, v string) { p.Name = v }},
		{section: i18n.T("tui.settings.section_general"), key: "path", kind: "string",
			get: func(p *domain.Project) string { return p.Path },
			set: func(p *domain.Project, v string) { p.Path = v }},
		{section: i18n.T("tui.settings.section_general"), key: "language", kind: "select",
			options: languageOptions,
			get:     func(p *domain.Project) string { return p.Language },
			set:     func(p *domain.Project, v string) { p.Language = v }},
		{section: i18n.T("tui.settings.section_general"), key: "provider", kind: "select",
			optionsFunc: providerOptionsFunc,
			get:         func(p *domain.Project) string { return p.Provider },
			set:         func(p *domain.Project, v string) { p.Provider = v }},
		{section: i18n.T("tui.settings.section_general"), key: "model", kind: "string",
			get: func(p *domain.Project) string { return p.Model },
			set: func(p *domain.Project, v string) { p.Model = v }},
		{section: i18n.T("tui.settings.section_general"), key: "status", kind: "select",
			options: statusOptions,
			get:     func(p *domain.Project) string { return string(p.Status) },
			set:     func(p *domain.Project, v string) { p.Status = domain.ProjectStatus(v) }},

		// ── Team ─────────────────────────────────────────────────────────────
		{kind: "section-header", section: "Team"},
		{section: "Team", key: "mode", kind: "select",
			options: teamModeOptions,
			get: func(p *domain.Project) string {
				if p.TeamConfig == nil { return domain.ProjectTeamModeInherit }
				return p.TeamConfig.Mode
			},
			set: func(p *domain.Project, val string) {
				if p.TeamConfig == nil { p.TeamConfig = &domain.ProjectTeamConfig{} }
				p.TeamConfig.Mode = val
			}},
		{section: "Team", key: "member_id", kind: "string",
			get: func(p *domain.Project) string {
				if p.TeamConfig == nil { return "" }
				return p.TeamConfig.MemberID
			},
			set: func(p *domain.Project, val string) {
				if p.TeamConfig == nil { p.TeamConfig = &domain.ProjectTeamConfig{} }
				p.TeamConfig.MemberID = val
			}},
		{section: "Team", key: "state_repo", kind: "string",
			get: func(p *domain.Project) string {
				if p.TeamConfig == nil { return "" }
				return p.TeamConfig.StateRepo
			},
			set: func(p *domain.Project, val string) {
				if p.TeamConfig == nil { p.TeamConfig = &domain.ProjectTeamConfig{} }
				p.TeamConfig.StateRepo = val
			}},
		{section: "Team", key: "state_path", kind: "string",
			get: func(p *domain.Project) string {
				if p.TeamConfig == nil { return "" }
				return p.TeamConfig.StatePath
			},
			set: func(p *domain.Project, val string) {
				if p.TeamConfig == nil { p.TeamConfig = &domain.ProjectTeamConfig{} }
				p.TeamConfig.StatePath = val
			}},

		// ── MCP Overrides ────────────────────────────────────────────────────
		{kind: "section-header", section: "MCP Services"},
		// GitLab
		{section: "MCP", key: "gitlab enabled", kind: "tri-bool", options: triBoolOptions,
			get:    func(p *domain.Project) string { return mcpServiceEnabled(p, "gitlab") },
			set:    v.mcpSetEnabled("gitlab"),
			source: v.mcpSource("gitlab", "enabled"),
			locked: v.mcpLocked("gitlab", "enabled")},
		{section: "MCP", key: "gitlab url", kind: "string",
			get:    func(p *domain.Project) string { return mcpServiceURL(p, "gitlab") },
			set:    v.mcpSetURL("gitlab"),
			source: v.mcpSource("gitlab", "url"),
			locked: v.mcpLocked("gitlab", "url")},
		{section: "MCP", key: "gitlab token", kind: "string",
			get: func(p *domain.Project) string { return mcpServiceToken(p, "gitlab") },
			set: v.mcpSetToken("gitlab")},
		{section: "MCP", key: "gitlab write", kind: "tri-bool", options: triBoolOptions,
			get: func(p *domain.Project) string { return mcpServiceWriteEnabled(p, "gitlab") },
			set: v.mcpSetWriteEnabled("gitlab")},
		// Jira
		{section: "MCP", key: "jira enabled", kind: "tri-bool", options: triBoolOptions,
			get:    func(p *domain.Project) string { return mcpServiceEnabled(p, "jira") },
			set:    v.mcpSetEnabled("jira"),
			source: v.mcpSource("jira", "enabled"),
			locked: v.mcpLocked("jira", "enabled")},
		{section: "MCP", key: "jira url", kind: "string",
			get:    func(p *domain.Project) string { return mcpServiceURL(p, "jira") },
			set:    v.mcpSetURL("jira"),
			source: v.mcpSource("jira", "url"),
			locked: v.mcpLocked("jira", "url")},
		// Figma
		{section: "MCP", key: "figma enabled", kind: "tri-bool", options: triBoolOptions,
			get:    func(p *domain.Project) string { return mcpServiceEnabled(p, "figma") },
			set:    v.mcpSetEnabled("figma"),
			source: v.mcpSource("figma", "enabled"),
			locked: v.mcpLocked("figma", "enabled")},
		// GSlides
		{section: "MCP", key: "gslides enabled", kind: "tri-bool", options: triBoolOptions,
			get:    func(p *domain.Project) string { return mcpServiceEnabled(p, "gslides") },
			set:    v.mcpSetEnabled("gslides"),
			source: v.mcpSource("gslides", "enabled"),
			locked: v.mcpLocked("gslides", "enabled")},
		// Team MCP server
		{section: "MCP", key: "team enabled", kind: "tri-bool", options: triBoolOptions,
			get: func(p *domain.Project) string { return mcpServiceEnabled(p, "team") },
			set: v.mcpSetEnabled("team")},

		// ── Agents ───────────────────────────────────────────────────────────
		{kind: "section-header", section: "Agents"},
		{section: "Agents", key: "agents", kind: "agents",
			get: func(p *domain.Project) string { return strings.Join(p.Agents, ", ") },
			set: func(p *domain.Project, val string) {
				if val == "" { p.Agents = nil; return }
				parts := strings.Split(val, ",")
				p.Agents = p.Agents[:0]
				for _, a := range parts {
					if s := strings.TrimSpace(a); s != "" { p.Agents = append(p.Agents, s) }
				}
			}},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// MCP helpers (operate on ProjectMCPConfig)
// ─────────────────────────────────────────────────────────────────────────────

func mcpServiceEnabled(p *domain.Project, name string) string {
	if p.MCPConfig == nil { return "(inherit)" }
	for _, svc := range p.MCPConfig.Services {
		if svc.Name == name && svc.Enabled != nil { return boolStr(*svc.Enabled) }
	}
	return "(inherit)"
}

func mcpServiceWriteEnabled(p *domain.Project, name string) string {
	if p.MCPConfig == nil { return "(inherit)" }
	for _, svc := range p.MCPConfig.Services {
		if svc.Name == name && svc.WriteEnabled != nil { return boolStr(*svc.WriteEnabled) }
	}
	return "(inherit)"
}

func (v *ProjectConfigView) mcpSetEnabled(name string) func(p *domain.Project, val string) {
	return func(p *domain.Project, val string) {
		if p.MCPConfig == nil { p.MCPConfig = &domain.ProjectMCPConfig{} }
		for i, svc := range p.MCPConfig.Services {
			if svc.Name == name {
				if val == "(inherit)" || val == "" { p.MCPConfig.Services[i].Enabled = nil } else { b := val == "true"; p.MCPConfig.Services[i].Enabled = &b }
				v.mcpChanged = true; return
			}
		}
		if val == "(inherit)" || val == "" { return }
		b := val == "true"
		p.MCPConfig.Services = append(p.MCPConfig.Services, domain.ProjectMCPService{Name: name, Enabled: &b})
		v.mcpChanged = true
	}
}

func (v *ProjectConfigView) mcpSetWriteEnabled(name string) func(p *domain.Project, val string) {
	return func(p *domain.Project, val string) {
		if p.MCPConfig == nil { p.MCPConfig = &domain.ProjectMCPConfig{} }
		for i, svc := range p.MCPConfig.Services {
			if svc.Name == name {
				if val == "(inherit)" || val == "" { p.MCPConfig.Services[i].WriteEnabled = nil } else { b := val == "true"; p.MCPConfig.Services[i].WriteEnabled = &b }
				v.mcpChanged = true; return
			}
		}
		if val == "(inherit)" || val == "" { return }
		b := val == "true"
		p.MCPConfig.Services = append(p.MCPConfig.Services, domain.ProjectMCPService{Name: name, WriteEnabled: &b})
		v.mcpChanged = true
	}
}

func mcpServiceURL(p *domain.Project, name string) string {
	if p.MCPConfig == nil { return "(inherit)" }
	for _, svc := range p.MCPConfig.Services {
		if svc.Name == name && svc.URL != "" { return svc.URL }
	}
	return "(inherit)"
}

func mcpServiceToken(p *domain.Project, name string) string {
	if p.MCPConfig == nil { return "(inherit)" }
	for _, svc := range p.MCPConfig.Services {
		if svc.Name == name && svc.TokenKey != "" { return svc.TokenKey }
	}
	return "(inherit)"
}

func (v *ProjectConfigView) mcpSetURL(name string) func(p *domain.Project, val string) {
	return func(p *domain.Project, val string) {
		if p.MCPConfig == nil { p.MCPConfig = &domain.ProjectMCPConfig{} }
		for i, svc := range p.MCPConfig.Services {
			if svc.Name == name {
				if val == "(inherit)" || val == "" { p.MCPConfig.Services[i].URL = "" } else { p.MCPConfig.Services[i].URL = val }
				v.mcpChanged = true; return
			}
		}
		if val == "(inherit)" || val == "" { return }
		p.MCPConfig.Services = append(p.MCPConfig.Services, domain.ProjectMCPService{Name: name, URL: val})
		v.mcpChanged = true
	}
}

func (v *ProjectConfigView) mcpSetToken(name string) func(p *domain.Project, val string) {
	return func(p *domain.Project, val string) {
		if p.MCPConfig == nil { p.MCPConfig = &domain.ProjectMCPConfig{} }
		for i, svc := range p.MCPConfig.Services {
			if svc.Name == name {
				if val == "(inherit)" || val == "" { p.MCPConfig.Services[i].TokenKey = "" } else { p.MCPConfig.Services[i].TokenKey = val }
				v.mcpChanged = true; return
			}
		}
		if val == "(inherit)" || val == "" { return }
		p.MCPConfig.Services = append(p.MCPConfig.Services, domain.ProjectMCPService{Name: name, TokenKey: val})
		v.mcpChanged = true
	}
}

func (v *ProjectConfigView) mcpSource(service, field string) func(p *domain.Project) string {
	return func(_ *domain.Project) string {
		if v.cfg.ResolveMCPSource == nil { return "" }
		_, source, _ := v.cfg.ResolveMCPSource(service, field)
		return source
	}
}

func (v *ProjectConfigView) mcpLocked(service, field string) func(p *domain.Project) bool {
	return func(_ *domain.Project) bool {
		if v.cfg.ResolveMCPSource == nil { return false }
		_, _, locked := v.cfg.ResolveMCPSource(service, field)
		return locked
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

	items := make([]widgets.SectionItem, 0, len(v.lines)+1)

	// Title item (as header)
	title := v.live.Name
	if v.dirty {
		title += "  " + theme.ColorTag(theme.AccentHex) + "● " + i18n.T("tui.settings.modified") + theme.TagColor
	}
	items = append(items, widgets.SectionItem{
		IsHeader: true,
		MainText: fmt.Sprintf("Projet: %s", title),
	})

	for i, line := range v.lines {
		if line.kind == "section-header" {
			items = append(items, widgets.SectionItem{
				IsHeader: true,
				MainText: line.section,
			})
			continue
		}

		val := ""
		if line.get != nil {
			val = line.get(v.live)
		}

		isLocked := line.locked != nil && line.locked(v.live)
		valDisplay := formatProjectValue(val, line.kind)

		// Source annotation
		sourceAnnotation := ""
		if line.source != nil {
			if src := line.source(v.live); src != "" {
				sourceAnnotation = fmt.Sprintf("  %s%s%s", theme.ColorTag(theme.TextMutedHex), src, theme.TagColor)
			}
		}

		readonlyMarker := ""
		if line.kind == "readonly" {
			readonlyMarker = fmt.Sprintf("  %s(%s)%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.settings.readonly"), theme.TagColor)
		}

		mainText := fmt.Sprintf("%-28s %s%s%s", line.key+":", valDisplay, sourceAnnotation, readonlyMarker)

		items = append(items, widgets.SectionItem{
			MainText:  mainText,
			Locked:    isLocked,
			Reference: i,
		})
	}

	v.list.SetItems(items)
	if savedIdx >= 0 {
		v.list.SelectIndex(savedIdx)
	}
}

func formatProjectValue(val, kind string) string {
	switch kind {
	case "bool":
		switch val {
		case "true":
			return fmt.Sprintf("%s✓ true%s", theme.ColorTag(theme.SuccessHex), theme.TagColor)
		case "false":
			return fmt.Sprintf("%s✗ false%s", theme.ColorTag(theme.ErrorHex), theme.TagColor)
		default:
			return fmt.Sprintf("%s%s%s", theme.ColorTag(theme.TextMutedHex), val, theme.TagColor)
		}
	case "tri-bool":
		switch val {
		case "true":
			return fmt.Sprintf("%s✓ %s%s", theme.ColorTag(theme.SuccessHex), i18n.T("tui.settings.yes"), theme.TagColor)
		case "false":
			return fmt.Sprintf("%s✗ %s%s", theme.ColorTag(theme.ErrorHex), i18n.T("tui.settings.no"), theme.TagColor)
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

func (v *ProjectConfigView) editByIndex(_ int, item widgets.SectionItem) {
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.lines) {
		return
	}
	line := v.lines[ref]
	if v.shell == nil || line.kind == "section-header" || line.kind == "readonly" || line.get == nil {
		return
	}

	// Check lock (enforced by team)
	if line.locked != nil && line.locked(v.live) {
		v.shell.ShowToastMsg(i18n.T("tui.project.locked"), false)
		return
	}

	switch line.kind {
	case "bool":
		v.toggleByRef(ref)

	case "tri-bool":
		opts := line.options
		if len(opts) == 0 {
			opts = []SelectOption{
				{Label: "↩ " + i18n.T("tui.settings.inherited"), Value: "(inherit)"},
				{Label: "✓ " + i18n.T("tui.settings.yes"), Value: "true"},
				{Label: "✗ " + i18n.T("tui.settings.no"), Value: "false"},
			}
		}
		cur := line.get(v.live)
		v.shell.ShowSelectModal(line.key, opts, cur, func(newVal string) {
			if line.set != nil {
				v.pushUndo()
				line.set(v.live, newVal)
				v.dirty = true
				v.renderLines()
			}
		})

	case "select":
		opts := line.options
		if line.optionsFunc != nil {
			opts = line.optionsFunc()
		}
		cur := line.get(v.live)
		v.shell.ShowSelectModal(line.key, opts, cur, func(newVal string) {
			if line.set != nil {
				v.pushUndo()
				line.set(v.live, newVal)
				v.dirty = true
				v.renderLines()
			}
		})

	case "agents":
		v.editAgents(line)

	default: // string
		cur := line.get(v.live)
		v.shell.ShowInputModal(line.key, cur, func(newVal string) {
			if line.set != nil {
				if line.validator != nil {
					if err := line.validator.Validate(newVal); err != nil {
						v.shell.ShowToastMsg("⚠ "+err.Error(), false)
						return
					}
				}
				v.pushUndo()
				line.set(v.live, newVal)
				v.dirty = true
				v.renderLines()
			}
		})
	}
}

func (v *ProjectConfigView) toggleSelected() {
	if v.list == nil {
		return
	}
	_, item, ok := v.list.CurrentItem()
	if !ok {
		return
	}
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.lines) {
		return
	}
	line := v.lines[ref]
	if line.locked != nil && line.locked(v.live) {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.project.locked"), false)
		}
		return
	}
	v.toggleByRef(ref)
}

func (v *ProjectConfigView) toggleByRef(ref int) {
	line := v.lines[ref]
	if line.set == nil {
		return
	}
	cur := line.get(v.live)

	switch line.kind {
	case "bool":
		newVal := "true"
		if cur == "true" { newVal = "false" }
		v.pushUndo()
		line.set(v.live, newVal)
		v.dirty = true
		v.renderLines()

	case "tri-bool":
		var newVal string
		switch cur {
		case "true": newVal = "false"
		case "false": newVal = "(inherit)"
		default: newVal = "true"
		}
		v.pushUndo()
		line.set(v.live, newVal)
		v.dirty = true
		v.renderLines()
	}
}

func (v *ProjectConfigView) editAgents(line projectConfigLine) {
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
		label := a
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
				if a != chosen { newAgents = append(newAgents, a) }
			}
			v.live.Agents = newAgents
		} else {
			v.live.Agents = append(v.live.Agents, chosen)
			sort.Strings(v.live.Agents)
		}
		v.dirty = true
		v.renderLines()
		v.editAgents(line) // Re-open for multi-toggle
	})
}

func (v *ProjectConfigView) pushUndo() {
	snapshot := deepCopyProject(v.live)
	v.undoStack.Push(snapshot)
}

// ─────────────────────────────────────────────────────────────────────────────
// Save
// ─────────────────────────────────────────────────────────────────────────────

func (v *ProjectConfigView) save() {
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
	v.shell.ShowToastMsg(i18n.T("tui.project.saved"), true)

	// Propose redeploy if MCP changed
	if v.mcpChanged && v.cfg.Deploy != nil {
		v.shell.ShowSelectModal(
			i18n.T("tui.project.mcp_changed"),
			[]SelectOption{
				{Label: i18n.T("tui.project.redeploy_now"), Value: "deploy"},
				{Label: i18n.T("tui.project.redeploy_later"), Value: "later"},
			},
			"",
			func(choice string) {
				if choice == "deploy" {
					go func() {
						if err := v.cfg.Deploy(ctx, v.live); err != nil {
							if v.app != nil {
								v.app.QueueUpdateDraw(func() {
									v.shell.ShowToastMsg(i18n.T("tui.project.deploy_error")+": "+err.Error(), false)
								})
							}
							return
						}
						if v.app != nil {
							v.app.QueueUpdateDraw(func() {
								v.shell.ShowToastMsg(i18n.T("tui.project.redeployed"), true)
							})
						}
					}()
				} else {
					v.shell.ShowToastMsg(i18n.T("tui.project.deploy_reminder"), true)
				}
				v.mcpChanged = false
			})
	}
}

// ContextCommands implements CommandProvider.
func (v *ProjectConfigView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{ID: "project.save", Label: i18n.T("tui.hints.save"), Aliases: []string{"save", "write", "sauvegarder"}, Description: i18n.T("tui.project.cmd_save"), Category: "Projet", Action: func() { v.save() }},
		{ID: "project.undo", Label: i18n.T("tui.hints.undo"), Aliases: []string{"undo", "annuler"}, Description: i18n.T("tui.settings.cmd_undo"), Category: "Projet", Action: func() { v.undo() }},
	}
}
