package views

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/viper"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ConfigView displays and allows editing of hub configuration.
// Uses a grouped read-only display with contextual omnibar commands
// and direct keyboard shortcuts for editing.
type ConfigView struct {
	app      *tview.Application
	content  *tview.Flex
	display  *tview.TextView
	viper    *viper.Viper
	shell    ShellAccess
	fields   []configField   // ordered list of editable fields
	cursor   int             // current field index for keyboard nav
	commands []ContextCommand // cached contextual commands
}

// configField represents a single editable config field.
type configField struct {
	key      string         // viper key (e.g., "cli.language")
	label    string         // short display label (e.g., "langue")
	section  string         // grouping section (e.g., "CLI")
	options  []SelectOption // nil = free text, non-nil = enum/bool
	isBool   bool           // true if this is a boolean toggle
}

// SelectOption represents a selectable option with a friendly label and a stored value.
type SelectOption struct {
	Label string // Friendly display (e.g. "Français")
	Value string // Stored value (e.g. "fr")
}

// ModalAction represents a button action in a scrollable modal.
type ModalAction struct {
	Label    string
	Callback func()
}

// ShellAccess provides access to shell overlay capabilities from views.
type ShellAccess interface {
	ShowInputModal(title, currentValue string, onConfirm func(newValue string))
	ShowPasswordModal(title string, onConfirm func(value string))
	ShowSelectModal(title string, options []SelectOption, currentValue string, onConfirm func(value string))
	ShowMultiSelectModal(title string, options []SelectOption, selected []string, onConfirm func(selected []string))
	ShowScrollableModal(title, content string, actions []ModalAction)
	ShowToastMsg(msg string, success bool)
	ShowInlineForm(cfg InlineFormConfig)
	// NavigateTo navigates to a registered view by ID.
	NavigateTo(viewID string)
	// SetProjectMode activates or deactivates project mode with the given project.
	// Passing nil deactivates project mode (returns to hub mode).
	SetProjectMode(project *ActiveProject)
	// ActiveProject returns the currently active project, or nil in hub mode.
	ActiveProject() *ActiveProject
}

// ActiveProject holds the minimal project context for the TUI project mode.
type ActiveProject struct {
	ID   string
	Name string
	Path string
}

var _ View = (*ConfigView)(nil)
var _ CommandProvider = (*ConfigView)(nil)

// NewConfigView creates a new config view.
func NewConfigView() *ConfigView { return &ConfigView{} }

// SetShell provides the shell reference for toast interactions.
func (v *ConfigView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *ConfigView) ID() string { return "config" }

// Title returns the display title.
func (v *ConfigView) Title() string { return "Configuration" }

// StatusHints returns keybinding hints for the omnibar.
func (v *ConfigView) StatusHints() string {
	return "j/k naviguer · Space toggle · Enter modifier · Ctrl+P commande"
}

// Mount builds the configuration display.
func (v *ConfigView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.content = content
	v.viper = loadConfigViper()
	v.cursor = 0

	// Build field definitions
	v.fields = buildConfigFields()

	// Display widget
	v.display = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	v.display.SetBackgroundColor(theme.BgPanel)
	v.display.SetBorderPadding(1, 1, 2, 2)

	v.render()
	v.buildCommands()

	content.AddItem(v.display, 0, 1, true)
}

// Unmount cleans up resources.
func (v *ConfigView) Unmount() {
	v.app = nil
	v.content = nil
	v.display = nil
	v.viper = nil
	v.commands = nil
}

// HandleKey processes view-specific key events.
func (v *ConfigView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEnter:
		v.editCurrentField()
		return nil
	}

	switch event.Rune() {
	case 'j':
		if v.cursor < len(v.fields)-1 {
			v.cursor++
			v.render()
		}
		return nil
	case 'k':
		if v.cursor > 0 {
			v.cursor--
			v.render()
		}
		return nil
	case ' ':
		v.toggleCurrentField()
		return nil
	case 'l':
		v.cycleCurrentField(1)
		return nil
	case 'h':
		v.cycleCurrentField(-1)
		return nil
	}

	return event
}

