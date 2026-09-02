package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/plugin"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// PluginsView displays plugin status and allows install/remove.
type PluginsView struct {
	app    *tview.Application
	text   *tview.TextView
	shell  ShellAccess
	status plugin.PluginStatus
}

var _ View = (*PluginsView)(nil)
var _ CommandProvider = (*PluginsView)(nil)

// NewPluginsView creates a new plugins view.
func NewPluginsView() *PluginsView {
	return &PluginsView{}
}

// SetShell provides the shell reference for modal interactions.
func (v *PluginsView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *PluginsView) ID() string { return "plugins" }

// Title returns the display title.
func (v *PluginsView) Title() string { return "Plugins" }

// StatusHints returns keybinding hints.
func (v *PluginsView) StatusHints() string {
	return "i installer · d désinstaller · r refresh · Esc retour"
}

// Mount builds the plugin status display.
func (v *PluginsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.text = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	v.text.SetBackgroundColor(theme.BgPanel)
	v.text.SetBorderPadding(1, 0, 2, 2)

	// Show loading placeholder immediately
	muted := theme.ColorTag(theme.TextMutedHex)
	v.text.SetText(fmt.Sprintf("\n  %sVérification des plugins...%s", muted, theme.TagColor))
	content.AddItem(v.text, 0, 1, true)

	// Load plugin status asynchronously
	go func() {
		status := plugin.RTKStatus()
		app.QueueUpdateDraw(func() {
			if v.text == nil {
				return
			}
			v.status = status
			v.render()
		})
	}()
}

// Unmount cleans up resources.
func (v *PluginsView) Unmount() {
	v.app = nil
	v.text = nil
}

// HandleKey processes plugin view key events.
func (v *PluginsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'i':
		v.installPlugin()
		return nil
	case 'd':
		v.removePlugin()
		return nil
	case 'r':
		v.refresh()
		return nil
	}
	return event
}

func (v *PluginsView) refresh() {
	v.status = plugin.RTKStatus()
	v.render()
}

func (v *PluginsView) render() {
	if v.text == nil {
		return
	}

	s := v.status
	var text string

	text += fmt.Sprintf("  %s%s%s\n\n",
		theme.ColorTag(theme.AccentHex), "Plugin: RTK", theme.TagColor)

	// Installed status
	if s.Installed {
		text += fmt.Sprintf("  %sÉtat:%s      [green]✓ installé[-]\n", theme.ColorTag(theme.TextSecondaryHex), theme.TagColor)
		text += fmt.Sprintf("  %sPath:%s      %s\n", theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, s.Path)
	} else {
		text += fmt.Sprintf("  %sÉtat:%s      [red]✗ non installé[-]\n", theme.ColorTag(theme.TextSecondaryHex), theme.TagColor)
	}

	// Binary status
	if s.BinaryFound {
		text += fmt.Sprintf("  %sBinary:%s    [green]✓[-] rtk %s\n", theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, s.BinaryVer)
	} else {
		text += fmt.Sprintf("  %sBinary:%s    [red]✗ non trouvé[-] (rtk non installé dans PATH)\n", theme.ColorTag(theme.TextSecondaryHex), theme.TagColor)
	}

	text += "\n"

	// Actions hint
	if !s.Installed {
		text += fmt.Sprintf("  %sAppuyez 'i' pour installer le plugin RTK%s\n", theme.ColorTag(theme.TextMutedHex), theme.TagColor)
	} else {
		text += fmt.Sprintf("  %sAppuyez 'd' pour désinstaller%s\n", theme.ColorTag(theme.TextMutedHex), theme.TagColor)
	}

	v.text.SetText(text)
}

func (v *PluginsView) installPlugin() {
	if v.shell == nil {
		return
	}
	err := plugin.RTKInstall()
	if err != nil {
		v.shell.ShowToastMsg("Install échoué: "+err.Error(), false)
	} else {
		v.shell.ShowToastMsg("Plugin RTK installé", true)
	}
	v.refresh()
}

func (v *PluginsView) removePlugin() {
	if v.shell == nil {
		return
	}
	if !v.status.Installed {
		v.shell.ShowToastMsg("Plugin non installé", false)
		return
	}
	err := plugin.RTKRemove()
	if err != nil {
		v.shell.ShowToastMsg("Suppression échouée: "+err.Error(), false)
	} else {
		v.shell.ShowToastMsg("Plugin RTK supprimé", true)
	}
	v.refresh()
}

// ContextCommands implements CommandProvider.
func (v *PluginsView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{ID: "plugins.install", Label: "Installer", Aliases: []string{"install"}, Description: "Installer le plugin", Category: "Plugins", Action: func() {}},
		{ID: "plugins.refresh", Label: "Rafraîchir", Aliases: []string{"refresh", "reload"}, Description: "Rafraîchir le statut", Category: "Plugins", Action: func() {}},
	}
}
