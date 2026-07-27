package views

import (
	"context"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// TakeoverView displays and manages takeover briefs.
type TakeoverView struct {
	app         *tview.Application
	resolveTeam ResolveTeamFunc
	list        *tview.List
	shell       ShellAccess
	briefs      []teamstate.TakeoverMeta
	onEnrich    func(project, ticketID string) error
}

var _ View = (*TakeoverView)(nil)

// NewTakeoverView creates a new takeover briefs view.
// resolveTeam is called on every refresh to obtain the effective team config.
func NewTakeoverView(resolveTeam ResolveTeamFunc) *TakeoverView {
	return &TakeoverView{resolveTeam: resolveTeam}
}

// SetShell provides the shell reference for modal interactions.
func (v *TakeoverView) SetShell(s ShellAccess) { v.shell = s }

// SetOnEnrich sets the callback for AI enrichment of a brief.
func (v *TakeoverView) SetOnEnrich(fn func(project, ticketID string) error) { v.onEnrich = fn }

// ID returns the view identifier.
func (v *TakeoverView) ID() string { return "takeover-briefs" }

// Title returns the display title.
func (v *TakeoverView) Title() string { return "Takeover Briefs" }

// StatusHints returns keybinding hints.
func (v *TakeoverView) StatusHints() string {
	return "j/k nav · Enter voir · e enrichir (AI) · r refresh"
}

// Mount builds the takeover briefs list.
func (v *TakeoverView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.list = tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSecondaryTextColor(theme.FgSecondary)
	v.list.SetBackgroundColor(theme.BgPanel)
	v.list.SetBorderPadding(1, 0, 2, 2)

	v.refresh()
	content.AddItem(v.list, 0, 1, true)
}

// Unmount cleans up resources.
func (v *TakeoverView) Unmount() {
	v.app = nil
	v.list = nil
}

// HandleKey processes takeover view key events.
func (v *TakeoverView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'r':
		v.refresh()
		return nil
	case 'e':
		v.enrichBrief()
		return nil
	}
	if event.Key() == tcell.KeyEnter {
		v.showBrief()
		return nil
	}
	return event
}

func (v *TakeoverView) getRepo() *teamstate.Repo {
	tc := v.resolveTeam()
	if !tc.Enabled {
		return nil
	}
	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	if !repo.IsCloned() {
		return nil
	}
	return repo
}

func (v *TakeoverView) refresh() {
	v.list.Clear()
	v.briefs = nil

	repo := v.getRepo()
	if repo == nil {
		v.list.AddItem("  Team non configurée", "", 0, nil)
		return
	}

	_ = repo.Pull(context.Background())

	briefs, err := repo.ListBriefs("")
	if err != nil {
		v.list.AddItem("  Erreur: "+err.Error(), "", 0, nil)
		return
	}
	v.briefs = briefs

	if len(briefs) == 0 {
		v.list.AddItem("  Aucun takeover brief", "  Les briefs sont créés lors des transferts de tickets", 0, nil)
		return
	}

	for _, b := range briefs {
		date := b.TransferDate.Format("2006-01-02")
		v.list.AddItem(
			fmt.Sprintf("  %s  %s → %s", b.TicketID, b.TransferredFrom, b.TransferredTo),
			fmt.Sprintf("    %s · %s · %s", date, b.Project, b.Reason),
			0, nil,
		)
	}
}

func (v *TakeoverView) showBrief() {
	idx := v.list.GetCurrentItem()
	if idx < 0 || idx >= len(v.briefs) {
		return
	}
	b := v.briefs[idx]

	repo := v.getRepo()
	if repo == nil {
		return
	}

	content, err := repo.ReadBrief(b.Project, b.TicketID)
	if err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg("Erreur lecture: "+err.Error(), false)
		}
		return
	}

	if v.shell != nil {
		v.shell.ShowScrollableModal("Brief: "+b.TicketID, content, []ModalAction{
			{Label: "Fermer", Callback: func() {}},
		})
	}
}

func (v *TakeoverView) enrichBrief() {
	idx := v.list.GetCurrentItem()
	if idx < 0 || idx >= len(v.briefs) {
		return
	}
	b := v.briefs[idx]

	if v.shell == nil || v.onEnrich == nil {
		return
	}

	v.shell.ShowToastMsg("Enrichissement AI en cours: "+b.TicketID+"...", true)

	go func() {
		err := v.onEnrich(b.Project, b.TicketID)
		if v.app != nil {
			v.app.QueueUpdateDraw(func() {
				if err != nil {
					v.shell.ShowToastMsg("Enrichissement échoué: "+err.Error(), false)
				} else {
					v.shell.ShowToastMsg("Brief enrichi: "+b.TicketID, true)
					v.refresh()
				}
			})
		}
	}()
}