// ContextCommands returns contextual commands for the omnibar.
func (v *ConfigView) ContextCommands() []ContextCommand {
	return v.commands
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering
// ─────────────────────────────────────────────────────────────────────────────

func (v *ConfigView) render() {
	var b strings.Builder

	currentSection := ""
	for i, f := range v.fields {
		// Section header
		if f.section != currentSection {
			if currentSection != "" {
				b.WriteString("\n")
			}
			currentSection = f.section
			b.WriteString(fmt.Sprintf("  %s%s%s\n\n",
				theme.ColorTag(theme.AccentHex), currentSection, theme.TagColor))
		}

		// Field line
		val := fmt.Sprintf("%v", v.viper.Get(f.key))
		displayVal := v.formatValue(f, val)

		if i == v.cursor {
			// Highlighted field
			b.WriteString(fmt.Sprintf("  %s▸%s  %s%-20s%s %s%s%s\n",
				theme.ColorTag(theme.ActionHex), theme.TagColor,
				theme.ColorTag(theme.TextPrimaryHex), f.label, theme.TagColor,
				theme.ColorTag(theme.ActionHex), displayVal, theme.TagColor))
		} else {
			// Normal field
			b.WriteString(fmt.Sprintf("     %s%-20s%s %s%s%s\n",
				theme.ColorTag(theme.TextSecondaryHex), f.label, theme.TagColor,
				theme.ColorTag(theme.TextPrimaryHex), displayVal, theme.TagColor))
		}
	}

	// Footer
	b.WriteString(fmt.Sprintf("\n\n  %s%s%s",
		theme.ColorTag(theme.TextMutedHex), config.ConfigPath(), theme.TagColor))

	v.display.SetText(b.String())
}

func (v *ConfigView) formatValue(f configField, rawValue string) string {
	if f.isBool {
		if rawValue == "true" {
			return "✓"
		}
		return "✗"
	}
	if f.options != nil {
		for _, opt := range f.options {
			if opt.Value == rawValue {
				return opt.Label
			}
		}
	}
	return rawValue
}

// ─────────────────────────────────────────────────────────────────────────────
// Editing
// ─────────────────────────────────────────────────────────────────────────────

func (v *ConfigView) toggleCurrentField() {
	if v.cursor < 0 || v.cursor >= len(v.fields) {
		return
	}
	f := v.fields[v.cursor]
	if !f.isBool {
		return
	}

	current := fmt.Sprintf("%v", v.viper.Get(f.key))
	if current == "true" {
		v.viper.Set(f.key, false)
	} else {
		v.viper.Set(f.key, true)
	}
	v.saveAndRefresh()
}

func (v *ConfigView) cycleCurrentField(direction int) {
	if v.cursor < 0 || v.cursor >= len(v.fields) {
		return
	}
	f := v.fields[v.cursor]
	if f.options == nil || f.isBool {
		return
	}

	current := fmt.Sprintf("%v", v.viper.Get(f.key))
	currentIdx := 0
	for i, opt := range f.options {
		if opt.Value == current {
			currentIdx = i
			break
		}
	}

	newIdx := (currentIdx + direction + len(f.options)) % len(f.options)
	v.viper.Set(f.key, f.options[newIdx].Value)
	v.saveAndRefresh()
}

func (v *ConfigView) editCurrentField() {
	if v.cursor < 0 || v.cursor >= len(v.fields) {
		return
	}
	f := v.fields[v.cursor]

	if f.isBool {
		v.toggleCurrentField()
		return
	}

	if f.options != nil {
		v.cycleCurrentField(1)
		return
	}

	// Free text — use shell input
	if v.shell != nil {
		current := fmt.Sprintf("%v", v.viper.Get(f.key))
		v.shell.ShowInputModal(f.label, current, func(newValue string) {
			v.viper.Set(f.key, newValue)
			v.saveAndRefresh()
		})
	}
}

func (v *ConfigView) setField(key, value string) {
	v.viper.Set(key, value)
	v.saveAndRefresh()
}

func (v *ConfigView) saveAndRefresh() {
	if err := v.viper.WriteConfigAs(config.ConfigPath()); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg("Erreur: "+err.Error(), false)
		}
		return
	}
	v.render()
	v.buildCommands() // refresh commands with new state
	if v.shell != nil {
		v.shell.ShowToastMsg("Configuration sauvegardée", true)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Contextual commands for omnibar
// ─────────────────────────────────────────────────────────────────────────────

func (v *ConfigView) buildCommands() {
	var cmds []ContextCommand

	for _, f := range v.fields {
		f := f // capture

		if f.isBool {
			// Toggle command: shows current state in description
			current := fmt.Sprintf("%v", v.viper.Get(f.key))
			var desc string
			if current == "true" {
				desc = "✓ → désactiver"
			} else {
				desc = "✗ → activer"
			}
			cmds = append(cmds, ContextCommand{
				ID:          "toggle." + f.key,
				Label:       f.label,
				Aliases:     []string{f.key},
				Description: desc,
				Category:    f.section,
				Action: func() {
					cur := fmt.Sprintf("%v", v.viper.Get(f.key))
					if cur == "true" {
						v.viper.Set(f.key, false)
					} else {
						v.viper.Set(f.key, true)
					}
					v.saveAndRefresh()
				},
			})
		} else if f.options != nil {
			// One command per possible value
			for _, opt := range f.options {
				opt := opt
				cmds = append(cmds, ContextCommand{
					ID:          f.key + "." + opt.Value,
					Label:       f.label + " " + opt.Label,
					Aliases:     []string{f.key, opt.Value},
					Description: f.section,
					Category:    f.section,
					Action: func() {
						v.setField(f.key, opt.Value)
					},
				})
			}
		} else {
			// Free text: command opens input
			cmds = append(cmds, ContextCommand{
				ID:          "edit." + f.key,
				Label:       f.label,
				Aliases:     []string{f.key},
				Description: "Modifier (texte libre)",
				Category:    f.section,
				Action: func() {
					if v.shell == nil {
						return
					}
					current := fmt.Sprintf("%v", v.viper.Get(f.key))
					v.shell.ShowInputModal(f.label, current, func(newValue string) {
						v.setField(f.key, newValue)
					})
				},
			})
		}
	}

	v.commands = cmds
}

// ─────────────────────────────────────────────────────────────────────────────
// Field definitions
// ─────────────────────────────────────────────────────────────────────────────

func buildConfigFields() []configField {
	return []configField{
		// CLI
		{key: "cli.language", label: "langue", section: "CLI", options: []SelectOption{
			{Label: "fr", Value: "fr"}, {Label: "en", Value: "en"},
		}},

		// Opencode
		{key: "opencode.channel", label: "canal", section: "Opencode", options: []SelectOption{
			{Label: "stable", Value: "stable"}, {Label: "beta", Value: "beta"},
		}},
		{key: "opencode.default_provider", label: "provider", section: "Opencode", options: []SelectOption{
			{Label: "bedrock", Value: "bedrock"},
			{Label: "anthropic", Value: "anthropic"},
			{Label: "openrouter", Value: "openrouter"},
			{Label: "github-copilot", Value: "github-copilot"},
		}},
		{key: "opencode.auto_update", label: "auto-update", section: "Opencode", isBool: true},
		{key: "opencode.install_dir", label: "install-dir", section: "Opencode"},

		// Provider auth
		{key: "provider.bedrock.auth_mode", label: "bedrock auth", section: "Provider", options: []SelectOption{
			{Label: "bearer", Value: "bearer"}, {Label: "profile", Value: "profile"}, {Label: "env", Value: "env"},
		}},
		{key: "provider.anthropic.auth_mode", label: "anthropic auth", section: "Provider", options: []SelectOption{
			{Label: "bearer", Value: "bearer"}, {Label: "profile", Value: "profile"}, {Label: "env", Value: "env"},
		}},
		{key: "provider.openrouter.auth_mode", label: "openrouter auth", section: "Provider", options: []SelectOption{
			{Label: "bearer", Value: "bearer"}, {Label: "profile", Value: "profile"}, {Label: "env", Value: "env"},
		}},

		// MCP
		{key: "mcp.figma.enabled", label: "figma", section: "MCP", isBool: true},
		{key: "mcp.figma.write_enabled", label: "figma écriture", section: "MCP", isBool: true},
		{key: "mcp.gitlab.enabled", label: "gitlab", section: "MCP", isBool: true},
		{key: "mcp.gitlab.write_enabled", label: "gitlab écriture", section: "MCP", isBool: true},
		{key: "mcp.gslides.enabled", label: "gslides", section: "MCP", isBool: true},
		{key: "mcp.gslides.write_enabled", label: "gslides écriture", section: "MCP", isBool: true},

		// Système
		{key: "worktree.auto_cleanup", label: "worktree cleanup", section: "Système", isBool: true},
		{key: "team.enabled", label: "team", section: "Système", isBool: true},
		{key: "websearch.enabled", label: "websearch", section: "Système", isBool: true},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Viper helpers
// ─────────────────────────────────────────────────────────────────────────────

// loadConfigViper creates a viper instance for reading/writing hub.toml.
func loadConfigViper() *viper.Viper {
	v := viper.New()
	v.SetConfigName("hub")
	v.SetConfigType("toml")
	v.AddConfigPath(config.HubDir())
	v.AddConfigPath(".")
	v.SetDefault("cli.language", "en")
	v.SetDefault("opencode.channel", "stable")
	v.SetDefault("opencode.auto_update", false)
	v.SetDefault("opencode.install_dir", filepath.Join(config.HubDir(), "bin"))
	v.SetDefault("websearch.enabled", false)
	_ = v.ReadInConfig()
	return v
}

// configFieldMeta is kept for backward compatibility with other views that
// may reference it. Use buildConfigFields() for the new config view.
var configFieldMeta = map[string][]SelectOption{
	"cli.language":                 {{Label: "Français", Value: "fr"}, {Label: "English", Value: "en"}},
	"opencode.channel":             {{Label: "Stable", Value: "stable"}, {Label: "Beta", Value: "beta"}},
	"opencode.default_provider":    {{Label: "Amazon Bedrock", Value: "bedrock"}, {Label: "Anthropic", Value: "anthropic"}, {Label: "OpenRouter", Value: "openrouter"}, {Label: "GitHub Copilot", Value: "github-copilot"}},
	"opencode.auto_update":         {{Label: "Activé", Value: "true"}, {Label: "Désactivé", Value: "false"}},
	"provider.bedrock.auth_mode":   {{Label: "Bearer token", Value: "bearer"}, {Label: "AWS Profile", Value: "profile"}, {Label: "Variables d'env", Value: "env"}},
	"provider.anthropic.auth_mode": {{Label: "Bearer token", Value: "bearer"}, {Label: "AWS Profile", Value: "profile"}, {Label: "Variables d'env", Value: "env"}},
	"provider.openrouter.auth_mode": {{Label: "Bearer token", Value: "bearer"}, {Label: "AWS Profile", Value: "profile"}, {Label: "Variables d'env", Value: "env"}},
	"mcp.figma.enabled":            {{Label: "Activé", Value: "true"}, {Label: "Désactivé", Value: "false"}},
	"mcp.figma.write_enabled":      {{Label: "Activé", Value: "true"}, {Label: "Désactivé", Value: "false"}},
	"mcp.gitlab.enabled":           {{Label: "Activé", Value: "true"}, {Label: "Désactivé", Value: "false"}},
	"mcp.gitlab.write_enabled":     {{Label: "Activé", Value: "true"}, {Label: "Désactivé", Value: "false"}},
	"mcp.gslides.enabled":          {{Label: "Activé", Value: "true"}, {Label: "Désactivé", Value: "false"}},
	"mcp.gslides.write_enabled":    {{Label: "Activé", Value: "true"}, {Label: "Désactivé", Value: "false"}},
	"worktree.auto_cleanup":        {{Label: "Activé", Value: "true"}, {Label: "Désactivé", Value: "false"}},
	"team.enabled":                 {{Label: "Activé", Value: "true"}, {Label: "Désactivé", Value: "false"}},
	"websearch.enabled":            {{Label: "Activé", Value: "true"}, {Label: "Désactivé", Value: "false"}},
}

// friendlyValue returns the display label for a constrained field value.
// Kept for backward compatibility with other views.
func friendlyValue(key, rawValue string) string {
	options, ok := configFieldMeta[key]
	if !ok {
		return rawValue
	}
	for _, opt := range options {
		if opt.Value == rawValue {
			return opt.Label
		}
	}
	return rawValue
}

// boolOptions is kept for backward compatibility.
var boolOptions = []SelectOption{
	{Label: "Activé", Value: "true"},
	{Label: "Désactivé", Value: "false"},
}

// sortedKeys returns sorted viper keys (kept for other views that may need it).
func sortedKeys(v *viper.Viper) []string {
	keys := v.AllKeys()
	sort.Strings(keys)
	return keys
}
