package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

// Wiki proposals on the card of the commit checkpoint (modernisation LD-J,
// CP-R4, tui-wiki-bricks): the card lists the open proposals of the
// session; its button opens them all in a list (wiki_proposals.go) where
// each one is accepted, modified, refused or left pending; oh applies the
// accepted ones when the checkpoint is passed, before the commit. Proposal
// texts come from agents: always escaped.

// wikiNewline stands for a line break in the one-line content field.
const wikiNewline = " ⏎ "

// Choices on a proposal (understood by the WikiService).
const (
	wikiAccept = "accept"
	wikiEdit   = "edit"
	wikiRefuse = "refuse"
	wikiPend   = "pending"
	// wikiEditor edits the content in $EDITOR (an edit for the service).
	wikiEditor = "editor"
)

// CheckpointWikiProposal is a wiki proposal on a checkpoint card.
type CheckpointWikiProposal = WikiProposalView

// CheckpointWikiDecider is implemented by session backends that can record
// the decision of the user on a proposal (optional: without it, the card
// only lists them).
type CheckpointWikiDecider interface {
	CheckpointWikiDecide(ctx context.Context, sessionID, id, choice, content string) error
}

// CheckpointWikiEditor is implemented by session backends that can edit
// the content of a proposal on several lines in $EDITOR (the terminal is
// suspended); it is called on the event loop.
type CheckpointWikiEditor interface {
	CheckpointWikiEdit(name, content string) (string, error)
}

// writeCheckpointWiki renders the wiki section of the card: one line per
// proposal, the list (button) shows their content.
func writeCheckpointWiki(b *strings.Builder, list []CheckpointWikiProposal) {
	if len(list) == 0 {
		return
	}
	fmt.Fprintf(b, "%s─ %s%s\n", theme.ColorTag(theme.TextMutedHex), i18n.Tf("tui.checkpoint.wiki", len(list)), theme.TagColor)
	accepted := 0
	for _, p := range list {
		page := p.Page
		if p.Section != "" {
			page += " § " + p.Section
		}
		head := strings.Join([]string{p.ID, firstText(p.Agent, "?"), page, wikiKind(p), p.Tag}, " · ")
		fmt.Fprintf(b, "  %s  %s\n", tview.Escape(head), muted(tview.Escape(wikiCardStatus(p.Status))))
		if p.Status == wikiStatusAccepted {
			accepted++
		}
	}
	fmt.Fprintf(b, "  %s\n", muted(i18n.Tf("tui.cpwiki.card_count", accepted, len(list)-accepted)))
	fmt.Fprintf(b, "  %s\n\n", muted(i18n.T("tui.checkpoint.wiki_hint")))
}

func wikiCardStatus(status string) string {
	if status == wikiStatusAccepted {
		return i18n.T("tui.checkpoint.wiki_accepted")
	}
	return i18n.T("tui.checkpoint.wiki_pending")
}

// wikiActions is the button of the card that opens the list of the
// proposals.
func (v *SessionsView) wikiActions(r *SessionRow, d *SessionDecision, list []CheckpointWikiProposal) []ModalAction {
	wd, ok := v.cfg.Backend.(CheckpointWikiDecider)
	if !ok || d.SessionID == "" || len(list) == 0 {
		return nil
	}
	dec, row := *d, r
	props := append([]CheckpointWikiProposal(nil), list...)
	return []ModalAction{{Label: i18n.Tf("tui.cpwiki.button", len(props)), Callback: func() {
		v.openCheckpointWiki(wd, row, &dec, props)
	}}}
}

// openCheckpointWiki shows the proposals of the card in a list (a view
// pushed over Sessions); Esc goes back to the card, Enter to the decision
// on the checkpoint.
func (v *SessionsView) openCheckpointWiki(wd CheckpointWikiDecider, r *SessionRow, d *SessionDecision, list []CheckpointWikiProposal) {
	if v.shell == nil {
		return
	}
	v.untrackDecision()
	cw := newCheckpointWikiView(v, wd, r, d, list)
	v.shell.PushView(cw)
}

// CheckpointWikiView is the list of the wiki proposals of a checkpoint
// card.
type CheckpointWikiView struct {
	sessions *SessionsView
	dec      SessionDecision
	row      *SessionRow
	shell    ShellAccess
	wiki     *wikiList
	// leaving: the view goes to the decision of the checkpoint (Enter), not
	// back to the card.
	leaving bool
}

