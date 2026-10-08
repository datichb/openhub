package checkpoint

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// State machine of a workflow session (P3-T03, P3-T04, P3-T06), fed by the
// daemon from the tool event stream:
//
//	workflow_checkpoint asked ─┬─ automatic in the mode ── let through (policy)
//	                           └─ paused ── ⏸ decision ─┬─ validated ── call runs ── passed
//	                                                    └─ refused ── instruction sent to the agent
//	subagent call ── counter (circuit breaker) ; subagent done ── agent ran (locks `after: <agent>`)
//
// Every change of the passed checkpoints, agents run or breaker changes the
// session rules (Rules), applied by the daemon to the session and its
// subagent sessions.

// Decision choices of a checkpoint (Reply.Decision). The tool only knows
// once / reject: a refusal is followed by the instruction, sent to the agent.
const (
	ChoiceApprove = "approve" // also "once"
	ChoiceFix     = "fix"     // fix first (reject + instruction)
	ChoiceOther   = "other"   // other instruction (reject + instruction)
	ChoiceReject  = "reject"
	ChoiceDismiss = "dismiss" // circuit breaker
)

// Data keys of a checkpoint decision payload.
const (
	DataCheckpoint = "checkpoint"
	DataSummary    = "summary"
	DataBehavior   = "behavior"
	DataAgent      = "agent"
	DataCount      = "count"
)

// timelineMax caps the stored timeline.
const timelineMax = 200

// Tool acts on the tool session of a decision.
type Tool interface {
	// ReplyPermission answers the permission request behind a decision.
	ReplyPermission(ctx context.Context, d domain.Decision, decision string) error
	// Steer sends an instruction to the running agent of a session.
	Steer(ctx context.Context, sessionID, text string) error
	// Refresh re-applies the session rules (after a state change).
	Refresh(ctx context.Context, sessionID string) error
}

// ErrNoState is returned when the service has no checkpoint store.
var ErrNoState = errors.New("checkpoint: no state store")

func (s *Service) update(ctx context.Context, id string, fn func(*domain.CheckpointState) error) (domain.CheckpointState, error) {
	if s.States == nil {
		return domain.CheckpointState{}, ErrNoState
	}
	return s.States.UpdateCheckpointState(ctx, id, fn)
}

func (s *Service) state(ctx context.Context, id string) (domain.CheckpointState, error) {
	if s.States == nil {
		return domain.CheckpointState{}, nil
	}
	return s.States.GetCheckpointState(ctx, id)
}

func (s *Service) record(st *domain.CheckpointState, kind, id, by, msg string) {
	st.Timeline = append(st.Timeline, domain.CheckpointEvent{At: s.now(), Kind: kind, ID: id, By: by, Message: msg})
	if n := len(st.Timeline); n > timelineMax {
		st.Timeline = st.Timeline[n-timelineMax:]
	}
}

