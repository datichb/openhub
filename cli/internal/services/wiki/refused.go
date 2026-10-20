package wiki

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
)

// Refused proposals, as the agents see them (p9-wiki-refusal follow-up). The
// user may say why he refused a proposal; without a way to learn it, the
// agents propose the same thing again. wiki_pending lists the recent
// refusals with their reason, and wiki_propose warns when a near-identical
// proposal was refused (it is not turned away: the agent may have a new reason).

// Recent refusals: the last RefusedShown proposals refused in the last
// RefusedWindow, the most recent first.
const (
	RefusedShown  = 20
	RefusedWindow = 30 * 24 * time.Hour
)

// decidedAt is when a proposal was decided (when it was filed, for a row
// that has no decision date).
func decidedAt(p domain.WikiProposal) time.Time {
	if p.DecidedAt != nil {
		return *p.DecidedAt
	}
	return p.CreatedAt
}

// Refused returns the recent refusals of a project, or of one of its
// sessions: at most RefusedShown, refused less than RefusedWindow ago, the
// most recent first.
func (s *Service) Refused(ctx context.Context, projectID, sessionID string) ([]domain.WikiProposal, error) {
	if s.Store == nil {
		return nil, ErrNoStore
	}
	all, err := s.Store.ListWikiProposals(ctx, domain.WikiProposalFilter{ProjectID: projectID, SessionID: sessionID,
		Statuses: []string{domain.WikiRefused}})
	if err != nil {
		return nil, err
	}
	since := s.now().Add(-RefusedWindow)
	out := all[:0:0]
	for _, p := range all {
		if !decidedAt(p).Before(since) {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return decidedAt(out[i]).After(decidedAt(out[j])) })
	if len(out) > RefusedShown {
		out = out[:RefusedShown]
	}
	return out, nil
}

// SameText reports whether two texts are the same once the case and the
// spaces are set aside: the notion of « near-identical » of a proposal.
func SameText(a, b string) bool {
	return strings.EqualFold(strings.Join(strings.Fields(a), " "), strings.Join(strings.Fields(b), " "))
}

// PriorRefusal returns the most recent proposal of the project that the
// user refused and that p repeats: same page, same text (case and spaces
// aside), whatever the section. The text the agent wrote is compared with
// the one the user refused, and with the original of a refused proposal he
// had edited first. nil when there is none.
func (s *Service) PriorRefusal(ctx context.Context, p domain.WikiProposal) (*domain.WikiProposal, error) {
	if s.Store == nil {
		return nil, ErrNoStore
	}
	refused, err := s.Store.ListWikiProposals(ctx, domain.WikiProposalFilter{ProjectID: p.ProjectID, Statuses: []string{domain.WikiRefused}})
	if err != nil {
		return nil, err
	}
	var prior *domain.WikiProposal
	for i := range refused {
		r := &refused[i]
		if r.Page != p.Page || r.Seq == p.Seq {
			continue
		}
		if !SameText(r.Content, p.Content) && (r.Original == "" || !SameText(r.Original, p.Content)) {
			continue
		}
		if prior == nil || !decidedAt(*r).Before(decidedAt(*prior)) {
			prior = r
		}
	}
	return prior, nil
}
