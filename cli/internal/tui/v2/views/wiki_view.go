package views

import (
	"context"
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// WikiView is « Propositions d'équipe (team-state) »: the pages of the
// wiki of the team-state and the proposals waiting there (teamstate
// WikiListPending), y accept, n refuse (confirmed). The wiki of a project
// and its queue are in project.wiki.
type WikiView struct {
	app         *tview.Application
	resolveTeam ResolveTeamFunc
	slist       *widgets.SectionedList
	header      *tview.TextView
	page        *widgets.ListDetailPage
	shell       ShellAccess
	proposals   []teamstate.WikiProposal
	pages       []string

	// proposalIndices maps SectionedList item indices to proposals slice indices.
	proposalIndices map[int]int
	// pageIndices maps SectionedList item indices to pages slice indices.
	pageIndices map[int]int

	// openRepo replaces the clone of the team-state (tests).
	openRepo func() teamstate.TeamStateWriter
}

var _ View = (*WikiView)(nil)
var _ CommandProvider = (*WikiView)(nil)

// NewWikiView creates a new wiki view.
// resolveTeam is called on every refresh to obtain the effective team config.
func NewWikiView(resolveTeam ResolveTeamFunc) *WikiView {
	return &WikiView{resolveTeam: resolveTeam}
}

// SetShell provides the shell reference for modal interactions.
func (v *WikiView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *WikiView) ID() string { return "team.wiki" }

// Title returns the display title.
func (v *WikiView) Title() string { return i18n.T("tui.teamwiki.title") }

// StatusHints returns keybinding hints.
func (v *WikiView) StatusHints() string {
	return keys.Hints(keys.Navigate.Hint(), "{/} "+i18n.T("tui.teamwiki.sections"), keys.Open.HintAs(i18n.T("tui.hints.see")),
		keys.Accept.HintAs(i18n.T("tui.wikilist.accept")), keys.Refuse.Hint(), keys.Refresh.Hint())
}

// Mount builds the wiki view with a descriptive header and SectionedList.
func (v *WikiView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	// Fixed-height descriptive header with word-wrap
	v.header = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false).
		SetWordWrap(true)
	v.header.SetBackgroundColor(theme.BgPanel)

	// Sectioned list for proposals + pages
	v.slist = widgets.NewSectionedList().SetApp(app)
	v.slist.SetItemChangedFunc(func(idx int, _ widgets.SectionItem) { v.showItemDetail(idx) })

	v.slist.SetItemSelectedFunc(func(idx int, item widgets.SectionItem) {
		if pi, ok := v.proposalIndices[idx]; ok {
			v.showProposal(pi)
			return
		}
		if pi, ok := v.pageIndices[idx]; ok {
			v.showPage(pi)
		}
	})

	// The header stays above the list, the detail on the right.
	v.page = widgets.NewListDetailPage(v.Title(), i18n.T("tui.catalog.detail"))
	v.page.AddHeader(v.header, 5)
	v.page.SetMain(v.slist)

	v.refresh()
	v.page.Mount(content)
}

// Unmount cleans up resources.
func (v *WikiView) Unmount() {
	v.app = nil
	v.slist = nil
	v.header = nil
	v.page = nil
}

// HandleKey processes wiki view key events.
func (v *WikiView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch {
	case keys.Refresh.Matches(event):
		v.refresh()
		return nil
	case keys.Accept.Matches(event):
		v.acceptSelected()
		return nil
	case keys.Refuse.Matches(event):
		v.rejectSelected()
		return nil
	}
	if event.Key() == tcell.KeyEnter {
		if idx, _, ok := v.slist.CurrentItem(); ok {
			if pi, exists := v.proposalIndices[idx]; exists {
				v.showProposal(pi)
			} else if pi, exists := v.pageIndices[idx]; exists {
				v.showPage(pi)
			}
		}
		return nil
	}
	return event
}

