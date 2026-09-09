package views

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// SettingsViewConfig holds external dependencies for the hub config view.
type SettingsViewConfig struct {
	// GetConfig returns the live hub config (not a copy).
	GetConfig func() *config.Config
	// ReloadConfig reloads the config from disk (for undo/refresh).
	// Returns the refreshed live config pointer.
	ReloadConfig func() *config.Config
	// SaveConfig persists the modified config to hub.toml.
	SaveConfig func(c *config.Config) error
	// CheckSecret tests whether a keychain key has a value stored.
	// Returns ("", nil) if absent, (masked, nil) if present.
	CheckSecret func(ctx context.Context, key string) (present bool, masked string)
	// SetSecret stores a new secret value in the keychain.
	SetSecret func(ctx context.Context, key, value string) error
}

// configLine represents a single editable line in the hub config view.
type configLine struct {
	section  string // e.g. "MCP GitLab" — internal ID (not translated)
	label    string // display label (translated)
	key      string // e.g. "enabled"
	kind     string // "bool", "string", "tokenkey", "section-header", "select", "tri-bool", "int", "link", "readonly"
	// options holds the allowed values for "select" kind fields.
	options []SelectOption
	// optionsFunc returns dynamic allowed values (takes precedence over options).
	optionsFunc func() []SelectOption
	// validator holds optional validation rules.
	validator *FieldValidator
	// linkTarget is the view ID to navigate to for "link" kind.
	linkTarget string
	// get/set operate on the live *config.Config pointer held by the view
	get func(c *config.Config) string
	set func(c *config.Config, v string)
}

// SettingsView displays and edits hub.toml line by line.
type SettingsView struct {
	app      *tview.Application
	list     *widgets.SectionedList
	shell    ShellAccess
	cfg      SettingsViewConfig
	mountGen uint64

	// live config being edited (copy from disk, modified in memory until saved)
	live  *config.Config
	dirty bool

	lines     []configLine
	undoStack *widgets.UndoStack[config.Config]
}

var _ View = (*SettingsView)(nil)
var _ CommandProvider = (*SettingsView)(nil)

// NewSettingsView creates the hub config view.
func NewSettingsView(cfg SettingsViewConfig) *SettingsView {
	return &SettingsView{
		cfg:       cfg,
		undoStack: widgets.NewUndoStack[config.Config](10),
	}
}

// SetShell provides shell access for modals/toasts.
func (v *SettingsView) SetShell(s ShellAccess) { v.shell = s }

func (v *SettingsView) ID() string    { return "settings" }
func (v *SettingsView) Title() string { return "Settings" }
func (v *SettingsView) StatusHints() string {
	return fmt.Sprintf("j/k %s · {/} %s · Space %s · Enter %s · w %s · u %s · r %s",
		i18n.T("tui.hints.nav"),
		i18n.T("tui.hints.navigate"),
		i18n.T("tui.hints.toggle"),
		i18n.T("tui.hints.edit"),
		i18n.T("tui.hints.save"),
		i18n.T("tui.hints.undo"),
		i18n.T("tui.hints.refresh"),
	)
}

