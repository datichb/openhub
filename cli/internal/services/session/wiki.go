package session

import (
	"context"
	"errors"
	"slices"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Wiki proposals on the card of the commit checkpoint (modernisation LD-J,
// CP-R4): the open proposals of the session, each accepted, edited or
// refused there; oh applies the accepted ones when the checkpoint is
// passed (the daemon, before the commit).

var errNoWiki = errors.New("session: no wiki proposal store")

// cardWiki returns the open proposals of the session when checkpoint id
// unlocks the commit (the last one, when another unlocks it again); those of the whole queue of the project when it
// synchronizes the wiki (`wiki: sync`, doc-sync, LD-D).
func (s *Service) cardWiki(ctx context.Context, d domain.Decision, id string) ([]domain.WikiProposal, bool) {
	if s.Wiki == nil || s.Checkpoints == nil || s.Sessions == nil {
		return nil, false
	}
	wf, _, err := s.Checkpoints.Workflow(ctx, d.SessionID)
	if err != nil || wf == nil {
		return nil, false
	}
	c, ok := wf.Checkpoint(id)
	if !ok || !slices.Contains(c.Unlocks, sessionspec.UnlockCommit) || c.CommitNext != "" {
		return nil, false // p5-fixes: the proposals wait for the last commit checkpoint
	}
	sess, err := s.Sessions.Get(ctx, d.SessionID)
	if err != nil {
		return nil, true
	}
	scope := sess.ID
	if c.Wiki == sessionspec.WikiSync {
		scope = ""
	}
	open, err := s.Wiki.Open(ctx, sess.ProjectID, scope)
	if err != nil {
		return nil, true
	}
	return open, true
}

// syncsWiki reports whether the workflow of a session synchronizes the
// wiki (a checkpoint `wiki: sync`): its user decides on the whole queue.
func (s *Service) syncsWiki(ctx context.Context, sessionID string) bool {
	if s.Checkpoints == nil {
		return false
	}
	wf, _, err := s.Checkpoints.Workflow(ctx, sessionID)
	if err != nil || wf == nil {
		return false
	}
	for _, c := range wf.Checkpoints {
		if c.Wiki == sessionspec.WikiSync {
			return true
		}
	}
	return false
}

// WikiProposals returns the proposals of a session (all statuses).
func (s *Service) WikiProposals(ctx context.Context, sessionID string) ([]domain.WikiProposal, error) {
	if s.Wiki == nil {
		return nil, errNoWiki
	}
	sess, err := s.Sessions.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return s.Wiki.List(ctx, sess.ProjectID, sess.ID)
}

// WikiDecide records the decision of the user on a proposal of a session
// (wiki.ChoiceAccept, ChoiceEdit with the new content, ChoiceRefuse with
// the optional reason as content, ChoicePend); a session that synchronizes the wiki decides on the whole
// queue of its project.
func (s *Service) WikiDecide(ctx context.Context, sessionID, id, choice, content string) (*domain.WikiProposal, error) {
	if s.Wiki == nil {
		return nil, errNoWiki
	}
	sess, err := s.Sessions.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	p, err := s.Wiki.Get(ctx, sess.ProjectID, id)
	if err != nil {
		return nil, err
	}
	if p.SessionID != sess.ID && !s.syncsWiki(ctx, sess.ID) {
		return nil, domain.ErrNotFound // another session's proposal
	}
	return s.Wiki.Decide(ctx, sess.ProjectID, id, choice, content, string(domain.ResolvedByOh))
}