func (v *WikiView) getRepo() teamstate.TeamStateWriter {
	if v.openRepo != nil {
		return v.openRepo()
	}
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

func (v *WikiView) refresh() {
	v.proposals = nil
	v.pages = nil
	v.proposalIndices = nil
	v.pageIndices = nil
	if v.slist != nil {
		v.slist.SetItems(nil)
	}
	setPageDetail(v.page, emptyDetail())

	repo := v.getRepo()
	if repo == nil {
		v.setHeaderText(i18n.T("tui.patterns.team_not_configured"))
		return
	}

	syncAsync(v.app, repo, v.shell, func(_ error) {
		v.renderContent(repo)
	})
}

func (v *WikiView) setHeaderText(text string) {
	if v.header == nil {
		return
	}
	secondary := theme.ColorTag(theme.TextSecondaryHex)
	reset := theme.TagReset
	v.header.SetText(fmt.Sprintf("%s%s%s", secondary, text, reset))
}

func (v *WikiView) renderContent(repo teamstate.TeamStateWriter) {
	if v.slist == nil {
		return
	}

	// Load proposals
	proposals, propErr := repo.WikiListPending()
	if propErr != nil {
		v.slist.SetItems([]widgets.SectionItem{
			{MainText: "  " + i18n.T("tui.settings.error") + ": " + propErr.Error()},
		})
		return
	}
	v.proposals = proposals

	// Load pages
	pages, pageErr := repo.WikiListPages()
	if pageErr != nil {
		v.slist.SetItems([]widgets.SectionItem{
			{MainText: "  " + i18n.T("tui.settings.error") + ": " + pageErr.Error()},
		})
		return
	}
	v.pages = pages

	// Update header text
	switch {
	case len(proposals) == 0 && len(pages) == 0:
		v.setHeaderText(i18n.T("tui.wiki.empty_state"))
	case len(proposals) > 0:
		v.setHeaderText(i18n.Tf("tui.teamwiki.pending_header", len(proposals)))
	default:
		v.setHeaderText(i18n.T("tui.wiki.pages_header_hint"))
	}

	var items []widgets.SectionItem
	v.proposalIndices = make(map[int]int)
	v.pageIndices = make(map[int]int)

	// ── Pending Proposals ──
	if len(proposals) > 0 {
		items = append(items, widgets.SectionItem{
			MainText: "  " + i18n.Tf("tui.wiki.section_pending", len(v.proposals)),
			IsHeader: true,
		})
		for i, p := range proposals {
			idx := len(items)
			items = append(items, v.buildProposalItem(p))
			v.proposalIndices[idx] = i
		}
	}

	// ── Wiki Pages ──
	if len(pages) > 0 {
		items = append(items, widgets.SectionItem{
			MainText: "  " + i18n.Tf("tui.wiki.section_pages", len(v.pages)),
			IsHeader: true,
		})
		for i, page := range pages {
			idx := len(items)
			muted := theme.ColorTag(theme.TextMutedHex)
			reset := theme.TagReset
			items = append(items, widgets.SectionItem{
				MainText: fmt.Sprintf("  %s\U0001F4C4%s %s", muted, reset, tview.Escape(page)),
			})
			v.pageIndices[idx] = i
		}
	}

	if len(items) == 0 {
		items = append(items, widgets.SectionItem{
			MainText: "  " + i18n.T("tui.wiki.empty_list"),
		})
	}

	v.slist.SetItems(items)
	idx, _, _ := v.slist.CurrentItem()
	v.showItemDetail(idx)
}

// showItemDetail fills the detail pane with the list item at idx (proposal
// or page).
func (v *WikiView) showItemDetail(idx int) {
	if v.page == nil {
		return
	}
	if pi, ok := v.proposalIndices[idx]; ok && pi >= 0 && pi < len(v.proposals) {
		p := v.proposals[pi]
		var d detailText
		d.title(tview.Escape(p.Page) + "  " + theme.ColorTag(theme.TextMutedHex) + tview.Escape("["+p.ID+"]") + theme.TagReset)
		d.field(i18n.T("tui.wiki.detail_author"), tview.Escape(p.Author))
		d.field(i18n.T("tui.wiki.detail_confidence"), tview.Escape(p.Confidence))
		d.field(i18n.T("tui.wiki.detail_project"), tview.Escape(p.Project))
		if !p.CreatedAt.IsZero() {
			d.field(i18n.T("tui.wiki.detail_date"), p.CreatedAt.Format("2006-01-02 15:04"))
		}
		d.keys(keys.Open.Keys(), i18n.T("tui.hints.see"), keys.Accept.Keys(), i18n.T("tui.wikilist.accept"), keys.Refuse.Keys(), keys.Refuse.Label())
		d.gap().para(tview.Escape(p.Content))
		v.page.SetDetail(d.String())
		return
	}
	if pi, ok := v.pageIndices[idx]; ok && pi >= 0 && pi < len(v.pages) {
		var d detailText
		d.title(tview.Escape(v.pages[pi]))
		d.field(i18n.T("tui.detail.path"), tview.Escape("wiki/"+v.pages[pi]+".md"))
		d.keys("Enter", i18n.T("tui.hints.see"), "r", i18n.T("tui.hints.refresh"))
		v.page.SetDetail(d.String())
		return
	}
	v.page.SetDetail(emptyDetail())
}

func (v *WikiView) buildProposalItem(p teamstate.WikiProposal) widgets.SectionItem {
	// Icon based on confidence level
	var icon string
	switch p.Confidence {
	case "CONFIRMED":
		icon = fmt.Sprintf("%s●%s", theme.ColorTag(theme.SuccessHex), theme.TagReset)
	case "INFERRED":
		icon = fmt.Sprintf("%s⚠%s", theme.ColorTag(theme.WarningHex), theme.TagReset)
	default: // UNCERTAIN or unknown
		icon = fmt.Sprintf("%s?%s", theme.ColorTag(theme.TextMutedHex), theme.TagReset)
	}

	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagReset

	// Main line: icon + ID + page + author + relative date
	relDate := wikiFormatTimeAgo(p.CreatedAt)
	mainText := fmt.Sprintf("  %s %s %s %s%s · %s%s",
		icon,
		tview.Escape("["+p.ID+"]"),
		tview.Escape(p.Page),
		muted, tview.Escape(p.Author),
		tview.Escape(relDate), reset)

	return widgets.SectionItem{
		MainText: mainText,
	}
}

func (v *WikiView) showProposal(proposalIdx int) {
	if proposalIdx < 0 || proposalIdx >= len(v.proposals) {
		return
	}
	if v.shell == nil {
		return
	}
	p := v.proposals[proposalIdx]

	// Build detail content
	content := fmt.Sprintf("%s     %s\n%s %s\n%s    %s\n%s       %s\n\n────────────────────────────────\n\n%s",
		i18n.T("tui.wiki.detail_author"), tview.Escape(p.Author),
		i18n.T("tui.wiki.detail_confidence"), tview.Escape(p.Confidence),
		i18n.T("tui.wiki.detail_project"), tview.Escape(p.Project),
		i18n.T("tui.wiki.detail_date"), p.CreatedAt.Format("2006-01-02 15:04"),
		tview.Escape(p.Content),
	)

	v.shell.ShowScrollableModal(tview.Escape(i18n.Tf("tui.wiki.proposal_title", p.Page)), content, []ModalAction{
		{Label: i18n.T("tui.wiki.btn_accept"), Callback: func() {
			v.doAcceptProposal(p)
		}},
		{Label: i18n.T("tui.teamwiki.btn_refuse"), Danger: true, Callback: func() {
			v.confirmReject(p)
		}},
		{Label: i18n.T("tui.wiki.btn_close"), Callback: func() {}},
	})
}

func (v *WikiView) showPage(pageIdx int) {
	if pageIdx < 0 || pageIdx >= len(v.pages) {
		return
	}
	if v.shell == nil {
		return
	}

	pageName := v.pages[pageIdx]
	repo := v.getRepo()
	if repo == nil {
		return
	}

	content, err := repo.WikiReadPage(pageName)
	if err != nil {
		v.shell.ShowToastMsg(i18n.Tf("tui.wiki.read_error", err.Error()), false)
		return
	}

	v.shell.ShowScrollableModal(tview.Escape(pageName), tview.Escape(content), []ModalAction{
		{Label: i18n.T("tui.wiki.btn_close"), Callback: func() {}},
	})
}

func (v *WikiView) acceptSelected() {
	if v.slist == nil {
		return
	}
	idx, _, ok := v.slist.CurrentItem()
	if !ok {
		return
	}
	pi, isProposal := v.proposalIndices[idx]
	if !isProposal || pi < 0 || pi >= len(v.proposals) {
		return
	}
	v.doAcceptProposal(v.proposals[pi])
}

func (v *WikiView) rejectSelected() {
	if v.slist == nil {
		return
	}
	idx, _, ok := v.slist.CurrentItem()
	if !ok {
		return
	}
	pi, isProposal := v.proposalIndices[idx]
	if !isProposal || pi < 0 || pi >= len(v.proposals) {
		return
	}
	v.confirmReject(v.proposals[pi])
}

// confirmReject refuses a proposal once confirmed (n, button).
func (v *WikiView) confirmReject(proposal teamstate.WikiProposal) {
	if v.shell == nil {
		return
	}
	v.shell.Confirm(i18n.T("tui.teamwiki.refuse_title"), i18n.Tf("tui.confirm.wiki_reject.detail", proposal.Page, proposal.ID), true, func() {
		v.doRejectProposal(proposal)
	})
}

func (v *WikiView) doAcceptProposal(proposal teamstate.WikiProposal) {
	repo := v.getRepo()
	if repo == nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.patterns.team_not_configured"), false)
		}
		return
	}

	// The application is read on the event loop only (Unmount clears it).
	app := v.app
	if app == nil {
		return
	}
	go func() {
		err := repo.WikiAcceptProposal(context.Background(), proposal.ID)
		app.QueueUpdateDraw(func() {
			if v.app == nil {
				return // unmounted meanwhile
			}
			if err != nil {
				if v.shell != nil {
					v.shell.ShowToastMsg(i18n.Tf("tui.wiki.accept_failed", err.Error()), false)
				}
				return
			}
			// Emit event (best-effort)
			_ = repo.AppendEvent(context.Background(), teamstate.Event{
				Timestamp: time.Now().UTC(),
				Actor:     v.resolveTeam().MemberID,
				Type:      teamstate.EventWikiAccepted,
				Project:   proposal.Project,
				Data: map[string]interface{}{
					"proposal_id": proposal.ID,
					"page":        proposal.Page,
				},
			})
			if v.shell != nil {
				v.shell.ShowToastMsg(i18n.Tf("tui.wiki.proposal_accepted", proposal.Page), true)
			}
			v.refresh()
		})
	}()
}

