package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Wiki proposals on the checkpoint card (modernisation LD-J).

var _ views.CheckpointWikiDecider = (*tuiSessions)(nil)

// cardWiki renders the proposals of a card for the view.
func cardWiki(list []domain.WikiProposal) []views.CheckpointWikiProposal {
	out := make([]views.CheckpointWikiProposal, 0, len(list))
	for _, p := range list {
		out = append(out, views.CheckpointWikiProposal{ID: p.ID(), Agent: p.Agent, Page: p.Page, Section: p.Section, Tag: p.Tag,
			Source: p.Source, Status: p.Status, Content: p.Content})
	}
	return out
}

// CheckpointWikiDecide records the decision of the user on a proposal.
func (t *tuiSessions) CheckpointWikiDecide(ctx context.Context, sessionID, id, choice, content string) error {
	svc, err := t.service(ctx)
	if err != nil {
		return err
	}
	_, err = svc.WikiDecide(ctx, sessionID, id, choice, content)
	return err
}
