package checkpoint

import (
	"context"
	"errors"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// GuardCreate checks that root may create tickets now: the checkpoint that
// unlocks `create` is passed (N9). Unlike closing, creating does not close
// the window: it stays open until the next mandatory checkpoint passes
// (CP-R8) or the checkpoint is asked again. nil when the workflow locks
// nothing.
func (s *Service) GuardCreate(ctx context.Context, root string) error {
	_, wf, err := s.session(ctx, root)
	if errors.Is(err, ErrNoWorkflow) {
		return nil
	}
	if err != nil {
		return err
	}
	c, ok := bundle.UnlockingCheckpoint(wf, sessionspec.UnlockCreate)
	if !ok {
		return nil
	}
	st, err := s.state(ctx, root)
	if err != nil {
		return err
	}
	if _, open := st.Unlocks[sessionspec.UnlockCreate]; !open {
		return &LockError{Msg: i18n.Tf("cmd.gateway.beads.locked_create", c.ID, c.LabelFor(i18n.Locale()))}
	}
	return nil
}

// relockOnClose locks again the operations of checkpoint id once a ticket
// is closed, except `create` and `update`: the tickets of a split are
// created (and those of a triage changed) after one approval, whatever is
// closed meanwhile.
func relockOnClose(st *domain.CheckpointState, id string) bool {
	keep := map[string]domain.CheckpointUnlock{}
	for _, op := range []string{sessionspec.UnlockCreate, sessionspec.UnlockUpdate} {
		if u, ok := st.Unlocks[op]; ok {
			keep[op] = u
		}
	}
	changed := relock(st, id)
	for op, u := range keep {
		if st.Unlocks == nil {
			st.Unlocks = map[string]domain.CheckpointUnlock{}
		}
		if _, still := st.Unlocks[op]; !still {
			st.Unlocks[op] = u
		}
	}
	return changed
}
