package session

import (
	"context"
	"fmt"

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
	type member struct{ title, session string }
	var members []member
	for _, c := range f.Children {
		members = append(members, member{c.Ticket, c.Session})
	}
	for _, x := range f.Fixes {
		members = append(members, member{fmt.Sprintf("fix %d", x.Round), x.Session}) // the fix rounds too
	}
	for _, m := range members {
		if m.session == "" {
			continue
		}
		st, err := s.Checkpoints.State(ctx, m.session)
		if err != nil {
			continue
		}
		for _, t := range st.Timeline {
			if t.Kind != domain.CheckpointPassed {
				continue
			}
			evs = append(evs, TraceEvent{Time: t.At, Kind: TraceFanout, Event: "child_checkpoint", Title: m.title, ToolSession: m.session,
				Checkpoint: t.ID, By: t.By, Text: t.Message})
		}
	}
	return evs
}