func (v *WikiView) doRejectProposal(proposal teamstate.WikiProposal) {
	repo := v.getRepo()
	if repo == nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.patterns.team_not_configured"), false)
		}
		return
	}

	// The application is read on the event loop only (Unmount clears it).
	app := v.app
	if app == nil {
		return
	}
	go func() {
		err := repo.WikiRejectProposal(context.Background(), proposal.ID)
		app.QueueUpdateDraw(func() {
			if v.app == nil {
				return // unmounted meanwhile
			}
			if err != nil {
				if v.shell != nil {
					v.shell.ShowToastMsg(i18n.Tf("tui.wiki.reject_failed", err.Error()), false)
				}
				return
			}
			// Emit event (best-effort)
			_ = repo.AppendEvent(context.Background(), teamstate.Event{
				Timestamp: time.Now().UTC(),
				Actor:     v.resolveTeam().MemberID,
				Type:      teamstate.EventWikiRejected,
				Project:   proposal.Project,
				Data: map[string]interface{}{
					"proposal_id": proposal.ID,
					"page":        proposal.Page,
				},
			})
			if v.shell != nil {
				v.shell.ShowToastMsg(i18n.Tf("tui.wiki.proposal_rejected", proposal.Page), true)
			}
			v.refresh()
		})
	}()
}

