package checkpoint

import (
	"context"
	"errors"
	"strings"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Order of the checkpoints and of the writes (r5-fixes, bundle/order.go).

// ErrOutOfOrder refuses a checkpoint called before the one it waits for
// (`after:`).
var ErrOutOfOrder = errors.New("checkpoint called before the one it waits for")

// OrderError is a checkpoint refused by its order (message for the agent:
// the checkpoint to pass first).
type OrderError struct {
	ID, Label         string
	First, FirstLabel string
}

func (e *OrderError) Error() string {
	return i18n.Tf("cmd.daemon.workflow.out_of_order", e.ID, e.Label, e.First, e.FirstLabel)
}

// Is matches ErrOutOfOrder.
func (e *OrderError) Is(target error) bool { return target == ErrOutOfOrder }

// orderError returns the refusal of checkpoint c when it waits for a
// checkpoint not passed yet (nil otherwise).
func (s *Service) orderError(ctx context.Context, sessionID, mode string, wf *sessionspec.WorkflowRuntime, c sessionspec.CheckpointDef) error {
	if c.After == "" {
		return nil
	}
	st, err := s.state(ctx, sessionID)
	if err != nil {
		return err
	}
	first := bundle.CheckpointWaitsFor(wf, mode, &st, c)
	if first == "" {
		return nil
	}
	lang := i18n.Locale()
	fc, _ := wf.Checkpoint(first)
	return &OrderError{ID: c.ID, Label: c.LabelFor(lang), First: first, FirstLabel: fc.LabelFor(lang)}
}

// EvaluateEdit answers an edit permission request of root relayed by the oh
// plugin: the edit of a `writes:` path held by `writes_after` is refused,
// with the message that tells the agent to wait for the checkpoint. effect
// "" keeps the decision of the tool.
func (s *Service) EvaluateEdit(ctx context.Context, root, action string, resources []string) (effect, message string, err error) {
	if action != sessionspec.ActionEdit || len(resources) == 0 {
		return "", "", nil
	}
	sess, wf, err := s.session(ctx, root)
	if errors.Is(err, ErrNoWorkflow) {
		return "", "", nil
	}
	if err != nil || len(wf.WritesGates) == 0 {
		return "", "", err
	}
	st, err := s.state(ctx, root)
	if err != nil {
		return "", "", err
	}
	for _, r := range resources {
		g, locked := bundle.LockedWrite(wf, sess.Mode, &st, r)
		if !locked {
			continue
		}
		c, _ := wf.Checkpoint(g.After)
		return string(sessionspec.EffectDeny), i18n.Tf("cmd.daemon.workflow.writes_locked", r, g.After, c.LabelFor(i18n.Locale()), strings.Join(g.Paths, ", ")), nil
	}
	return "", "", nil
}

// outOfOrder reports whether checkpoint c waits for a checkpoint not passed
// yet: it is let through, so that the call fails with the reason instead of
// asking the user.
func (s *Service) outOfOrder(ctx context.Context, sess *domain.Session, wf *sessionspec.WorkflowRuntime, c sessionspec.CheckpointDef) bool {
	return errors.Is(s.orderError(ctx, sess.ID, sess.Mode, wf, c), ErrOutOfOrder)
}
