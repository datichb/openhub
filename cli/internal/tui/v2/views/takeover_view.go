package views

import (
	"context"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/textfmt"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// TakeoverView displays and manages takeover briefs.
type TakeoverView struct {
	app         *tview.Application
	resolveTeam ResolveTeamFunc
	list        *tview.List
	page        *widgets.ListDetailPage
	shell       ShellAccess
	briefs      []teamstate.TakeoverMeta
	onEnrich    func(project, ticketID string) error
	loads       loadSlot // the reads of the briefs: only the latest is applied
	briefRead   loadSlot // the read of the brief being opened

	// read replaces the read of the briefs (tests).
	read func(repo takeoverReader) (takeoverData, error)
}

// takeoverEnrichKey asks the AI to enrich the brief under the cursor.
var takeoverEnrichKey = keys.Own("takeover.enrich", 'e', "tui.hints.enrich_ai")

// takeoverReader is what the view reads of the team-state: whether the clone
// is there, and the briefs.
type takeoverReader interface {
	teamstate.RepoInfo
	teamstate.BriefReader
}

// takeoverState is the team-state of the view: pulled, then read.
type takeoverState interface {
	pullableRepo
	takeoverReader
}

// takeoverData is what a read of the briefs gives back.
type takeoverData struct {
	cloned bool
	briefs []teamstate.TakeoverMeta
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
func (v *TakeoverView) ID() string { return "team.briefs" }

// Title returns the display title.
func (v *TakeoverView) Title() string { return i18n.T("tui.team.briefs") }

// StatusHints returns keybinding hints.
func (v *TakeoverView) StatusHints() string {
	return keys.Hints(keys.Navigate.HintAs(i18n.T("tui.hints.nav")), keys.Open.HintAs(i18n.T("tui.hints.see")), takeoverEnrichKey.Hint(), keys.Refresh.Hint())
}

// Mount builds the takeover briefs list.
func (v *TakeoverView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.list = tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSecondaryTextColor(theme.FgSecondary).
		SetSelectedBackgroundColor(theme.BgElement).
		SetSelectedTextColor(theme.FgPrimary)
	v.list.SetBackgroundColor(theme.BgPanel)
	v.list.SetChangedFunc(func(idx int, _, _ string, _ rune) { v.showDetail(idx) })
	v.page = newListPage(v.Title(), v.list)

	v.refresh()
	v.page.Mount(content)
}

// showDetail fills the detail pane with the brief at idx.
func (v *TakeoverView) showDetail(idx int) {
	if v.page == nil {
		return
	}
	if idx < 0 || idx >= len(v.briefs) {
		v.page.SetDetail(emptyDetail())
		return
	}
	b := v.briefs[idx]
	var d detailText
	d.title(tview.Escape(b.TicketID))
	d.para(i18n.T("tui.detail.takeover.desc"))
	d.field(i18n.T("tui.detail.project"), tview.Escape(b.Project))
	d.field(i18n.T("tui.detail.from"), tview.Escape(b.TransferredFrom))
	d.field(i18n.T("tui.detail.to"), tview.Escape(b.TransferredTo))
	if !b.TransferDate.IsZero() {
		d.field(i18n.T("tui.detail.date"), textfmt.DateTime(b.TransferDate))
	}
	reason := b.Reason
	if b.StaleDays > 0 {
		reason += " · " + i18n.Tf("tui.detail.stale_days", b.StaleDays)
	}
	d.field(i18n.T("tui.detail.reason"), tview.Escape(reason))
	pairs := []string{keys.Open.Keys(), i18n.T("tui.hints.see")}
	if v.onEnrich != nil {
		pairs = append(pairs, takeoverEnrichKey.Keys(), takeoverEnrichKey.Label())
	}
	d.keys(append(pairs, keys.Refresh.Keys(), keys.Refresh.Label())...)
	v.page.SetDetail(d.String())
}

// Unmount cleans up resources.
func (v *TakeoverView) Unmount() {
	v.loads.Invalidate()
	v.briefRead.Invalidate()
	v.app = nil
	v.list = nil
	v.page = nil
}

// HandleKey processes takeover view key events.
func (v *TakeoverView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch {
	case keys.Refresh.Matches(event):
		v.refresh()
	case takeoverEnrichKey.Matches(event):
		v.enrichBrief()
	case keys.Open.Matches(event):
		v.showBrief()
	default:
		return event
	}
	return nil
}

// newRepo is the team-state of the team, without touching the disk (nil
// without a team).
func (v *TakeoverView) newRepo() takeoverState {
	tc := v.resolveTeam()
	if !tc.Enabled {
		return nil
	}
	return teamstate.NewRepo(tc.StateRepo, tc.StatePath)
}

// refresh pulls the team-state, then reads the briefs off the event loop.
func (v *TakeoverView) refresh() {
	if v.list == nil {
		return
	}
	v.list.Clear()
	v.briefs = nil

	repo := v.newRepo()
	if repo == nil {
		v.loads.Invalidate()
		v.list.AddItem("  "+stateText(stateEmpty, i18n.T("tui.takeover.team_not_configured")), "", 0, nil)
		v.showDetail(-1)
		return
	}
	v.list.AddItem("  "+stateText(stateLoading, ""), "", 0, nil)
	v.showDetail(-1)

	read := v.read
	if read == nil {
		read = readTakeover
	}
	fetch := func(context.Context) (takeoverData, error) { return read(repo) }
	syncLoad(v.app, shellContext(v.shell), &v.loads, repo, v.shell, fetch, func(d takeoverData, err error) {
		v.renderBriefs(d, err)
	})
}

// readTakeover reads the briefs. It reads files: call it off the event loop.
func readTakeover(repo takeoverReader) (takeoverData, error) {
	if !repo.IsCloned() {
		return takeoverData{}, nil
	}
	briefs, err := repo.ListBriefs("")
	return takeoverData{cloned: true, briefs: briefs}, err
}

func (v *TakeoverView) renderBriefs(d takeoverData, err error) {
	if v.list == nil {
		return
	}
	v.list.Clear()
	v.briefs = nil
	defer func() { v.showDetail(v.list.GetCurrentItem()) }()

	switch {
	case err != nil:
		v.list.AddItem("  "+stateText(stateError, err.Error()), "", 0, nil)
		return
	case !d.cloned:
		v.list.AddItem("  "+stateText(stateEmpty, i18n.T("tui.takeover.team_not_configured")), "", 0, nil)
		return
	case len(d.briefs) == 0:
		v.list.AddItem("  "+stateText(stateEmpty, i18n.T("tui.takeover.no_briefs")), i18n.T("tui.takeover.no_briefs_desc"), 0, nil)
		return
	}
	v.briefs = d.briefs

	for _, b := range d.briefs {
		v.list.AddItem(
			fmt.Sprintf("  %s  %s → %s", tview.Escape(b.TicketID), tview.Escape(b.TransferredFrom), tview.Escape(b.TransferredTo)),
			fmt.Sprintf("    %s · %s · %s", textfmt.Date(b.TransferDate), tview.Escape(b.Project), tview.Escape(b.Reason)),
			0, nil,
		)
	}
}

func (v *TakeoverView) showBrief() {
	idx := v.list.GetCurrentItem()
	if idx < 0 || idx >= len(v.briefs) || v.shell == nil {
		return
	}
	b := v.briefs[idx]
	repo := v.newRepo()
	if repo == nil {
		return
	}
	sh := v.shell
	load(v.app, shellContext(sh), &v.briefRead, func(context.Context) (string, error) {
		return repo.ReadBrief(b.Project, b.TicketID)
	}, func(content string, err error) {
		if err != nil {
			sh.ShowToastMsg(i18n.T("tui.takeover.read_error")+err.Error(), false)
			return
		}
		sh.ShowScrollableModal(i18n.Tf("tui.takeover.brief_title", b.TicketID), content, []ModalAction{
			{Label: i18n.T("tui.takeover.close"), Callback: func() {}},
		})
	})
}

func (v *TakeoverView) enrichBrief() {
	idx := v.list.GetCurrentItem()
	if idx < 0 || idx >= len(v.briefs) {
		return
	}
	b := v.briefs[idx]

	// The application is read on the event loop only (Unmount clears it).
	app := v.app
	if v.shell == nil || v.onEnrich == nil {
		return
	}
	enrich := v.onEnrich

	v.shell.ShowToastMsg(i18n.Tf("tui.takeover.enriching", b.TicketID), true)

	safego.Go(func() { // an action, not a read: its result is applied even if the list was refreshed
		err := enrich(b.Project, b.TicketID)
		if app == nil {
			return // no event loop (tests)
		}
		app.QueueUpdateDraw(func() {
			if v.app == nil {
				return // unmounted meanwhile
			}
			if err != nil {
				v.shell.ShowToastMsg(i18n.Tf("tui.takeover.enrich_failed", err.Error()), false)
				return
			}
			v.shell.ShowToastMsg(i18n.Tf("tui.takeover.enriched", b.TicketID), true)
			v.refresh()
		})
	})
}