// Rules returns the session rules of a session (nil when it does not run a workflow).
func (s *Service) Rules(ctx context.Context, sessionID string) ([]sessionspec.PermissionRule, error) {
	sess, wf, err := s.session(ctx, sessionID)
	if errors.Is(err, ErrNoWorkflow) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	st, err := s.state(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return bundle.SessionRules(wf, sess.Mode, &st), nil
}

// Asked handles a workflow_checkpoint permission request of root (asked by
// toolSession). auto = let it through now (automatic in the mode, or an
// unknown checkpoint: the call then fails with the list of checkpoints);
// otherwise d is the ⏸ decision to record. relocked reports that the
// operations the checkpoint unlocked are locked again (a new round): the
// session rules changed.
func (s *Service) Asked(ctx context.Context, root, toolSession string, p adapters.PendingDecision) (d *domain.Decision, auto, relocked bool, err error) {
	sess, wf, err := s.session(ctx, root)
	if err != nil {
		return nil, false, false, err
	}
	var id, summary string
	if p.Call != nil {
		id, _ = p.Call.Input["id"].(string)
		summary, _ = p.Call.Input["summary"].(string)
	}
	c, known := wf.Checkpoint(id)
	behavior := c.Behavior(sess.Mode)
	if !known || !bundle.NeedsDecision(behavior) {
		return nil, true, false, nil
	}
	id = c.ID
	label := c.LabelFor(i18n.Locale())
	d = &domain.Decision{SessionID: root, GroupKey: sess.GroupKey, Kind: domain.DecisionCheckpoint, ToolRef: p.ID,
		Payload: domain.DecisionPayload{Title: label, Message: summary, Data: map[string]any{
			DataCheckpoint: id, DataSummary: summary, DataBehavior: behavior}}}
	if toolSession != "" && toolSession != root {
		d.Payload.Data[domain.DataToolSession] = toolSession
	}
	d.ID = domain.DecisionID(d.Kind, root, p.ID)
	_, err = s.update(ctx, root, func(st *domain.CheckpointState) error {
		relocked = relock(st, id) // asked again: a new round
		changed := relocked
		if st.Waiting != id {
			st.Waiting = id
			s.record(st, domain.CheckpointWaiting, id, "", summary)
			changed = true
		}
		if !changed {
			return errUnchanged
		}
		return nil
	})
	if errors.Is(err, errUnchanged) {
		err = nil
	}
	return d, false, relocked, err
}

// Settled is called when the request of a checkpoint decision was answered
// in the tool (or vanished): the session no longer waits.
func (s *Service) Settled(ctx context.Context, d domain.Decision) {
	id, _ := d.Payload.Data[DataCheckpoint].(string)
	_, _ = s.update(ctx, d.SessionID, func(st *domain.CheckpointState) error {
		if st.Waiting == id {
			st.Waiting = ""
		}
		return nil
	})
}

// pass records a checkpoint passed (the workflow_checkpoint call ran) and
// opens the window of the operations it unlocks.
func (s *Service) pass(ctx context.Context, sess *domain.Session, c sessionspec.CheckpointDef) (domain.CheckpointApproval, error) {
	sessionID, id := sess.ID, c.ID
	head := ""
	if len(c.Unlocks) > 0 {
		head, _ = s.head(ctx, sess.LaunchPath)
	}
	var ap domain.CheckpointApproval
	_, err := s.update(ctx, sessionID, func(st *domain.CheckpointState) error {
		ap = st.Approved[id]
		delete(st.Approved, id)
		if ap.By == "" {
			ap.By = "mode"
			if st.Waiting == id {
				ap.By = string(domain.ResolvedByTool) // validated in the tool UI
			}
		}
		if st.Passed == nil {
			st.Passed = map[string]time.Time{}
		}
		st.Passed[id] = s.now()
		if st.Waiting == id {
			st.Waiting = ""
		}
		if ap.By != "mode" {
			st.Consecutive = 0 // a validation is a user intervention
		}
		for _, op := range c.Unlocks {
			if st.Unlocks == nil {
				st.Unlocks = map[string]domain.CheckpointUnlock{}
			}
			st.Unlocks[op] = domain.CheckpointUnlock{Checkpoint: id, At: s.now(), Head: head}
		}
		s.record(st, domain.CheckpointPassed, id, ap.By, ap.Message)
		return nil
	})
	return ap, err
}

// relock closes the window of the operations unlocked by checkpoint id.
func relock(st *domain.CheckpointState, id string) bool {
	changed := false
	for op, u := range st.Unlocks {
		if strings.EqualFold(u.Checkpoint, id) {
			delete(st.Unlocks, op)
			changed = true
		}
	}
	return changed
}

// OnCall follows the tool calls of root (and of its subagent sessions):
// delegations feed the circuit breaker and the `after: <agent>` locks.
// changed reports that the session rules changed; raised is a circuit
// breaker decision to record.
func (s *Service) OnCall(ctx context.Context, root string, call adapters.ToolCall) (changed bool, raised *domain.Decision, err error) {
	if call.Action != sessionspec.ActionSubagent {
		return false, nil, nil
	}
	sess, wf, err := s.session(ctx, root)
	if err != nil {
		if errors.Is(err, ErrNoWorkflow) {
			err = nil
		}
		return false, nil, err
	}
	agent, _ := call.Input["agent"].(string)
	switch call.Status {
	case adapters.CallCalled:
		var tripped bool
		var count int
		_, err = s.update(ctx, root, func(st *domain.CheckpointState) error {
			st.Consecutive++
			count = st.Consecutive
			if wf.MaxConsecutiveSubagents > 0 && st.Consecutive >= wf.MaxConsecutiveSubagents && !st.Breaker {
				st.Breaker, tripped = true, true
				s.record(st, domain.CheckpointBreaker, agent, "", "")
			}
			return nil
		})
		if err != nil || !tripped {
			return false, nil, err
		}
		d := &domain.Decision{SessionID: root, GroupKey: sess.GroupKey, Kind: domain.DecisionCircuit, ToolRef: fmt.Sprintf("%d", s.now().UnixNano()),
			Payload: domain.DecisionPayload{Title: i18n.Tf("tui.checkpoint.breaker_title", count),
				Data: map[string]any{DataCount: count, DataAgent: agent}}}
		d.ID = domain.DecisionID(d.Kind, root, d.ToolRef)
		return true, d, nil
	case adapters.CallOK:
		if agent == "" {
			// The success event does not repeat the input: the daemon
			// passes the agent of the call it saw.
			return false, nil, nil
		}
		_, err = s.update(ctx, root, func(st *domain.CheckpointState) error {
			if st.HasRun(agent) {
				return errUnchanged
			}
			st.Ran = append(st.Ran, agent)
			s.record(st, domain.CheckpointDelegate, agent, "", "")
			return nil
		})
		if errors.Is(err, errUnchanged) {
			return false, nil, nil
		}
		return err == nil && gates(wf, agent), nil, err
	}
	return false, nil, nil
}

var errUnchanged = errors.New("unchanged")

// gates reports whether an agent releases a lock.
func gates(wf *sessionspec.WorkflowRuntime, agent string) bool {
	for _, g := range wf.Gates {
		if g.After == agent {
			return true
		}
	}
	return false
}

// OnUserInput resets the circuit breaker counter (the user spoke to the session).
func (s *Service) OnUserInput(ctx context.Context, root string) {
	_, _ = s.update(ctx, root, func(st *domain.CheckpointState) error {
		if st.Consecutive == 0 {
			return errUnchanged
		}
		st.Consecutive = 0
		return nil
	})
}

// Resolve answers a checkpoint or circuit breaker decision (SessionService
// resolver, run after oh claimed the decision).
func (s *Service) Resolve(ctx context.Context, tool Tool, d domain.Decision, choice, message string) error {
	message = strings.TrimSpace(message)
	switch d.Kind {
	case domain.DecisionCircuit:
		if choice != "" && choice != ChoiceDismiss && choice != ChoiceApprove && choice != "once" {
			return fmt.Errorf("invalid choice %q (dismiss)", choice)
		}
		if _, err := s.update(ctx, d.SessionID, func(st *domain.CheckpointState) error {
			st.Breaker, st.Consecutive = false, 0
			return nil
		}); err != nil {
			return err
		}
		if err := tool.Refresh(ctx, d.SessionID); err != nil {
			return err
		}
		if message != "" {
			return tool.Steer(ctx, d.SessionID, message)
		}
		return nil
	case domain.DecisionCheckpoint:
	default:
		return fmt.Errorf("checkpoint: cannot resolve a %s decision", d.Kind)
	}
	id, _ := d.Payload.Data[DataCheckpoint].(string)
	label := d.Payload.Title
	switch choice {
	case ChoiceApprove, "once", "":
		if _, err := s.update(ctx, d.SessionID, func(st *domain.CheckpointState) error {
			if st.Approved == nil {
				st.Approved = map[string]domain.CheckpointApproval{}
			}
			st.Approved[id] = domain.CheckpointApproval{By: string(domain.ResolvedByOh), Message: message}
			return nil
		}); err != nil {
			return err
		}
		return tool.ReplyPermission(ctx, d, "once")
	case ChoiceFix, ChoiceOther, ChoiceReject:
		if (choice == ChoiceOther || choice == ChoiceFix) && message == "" {
			return errors.New(i18n.T("tui.checkpoint.message_required"))
		}
		if err := tool.ReplyPermission(ctx, d, "reject"); err != nil {
			return err
		}
		_, _ = s.update(ctx, d.SessionID, func(st *domain.CheckpointState) error {
			if st.Waiting == id {
				st.Waiting = ""
			}
			st.Consecutive = 0
			s.record(st, domain.CheckpointRefused, id, string(domain.ResolvedByOh), message)
			return nil
		})
		// The tool does not forward the refusal message to the agent.
		text := i18n.Tf("tui.checkpoint.refused_steer", id, label)
		if message != "" {
			text += "\n" + i18n.Tf("tui.checkpoint.refused_instruction", message)
		}
		return tool.Steer(ctx, d.SessionID, text)
	}
	return fmt.Errorf("invalid choice %q (approve | fix | other | reject)", choice)
}

// State returns the checkpoint state of a session.
func (s *Service) State(ctx context.Context, sessionID string) (domain.CheckpointState, error) {
	return s.state(ctx, sessionID)
}

// Workflow returns the workflow runtime and mode of a session (nil when it
// does not run a workflow).
func (s *Service) Workflow(ctx context.Context, sessionID string) (*sessionspec.WorkflowRuntime, string, error) {
	sess, wf, err := s.session(ctx, sessionID)
	if errors.Is(err, ErrNoWorkflow) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	return wf, sess.Mode, nil
}

// Evaluate answers the tool plugin (level 3) for a permission about to be
// asked: "allow" for a workflow_checkpoint call that does not wait for the
// user in the session mode (or an unknown checkpoint, whose call then fails
// with the list), "" to keep the rendered decision.
func (s *Service) Evaluate(ctx context.Context, root, action string, input map[string]any) (effect string, err error) {
	if action != bundle.CheckpointAction() {
		return "", nil
	}
	sess, wf, err := s.session(ctx, root)
	if errors.Is(err, ErrNoWorkflow) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	id, _ := input["id"].(string)
	c, known := wf.Checkpoint(strings.TrimSpace(id))
	if !known || !bundle.NeedsDecision(c.Behavior(sess.Mode)) {
		return string(sessionspec.EffectAllow), nil
	}
	return "", nil
}
