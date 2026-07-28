package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// TeamStatusView displays team status (who works on what) and activity.
type TeamStatusView struct {
	app         *tview.Application
	resolveTeam ResolveTeamFunc
	shell       ShellAccess
	tv          *tview.TextView
}

var _ View = (*TeamStatusView)(nil)

// NewTeamStatusView creates a new team status view.
// resolveTeam is called on every render to obtain the effective team config
// for the currently active project (per-project override → hub fallback).
func NewTeamStatusView(resolveTeam ResolveTeamFunc) *TeamStatusView {
	return &TeamStatusView{resolveTeam: resolveTeam}
}

// SetShell provides the shell reference for toast notifications.
func (v *TeamStatusView) SetShell(s ShellAccess) { v.shell = s }

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

	v.syncAndRender()
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
		v.syncAndRender()
		return nil
	}
	return event
}

// syncAndRender performs an async pull then re-renders the status view.
func (v *TeamStatusView) syncAndRender() {
	tc := v.resolveTeam()
	if !tc.Enabled {
		v.render(tc, nil)
		return
	}
	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	if !repo.IsCloned() {
		v.render(tc, nil)
		return
	}
	syncAsync(v.app, repo, v.shell, func(_ error) {
		v.render(tc, repo)
	})
}

func (v *TeamStatusView) render(tc TeamResolution, _ *teamstate.Repo) {
	if v.tv == nil {
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n  [::b]Statut de l'équipe%s\n\n", theme.TagReset))

	if !tc.Enabled {
		sb.WriteString(fmt.Sprintf("  %sÉquipe non configurée pour ce projet.%s\n\n",
			theme.ColorTag(theme.TextSecondaryHex), theme.TagColor))
		sb.WriteString(fmt.Sprintf("  %sUtilisez 'team configure' dans l'omnibar pour configurer.%s\n",
			theme.ColorTag(theme.TextSecondaryHex), theme.TagColor))
		v.tv.SetText(sb.String())
		return
	}

	sb.WriteString(fmt.Sprintf("  %sRepo :%s    %s\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, tc.StateRepo))
	sb.WriteString(fmt.Sprintf("  %sMembre :%s  %s\n\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, tc.MemberID))

	sb.WriteString(fmt.Sprintf("  %s─── Activité récente ───%s\n\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor))
	sb.WriteString(fmt.Sprintf("  %sAucune activité récente disponible.%s\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor))

	v.tv.SetText(sb.String())
}
