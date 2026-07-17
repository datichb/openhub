package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// StatusView displays the real hub and project status.
type StatusView struct {
	app    *tview.Application
	appCtx *app.App
	tv     *tview.TextView
}

var _ View = (*StatusView)(nil)

// NewStatusView creates a new status view.
func NewStatusView(a *app.App) *StatusView {
	return &StatusView{appCtx: a}
}

// ID returns the view identifier.
func (v *StatusView) ID() string { return "status" }

// Title returns the display title.
func (v *StatusView) Title() string { return "Status" }

// StatusHints returns keybinding hints.
func (v *StatusView) StatusHints() string { return "r refresh · Esc retour" }

// Mount builds the status display with real data.
func (v *StatusView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.tv = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	v.tv.SetBackgroundColor(theme.BgPanel)
	v.tv.SetBorderPadding(1, 0, 2, 2)

	v.render()
	content.AddItem(v.tv, 0, 1, true)
}

// Unmount cleans up resources.
func (v *StatusView) Unmount() {
	v.app = nil
	v.tv = nil
}

// HandleKey processes status view key events.
func (v *StatusView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Rune() == 'r' {
		v.render()
		return nil
	}
	return event
}

func (v *StatusView) render() {
	if v.tv == nil {
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n  [::b]Status du Hub%s\n\n", theme.TagReset))

	// Hub name
	hubName := "OpenHub"
	if v.appCtx != nil && v.appCtx.Config != nil {
		hubName = v.appCtx.Config.Name
	}
	sb.WriteString(fmt.Sprintf("  %sHub :%s              %s\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, hubName))

	// Config path
	sb.WriteString(fmt.Sprintf("  %sConfig :%s           %s\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, config.ConfigPath()))

	// Language
	lang := "en"
	if v.appCtx != nil && v.appCtx.Config != nil {
		lang = v.appCtx.Config.CLI.Language
	}
	sb.WriteString(fmt.Sprintf("  %sLangue :%s           %s\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, lang))

	// Opencode version
	ocVer, err := opencode.Version()
	if err != nil {
		ocVer = "non trouvé"
	}
	channel := "stable"
	if v.appCtx != nil && v.appCtx.Config != nil {
		channel = v.appCtx.Config.Opencode.Channel
	}
	sb.WriteString(fmt.Sprintf("  %sopencode :%s         %s (%s)\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, ocVer, channel))

	// Provider
	provider := "—"
	if v.appCtx != nil && v.appCtx.Config != nil {
		provider = v.appCtx.Config.Opencode.DefaultProvider
		if provider == "" {
			provider = "non configuré"
		}
	}
	sb.WriteString(fmt.Sprintf("  %sProvider :%s         %s\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, provider))

	// Projects count
	sb.WriteString("\n")
	projectCount := 0
	if v.appCtx != nil && v.appCtx.Projects != nil {
		projects, _ := v.appCtx.Projects.List(context.Background(), "")
		projectCount = len(projects)
	}
	sb.WriteString(fmt.Sprintf("  %sProjets :%s          %d enregistrés\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, projectCount))

	v.tv.SetText(sb.String())
}
