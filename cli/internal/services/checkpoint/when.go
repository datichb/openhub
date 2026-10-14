package checkpoint

import (
	"context"
	"sort"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Conditions of a checkpoint evaluated by oh (R3, CP-R7, E6): `when:` only
// adds a pause (an automatic checkpoint pauses when a condition holds, a
// conditional one only then); a mandatory checkpoint always pauses.

// maxLabelTickets bounds the tickets whose labels are read.
const maxLabelTickets = 5

// behavior returns the behavior of c for sess, its conditions evaluated.
func (s *Service) behavior(ctx context.Context, sess *domain.Session, c sessionspec.CheckpointDef) string {
	if _, _, ok := s.delegation(ctx, sess, c); ok {
		// R6, CP-R3: replaced by the merge checkpoint of the parent.
		return sessionspec.CheckpointAuto
	}
	if c.When == nil {
		return c.Behavior(sess.Mode)
	}
	return c.EffectiveBehavior(sess.Mode, s.whenFacts(ctx, sess, c.When))
}

// whenFacts gathers what the conditions of w need to know of sess (the
// labels and changes only when w asks for them).
func (s *Service) whenFacts(ctx context.Context, sess *domain.Session, w *sessionspec.WhenDef) sessionspec.WhenFacts {
	f := sessionspec.WhenFacts{Entry: sess.EntryAgent}
	for k := range sess.Outputs {
		f.Outputs = append(f.Outputs, k)
	}
	sort.Strings(f.Outputs)
	if len(w.Paths) > 0 {
		var err error
		f.Paths, err = s.changedPaths(ctx, sess)
		f.PathsUnknown = err != nil && gitutil.LooksLikeRepository(sess.LaunchPath)
	}
	if len(w.Labels) > 0 {
		tickets := append([]string(nil), sess.Tickets...)
		if st, err := s.state(ctx, sess.ID); err == nil {
			tickets = appendNew(tickets, st.Claimed...)
		}
		if len(tickets) > maxLabelTickets {
			tickets = tickets[:maxLabelTickets]
		}
		labels := s.Labels
		if labels == nil {
			labels = beadsLabels
		}
		for _, t := range tickets {
			if l, err := labels(ctx, sess.LaunchPath, t); err == nil {
				f.Labels = append(f.Labels, l...)
			}
		}
	}
	return f
}

func beadsLabels(_ context.Context, dir, ticket string) ([]string, error) {
	d, err := beads.Show(dir, ticket)
	if err != nil {
		return nil, err
	}
	return d.Labels, nil
}
