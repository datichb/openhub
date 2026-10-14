package checkpoint

import (
	"context"
	"errors"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// GuardUpdate checks that root may change tickets now (r9b, triage): the
// checkpoint that unlocks `update` is passed. Like `create`, the window
// stays open until the next mandatory checkpoint passes (CP-R8) or the
// checkpoint is asked again. closing: the change closes tickets (`bd
// duplicate`, `bd supersede`…); it waits for this checkpoint only in a
// workflow where no checkpoint unlocks `close` (GuardClose applies
// otherwise). nil when the workflow locks nothing.
func (s *Service) GuardUpdate(ctx context.Context, root string, closing bool) error {
	_, wf, err := s.session(ctx, root)
	if errors.Is(err, ErrNoWorkflow) {
		return nil
	}
	if err != nil {
		return err
	}
	c, ok := bundle.UnlockingCheckpoint(wf, sessionspec.UnlockUpdate)
	if !ok {
		return nil
	}
	if _, locksClose := bundle.UnlockingCheckpoint(wf, sessionspec.UnlockClose); closing && locksClose {
		return nil
	}
	st, err := s.state(ctx, root)
	if err != nil {
		return err
	}
	if _, open := st.Unlocks[sessionspec.UnlockUpdate]; !open {
		return &LockError{Msg: i18n.Tf("cmd.gateway.beads.locked_update", c.ID, c.LabelFor(i18n.Locale()))}
	}
	return nil
}
