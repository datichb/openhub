package views

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/viper"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ConfigView displays and allows editing of hub configuration.
type ConfigView struct {
	app   *tview.Application
	table *tview.Table
	keys  []string
	viper *viper.Viper
	shell ShellAccess
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
}

var _ View = (*ConfigView)(nil)

// boolOptions defines the standard options for boolean config fields.
var boolOptions = []SelectOption{
	{Label: "Activé", Value: "true"},
	{Label: "Désactivé", Value: "false"},
}

// configFieldMeta maps config keys to their allowed options.
// Fields present here get a dropdown selector; all others use free-text input.
var configFieldMeta = map[string][]SelectOption{
	"cli.language": {
		{Label: "Français", Value: "fr"},
		{Label: "English", Value: "en"},
	},
	"opencode.channel": {
		{Label: "Stable", Value: "stable"},
		{Label: "Beta", Value: "beta"},
	},
	"opencode.default_provider": {
		{Label: "Amazon Bedrock", Value: "bedrock"},
		{Label: "Anthropic", Value: "anthropic"},
		{Label: "OpenRouter", Value: "openrouter"},
		{Label: "GitHub Copilot", Value: "github-copilot"},
	},
	"opencode.auto_update":       boolOptions,
	"provider.bedrock.auth_mode": {
		{Label: "Bearer token", Value: "bearer"},
		{Label: "AWS Profile", Value: "profile"},
		{Label: "Variables d'env", Value: "env"},
	},
	"provider.anthropic.auth_mode": {
		{Label: "Bearer token", Value: "bearer"},
		{Label: "AWS Profile", Value: "profile"},
		{Label: "Variables d'env", Value: "env"},
	},
	"provider.openrouter.auth_mode": {
		{Label: "Bearer token", Value: "bearer"},
		{Label: "AWS Profile", Value: "profile"},
		{Label: "Variables d'env", Value: "env"},
	},
	"mcp.figma.enabled":         boolOptions,
	"mcp.figma.write_enabled":   boolOptions,
	"mcp.gitlab.enabled":        boolOptions,
	"mcp.gitlab.write_enabled":  boolOptions,
	"mcp.gslides.enabled":       boolOptions,
	"mcp.gslides.write_enabled": boolOptions,
	"worktree.auto_cleanup":     boolOptions,
	"team.enabled":              boolOptions,
	"websearch.enabled":         boolOptions,
}

// friendlyValue returns the display label for a constrained field value,
// or the raw value if unconstrained or not found.
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

// NewConfigView creates a new config view.
func NewConfigView() *ConfigView { return &ConfigView{} }

// SetShell provides the shell reference for modal interactions.
func (v *ConfigView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *ConfigView) ID() string { return "config" }

// Title returns the display title.
func (v *ConfigView) Title() string { return "Configuration" }

// StatusHints returns keybinding hints.
func (v *ConfigView) StatusHints() string {
	return "j/k naviguer · Enter modifier · Esc retour"
}

// Mount builds the configuration table.
func (v *ConfigView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.viper = loadConfigViper()

	v.table = tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	v.table.SetBackgroundColor(theme.BgPanel)
	v.table.SetBorderPadding(1, 0, 2, 2)
	v.table.SetSelectedStyle(tcell.StyleDefault.
		Background(theme.BgElement).
		Foreground(theme.FgPrimary))

	v.populateTable()

	// Handle Enter to edit
	v.table.SetSelectedFunc(func(row, col int) {
		if row == 0 || row-1 >= len(v.keys) {
			return
		}
		key := v.keys[row-1]
		currentVal := fmt.Sprintf("%v", v.viper.Get(key))
		v.editKey(key, currentVal)
	})

	content.AddItem(v.table, 0, 1, true)
}

// Unmount cleans up resources.
func (v *ConfigView) Unmount() {
	v.app = nil
	v.table = nil
	v.viper = nil
}

// HandleKey processes view-specific key events.
func (v *ConfigView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	return event
}

func (v *ConfigView) populateTable() {
	v.table.Clear()

	// Header row
	headerStyle := tcell.StyleDefault.Foreground(theme.Accent).Bold(true)
	v.table.SetCell(0, 0, tview.NewTableCell("  Clé").SetStyle(headerStyle).SetSelectable(false))
	v.table.SetCell(0, 1, tview.NewTableCell("Valeur").SetStyle(headerStyle).SetSelectable(false))

	// Collect and sort keys
	allKeys := v.viper.AllKeys()
	sort.Strings(allKeys)
	v.keys = allKeys

	for i, key := range allKeys {
		val := fmt.Sprintf("%v", v.viper.Get(key))
		displayVal := friendlyValue(key, val)

		keyCell := tview.NewTableCell("  " + key).
			SetTextColor(theme.FgSecondary).
			SetExpansion(1)
		valCell := tview.NewTableCell(displayVal).
			SetTextColor(theme.FgPrimary).
			SetExpansion(1)

		v.table.SetCell(i+1, 0, keyCell)
		v.table.SetCell(i+1, 1, valCell)
	}

	// Footer: config file path
	footerRow := len(allKeys) + 2
	pathCell := tview.NewTableCell(fmt.Sprintf("  Fichier : %s", config.ConfigPath())).
		SetTextColor(theme.FgMuted).
		SetSelectable(false)
	v.table.SetCell(footerRow, 0, pathCell)
}

func (v *ConfigView) editKey(key, currentValue string) {
	if v.shell != nil {
		if options, ok := configFieldMeta[key]; ok {
			// Constrained field → dropdown selector
			v.shell.ShowSelectModal("Modifier "+key, options, currentValue, func(newValue string) {
				v.viper.Set(key, newValue)
				if err := v.viper.WriteConfigAs(config.ConfigPath()); err == nil {
					v.populateTable()
					v.shell.ShowToastMsg("Configuration sauvegardée", true)
				} else {
					v.shell.ShowToastMsg("Erreur: "+err.Error(), false)
				}
			})
		} else {
			// Free-text field → input modal
			v.shell.ShowInputModal("Modifier "+key, currentValue, func(newValue string) {
				v.viper.Set(key, newValue)
				if err := v.viper.WriteConfigAs(config.ConfigPath()); err == nil {
					v.populateTable()
					v.shell.ShowToastMsg("Configuration sauvegardée", true)
				} else {
					v.shell.ShowToastMsg("Erreur: "+err.Error(), false)
				}
			})
		}
		return
	}

	// Fallback: inline input field if no shell access
	var input *tview.InputField
	input = tview.NewInputField().
		SetLabel(key + " = ").
		SetLabelColor(theme.Accent).
		SetText(currentValue).
		SetFieldBackgroundColor(theme.BgElement).
		SetFieldTextColor(theme.FgPrimary).
		SetDoneFunc(func(tcellKey tcell.Key) {
			if tcellKey == tcell.KeyEnter {
				newVal := input.GetText()
				v.viper.Set(key, newVal)
				_ = v.viper.WriteConfigAs(config.ConfigPath())
				v.populateTable()
			}
			// Restore table focus
			v.app.SetFocus(v.table)
		})
	input.SetBackgroundColor(theme.BgElement)

	// Temporarily replace the table as the focused widget
	v.app.SetFocus(input)
}

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
