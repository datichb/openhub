package wiki

import (
	"context"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
)

// Proposals of a remote session (p3-remote-wiki): the job serves the wiki
// of its clone and queues the proposals of its agents, never applies them;
// they go back to the machine in the artifacts of the job (wiki.json) and
// are queued there, in the queue of the project, to be validated like the
// local ones (card of the commit checkpoint, doc-sync).

// Imported is a proposal of the job to queue on the machine.
type Imported struct {
	// ID is its id in the job (wp-<n> of the job queue).
	ID          string
	Agent       string
	ToolSession string
	Proposal
	CreatedAt time.Time
}

// Import queues the proposals of the job of a remote session in the queue
// of its project. The artifact is not trusted: each proposal is cleaned and
// bounded again like a wiki_propose, queued as pending whatever its status
// in the job, and a proposal the session already has (same page, section
// and content, any status: a second fetch) is kept, not copied. It returns
// the local id of each proposal of the job (job id → local id: the queue
// of the machine numbers its own) and the number added.
func (s *Service) Import(ctx context.Context, projectID, sessionID string, list []Imported) (ids map[string]string, added int, err error) {
	if s.Store == nil {
		return nil, 0, ErrNoStore
	}
	have, err := s.Store.ListWikiProposals(ctx, domain.WikiProposalFilter{ProjectID: projectID, SessionID: sessionID})
	if err != nil {
		return nil, 0, err
	}
	ids = map[string]string{}
	for i, e := range list {
		if i >= MaxPerSession {
			break
		}
		clean, err := CleanProposal(e.Proposal, s.SessionsDir)
		if err != nil {
			continue
		}
		if p, ok := sameProposal(have, clean); ok {
			ids[e.ID] = p.ID()
			continue
		}
		at := e.CreatedAt
		if at.IsZero() {
			at = s.now()
		}
		agent := oneLine(e.Agent, MaxSection)
		if agent == "" {
			agent = "?"
		}
		p := domain.WikiProposal{ProjectID: projectID, SessionID: sessionID, ToolSession: oneLine(e.ToolSession, MaxSection), Agent: agent,
			Page: clean.Page, Section: clean.Section, Content: clean.Content, Tag: clean.Tag, Source: clean.Source,
			Status: domain.WikiPending, CreatedAt: at}
		if err := s.Store.AddWikiProposal(ctx, &p, MaxPerSession, MaxPerProject); err != nil {
			return ids, added, err // domain.ErrWikiLimit: the queue is full
		}
		have = append(have, p)
		ids[e.ID] = p.ID()
		added++
	}
	return ids, added, nil
}

func sameProposal(list []domain.WikiProposal, c Proposal) (domain.WikiProposal, bool) {
	for _, p := range list {
		content := p.Content
		if p.Original != "" {
			content = p.Original // edited on the card: the agent's content
		}
		if p.Page == c.Page && strings.EqualFold(p.Section, c.Section) && content == c.Content {
			return p, true
		}
	}
	return domain.WikiProposal{}, false
}
