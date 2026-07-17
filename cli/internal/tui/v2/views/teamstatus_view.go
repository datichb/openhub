package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// TeamStatusView displays team status (who works on what) and activity.
type TeamStatusView struct {
	app    *tview.Application
	appCtx *app.App
	tv     *tview.TextView
}

var _ View = (*TeamStatusView)(nil)

// NewTeamStatusView creates a new team status view.
func NewTeamStatusView(a *app.App) *TeamStatusView {
	return &TeamStatusView{appCtx: a}
}

// ID returns the view identifier.
func (v *TeamStatusView) ID() string { return "team.status" }

// Title returns the display title.
func (v *TeamStatusView) Title() string { return "Team Status" }

// StatusHints returns keybinding hints.
func (v *TeamStatusView) StatusHints() string { return "r refresh · Esc retour" }

// Mount builds the team status display.
func (v *TeamStatusView) Mount(content *tview.Flex, app *tview.Application) {
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
func (v *TeamStatusView) Unmount() {
	v.app = nil
	v.tv = nil
}

// HandleKey processes key events.
func (v *TeamStatusView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Rune() == 'r' {
		v.render()
		return nil
	}
	return event
}

func (v *TeamStatusView) render() {
	if v.tv == nil {
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n  [::b]Statut de l'équipe%s\n\n", theme.TagReset))

	// Check if team is configured
	if v.appCtx == nil || v.appCtx.Config == nil || !v.appCtx.Config.Team.Enabled {
		sb.WriteString(fmt.Sprintf("  %sÉquipe non configurée.%s\n\n",
			theme.ColorTag(theme.TextSecondaryHex), theme.TagColor))
		sb.WriteString(fmt.Sprintf("  %sUtilisez 'oh team init' pour configurer l'équipe.%s\n",
			theme.ColorTag(theme.TextSecondaryHex), theme.TagColor))
		v.tv.SetText(sb.String())
		return
	}

	sb.WriteString(fmt.Sprintf("  %sRepo :%s    %s\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, v.appCtx.Config.Team.StateRepo))
	sb.WriteString(fmt.Sprintf("  %sMembre :%s  %s\n\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, v.appCtx.Config.Team.MemberID))

	sb.WriteString(fmt.Sprintf("  %s─── Activité récente ───%s\n\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor))
	sb.WriteString(fmt.Sprintf("  %sAucune activité récente disponible.%s\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor))

	v.tv.SetText(sb.String())
}