// ContextCommands implements CommandProvider.
func (v *WikiView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{ID: "wiki.accept", Label: i18n.T("tui.wiki.cmd_accept"), Aliases: []string{"accept", "approve", "merge"}, Description: i18n.T("tui.wiki.cmd_accept_desc"), Category: i18n.T("tui.teamwiki.title"), Action: func() { v.acceptSelected() }},
		{ID: "wiki.reject", Label: i18n.T("tui.teamwiki.cmd_refuse"), Aliases: []string{"reject", "refuse", "decline"}, Description: i18n.T("tui.teamwiki.cmd_refuse_desc"), Category: i18n.T("tui.teamwiki.title"), Action: func() { v.rejectSelected() }},
	}
}

// wikiFormatTimeAgo returns a human-readable relative time string.
func wikiFormatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return i18n.T("tui.wiki.time_just_now")
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return i18n.T("tui.wiki.time_1min")
		}
		return i18n.Tf("tui.wiki.time_nmin", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return i18n.T("tui.wiki.time_1hour")
		}
		return i18n.Tf("tui.wiki.time_nhours", h)
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return i18n.T("tui.wiki.time_yesterday")
		}
		return i18n.Tf("tui.wiki.time_ndays", days)
	default:
		return t.Format("02 Jan")
	}
}
