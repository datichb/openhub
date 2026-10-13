package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// Wiki proposals on the card of the commit checkpoint (modernisation LD-J,
// CP-R4): the open proposals of the session, each accepted, edited or
// refused from the card; oh applies the accepted ones when the checkpoint
// is passed, before the commit. Proposal texts come from agents: always
// escaped.

// checkpointCardWiki is the number of proposals with their own action on
// the card.
const checkpointCardWiki = 4

// wikiNewline stands for a line break in the one-line content field.
const wikiNewline = " ⏎ "

// Choices on a proposal (understood by the WikiService).
const (
	wikiAccept = "accept"
	wikiEdit   = "edit"
	wikiRefuse = "refuse"
	wikiPend   = "pending"
)

// CheckpointWikiProposal is a wiki proposal on a checkpoint card.
type CheckpointWikiProposal struct {
	ID      string
	Agent   string
	Page    string
	Section string
	Tag     string
	Source  string
	Status  string // pending | accepted
	Content string
}

// CheckpointWikiDecider is implemented by session backends that can record
// the decision of the user on a proposal (optional: without it, the card
// only lists them).
type CheckpointWikiDecider interface {
	CheckpointWikiDecide(ctx context.Context, sessionID, id, choice, content string) error
}

// writeCheckpointWiki renders the wiki section of the card.
func writeCheckpointWiki(b *strings.Builder, list []CheckpointWikiProposal) {
	if len(list) == 0 {
		return
	}
	fmt.Fprintf(b, "%s─ %s%s\n", theme.ColorTag(theme.TextMutedHex), i18n.Tf("tui.checkpoint.wiki", len(list)), theme.TagColor)
	for _, p := range list {
		page := p.Page
		if p.Section != "" {
			page += " § " + p.Section
		}
		head := fmt.Sprintf("%s · %s · %s · %s", p.ID, firstText(p.Agent, "?"), page, p.Tag)
		fmt.Fprintf(b, "  %s  %s\n", tview.Escape(head), muted(tview.Escape(wikiStatus(p.Status))))
		lines := strings.Split(p.Content, "\n")
		for i, l := range lines {
			if i == 2 {
				fmt.Fprintf(b, "    %s\n", muted("…"))
				break
			}
			fmt.Fprintf(b, "    %s\n", muted(tview.Escape(clipText(l, 160))))
		}
	}
	fmt.Fprintf(b, "  %s\n\n", muted(i18n.T("tui.checkpoint.wiki_hint")))
}

func wikiStatus(status string) string {
	if status == "accepted" {
		return i18n.T("tui.checkpoint.wiki_accepted")
	}
	return i18n.T("tui.checkpoint.wiki_pending")
}

// wikiActions are the actions of the card on the proposals: accept them
// all, then one per proposal (the first ones).
func (v *SessionsView) wikiActions(r *SessionRow, d *SessionDecision, list []CheckpointWikiProposal) []ModalAction {
	wd, ok := v.cfg.Backend.(CheckpointWikiDecider)
	if !ok || d.SessionID == "" || len(list) == 0 {
		return nil
	}
	dec, row := *d, r
	var out []ModalAction
	if len(list) > 1 {
		all := append([]CheckpointWikiProposal(nil), list...)
		out = append(out, ModalAction{Label: i18n.Tf("tui.checkpoint.wiki_accept_all", len(all)), Callback: func() {
			v.wikiDecide(row, &dec, func(ctx context.Context) error {
				for _, p := range all {
					if p.Status != "accepted" {
						if err := wd.CheckpointWikiDecide(ctx, dec.SessionID, p.ID, wikiAccept, ""); err != nil {
							return err
						}
					}
				}
				return nil
			})
		}})
	}
	for i, p := range list {
		if i == checkpointCardWiki {
			break
		}
		wp := p
		out = append(out, ModalAction{Label: i18n.Tf("tui.checkpoint.wiki_decide", wp.ID), Callback: func() {
			v.wikiForm(wd, row, &dec, wp)
		}})
	}
	return out
}

// wikiForm asks for the decision on a proposal: accept, edit (the content
// on one line, ⏎ for a line break), refuse, or leave it pending.
func (v *SessionsView) wikiForm(wd CheckpointWikiDecider, r *SessionRow, d *SessionDecision, p CheckpointWikiProposal) {
	if v.shell == nil {
		return
	}
	dec := *d
	choice := wikiAccept
	content := strings.ReplaceAll(p.Content, "\n", wikiNewline)
	v.shell.ShowInlineForm(InlineFormConfig{
		Title: tview.Escape(i18n.Tf("tui.checkpoint.wiki_form_title", p.ID, p.Page)),
		Fields: []FormField{
			{Key: "choice", Label: i18n.T("tui.inbox.decision"), Type: FieldSelect, Default: choice, Options: []SelectOption{
				{Label: i18n.T("tui.checkpoint.wiki_choice.accept"), Value: wikiAccept},
				{Label: i18n.T("tui.checkpoint.wiki_choice.edit"), Value: wikiEdit},
				{Label: i18n.T("tui.checkpoint.wiki_choice.refuse"), Value: wikiRefuse},
				{Label: i18n.T("tui.checkpoint.wiki_choice.pending"), Value: wikiPend},
			}},
			{Key: "content", Label: i18n.T("tui.checkpoint.wiki_content"), Type: FieldText, Default: content,
				Hint: i18n.T("tui.checkpoint.wiki_content_hint")},
		},
		OnSubmit: func(values map[string]string, _ map[string][]string) {
			ch := values["choice"]
			text := fromWikiField(values["content"])
			if ch == wikiAccept && text != fromWikiField(content) {
				ch = wikiEdit // the content was changed: an edit
			}
			if ch != wikiEdit {
				text = ""
			}
			v.wikiDecide(r, &dec, func(ctx context.Context) error {
				return wd.CheckpointWikiDecide(ctx, dec.SessionID, p.ID, ch, text)
			})
		},
		OnCancel: func() { v.openCheckpoint(r, &dec) },
	})
	v.trackDecision(dec.ID)
}

// fromWikiField restores the line breaks of the one-line content field.
func fromWikiField(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, wikiNewline, "\n"), strings.TrimSpace(wikiNewline), "\n"))
}

// wikiDecide records decisions off the event loop, then shows the card
// again.
func (v *SessionsView) wikiDecide(r *SessionRow, d *SessionDecision, fn func(ctx context.Context) error) {
	dec := *d
	v.async(func(ctx context.Context) (string, error) { return "", fn(ctx) }, func(string) {
		v.toast(i18n.T("tui.checkpoint.wiki_recorded"), true)
		v.openCheckpoint(r, &dec)
	})
}
