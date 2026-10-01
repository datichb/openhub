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
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// WikiView displays wiki pages and pending proposals for human review.
type WikiView struct {
	app         *tview.Application
	resolveTeam ResolveTeamFunc
	slist       *widgets.SectionedList
	header      *tview.TextView
	contentFlex *tview.Flex
	shell       ShellAccess
	proposals   []teamstate.WikiProposal
	pages       []string

	// proposalIndices maps SectionedList item indices to proposals slice indices.
	proposalIndices map[int]int
	// pageIndices maps SectionedList item indices to pages slice indices.
	pageIndices map[int]int
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
func (v *WikiView) Title() string { return i18n.T("tui.team.wiki") }

// StatusHints returns keybinding hints.
func (v *WikiView) StatusHints() string {
	return fmt.Sprintf("j/k %s · {/} %s · Enter %s · a %s · x %s · r %s",
		i18n.T("tui.hints.nav"), "sections",
		i18n.T("tui.hints.see"), "accept", "reject",
		i18n.T("tui.hints.refresh"))
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
	v.header.SetBorderPadding(1, 0, 2, 2)

	// Sectioned list for proposals + pages
	v.slist = widgets.NewSectionedList().SetApp(app)
	v.slist.SetBorderPadding(0, 0, 2, 2)

	v.slist.SetItemSelectedFunc(func(idx int, item widgets.SectionItem) {
		if pi, ok := v.proposalIndices[idx]; ok {
			v.showProposal(pi)
			return
		}
		if pi, ok := v.pageIndices[idx]; ok {
			v.showPage(pi)
		}
	})

	// Vertical layout: header (fixed) + list (flexible)
	v.contentFlex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(v.header, 5, 0, false).
		AddItem(v.slist, 0, 1, true)
	v.contentFlex.SetBackgroundColor(theme.BgPanel)

	v.refresh()
	content.AddItem(v.contentFlex, 0, 1, true)
}

// Unmount cleans up resources.
func (v *WikiView) Unmount() {
	v.app = nil
	v.slist = nil
	v.header = nil
	v.contentFlex = nil
}

// HandleKey processes wiki view key events.
func (v *WikiView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'r':
		v.refresh()
		return nil
	case 'a':
		v.acceptSelected()
		return nil
	case 'x':
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
	if len(proposals) == 0 && len(pages) == 0 {
		v.setHeaderText("Wiki is empty. Proposals from agents will appear here for review.")
	} else if len(proposals) > 0 {
		v.setHeaderText(fmt.Sprintf("%d pending proposal(s) to review. Press Enter to view, 'a' to accept, 'x' to reject.", len(proposals)))
	} else {
		v.setHeaderText("Team wiki pages. Press Enter to view content.")
	}

	var items []widgets.SectionItem
	v.proposalIndices = make(map[int]int)
	v.pageIndices = make(map[int]int)

	// ── Pending Proposals ──
	if len(proposals) > 0 {
		items = append(items, widgets.SectionItem{
			MainText: fmt.Sprintf("  Pending Proposals (%d)", len(proposals)),
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
			MainText: fmt.Sprintf("  Wiki Pages (%d)", len(pages)),
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
			MainText: "  No wiki pages or proposals yet.",
		})
	}

	v.slist.SetItems(items)
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
	mainText := fmt.Sprintf("  %s [%s] %s %s%s · %s%s",
		icon,
		tview.Escape(p.ID),
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
	content := fmt.Sprintf("Author:     %s\nConfidence: %s\nProject:    %s\nDate:       %s\n\n────────────────────────────────\n\n%s",
		tview.Escape(p.Author),
		tview.Escape(p.Confidence),
		tview.Escape(p.Project),
		p.CreatedAt.Format("2006-01-02 15:04"),
		tview.Escape(p.Content),
	)

	v.shell.ShowScrollableModal("Proposal: "+tview.Escape(p.Page), content, []ModalAction{
		{Label: "[Accept]", Callback: func() {
			v.doAcceptProposal(p)
		}},
		{Label: "[Reject]", Callback: func() {
			v.doRejectProposal(p)
		}},
		{Label: "[Close]", Callback: func() {}},
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
		v.shell.ShowToastMsg("Read error: "+err.Error(), false)
		return
	}

	v.shell.ShowScrollableModal(tview.Escape(pageName), tview.Escape(content), []ModalAction{
		{Label: "[Close]", Callback: func() {}},
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
	v.doRejectProposal(v.proposals[pi])
}

func (v *WikiView) doAcceptProposal(proposal teamstate.WikiProposal) {
	repo := v.getRepo()
	if repo == nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.patterns.team_not_configured"), false)
		}
		return
	}

	go func() {
		err := repo.WikiAcceptProposal(context.Background(), proposal.ID)
		if v.app == nil {
			return
		}
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				if v.shell != nil {
					v.shell.ShowToastMsg("Accept failed: "+err.Error(), false)
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
				v.shell.ShowToastMsg("Proposal accepted: "+proposal.Page, true)
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

	go func() {
		err := repo.WikiRejectProposal(context.Background(), proposal.ID)
		if v.app == nil {
			return
		}
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				if v.shell != nil {
					v.shell.ShowToastMsg("Reject failed: "+err.Error(), false)
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
				v.shell.ShowToastMsg("Proposal rejected: "+proposal.Page, true)
			}
			v.refresh()
		})
	}()
}

// ContextCommands implements CommandProvider.
func (v *WikiView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{ID: "wiki.accept", Label: "Accept", Aliases: []string{"accept", "approve", "merge"}, Description: "Accept selected proposal", Category: "Wiki", Action: func() { v.acceptSelected() }},
		{ID: "wiki.reject", Label: "Reject", Aliases: []string{"reject", "refuse", "decline"}, Description: "Reject selected proposal", Category: "Wiki", Action: func() { v.rejectSelected() }},
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
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d min ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("02 Jan")
	}
}
