package views

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

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
	// ToolVersion returns the machine tool client version (pinned version
	// check; "" = unknown). Nil = unknown.
	ToolVersion func() string
}

// SettingsView displays and edits hub.toml using the unified configField system.
type SettingsView struct {
	app      *tview.Application
	list     *widgets.SectionedList
	shell    ShellAccess
	cfg      SettingsViewConfig
	mountGen uint64

	// live config being edited (pointer to in-memory config, auto-saved on mutation)
	live *config.Config

	fields    []configField
	undoStack *widgets.UndoStack[config.Config]
	autoSaver *AutoSaver
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
func (v *SettingsView) Title() string { return i18n.T("tui.settings.title") }
func (v *SettingsView) StatusHints() string {
	return fmt.Sprintf("j/k %s · {/} %s · Space %s · Enter %s · u %s · r %s",
		i18n.T("tui.hints.nav"),
		i18n.T("tui.hints.navigate"),
		i18n.T("tui.hints.toggle"),
		i18n.T("tui.hints.edit"),
		i18n.T("tui.hints.undo"),
		i18n.T("tui.hints.refresh"),
	)
}

// Mount builds and displays the view.
func (v *SettingsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.mountGen++
	gen := v.mountGen
	v.undoStack.Clear()
	v.live = v.cfg.GetConfig() // synchronous: always available for save()

	// Initialize auto-saver (500ms debounce for local hub.toml)
	v.autoSaver = NewAutoSaver(500*time.Millisecond, app, func() {
		v.doSave()
	})

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

func (v *SettingsView) Unmount() {
	v.mountGen++ // invalidate in-flight async goroutine
	// Flush any pending auto-save
	if v.autoSaver != nil {
		v.autoSaver.Flush()
	}
	v.undoStack.Clear()
	v.app = nil
	v.list = nil
}

func (v *SettingsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.list == nil {
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
	case 'r':
		v.refresh()
		return nil
	}
	return event
}

func (v *SettingsView) undo() {
	prev, ok := v.undoStack.Pop()
	if !ok {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.config.nothing_to_undo"), true)
		}
		return
	}
	*v.live = prev
	// Cancel any pending auto-save, then save the restored state
	if v.autoSaver != nil {
		v.autoSaver.Cancel()
	}
	v.doSave()
	v.renderFields()
	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.config.undone"), true)
	}
}

func (v *SettingsView) refresh() {
	if v.cfg.ReloadConfig != nil {
		v.live = v.cfg.ReloadConfig()
	}
	v.undoStack.Clear()
	if v.autoSaver != nil {
		v.autoSaver.Cancel()
	}
	v.renderFields()
	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.config.reloaded"), true)
	}
}

