package session

import (
	"context"

	"github.com/datichb/openhub/cli/internal/domain"
)

// fanoutEvents are the events of the fan-out of a parent session (R6): the
// steps oh recorded and the checkpoints passed in each child, so that the
// trace of the parent shows what its children did (traceability).
func (s *Service) fanoutEvents(ctx context.Context, sess *domain.Session) []TraceEvent {
	f := sess.Fanout
	if f == nil {
		return nil
	}
	var evs []TraceEvent
	if f.Role != domain.RoleParent {
		return []TraceEvent{{Time: sess.StartedAt, Kind: TraceFanout, Event: string(f.Role), Title: f.Ticket, Parent: f.Parent, Text: f.Branch}}
	}
	for _, e := range f.Events {
		evs = append(evs, TraceEvent{Time: e.At, Kind: TraceFanout, Event: e.Kind, Title: e.Ticket, ToolSession: e.Session, Text: e.Message})
	}
	if s.Checkpoints == nil {
		return evs
	}
	for _, c := range f.Children {
		if c.Session == "" {
			continue
		}
		st, err := s.Checkpoints.State(ctx, c.Session)
		if err != nil {
			continue
		}
		for _, t := range st.Timeline {
			if t.Kind != domain.CheckpointPassed {
				continue
			}
			evs = append(evs, TraceEvent{Time: t.At, Kind: TraceFanout, Event: "child_checkpoint", Title: c.Ticket, ToolSession: c.Session,
				Checkpoint: t.ID, By: t.By, Text: t.Message})
		}
	}
	return evs
}