// Mount builds and displays the view.
func (v *SettingsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.mountGen++
	gen := v.mountGen
	v.dirty = false
	v.undoStack.Clear()
	v.live = v.cfg.GetConfig() // synchronous: always available for save()

	// Show loading placeholder immediately
	loading := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	loading.SetBackgroundColor(theme.BgPanel)
	muted := theme.ColorTag(theme.TextMutedHex)
	loading.SetText(fmt.Sprintf("\n  %s%s%s", muted, i18n.T("tui.settings.loading"), theme.TagColor))
	content.AddItem(loading, 0, 1, true)

	// Build list asynchronously (CheckSecret may do I/O)
	go func() {
		app.QueueUpdateDraw(func() {
			if v.app == nil || v.mountGen != gen {
				return
			}

			v.list = widgets.NewSectionedList()
			v.list.SetApp(app)
			v.list.SetBorderPadding(1, 0, 2, 2)

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

func (v *SettingsView) Unmount() {
	if v.dirty && v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.settings.unsaved"), false)
	}
	v.undoStack.Clear()
	v.app = nil
	v.list = nil
}

func (v *SettingsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
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
	case 'r':
		// Refresh from disk
		if v.cfg.ReloadConfig != nil {
			v.live = v.cfg.ReloadConfig()
		}
		v.dirty = false
		v.undoStack.Clear()
		v.renderLines()
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.settings.refreshed"), true)
		}
		return nil
	}
	return event
}

func (v *SettingsView) undo() {
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

// ContextCommands implements CommandProvider for omnibar integration.
func (v *SettingsView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{
			ID: "settings.save", Label: i18n.T("tui.hints.save"),
			Aliases: []string{"save", "write", "sauvegarder"},
			Description: i18n.T("tui.settings.cmd_save"), Category: "Settings",
			Action: v.save,
		},
		{
			ID: "settings.refresh", Label: i18n.T("tui.hints.refresh"),
			Aliases: []string{"refresh", "reload", "rafraîchir"},
			Description: i18n.T("tui.settings.cmd_refresh"), Category: "Settings",
			Action: func() {
				if v.cfg.ReloadConfig != nil {
					v.live = v.cfg.ReloadConfig()
				}
				v.dirty = false
				v.undoStack.Clear()
				v.renderLines()
			},
		},
		{
			ID: "settings.undo", Label: i18n.T("tui.hints.undo"),
			Aliases: []string{"undo", "annuler"},
			Description: i18n.T("tui.settings.cmd_undo"), Category: "Settings",
			Action: v.undo,
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Line definitions — maps every hub.toml field to a configLine
// ─────────────────────────────────────────────────────────────────────────────

func (v *SettingsView) buildLines() {
	languageOptions := []SelectOption{
		{Label: "Français", Value: "fr"},
		{Label: "English", Value: "en"},
	}
	channelOptions := []SelectOption{
		{Label: "stable", Value: "stable"},
		{Label: "canary", Value: "canary"},
	}
	providerOptions := func() []SelectOption {
		opts := make([]SelectOption, 0)
		for _, p := range provider.AllProviders() {
			opts = append(opts, SelectOption{Label: string(p), Value: string(p)})
		}
		return opts
	}
	triBoolOptions := []SelectOption{
		{Label: "↩ " + i18n.T("tui.settings.inherited"), Value: "(hérité)"},
		{Label: "✓ " + i18n.T("tui.settings.enabled"), Value: "true"},
		{Label: "✗ " + i18n.T("tui.settings.disabled"), Value: "false"},
	}

	v.lines = []configLine{
		// ── Général ──────────────────────────────────────────────────────────
		{kind: "section-header", section: "Général", label: i18n.T("tui.settings.section_general")},
		{section: "Général", key: "name", kind: "string", label: i18n.T("tui.settings.section_general"),
			get: func(c *config.Config) string { return c.Name },
			set: func(c *config.Config, val string) { c.Name = val }},

		// ── CLI ──────────────────────────────────────────────────────────────
		{kind: "section-header", section: "CLI", label: "CLI"},
		{section: "CLI", key: "language", kind: "select", label: "CLI",
			options: languageOptions,
			validator: &FieldValidator{AllowedValues: []string{"fr", "en"}},
			get:     func(c *config.Config) string { return c.CLI.Language },
			set:     func(c *config.Config, val string) { c.CLI.Language = val }},

		// ── Opencode ─────────────────────────────────────────────────────────
		{kind: "section-header", section: "Opencode", label: "Opencode"},
		{section: "Opencode", key: "version", kind: "readonly", label: "Opencode",
			get: func(c *config.Config) string { return c.Opencode.Version }},
		{section: "Opencode", key: "channel", kind: "select", label: "Opencode",
			options:   channelOptions,
			validator: &FieldValidator{AllowedValues: []string{"stable", "canary"}},
			get:       func(c *config.Config) string { return c.Opencode.Channel },
			set:       func(c *config.Config, val string) { c.Opencode.Channel = val }},
		{section: "Opencode", key: "auto_update", kind: "bool", label: "Opencode",
			get: func(c *config.Config) string { return boolStr(c.Opencode.AutoUpdate) },
			set: func(c *config.Config, val string) { c.Opencode.AutoUpdate = val == "true" }},
		{section: "Opencode", key: "default_provider", kind: "select", label: "Opencode",
			optionsFunc: providerOptions,
			validator:   &FieldValidator{AllowedFunc: func() []string {
				names := provider.AllProviders()
				s := make([]string, len(names))
				for i, n := range names { s[i] = string(n) }
				return s
			}, AllowEmpty: true},
			get: func(c *config.Config) string { return c.Opencode.DefaultProvider },
			set: func(c *config.Config, val string) { c.Opencode.DefaultProvider = val }},

		// ── Deploy ───────────────────────────────────────────────────────────
		{kind: "section-header", section: "Deploy", label: "Deploy"},
		{section: "Deploy", key: "disable_native_agents", kind: "readonly", label: "Deploy",
			get: func(c *config.Config) string {
				if len(c.Deploy.DisableNativeAgents) == 0 {
					return fmt.Sprintf("(%s)", i18n.T("tui.settings.default"))
				}
				return fmt.Sprintf("%v", c.Deploy.DisableNativeAgents)
			}},

		// ── MCP GitLab ───────────────────────────────────────────────────────
		{kind: "section-header", section: "MCP GitLab", label: "MCP GitLab"},
		{section: "MCP GitLab", key: "enabled", kind: "bool", label: "MCP GitLab",
			get: func(c *config.Config) string { return boolStr(c.MCP.Gitlab.Enabled) },
			set: func(c *config.Config, val string) { c.MCP.Gitlab.Enabled = val == "true" }},
		{section: "MCP GitLab", key: "token_key", kind: "tokenkey", label: "MCP GitLab",
			get: func(c *config.Config) string { return c.MCP.Gitlab.Token },
			set: func(c *config.Config, val string) { c.MCP.Gitlab.Token = val }},
		{section: "MCP GitLab", key: "write_enabled", kind: "bool", label: "MCP GitLab",
			get: func(c *config.Config) string { return boolStr(c.MCP.Gitlab.WriteEnabled) },
			set: func(c *config.Config, val string) { c.MCP.Gitlab.WriteEnabled = val == "true" }},
		{section: "MCP GitLab", key: "url", kind: "string", label: "MCP GitLab",
			get: func(c *config.Config) string { return c.MCP.Gitlab.URL },
			set: func(c *config.Config, val string) { c.MCP.Gitlab.URL = val }},

		// ── MCP Jira ─────────────────────────────────────────────────────────
		{kind: "section-header", section: "MCP Jira", label: "MCP Jira"},
		{section: "MCP Jira", key: "enabled", kind: "bool", label: "MCP Jira",
			get: func(c *config.Config) string { return boolStr(c.MCP.Jira.Enabled) },
			set: func(c *config.Config, val string) { c.MCP.Jira.Enabled = val == "true" }},
		{section: "MCP Jira", key: "token_key", kind: "tokenkey", label: "MCP Jira",
			get: func(c *config.Config) string { return c.MCP.Jira.Token },
			set: func(c *config.Config, val string) { c.MCP.Jira.Token = val }},
		// NOTE: write_enabled removed for Jira — Jira does not use this feature.
		{section: "MCP Jira", key: "url", kind: "string", label: "MCP Jira",
			get: func(c *config.Config) string { return c.MCP.Jira.URL },
			set: func(c *config.Config, val string) { c.MCP.Jira.URL = val }},

		// ── MCP Figma ────────────────────────────────────────────────────────
		{kind: "section-header", section: "MCP Figma", label: "MCP Figma"},
		{section: "MCP Figma", key: "enabled", kind: "bool", label: "MCP Figma",
			get: func(c *config.Config) string { return boolStr(c.MCP.Figma.Enabled) },
			set: func(c *config.Config, val string) { c.MCP.Figma.Enabled = val == "true" }},
		{section: "MCP Figma", key: "token_key", kind: "tokenkey", label: "MCP Figma",
			get: func(c *config.Config) string { return c.MCP.Figma.Token },
			set: func(c *config.Config, val string) { c.MCP.Figma.Token = val }},

		// ── MCP Gslides (NEW) ────────────────────────────────────────────────
		{kind: "section-header", section: "MCP Gslides", label: "MCP Gslides"},
		{section: "MCP Gslides", key: "enabled", kind: "bool", label: "MCP Gslides",
			get: func(c *config.Config) string { return boolStr(c.MCP.Gslides.Enabled) },
			set: func(c *config.Config, val string) { c.MCP.Gslides.Enabled = val == "true" }},
		{section: "MCP Gslides", key: "token_key", kind: "tokenkey", label: "MCP Gslides",
			get: func(c *config.Config) string { return c.MCP.Gslides.Token },
			set: func(c *config.Config, val string) { c.MCP.Gslides.Token = val }},

		// ── Worktree ─────────────────────────────────────────────────────────
		{kind: "section-header", section: "Worktree", label: "Worktree"},
		{section: "Worktree", key: "auto_cleanup", kind: "bool", label: "Worktree",
			get: func(c *config.Config) string { return boolStr(c.Worktree.AutoCleanup) },
			set: func(c *config.Config, val string) { c.Worktree.AutoCleanup = val == "true" }},
		{section: "Worktree", key: "base_branch", kind: "string", label: "Worktree",
			get: func(c *config.Config) string { return c.Worktree.BaseBranch },
			set: func(c *config.Config, val string) { c.Worktree.BaseBranch = val }},
		{section: "Worktree", key: "branch_pattern", kind: "string", label: "Worktree",
			get: func(c *config.Config) string { return c.Worktree.BranchPattern },
			set: func(c *config.Config, val string) { c.Worktree.BranchPattern = val }},

		// ── Tracker ──────────────────────────────────────────────────────────
		{kind: "section-header", section: "Tracker", label: i18n.T("tui.settings.section_tracker")},
		{section: "Tracker", key: "enabled", kind: "tri-bool", label: i18n.T("tui.settings.section_tracker"),
			options: triBoolOptions,
			get: func(c *config.Config) string {
				if c.Tracker.Enabled == nil { return "(hérité)" }
				return boolStr(*c.Tracker.Enabled)
			},
			set: func(c *config.Config, val string) {
				if val == "(hérité)" || val == "" { c.Tracker.Enabled = nil; return }
				b := val == "true"; c.Tracker.Enabled = &b
			}},
		{section: "Tracker", key: "auto_sync", kind: "tri-bool", label: i18n.T("tui.settings.section_tracker"),
			options: triBoolOptions,
			get: func(c *config.Config) string {
				if c.Tracker.AutoSync == nil { return "(hérité)" }
				return boolStr(*c.Tracker.AutoSync)
			},
			set: func(c *config.Config, val string) {
				if val == "(hérité)" || val == "" { c.Tracker.AutoSync = nil; return }
				b := val == "true"; c.Tracker.AutoSync = &b
			}},
		{section: "Tracker", key: "push_labels", kind: "tri-bool", label: i18n.T("tui.settings.section_tracker"),
			options: triBoolOptions,
			get: func(c *config.Config) string {
				if c.Tracker.PushLabels == nil { return "(hérité)" }
				return boolStr(*c.Tracker.PushLabels)
			},
			set: func(c *config.Config, val string) {
				if val == "(hérité)" || val == "" { c.Tracker.PushLabels = nil; return }
				b := val == "true"; c.Tracker.PushLabels = &b
			}},
		{section: "Tracker", key: "auto_plan_assigned", kind: "tri-bool", label: i18n.T("tui.settings.section_tracker"),
			options: triBoolOptions,
			get: func(c *config.Config) string {
				if c.Tracker.AutoPlanAssigned == nil { return "(hérité)" }
				return boolStr(*c.Tracker.AutoPlanAssigned)
			},
			set: func(c *config.Config, val string) {
				if val == "(hérité)" || val == "" { c.Tracker.AutoPlanAssigned = nil; return }
				b := val == "true"; c.Tracker.AutoPlanAssigned = &b
			}},
		{section: "Tracker", key: "max_auto_plan_per_member", kind: "int", label: i18n.T("tui.settings.section_tracker"),
			validator: &FieldValidator{Numeric: true, MinInt: intPtr(0), MaxInt: intPtr(100), AllowEmpty: true},
			get: func(c *config.Config) string {
				if c.Tracker.MaxAutoPlanPerMember == nil { return "(hérité)" }
				return strconv.Itoa(*c.Tracker.MaxAutoPlanPerMember)
			},
			set: func(c *config.Config, val string) {
				if val == "(hérité)" || val == "" { c.Tracker.MaxAutoPlanPerMember = nil; return }
				if n, err := strconv.Atoi(val); err == nil { c.Tracker.MaxAutoPlanPerMember = &n }
			}},

		// ── Raccourcis ───────────────────────────────────────────────────────
		{kind: "section-header", section: "Raccourcis", label: i18n.T("tui.settings.section_shortcuts")},
		{section: "Raccourcis", key: "models", kind: "link", label: i18n.T("tui.settings.section_shortcuts"),
			linkTarget: "models",
			get:        func(_ *config.Config) string { return i18n.T("tui.settings.link_models") }},
		{section: "Raccourcis", key: "teams", kind: "link", label: i18n.T("tui.settings.section_shortcuts"),
			linkTarget: "teams",
			get:        func(_ *config.Config) string { return i18n.T("tui.settings.link_teams") }},
		{section: "Raccourcis", key: "provider", kind: "link", label: i18n.T("tui.settings.section_shortcuts"),
			linkTarget: "provider",
			get:        func(_ *config.Config) string { return i18n.T("tui.settings.link_provider") }},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

func (v *SettingsView) renderLines() {
	if v.list == nil || v.live == nil {
		return
	}
	savedIdx := v.list.GetCurrentItem()
	ctx := context.Background()

	items := make([]widgets.SectionItem, 0, len(v.lines))
	for i, line := range v.lines {
		switch line.kind {
		case "section-header":
			title := line.section
			// Append dirty indicator to the first section header
			if i == 0 && v.dirty {
				title += "  " + theme.ColorTag(theme.AccentHex) + "● " + i18n.T("tui.settings.modified") + theme.TagColor
			}
			items = append(items, widgets.SectionItem{
				IsHeader: true,
				MainText: title,
			})

		default:
			val := ""
			if line.get != nil {
				val = line.get(v.live)
			}

			valDisplay := v.formatValue(val, line, ctx)
			mainText := fmt.Sprintf("%-24s %s", line.key+":", valDisplay)

			items = append(items, widgets.SectionItem{
				MainText:  mainText,
				Reference: i,
			})
		}
	}

	v.list.SetItems(items)
	if savedIdx >= 0 {
		v.list.SelectIndex(savedIdx)
	}
}

func (v *SettingsView) formatValue(val string, line configLine, ctx context.Context) string {
	switch line.kind {
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

	case "select":
		if val == "" {
			return fmt.Sprintf("%s(%s)%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.settings.empty"), theme.TagColor)
		}
		// Show label from options if available
		opts := line.options
		if line.optionsFunc != nil {
			opts = line.optionsFunc()
		}
		for _, o := range opts {
			if o.Value == val {
				return fmt.Sprintf("%s%s%s", theme.ColorTag(theme.TextSecondaryHex), o.Label, theme.TagColor)
			}
		}
		return val

	case "int":
		if val == "(hérité)" || val == "" {
			return fmt.Sprintf("%s↩ %s%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.settings.inherited"), theme.TagColor)
		}
		return val

	case "tokenkey":
		if val == "" {
			return fmt.Sprintf("%s(%s)%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.settings.not_configured"), theme.TagColor)
		}
		display := fmt.Sprintf("%s%s%s", theme.ColorTag(theme.TextSecondaryHex), val, theme.TagColor)
		present, masked := v.cfg.CheckSecret(ctx, val)
		if present {
			display += fmt.Sprintf("  %s✓ %s%s", theme.ColorTag(theme.SuccessHex), masked, theme.TagColor)
		} else {
			display += fmt.Sprintf("  %s✗ %s%s", theme.ColorTag(theme.ErrorHex), i18n.T("tui.settings.absent"), theme.TagColor)
		}
		return display

	case "link":
		return fmt.Sprintf("%s→ %s%s", theme.ColorTag(theme.AccentHex), val, theme.TagColor)

	case "readonly":
		if val == "" {
			return fmt.Sprintf("%s(%s)%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.settings.empty"), theme.TagColor)
		}
		return fmt.Sprintf("%s%s%s  %s(%s)%s", theme.ColorTag(theme.TextMutedHex), val, theme.TagColor,
			theme.ColorTag(theme.TextMutedHex), i18n.T("tui.settings.readonly"), theme.TagColor)

	default: // string
		if val == "" {
			return fmt.Sprintf("%s(%s)%s", theme.ColorTag(theme.TextMutedHex), i18n.T("tui.settings.empty"), theme.TagColor)
		}
		return val
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Editing
// ─────────────────────────────────────────────────────────────────────────────

func (v *SettingsView) editByIndex(index int, item widgets.SectionItem) {
	ref, ok := item.Reference.(int)
	if !ok || ref < 0 || ref >= len(v.lines) {
		return
	}
	line := v.lines[ref]
	if line.kind == "section-header" || line.get == nil {
		return
	}
	if v.shell == nil {
		return
	}

	switch line.kind {
	case "bool":
		v.toggleByRef(ref)

	case "tri-bool":
		opts := line.options
		if len(opts) == 0 {
			opts = []SelectOption{
				{Label: "↩ " + i18n.T("tui.settings.inherited"), Value: "(hérité)"},
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
				// Validate
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

	case "int":
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

	case "tokenkey":
		v.shell.ShowSelectModal(
			line.key,
			[]SelectOption{
				{Label: i18n.T("tui.settings.edit_key_name"), Value: "keyname"},
				{Label: i18n.T("tui.settings.edit_secret"), Value: "secret"},
				{Label: i18n.T("tui.settings.cancel"), Value: ""},
			},
			"",
			func(choice string) {
				switch choice {
				case "keyname":
					v.shell.ShowInputModal(i18n.T("tui.settings.key_name"), line.get(v.live), func(newKey string) {
						if newKey != "" {
							v.pushUndo()
							line.set(v.live, newKey)
							v.dirty = true
							v.renderLines()
						}
					})
				case "secret":
					keyName := line.get(v.live)
					if keyName == "" {
						return
					}
					v.shell.ShowPasswordModal(i18n.T("tui.settings.new_secret_value"), func(value string) {
						if value == "" {
							return
						}
						ctx := context.Background()
						if err := v.cfg.SetSecret(ctx, keyName, value); err != nil {
							v.shell.ShowToastMsg(i18n.T("tui.settings.error")+": "+err.Error(), false)
							return
						}
						v.shell.ShowToastMsg(i18n.T("tui.settings.secret_updated"), true)
						v.renderLines()
					})
				}
			})

	case "link":
		if line.linkTarget != "" && v.shell != nil {
			v.shell.NavigateTo(line.linkTarget)
		}

	case "readonly":
		v.shell.ShowToastMsg(i18n.T("tui.settings.readonly"), false)

	default: // string
		cur := ""
		if line.get != nil {
			cur = line.get(v.live)
		}
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

func (v *SettingsView) toggleSelected() {
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
	v.toggleByRef(ref)
}

func (v *SettingsView) toggleByRef(ref int) {
	line := v.lines[ref]
	if line.kind != "bool" || line.set == nil {
		return
	}
	cur := line.get(v.live)
	newVal := "true"
	if cur == "true" {
		newVal = "false"
	}
	v.pushUndo()
	line.set(v.live, newVal)
	v.dirty = true
	v.renderLines()
}

func (v *SettingsView) pushUndo() {
	snapshot := deepCopyConfig(v.live)
	v.undoStack.Push(snapshot)
}

// ─────────────────────────────────────────────────────────────────────────────
// Save
// ─────────────────────────────────────────────────────────────────────────────

func (v *SettingsView) save() {
	if v.shell == nil {
		return
	}
	if v.live == nil {
		v.shell.ShowToastMsg(i18n.T("tui.settings.loading"), false)
		return
	}

	// Global validation pass
	var errors []string
	for _, line := range v.lines {
		if line.kind == "section-header" || line.get == nil || line.validator == nil {
			continue
		}
		val := line.get(v.live)
		if err := line.validator.Validate(val); err != nil {
			errors = append(errors, fmt.Sprintf("%s.%s: %s", line.section, line.key, err.Error()))
		}
	}
	if len(errors) > 0 {
		v.shell.ShowToastMsg(fmt.Sprintf("⚠ %d %s", len(errors), i18n.T("tui.settings.validation_errors")), false)
		return
	}

	if err := v.cfg.SaveConfig(v.live); err != nil {
		v.shell.ShowToastMsg(i18n.T("tui.settings.save_error")+": "+err.Error(), false)
		return
	}
	v.dirty = false
	v.undoStack.Clear()
	v.shell.ShowToastMsg(i18n.T("tui.settings.saved"), true)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// formatConfigValue is kept for backward compat with any code that references it.
func formatConfigValue(val, kind string) string {
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
	case "tokenkey":
		if val == "" {
			return fmt.Sprintf("%s(non configuré)%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor)
		}
		return fmt.Sprintf("%s%s%s", theme.ColorTag(theme.TextSecondaryHex), val, theme.TagColor)
	default:
		if val == "" {
			return fmt.Sprintf("%s(vide)%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor)
		}
		return val
	}
}
