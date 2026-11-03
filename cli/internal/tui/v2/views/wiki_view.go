package views

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
	openRepo func() wikiState
	loads    loadSlot // the reads of the proposals and pages: only the latest is applied
	pageRead loadSlot // the read of the page being opened
}

// wikiState is the team-state of the view: pulled, its wiki read and
// answered, and the activity log appended to.
type wikiState interface {
	pullableRepo
	teamstate.RepoInfo
	teamstate.WikiReader
	teamstate.WikiWriter
	teamstate.EventWriter
}

// wikiData is what a read of the team wiki gives back.
type wikiData struct {
	cloned    bool
	proposals []teamstate.WikiProposal
	pages     []string
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
	v.loads.Invalidate()
	v.pageRead.Invalidate()
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
	if keys.Open.Matches(event) {
		if v.slist == nil {
			return nil
		}
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

// newRepo is the team-state of the team, without touching the disk (nil
// without a team).
func (v *WikiView) newRepo() wikiState {
	if v.openRepo != nil {
		return v.openRepo()
	}
	tc := v.resolveTeam()
	if !tc.Enabled {
		return nil
	}
	return teamstate.NewRepo(tc.StateRepo, tc.StatePath)
}

// cloned tells whether repo has its clone. It stats the disk: off the event
// loop.
func (v *WikiView) cloned(repo wikiState) bool {
	return v.openRepo != nil || repo.IsCloned()
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

	repo := v.newRepo()
	if repo == nil {
		v.loads.Invalidate()
		v.setHeader(stateEmpty, i18n.T("tui.patterns.team_not_configured"))
		return
	}
	v.setHeader(stateLoading, "")

	fetch := func(ctx context.Context) (wikiData, error) {
		if !v.cloned(repo) {
			return wikiData{}, nil
		}
		proposals, err := repo.WikiListPending(ctx)
		if err != nil {
			return wikiData{}, err
		}
		pages, err := repo.WikiListPages(ctx)
		return wikiData{cloned: true, proposals: proposals, pages: pages}, err
	}
	syncLoad(v.app, shellContext(v.shell), &v.loads, repo, v.shell, fetch, func(d wikiData, err error) {
		v.render(d, err)
	})
}

// setHeader writes a state in the header.
func (v *WikiView) setHeader(kind stateKind, msg string) {
	if v.header != nil {
		v.header.SetText(stateText(kind, msg))
	}
}

func (v *WikiView) setHeaderText(text string) {
	if v.header == nil {
		return
	}
	v.header.SetText(theme.Paint(theme.TextSecondaryHex, tview.Escape(text)))
}

func (v *WikiView) render(d wikiData, err error) {
	if v.slist == nil {
		return
	}
	switch {
	case err != nil:
		v.setHeader(stateError, err.Error())
		return
	case !d.cloned:
		v.setHeader(stateEmpty, i18n.T("tui.patterns.team_not_configured"))
		return
	}
	proposals, pages := d.proposals, d.pages
	v.proposals, v.pages = proposals, pages

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
			items = append(items, widgets.SectionItem{
				MainText: fmt.Sprintf("  %s %s", theme.Paint(theme.TextMutedHex, "\U0001F4C4"), tview.Escape(page)),
			})
			v.pageIndices[idx] = i
		}
	}

	if len(items) == 0 {
		items = append(items, widgets.SectionItem{
			MainText: "  " + stateText(stateEmpty, i18n.T("tui.wiki.empty_list")),
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
		d.title(tview.Escape(p.Page) + "  " + theme.Paint(theme.TextMutedHex, bracketed(p.ID)))
		d.field(i18n.T("tui.wiki.detail_author"), tview.Escape(p.Author))
		d.field(i18n.T("tui.wiki.detail_confidence"), tview.Escape(p.Confidence))
		d.field(i18n.T("tui.wiki.detail_project"), tview.Escape(p.Project))
		if !p.CreatedAt.IsZero() {
			d.field(i18n.T("tui.wiki.detail_date"), textfmt.DateTime(p.CreatedAt))
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
		d.keys(keys.Open.Keys(), i18n.T("tui.hints.see"), keys.Refresh.Keys(), keys.Refresh.Label())
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
		icon = theme.Paint(theme.SuccessHex, theme.SymSuccess)
	case "INFERRED":
		icon = theme.Paint(theme.WarningHex, theme.SymWarning)
	default: // UNCERTAIN or unknown
		icon = theme.Paint(theme.TextMutedHex, "?")
	}

	// Main line: icon + ID + page + author + relative date
	return widgets.SectionItem{
		MainText: fmt.Sprintf("  %s %s %s %s",
			icon, bracketed(p.ID), tview.Escape(p.Page),
			theme.Paint(theme.TextMutedHex, tview.Escape(p.Author)+" · "+tview.Escape(textfmt.AgoOrDate(p.CreatedAt)))),
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
		i18n.T("tui.wiki.detail_date"), textfmt.DateTime(p.CreatedAt),
		tview.Escape(p.Content),
	)

	v.shell.ShowScrollableModal(tview.Escape(i18n.Tf("tui.wiki.proposal_title", p.Page)), content, []ModalAction{
		{Label: i18n.T("tui.wiki.btn_accept"), Callback: func() {
			v.decide(p, true, "")
		}},
		{Label: i18n.T("tui.teamwiki.btn_refuse"), Danger: true, Callback: func() {
			v.askRejectReason(p)
		}},
		{Label: i18n.T("tui.wiki.btn_close"), Callback: func() {}},
	})
}

func (v *WikiView) showPage(pageIdx int) {
	if pageIdx < 0 || pageIdx >= len(v.pages) || v.shell == nil {
		return
	}
	pageName := v.pages[pageIdx]
	repo := v.newRepo()
	if repo == nil {
		return
	}
	sh := v.shell
	load(v.app, shellContext(sh), &v.pageRead, func(ctx context.Context) (string, error) {
		return repo.WikiReadPage(ctx, pageName)
	}, func(content string, err error) {
		if err != nil {
			sh.ShowToastMsg(i18n.Tf("tui.wiki.read_error", err.Error()), false)
			return
		}
		sh.ShowScrollableModal(tview.Escape(pageName), tview.Escape(content), []ModalAction{
			{Label: i18n.T("tui.wiki.btn_close"), Callback: func() {}},
		})
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
	v.decide(v.proposals[pi], true, "")
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
	v.askRejectReason(v.proposals[pi])
}

// askRejectReason refuses a proposal once the user gave the reason (n,
// button): the proposal vanishes from the team-state, so its author must be
// able to read why. Validating the reason is the confirmation; an empty one
// refuses nothing.
func (v *WikiView) askRejectReason(proposal teamstate.WikiProposal) {
	if v.shell == nil {
		return
	}
	v.shell.ShowInputModal(i18n.Tf("tui.teamwiki.refuse_reason_title", proposal.Page, proposal.ID), "", func(reason string) {
		reason = strings.Join(strings.Fields(reason), " ")
		if reason == "" {
			v.shell.ShowToastMsg(i18n.T("tui.teamwiki.reason_required"), false)
			return
		}
		v.decide(proposal, false, reason)
	})
}

// decide accepts or refuses a proposal off the event loop, writes the event
// of the decision, then refreshes the list.
func (v *WikiView) decide(proposal teamstate.WikiProposal, accept bool, reason string) {
	repo := v.newRepo()
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
	actor := v.resolveTeam().MemberID
	ctx := shellContext(v.shell)
	run := func(ctx context.Context, id string) error { return repo.WikiRejectProposal(ctx, id, reason) }
	eventType, failedKey, doneKey := teamstate.EventWikiRejected, "tui.wiki.reject_failed", "tui.wiki.proposal_rejected"
	data := map[string]interface{}{"proposal_id": proposal.ID, "page": proposal.Page, "reason": reason}
	if accept {
		run, eventType, failedKey, doneKey = repo.WikiAcceptProposal, teamstate.EventWikiAccepted, "tui.wiki.accept_failed", "tui.wiki.proposal_accepted"
		data = map[string]interface{}{"proposal_id": proposal.ID, "page": proposal.Page}
	}
	safego.Go(func() { // an action, not a read: its result is applied even if the view was refreshed
		var err error
		if !v.cloned(repo) {
			err = errors.New(i18n.T("tui.patterns.team_not_configured"))
		} else if err = run(ctx, proposal.ID); err == nil {
			// Emit event (best-effort)
			_ = repo.AppendEvent(ctx, teamstate.Event{
				Timestamp: time.Now().UTC(),
				Actor:     actor,
				Type:      eventType,
				Project:   proposal.Project,
				Data:      data,
			})
		}
		app.QueueUpdateDraw(func() {
			if v.app == nil {
				return // unmounted meanwhile
			}
			if err != nil {
				if v.shell != nil {
					v.shell.ShowToastMsg(i18n.Tf(failedKey, err.Error()), false)
				}
				return
			}
			if v.shell != nil {
				v.shell.ShowToastMsg(i18n.Tf(doneKey, proposal.Page), true)
			}
			v.refresh()
		})
	})
}

// ContextCommands implements CommandProvider.
func (v *WikiView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{ID: "wiki.accept", Label: i18n.T("tui.wiki.cmd_accept"), Aliases: []string{"accept", "approve", "merge"}, Description: i18n.T("tui.wiki.cmd_accept_desc"), Category: i18n.T("tui.teamwiki.title"), Action: func() { v.acceptSelected() }},
		{ID: "wiki.reject", Label: i18n.T("tui.teamwiki.cmd_refuse"), Aliases: []string{"reject", "refuse", "decline"}, Description: i18n.T("tui.teamwiki.cmd_refuse_desc"), Category: i18n.T("tui.teamwiki.title"), Action: func() { v.rejectSelected() }},
	}
}