var _ View = (*CheckpointWikiView)(nil)

func newCheckpointWikiView(v *SessionsView, wd CheckpointWikiDecider, r *SessionRow, d *SessionDecision, list []CheckpointWikiProposal) *CheckpointWikiView {
	cw := &CheckpointWikiView{sessions: v, dec: *d, row: r, shell: v.shell}
	sessionID := d.SessionID
	cfg := wikiListOpts{
		Title:  i18n.Tf("tui.cpwiki.title", firstText(d.Summary, d.ID), len(list)),
		Layout: DetailBelow,
		Shell:  v.shell,
		Decide: func(ctx context.Context, id, choice, content string) error {
			return wd.CheckpointWikiDecide(ctx, sessionID, id, choice, content)
		},
		OnOpen:    cw.decideCheckpoint,
		OpenLabel: i18n.T("tui.cpwiki.decide"),
	}
	if ed, ok := v.cfg.Backend.(CheckpointWikiEditor); ok {
		cfg.Edit = ed.CheckpointWikiEdit
	}
	cw.wiki = newWikiList(cfg)
	cw.wiki.set(list, nil)
	return cw
}

// SetShell injects the shell.
func (cw *CheckpointWikiView) SetShell(s ShellAccess) {
	cw.shell = s
	cw.wiki.cfg.Shell = s
	cw.wiki.list.cfg.Shell = s
}

// ID returns the view identifier.
func (cw *CheckpointWikiView) ID() string { return "sessions.checkpoint_wiki" }

// Title returns the display title.
func (cw *CheckpointWikiView) Title() string { return i18n.T("tui.cpwiki.view_title") }

// StatusHints returns the keys of the list, then Esc.
func (cw *CheckpointWikiView) StatusHints() string {
	return keys.Hints(cw.wiki.hints(), keys.Back.HintAs(i18n.T("tui.cpwiki.back")))
}

// Mount shows the list.
func (cw *CheckpointWikiView) Mount(content *tview.Flex, app *tview.Application) {
	ctx := context.Background()
	if cw.shell != nil {
		ctx = cw.shell.Context()
	}
	cw.leaving = false
	cw.wiki.mount(app, ctx)
	content.AddItem(cw.wiki.list.Primitive(), 0, 1, true)
	if app != nil {
		app.SetFocus(cw.wiki.list.Focusable())
	}
}

// Unmount goes back to the card (Esc), unless the view went to the
// decision or elsewhere (the Sessions view is not shown again then).
func (cw *CheckpointWikiView) Unmount() {
	app := cw.wiki.app
	cw.wiki.unmount()
	if cw.leaving || app == nil {
		return
	}
	v, dec, row := cw.sessions, cw.dec, cw.row
	// After the pop (Sessions mounted again); QueueUpdate waits for the
	// loop: never called on it.
	go app.QueueUpdateDraw(func() {
		if v.app != nil { // Sessions is shown again: back to its card
			v.openCheckpoint(row, &dec)
		}
	})
}

// HandleKey forwards the keys to the list.
func (cw *CheckpointWikiView) HandleKey(ev *tcell.EventKey) *tcell.EventKey {
	return cw.wiki.list.HandleKey(ev)
}

// decideCheckpoint is Enter: back to Sessions, then the decision form of
// the checkpoint (the card is read again: verification, approver).
func (cw *CheckpointWikiView) decideCheckpoint() {
	cw.leaving = true
	v, dec, row := cw.sessions, cw.dec, cw.row
	app := cw.wiki.app
	if cw.shell != nil {
		cw.shell.PopView()
	}
	if app == nil {
		return
	}
	go app.QueueUpdateDraw(func() {
		if v.app == nil {
			return
		}
		cb, ok := v.cfg.Backend.(CheckpointBackend)
		if !ok {
			v.checkpointForm(row, &dec, CheckpointApproverView{}, checkpointApprove)
			return
		}
		var card CheckpointCardView
		v.async(func(ctx context.Context) (string, error) {
			var err error
			card, err = cb.CheckpointCard(ctx, dec.ID)
			return "", err
		}, func(string) {
			v.checkpointForm(row, &dec, card.Approver, checkpointDefaultChoice(card.Verify), checkpointExtraChoices(card.Verify)...)
		})
	})
}

// fromWikiField restores the line breaks of the one-line content field.
func fromWikiField(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, wikiNewline, "\n"), strings.TrimSpace(wikiNewline), "\n"))
}