// ContextCommands implements CommandProvider for omnibar integration.
func (v *SettingsView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{
			ID: "settings.refresh", Label: i18n.T("tui.hints.refresh"),
			Aliases:     []string{"refresh", "reload", "rafraîchir"},
			Description: i18n.T("tui.settings.cmd_refresh"), Category: "Settings",
			Action: v.refresh,
		},
		{
			ID: "settings.undo", Label: i18n.T("tui.hints.undo"),
			Aliases:     []string{"undo", "annuler"},
			Description: i18n.T("tui.settings.cmd_undo"), Category: "Settings",
			Action: v.undo,
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Field definitions — maps every hub.toml field to a configField
// ─────────────────────────────────────────────────────────────────────────────

func (v *SettingsView) buildFields() {
	languageOptions := []SelectOption{
		{Label: "Français", Value: "fr"},
		{Label: "English", Value: "en"},
	}
	providerOptions := func() []SelectOption {
		opts := make([]SelectOption, 0)
		for _, p := range provider.AllProviders() {
			opts = append(opts, SelectOption{Label: string(p), Value: string(p)})
		}
		return opts
	}

	v.fields = []configField{
		// ── General ─────────────────────────────────────────────────────────
		{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.config.section.general")},
		{Key: "name", Kind: CfgFieldReadonly, Label: i18n.T("tui.config.field.name.label"),
			Description: i18n.T("tui.config.field.name.desc"),
			Get:         func() string { return v.live.Name }},
		{Key: "teams", Kind: CfgFieldLink, Label: i18n.T("tui.config.link.teams"), LinkTarget: "teams",
			Get: func() string { return "" }},

		// ── CLI ─────────────────────────────────────────────────────────────
		{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.config.section.cli")},
		{Key: "language", Kind: CfgFieldSelect, Label: i18n.T("tui.config.field.language.label"),
			Description: i18n.T("tui.config.field.language.desc"),
			Options:     languageOptions,
			Validator:   &FieldValidator{AllowedValues: []string{"fr", "en"}},
			Get:         func() string { return v.live.CLI.Language },
			Set:         func(val string) { v.live.CLI.Language = val }},

		// ── LLM ─────────────────────────────────────────────────────────────
		{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.config.section.llm")},
		{Key: "default_provider", Kind: CfgFieldSelect, Label: i18n.T("tui.config.field.default_provider.label"),
			Description: i18n.T("tui.config.field.default_provider.desc"),
			OptionsFunc: providerOptions,
			Validator: &FieldValidator{AllowedFunc: func() []string {
				names := provider.AllProviders()
				s := make([]string, len(names))
				for i, n := range names {
					s[i] = string(n)
				}
				return s
			}, AllowEmpty: true},
			Get: func() string { return v.live.LLM.DefaultProvider },
			Set: func(val string) { v.live.LLM.DefaultProvider = val }},
		{Key: "provider", Kind: CfgFieldLink, Label: i18n.T("tui.config.link.provider"), LinkTarget: "provider",
			Get: func() string { return "" }},
		{Key: "models", Kind: CfgFieldLink, Label: i18n.T("tui.config.link.models"), LinkTarget: "models",
			Get: func() string { return "" }},

		// ── Sessions (v5) ───────────────────────────────────────────────────
		{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.config.section.sessions")},
		{Key: "session_attach", Kind: CfgFieldSelect, Label: i18n.T("tui.config.field.session_attach.label"),
			Description: i18n.T("tui.config.field.session_attach.desc"),
			Options: []SelectOption{
				{Label: i18n.T("tui.config.attach.auto"), Value: "auto"},
				{Label: "iTerm2", Value: "iterm"},
				{Label: "Terminal.app", Value: "terminal"},
				{Label: "tmux", Value: "tmux"},
				{Label: i18n.T("tui.config.attach.browser"), Value: "browser"},
				{Label: i18n.T("tui.config.attach.suspend"), Value: "suspend"},
			},
			Validator: &FieldValidator{AllowedValues: []string{"auto", "iterm", "terminal", "tmux", "browser", "suspend"}, AllowEmpty: true},
			Get: func() string {
				if v.live.Session.Attach == "" {
					return "auto"
				}
				return v.live.Session.Attach
			},
			Set: func(val string) { v.live.Session.Attach = val }},
		{Key: "session_iterm_style", Kind: CfgFieldSelect, Label: i18n.T("tui.config.field.session_iterm_style.label"),
			Description: i18n.T("tui.config.field.session_iterm_style.desc"),
			Options: []SelectOption{
				{Label: i18n.T("tui.config.iterm.tab"), Value: "tab"},
				{Label: i18n.T("tui.config.iterm.split"), Value: "split"},
				{Label: i18n.T("tui.config.iterm.window"), Value: "window"},
			},
			Validator: &FieldValidator{AllowedValues: []string{"tab", "split", "window"}, AllowEmpty: true},
			Get: func() string {
				if v.live.Session.ITermStyle == "" {
					return "tab"
				}
				return v.live.Session.ITermStyle
			},
			Set: func(val string) { v.live.Session.ITermStyle = val }},
		{Key: "session_idle_sleep", Kind: CfgFieldInt, Label: i18n.T("tui.config.field.session_idle_sleep.label"),
			Description: i18n.T("tui.config.field.session_idle_sleep.desc"),
			Placeholder: "5",
			Validator:   &FieldValidator{Numeric: true, MinInt: intPtr(1), MaxInt: intPtr(1440), AllowEmpty: true},
			Get: func() string {
				if v.live.Session.IdleSleepMinutes == 0 {
					return ""
				}
				return strconv.Itoa(v.live.Session.IdleSleepMinutes)
			},
			Set: func(val string) { v.live.Session.IdleSleepMinutes, _ = strconv.Atoi(val) }},
	}
	v.fields = append(v.fields, v.execSettingsFields()...)
	v.fields = append(v.fields, []configField{
		// ── Workflows ───────────────────────────────────────────────────────
		{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.category.workflows")},
		{Key: "workflows", Kind: CfgFieldLink, Label: i18n.T("tui.config.link.workflow"), LinkTarget: "workflows",
			Get: func() string { return "" }},

		// ── MCP GitLab ──────────────────────────────────────────────────────
		{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.config.section.mcp_gitlab")},
		{Key: "enabled", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.mcp_enabled.label"),
			Description: i18n.T("tui.config.field.mcp_enabled.desc"), Section: "MCP GitLab",
			Get: func() string { return boolStr(v.live.MCP.Gitlab.Enabled) },
			Set: func(val string) { v.live.MCP.Gitlab.Enabled = val == "true" }},
		{Key: "token_key", Kind: CfgFieldPassword, Label: i18n.T("tui.config.field.mcp_token.label"),
			Description: i18n.T("tui.config.field.mcp_token.desc"), Section: "MCP GitLab",
			Get: func() string { return v.live.MCP.Gitlab.Token },
			Set: func(val string) { v.live.MCP.Gitlab.Token = val }},
		{Key: "write_enabled", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.mcp_write.label"),
			Description: i18n.T("tui.config.field.mcp_write.desc"), Section: "MCP GitLab",
			Get: func() string { return boolStr(v.live.MCP.Gitlab.WriteEnabled) },
			Set: func(val string) { v.live.MCP.Gitlab.WriteEnabled = val == "true" }},
		{Key: "url", Kind: CfgFieldString, Label: i18n.T("tui.config.field.mcp_url.label"),
			Description: i18n.T("tui.config.field.mcp_url.desc"), Placeholder: i18n.T("tui.config.field.mcp_url.placeholder"),
			Section: "MCP GitLab",
			Get:     func() string { return v.live.MCP.Gitlab.URL },
			Set:     func(val string) { v.live.MCP.Gitlab.URL = val }},

		// ── MCP Jira ────────────────────────────────────────────────────────
		{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.config.section.mcp_jira")},
		{Key: "enabled", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.mcp_enabled.label"),
			Description: i18n.T("tui.config.field.mcp_enabled.desc"), Section: "MCP Jira",
			Get: func() string { return boolStr(v.live.MCP.Jira.Enabled) },
			Set: func(val string) { v.live.MCP.Jira.Enabled = val == "true" }},
		{Key: "token_key", Kind: CfgFieldPassword, Label: i18n.T("tui.config.field.mcp_token.label"),
			Description: i18n.T("tui.config.field.mcp_token.desc"), Section: "MCP Jira",
			Get: func() string { return v.live.MCP.Jira.Token },
			Set: func(val string) { v.live.MCP.Jira.Token = val }},
		{Key: "url", Kind: CfgFieldString, Label: i18n.T("tui.config.field.mcp_url.label"),
			Description: i18n.T("tui.config.field.mcp_url.desc"), Placeholder: i18n.T("tui.config.field.mcp_url.placeholder"),
			Section: "MCP Jira",
			Get:     func() string { return v.live.MCP.Jira.URL },
			Set:     func(val string) { v.live.MCP.Jira.URL = val }},

		// ── MCP Figma ───────────────────────────────────────────────────────
		{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.config.section.mcp_figma")},
		{Key: "enabled", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.mcp_enabled.label"),
			Description: i18n.T("tui.config.field.mcp_enabled.desc"), Section: "MCP Figma",
			Get: func() string { return boolStr(v.live.MCP.Figma.Enabled) },
			Set: func(val string) { v.live.MCP.Figma.Enabled = val == "true" }},
		{Key: "token_key", Kind: CfgFieldPassword, Label: i18n.T("tui.config.field.mcp_token.label"),
			Description: i18n.T("tui.config.field.mcp_token.desc"), Section: "MCP Figma",
			Get: func() string { return v.live.MCP.Figma.Token },
			Set: func(val string) { v.live.MCP.Figma.Token = val }},

		// ── MCP Google Slides ───────────────────────────────────────────────
		{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.config.section.mcp_gslides")},
		{Key: "enabled", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.mcp_enabled.label"),
			Description: i18n.T("tui.config.field.mcp_enabled.desc"), Section: "MCP Gslides",
			Get: func() string { return boolStr(v.live.MCP.Gslides.Enabled) },
			Set: func(val string) { v.live.MCP.Gslides.Enabled = val == "true" }},
		{Key: "token_key", Kind: CfgFieldPassword, Label: i18n.T("tui.config.field.mcp_token.label"),
			Description: i18n.T("tui.config.field.mcp_token.desc"), Section: "MCP Gslides",
			Get: func() string { return v.live.MCP.Gslides.Token },
			Set: func(val string) { v.live.MCP.Gslides.Token = val }},

		// ── Worktree ────────────────────────────────────────────────────────
		{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.config.section.worktree")},
		{Key: "auto_cleanup", Kind: CfgFieldBool, Label: i18n.T("tui.config.field.auto_cleanup.label"),
			Description: i18n.T("tui.config.field.auto_cleanup.desc"),
			Get:         func() string { return boolStr(v.live.Worktree.AutoCleanup) },
			Set:         func(val string) { v.live.Worktree.AutoCleanup = val == "true" }},
		{Key: "base_branch", Kind: CfgFieldString, Label: i18n.T("tui.config.field.base_branch.label"),
			Description: i18n.T("tui.config.field.base_branch.desc"),
			Placeholder: i18n.T("tui.config.field.base_branch.placeholder"),
			Get:         func() string { return v.live.Worktree.BaseBranch },
			Set:         func(val string) { v.live.Worktree.BaseBranch = val }},
		{Key: "branch_pattern", Kind: CfgFieldString, Label: i18n.T("tui.config.field.branch_pattern.label"),
			Description: i18n.T("tui.config.field.branch_pattern.desc"),
			Placeholder: i18n.T("tui.config.field.branch_pattern.placeholder"),
			Get:         func() string { return v.live.Worktree.BranchPattern },
			Set:         func(val string) { v.live.Worktree.BranchPattern = val }},

		// ── Tracker (dynamic: tri-state if team configured, bool if solo) ──
		{Kind: CfgFieldSectionHeader, Label: i18n.T("tui.config.section.tracker")},
		v.trackerBoolField("enabled", i18n.T("tui.config.field.tracker_enabled.label"),
			i18n.T("tui.config.field.tracker_enabled.desc"),
			func() *bool { return v.live.Tracker.Enabled },
			func(b *bool) { v.live.Tracker.Enabled = b }),
		v.trackerBoolField("auto_sync", i18n.T("tui.config.field.auto_sync.label"),
			i18n.T("tui.config.field.auto_sync.desc"),
			func() *bool { return v.live.Tracker.AutoSync },
			func(b *bool) { v.live.Tracker.AutoSync = b }),
		v.trackerBoolField("push_labels", i18n.T("tui.config.field.push_labels.label"),
			i18n.T("tui.config.field.push_labels.desc"),
			func() *bool { return v.live.Tracker.PushLabels },
			func(b *bool) { v.live.Tracker.PushLabels = b }),
		v.trackerBoolField("auto_plan_assigned", i18n.T("tui.config.field.auto_plan_assigned.label"),
			i18n.T("tui.config.field.auto_plan_assigned.desc"),
			func() *bool { return v.live.Tracker.AutoPlanAssigned },
			func(b *bool) { v.live.Tracker.AutoPlanAssigned = b }),
		{Key: "max_auto_plan_per_member", Kind: CfgFieldInt, Label: i18n.T("tui.config.field.max_auto_plan.label"),
			Description: i18n.T("tui.config.field.max_auto_plan.desc"),
			Placeholder: i18n.T("tui.config.field.max_auto_plan.placeholder"),
			Validator:   &FieldValidator{Numeric: true, MinInt: intPtr(0), MaxInt: intPtr(100), AllowEmpty: true},
			Get: func() string {
				if v.live.Tracker.MaxAutoPlanPerMember == nil {
					return ""
				}
				return strconv.Itoa(*v.live.Tracker.MaxAutoPlanPerMember)
			},
			Set: func(val string) {
				if val == "" {
					v.live.Tracker.MaxAutoPlanPerMember = nil
					return
				}
				if n, err := strconv.Atoi(val); err == nil {
					v.live.Tracker.MaxAutoPlanPerMember = &n
				}
			}},
		{Key: "tracker_url", Kind: CfgFieldString, Label: i18n.T("tui.config.field.tracker_hub_url.label"),
			Description: i18n.T("tui.config.field.tracker_hub_url.desc"),
			Placeholder: i18n.T("tui.config.field.tracker_hub_url.placeholder"),
			Get:         func() string { return v.live.Tracker.TrackerURL },
			Set:         func(val string) { v.live.Tracker.TrackerURL = val }},
		{Key: "tracker_token_key", Kind: CfgFieldPassword, Label: i18n.T("tui.config.field.tracker_hub_token.label"),
			Description: i18n.T("tui.config.field.tracker_hub_token.desc"),
			Section:     "Tracker",
			Get:         func() string { return v.live.Tracker.TrackerTokenKey },
			Set:         func(val string) { v.live.Tracker.TrackerTokenKey = val }},
		v.trackerBoolField("write_enabled", i18n.T("tui.config.field.tracker_write_enabled.label"),
			i18n.T("tui.config.field.tracker_write_enabled.desc"),
			func() *bool { return v.live.Tracker.WriteEnabled },
			func(b *bool) { v.live.Tracker.WriteEnabled = b }),
	}...)
	v.fields = append(v.fields, v.remoteFields()...)
	v.fields = insertFieldsAfter(v.fields, "session_idle_sleep", v.budgetFields())
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

func (v *SettingsView) renderFields() {
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
// Editing (delegates to shared editConfigField / toggleConfigField)
// ─────────────────────────────────────────────────────────────────────────────

func (v *SettingsView) onItemSelected(_ int, item widgets.SectionItem) {
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

	// Special handling for tokenkey fields (2-step: edit key name or set secret)
	if f.Kind == CfgFieldPassword && f.Section != "" {
		v.editTokenKey(f)
		return
	}

	v.pushUndo()
	editConfigField(v.shell, f, func() {
		v.scheduleAutoSave()
		v.renderFields()
	})
}

func (v *SettingsView) onToggleSelected() {
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
	v.pushUndo()
	toggleConfigField(f)
	v.scheduleAutoSave()
	v.renderFields()
}

// editTokenKey handles the 2-step edit for token/keychain fields:
// first choose between editing the key name or setting the secret value.
func (v *SettingsView) editTokenKey(f *configField) {
	v.shell.ShowSelectModal(
		f.Label,
		[]SelectOption{
			{Label: i18n.T("tui.settings.edit_key_name"), Value: "keyname"},
			{Label: i18n.T("tui.settings.edit_secret"), Value: "secret"},
			{Label: i18n.T("tui.settings.cancel"), Value: ""},
		},
		"",
		func(choice string) {
			switch choice {
			case "keyname":
				v.shell.ShowInputModal(i18n.T("tui.settings.key_name"), f.Get(), func(newKey string) {
					if newKey != "" {
						v.pushUndo()
						f.Set(newKey)
						v.scheduleAutoSave()
						v.renderFields()
					}
				})
			case "secret":
				keyName := f.Get()
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
					v.renderFields()
				})
			}
		})
}

func (v *SettingsView) pushUndo() {
	snapshot := deepCopyConfig(v.live)
	v.undoStack.Push(snapshot)
}

func (v *SettingsView) scheduleAutoSave() {
	if v.autoSaver != nil {
		v.autoSaver.Schedule()
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Save (auto-save: called by AutoSaver, also used by undo)
// ─────────────────────────────────────────────────────────────────────────────

func (v *SettingsView) doSave() {
	if v.live == nil {
		return
	}

	// Validate all fields before saving
	errs := validateAllFields(v.fields)
	if len(errs) > 0 {
		if v.shell != nil {
			v.shell.ShowToastMsg(fmt.Sprintf("⚠ %d %s", len(errs), i18n.T("tui.settings.validation_errors")), false)
		}
		return
	}

	if err := v.cfg.SaveConfig(v.live); err != nil {
		if v.shell != nil {
			if errors.Is(err, config.ErrExternalModification) {
				// Reload from disk and notify user
				if v.cfg.ReloadConfig != nil {
					v.live = v.cfg.ReloadConfig()
				}
				v.undoStack.Clear()
				v.renderFields()
				v.shell.ShowToastMsg(i18n.T("tui.config.external_modification"), false)
				return
			}
			v.shell.ShowToastMsg(i18n.Tf("tui.config.save_failed", err.Error()), false)
		}
		return
	}

	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.config.autosaved"), true)
	}
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

// trackerBoolField builds a configField for a tracker *bool hub field.
// Always tri-state: nil = "non configuré", true = activé, false = désactivé.
// The Scope is ScopeHub so formatFieldValue does NOT add inheritance parenthetical
// (the hub is the top level — it does not inherit from anyone).
func (v *SettingsView) trackerBoolField(
	key, label, desc string,
	getter func() *bool,
	setter func(*bool),
) configField {
	return configField{
		Key:         key,
		Kind:        CfgFieldTriBool,
		Label:       label,
		Description: desc,
		Scope:       ScopeHub,
		Get: func() string {
			p := getter()
			if p == nil {
				return "" // not configured
			}
			return boolStr(*p)
		},
		Set: func(val string) {
			if val == "" {
				setter(nil) // reset to not configured
				return
			}
			b := val == "true"
			setter(&b)
		},
	}
}
