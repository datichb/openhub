package views

import (
	"context"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
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
	section string // e.g. "MCP GitLab"
	key     string // e.g. "enabled"
	kind    string // "bool", "string", "tokenkey", "section-header"
	// get/set operate on the live *config.Config pointer held by the view
	get func(c *config.Config) string
	set func(c *config.Config, v string)
}

// SettingsView displays and edits hub.toml line by line.
type SettingsView struct {
	app    *tview.Application
	list   *tview.List
	shell  ShellAccess
	cfg    SettingsViewConfig

	// live config being edited (copy from disk, modified in memory until saved)
	live  *config.Config
	dirty bool

	lines []configLine
}

var _ View = (*SettingsView)(nil)
var _ CommandProvider = (*SettingsView)(nil)

// NewSettingsView creates the hub config view.
func NewSettingsView(cfg SettingsViewConfig) *SettingsView {
	return &SettingsView{cfg: cfg}
}

// SetShell provides shell access for modals/toasts.
func (v *SettingsView) SetShell(s ShellAccess) { v.shell = s }

func (v *SettingsView) ID() string      { return "settings" }
func (v *SettingsView) Title() string   { return "Settings" }
func (v *SettingsView) StatusHints() string {
	return fmt.Sprintf("j/k nav · Space %s · Enter %s · w %s · u %s · r %s",
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
	v.dirty = false

	// Show loading placeholder immediately
	loading := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	loading.SetBackgroundColor(theme.BgPanel)
	muted := theme.ColorTag(theme.TextMutedHex)
	loading.SetText(fmt.Sprintf("\n  %sChargement de la configuration...%s", muted, theme.TagColor))
	content.AddItem(loading, 0, 1, true)

	// Load config and build list asynchronously
	go func() {
		live := v.cfg.GetConfig()
		app.QueueUpdateDraw(func() {
			if v.app == nil {
				return // view was unmounted before the goroutine finished
			}
			v.live = live

			v.list = tview.NewList().
				ShowSecondaryText(true).
				SetHighlightFullLine(true).
				SetMainTextColor(theme.FgPrimary).
				SetSecondaryTextColor(theme.FgSecondary)
			v.list.SetBackgroundColor(theme.BgPanel)
			v.list.SetBorderPadding(1, 0, 2, 2)

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
		v.shell.ShowToastMsg("⚠ Modifications non sauvegardées — w pour sauvegarder", false)
	}
	v.app = nil
	v.list = nil
}

func (v *SettingsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEnter:
		v.editSelected()
		return nil
	}
	switch event.Rune() {
	case 'e':
		v.editSelected()
		return nil
	case ' ':
		v.toggleSelected()
		return nil
	case 'w':
		v.save()
		return nil
	case 'u':
		// Undo: reload from disk (discards in-memory mutations)
		if v.cfg.ReloadConfig != nil {
			v.live = v.cfg.ReloadConfig()
		}
		v.dirty = false
		v.renderLines()
		if v.shell != nil {
			v.shell.ShowToastMsg("↩ Annulé (rechargé depuis disque)", true)
		}
		return nil
	case 'r':
		// Refresh from disk
		if v.cfg.ReloadConfig != nil {
			v.live = v.cfg.ReloadConfig()
		}
		v.dirty = false
		v.renderLines()
		if v.shell != nil {
			v.shell.ShowToastMsg("↻ Rafraîchi", true)
		}
		return nil
	}
	return event
}

// ContextCommands implements CommandProvider for omnibar integration.
func (v *SettingsView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{
			ID:          "settings.save",
			Label:       "Sauvegarder",
			Aliases:     []string{"save", "write"},
			Description: "Sauvegarder la configuration",
			Category:    "Settings",
			Action:      v.save,
		},
		{
			ID:          "settings.refresh",
			Label:       "Rafraîchir",
			Aliases:     []string{"refresh", "reload"},
			Description: "Recharger depuis le disque",
			Category:    "Settings",
			Action: func() {
				if v.cfg.ReloadConfig != nil {
					v.live = v.cfg.ReloadConfig()
				}
				v.dirty = false
				v.renderLines()
			},
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Line definitions — maps every hub.toml field to a configLine
// ─────────────────────────────────────────────────────────────────────────────

func (v *SettingsView) buildLines() {
	v.lines = []configLine{
		{kind: "section-header", section: "CLI"},
		{section: "CLI", key: "language", kind: "string",
			get: func(c *config.Config) string { return c.CLI.Language },
			set: func(c *config.Config, val string) { c.CLI.Language = val }},

		{kind: "section-header", section: "Opencode"},
		{section: "Opencode", key: "version", kind: "string",
			get: func(c *config.Config) string { return c.Opencode.Version },
			set: func(c *config.Config, val string) { c.Opencode.Version = val }},
		{section: "Opencode", key: "channel", kind: "string",
			get: func(c *config.Config) string { return c.Opencode.Channel },
			set: func(c *config.Config, val string) { c.Opencode.Channel = val }},
		{section: "Opencode", key: "auto_update", kind: "bool",
			get: func(c *config.Config) string { return boolStr(c.Opencode.AutoUpdate) },
			set: func(c *config.Config, val string) { c.Opencode.AutoUpdate = val == "true" }},
		{section: "Opencode", key: "default_provider", kind: "string",
			get: func(c *config.Config) string { return c.Opencode.DefaultProvider },
			set: func(c *config.Config, val string) { c.Opencode.DefaultProvider = val }},

		{kind: "section-header", section: "MCP GitLab"},
		{section: "MCP GitLab", key: "enabled", kind: "bool",
			get: func(c *config.Config) string { return boolStr(c.MCP.Gitlab.Enabled) },
			set: func(c *config.Config, val string) { c.MCP.Gitlab.Enabled = val == "true" }},
		{section: "MCP GitLab", key: "token_key", kind: "tokenkey",
			get: func(c *config.Config) string { return c.MCP.Gitlab.Token },
			set: func(c *config.Config, val string) { c.MCP.Gitlab.Token = val }},
		{section: "MCP GitLab", key: "write_enabled", kind: "bool",
			get: func(c *config.Config) string { return boolStr(c.MCP.Gitlab.WriteEnabled) },
			set: func(c *config.Config, val string) { c.MCP.Gitlab.WriteEnabled = val == "true" }},
		{section: "MCP GitLab", key: "url", kind: "string",
			get: func(c *config.Config) string { return c.MCP.Gitlab.URL },
			set: func(c *config.Config, val string) { c.MCP.Gitlab.URL = val }},

		{kind: "section-header", section: "MCP Jira"},
		{section: "MCP Jira", key: "enabled", kind: "bool",
			get: func(c *config.Config) string { return boolStr(c.MCP.Jira.Enabled) },
			set: func(c *config.Config, val string) { c.MCP.Jira.Enabled = val == "true" }},
		{section: "MCP Jira", key: "token_key", kind: "tokenkey",
			get: func(c *config.Config) string { return c.MCP.Jira.Token },
			set: func(c *config.Config, val string) { c.MCP.Jira.Token = val }},
		{section: "MCP Jira", key: "write_enabled", kind: "bool",
			get: func(c *config.Config) string { return boolStr(c.MCP.Jira.WriteEnabled) },
			set: func(c *config.Config, val string) { c.MCP.Jira.WriteEnabled = val == "true" }},
		{section: "MCP Jira", key: "url", kind: "string",
			get: func(c *config.Config) string { return c.MCP.Jira.URL },
			set: func(c *config.Config, val string) { c.MCP.Jira.URL = val }},

		{kind: "section-header", section: "MCP Figma"},
		{section: "MCP Figma", key: "enabled", kind: "bool",
			get: func(c *config.Config) string { return boolStr(c.MCP.Figma.Enabled) },
			set: func(c *config.Config, val string) { c.MCP.Figma.Enabled = val == "true" }},
		{section: "MCP Figma", key: "token_key", kind: "tokenkey",
			get: func(c *config.Config) string { return c.MCP.Figma.Token },
			set: func(c *config.Config, val string) { c.MCP.Figma.Token = val }},

		{kind: "section-header", section: "Worktree"},
		{section: "Worktree", key: "auto_cleanup", kind: "bool",
			get: func(c *config.Config) string { return boolStr(c.Worktree.AutoCleanup) },
			set: func(c *config.Config, val string) { c.Worktree.AutoCleanup = val == "true" }},
		{section: "Worktree", key: "base_branch", kind: "string",
			get: func(c *config.Config) string { return c.Worktree.BaseBranch },
			set: func(c *config.Config, val string) { c.Worktree.BaseBranch = val }},
		{section: "Worktree", key: "branch_pattern", kind: "string",
			get: func(c *config.Config) string { return c.Worktree.BranchPattern },
			set: func(c *config.Config, val string) { c.Worktree.BranchPattern = val }},

		{kind: "section-header", section: "Tracker (overrides locaux)"},
		{section: "Tracker", key: "enabled", kind: "bool",
			get: func(c *config.Config) string {
				if c.Tracker.Enabled == nil {
					return "(hérité)"
				}
				return boolStr(*c.Tracker.Enabled)
			},
			set: func(c *config.Config, val string) {
				if val == "(hérité)" || val == "" {
					c.Tracker.Enabled = nil
					return
				}
				b := val == "true"
				c.Tracker.Enabled = &b
			}},
		{section: "Tracker", key: "auto_sync", kind: "bool",
			get: func(c *config.Config) string {
				if c.Tracker.AutoSync == nil {
					return "(hérité)"
				}
				return boolStr(*c.Tracker.AutoSync)
			},
			set: func(c *config.Config, val string) {
				if val == "(hérité)" || val == "" {
					c.Tracker.AutoSync = nil
					return
				}
				b := val == "true"
				c.Tracker.AutoSync = &b
			}},
		{section: "Tracker", key: "push_labels", kind: "bool",
			get: func(c *config.Config) string {
				if c.Tracker.PushLabels == nil {
					return "(hérité)"
				}
				return boolStr(*c.Tracker.PushLabels)
			},
			set: func(c *config.Config, val string) {
				if val == "(hérité)" || val == "" {
					c.Tracker.PushLabels = nil
					return
				}
				b := val == "true"
				c.Tracker.PushLabels = &b
			}},
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
	v.list.Clear()
	ctx := context.Background()

	for _, line := range v.lines {
		if line.kind == "section-header" {
			// Section headers are non-selectable separator items
			v.list.AddItem(
				fmt.Sprintf("  %s─── %s ──────────────────%s", theme.ColorTag(theme.AccentHex), line.section, theme.TagColor),
				"", 0, nil)
			continue
		}

		val := ""
		if line.get != nil {
			val = line.get(v.live)
		}

		// Format value display
		valDisplay := formatConfigValue(val, line.kind)

		// Token key: append keychain status
		tokenStatus := ""
		if line.kind == "tokenkey" && val != "" {
			present, masked := v.cfg.CheckSecret(ctx, val)
			if present {
				tokenStatus = fmt.Sprintf("  %s✓ %s%s", theme.ColorTag("#4CAF50"), masked, theme.TagColor)
			} else {
				tokenStatus = fmt.Sprintf("  %s✗ absent%s", theme.ColorTag("#FF5252"), theme.TagColor)
			}
		}

		dirty := ""
		if v.dirty {
			dirty = "" // could add a marker but keep it clean
		}
		_ = dirty

		main := fmt.Sprintf("  %-22s %s%s", line.key+":", valDisplay, tokenStatus)
		v.list.AddItem(main, "", 0, nil)
	}
	if savedIdx >= 0 && savedIdx < v.list.GetItemCount() {
		v.list.SetCurrentItem(savedIdx)
	}
}

func formatConfigValue(val, kind string) string {
	switch kind {
	case "bool":
		switch val {
		case "true":
			return fmt.Sprintf("%s✓ true%s", theme.ColorTag("#4CAF50"), theme.TagColor)
		case "false":
			return fmt.Sprintf("%s✗ false%s", theme.ColorTag("#FF5252"), theme.TagColor)
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

// ─────────────────────────────────────────────────────────────────────────────
// Editing
// ─────────────────────────────────────────────────────────────────────────────

func (v *SettingsView) selectedLine() (configLine, bool) {
	if v.list == nil || v.list.GetItemCount() == 0 {
		return configLine{}, false
	}
	idx := v.list.GetCurrentItem()
	if idx < 0 || idx >= len(v.lines) {
		return configLine{}, false
	}
	line := v.lines[idx]
	if line.kind == "section-header" || line.get == nil {
		return configLine{}, false
	}
	return line, true
}

func (v *SettingsView) toggleSelected() {
	line, ok := v.selectedLine()
	if !ok || line.kind != "bool" {
		return
	}
	cur := line.get(v.live)
	newVal := "true"
	if cur == "true" {
		newVal = "false"
	} else if cur == "(hérité)" {
		newVal = "true"
	}
	line.set(v.live, newVal)
	v.dirty = true
	v.renderLines()
}

func (v *SettingsView) editSelected() {
	line, ok := v.selectedLine()
	if !ok {
		return
	}
	if v.shell == nil {
		return
	}

	switch line.kind {
	case "bool":
		v.toggleSelected()

	case "tokenkey":
		// Contextual menu: modify key name OR secret value
		v.shell.ShowSelectModal(
			"Modifier "+line.key,
			[]SelectOption{
				{Label: "Modifier le nom de la clé (hub.toml)", Value: "keyname"},
				{Label: "Modifier la valeur du secret (keychain)", Value: "secret"},
				{Label: "Annuler", Value: ""},
			},
			"",
			func(choice string) {
				switch choice {
				case "keyname":
					v.shell.ShowInputModal("Nom de la clé", line.get(v.live), func(newKey string) {
						if newKey != "" {
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
				v.shell.ShowPasswordModal("Nouvelle valeur pour le secret", func(value string) {
					if value == "" {
						return
					}
					ctx := context.Background()
					if err := v.cfg.SetSecret(ctx, keyName, value); err != nil {
						v.shell.ShowToastMsg("Erreur: "+err.Error(), false)
						return
					}
					v.shell.ShowToastMsg("Secret mis à jour", true)
					v.renderLines()
				})
			}
		})

	default: // string
		cur := ""
		if line.get != nil {
			cur = line.get(v.live)
		}
		v.shell.ShowInputModal(line.key, cur, func(newVal string) {
			if line.set != nil {
				line.set(v.live, newVal)
				v.dirty = true
				v.renderLines()
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Save
// ─────────────────────────────────────────────────────────────────────────────

func (v *SettingsView) save() {
	if v.live == nil || v.shell == nil {
		return
	}
	if err := v.cfg.SaveConfig(v.live); err != nil {
		v.shell.ShowToastMsg("Erreur de sauvegarde: "+err.Error(), false)
		return
	}
	v.dirty = false
	v.shell.ShowToastMsg("hub.toml sauvegardé", true)
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
