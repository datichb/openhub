package checkpoint

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Evidence, shell calls, fix rounds and windows (R3: E2, E5, CP-R8), fed by
// the tool calls of the session (OnCall).

// shellMax bounds the shell calls kept in the state.
const shellMax = 100

// Evidence is a command the agent says it ran, with its exit code
// (workflow_checkpoint evidence).
type Evidence struct {
	Cmd  string `json:"cmd"`
	Exit int    `json:"exit"`
}

// Data keys of a checkpoint decision (R3).
const (
	DataUnobserved = "unobserved" // evidence not observed by oh
	DataUnresolved = "unresolved" // ticket whose fix rounds reached the cap
)

// evidenceOf reads the evidence of a workflow_checkpoint call input.
func evidenceOf(input map[string]any) []Evidence {
	list, _ := input["evidence"].([]any)
	var out []Evidence
	for _, e := range list {
		m, _ := e.(map[string]any)
		cmd, _ := m["cmd"].(string)
		if cmd = strings.TrimSpace(cmd); cmd == "" {
			continue
		}
		ev := Evidence{Cmd: cmd}
		switch x := m["exit"].(type) {
		case float64:
			ev.Exit = int(x)
		case int:
			ev.Exit = x
		}
		out = append(out, ev)
	}
	return out
}

// checkEvidence checks the evidence against the shell calls observed: a
// command no shell call ran is "not observed"; one that failed while the
// agent reports 0 is contradicted.
func checkEvidence(st *domain.CheckpointState, list []Evidence) []domain.CheckpointEvidence {
	out := make([]domain.CheckpointEvidence, 0, len(list))
	for _, e := range list {
		r := domain.CheckpointEvidence{Cmd: e.Cmd, Exit: e.Exit}
		want := normalizeCmd(e.Cmd)
		for i := len(st.Shell) - 1; i >= 0; i-- {
			sc := st.Shell[i]
			if normalizeCmd(sc.Command) != want || sc.Status == adapters.CallCalled {
				continue
			}
			r.Observed = true
			r.Contradicted = sc.Status == adapters.CallFailed && e.Exit == 0
			break
		}
		out = append(out, r)
	}
	return out
}

func normalizeCmd(s string) string { return strings.Join(strings.Fields(s), " ") }

// recordEvidence stores the evidence of checkpoint id and traces the ones
// not observed; it returns them.
func (s *Service) recordEvidence(st *domain.CheckpointState, id string, list []Evidence) []string {
	if len(list) == 0 {
		return nil
	}
	checked := checkEvidence(st, list)
	if slices.Equal(st.Evidence[id], checked) {
		return unobservedOf(checked) // already recorded (asked, then passed)
	}
	if st.Evidence == nil {
		st.Evidence = map[string][]domain.CheckpointEvidence{}
	}
	st.Evidence[id] = checked
	unobserved := unobservedOf(checked)
	for _, cmd := range unobserved {
		s.record(st, domain.CheckpointUnobserved, id, "", cmd)
	}
	return unobserved
}

func unobservedOf(list []domain.CheckpointEvidence) []string {
	var out []string
	for _, e := range list {
		if !e.Observed || e.Contradicted {
			out = append(out, e.Cmd)
		}
	}
	return out
}

// onShell records a shell call of the session; once a call ends with the
// commit window open, HEAD moved and nothing left to commit, the window
// closes (CP-R8: the work was committed, in one commit or several in a
// row). changed reports that the session rules changed.
func (s *Service) onShell(ctx context.Context, sess *domain.Session, call adapters.ToolCall) (changed bool, err error) {
	cmd, _ := call.Input["command"].(string)
	var commitWindow *domain.CheckpointUnlock
	_, err = s.update(ctx, sess.ID, func(st *domain.CheckpointState) error {
		if call.Status == adapters.CallCalled {
			if strings.TrimSpace(cmd) == "" {
				return errUnchanged
			}
			st.Shell = append(st.Shell, domain.ShellCall{ID: call.ID, Command: cmd, Status: call.Status, At: s.now()})
			if n := len(st.Shell); n > shellMax {
				st.Shell = st.Shell[n-shellMax:]
			}
			return nil
		}
		found := false
		for i := len(st.Shell) - 1; i >= 0; i-- {
			if st.Shell[i].ID == call.ID && call.ID != "" {
				st.Shell[i].Status = call.Status
				found = true
				break
			}
		}
		if u, ok := st.Unlocks[sessionspec.UnlockCommit]; ok {
			commitWindow = &u
		}
		if !found {
			return errUnchanged
		}
		return nil
	})
	if errors.Is(err, errUnchanged) {
		err = nil
	}
	if err != nil || commitWindow == nil || call.Status == adapters.CallCalled {
		return false, err
	}
	head, dirty, herr := s.headErr(ctx, sess.LaunchPath)
	if herr != nil {
		return false, nil //nolint:nilerr // git unavailable: the window stays as it is
	}
	if head == "" || head == commitWindow.Head || dirty {
		return false, nil // not committed, or more to commit (one commit per subtask)
	}
	_, err = s.update(ctx, sess.ID, func(st *domain.CheckpointState) error {
		u, ok := st.Unlocks[sessionspec.UnlockCommit]
		if !ok || u.Head != commitWindow.Head {
			return errUnchanged
		}
		delete(st.Unlocks, sessionspec.UnlockCommit)
		s.record(st, domain.CheckpointWindow, u.Checkpoint, "", sessionspec.UnlockCommit)
		changed = true
		return nil
	})
	if errors.Is(err, errUnchanged) {
		err = nil
	}
	return changed, err
}

// closeCreateWindow closes the create window when another mandatory
// checkpoint passes (CP-R8: the tickets were created before it).
func (s *Service) closeCreateWindow(st *domain.CheckpointState, c sessionspec.CheckpointDef) {
	if !c.Mandatory && len(c.Unlocks) == 0 {
		return
	}
	u, ok := st.Unlocks[sessionspec.UnlockCreate]
	if !ok || strings.EqualFold(u.Checkpoint, c.ID) {
		return
	}
	delete(st.Unlocks, sessionspec.UnlockCreate)
	s.record(st, domain.CheckpointWindow, u.Checkpoint, "", sessionspec.UnlockCreate)
}

// currentTicket is the ticket the session works on: the last one claimed
// and not closed, else the first ticket of the session.
func currentTicket(sess *domain.Session, st *domain.CheckpointState) string {
	for i := len(st.Claimed) - 1; i >= 0; i-- {
		if !containsFold(st.Closed, st.Claimed[i]) {
			return st.Claimed[i]
		}
	}
	for _, t := range sess.Tickets {
		if !containsFold(st.Closed, t) {
			return t
		}
	}
	return ""
}

// countRound counts a developer → reviewer pair of the current ticket
// (E5). Past limits.max_fix_rounds fix rounds (the first pair is the
// implementation), the ticket is unresolved: the developer is locked until
// the user steps in at the commit checkpoint or the ticket is closed.
// tripped reports that the lock was just set.
func (s *Service) countRound(wf *sessionspec.WorkflowRuntime, sess *domain.Session, st *domain.CheckpointState, agent string) (tripped bool) {
	prev := st.LastDelegate
	st.LastDelegate = agent
	if !bundle.IsReviewer(agent) || !bundle.IsDeveloper(prev) {
		return false
	}
	t := currentTicket(sess, st)
	if st.Rounds == nil {
		st.Rounds = map[string]int{}
	}
	st.Rounds[t]++
	if wf.MaxFixRounds > 0 && st.Rounds[t] > wf.MaxFixRounds && st.Unresolved == "" {
		st.Unresolved = firstNonEmpty(t, "-")
		s.record(st, domain.CheckpointFixLimit, st.Unresolved, "", "")
		return true
	}
	return false
}

// resetRounds forgets the rounds of ticket t (closed, or the user asked
// for more work at the commit checkpoint).
func resetRounds(st *domain.CheckpointState, t string) bool {
	changed := false
	if st.Unresolved != "" && (t == "" || strings.EqualFold(st.Unresolved, t) || st.Unresolved == "-") {
		st.Unresolved = ""
		changed = true
	}
	if t == "" {
		if len(st.Rounds) > 0 {
			st.Rounds, changed = nil, true
		}
		return changed
	}
	if _, ok := st.Rounds[t]; ok {
		delete(st.Rounds, t)
		changed = true
	}
	return changed
}

// askedR3 completes the decision of a paused checkpoint: the evidence of
// the agent checked (CP-R5) and, at the commit checkpoint, the ticket whose
// fix rounds reached the cap. changed reports a state change.
func (s *Service) askedR3(st *domain.CheckpointState, d *domain.Decision, c sessionspec.CheckpointDef, evidence []Evidence) (changed bool) {
	before := len(st.Timeline)
	if unobserved := s.recordEvidence(st, c.ID, evidence); len(unobserved) > 0 {
		d.Payload.Data[DataUnobserved] = unobserved
	}
	if st.Unresolved != "" && slices.Contains(c.Unlocks, sessionspec.UnlockCommit) {
		d.Payload.Data[DataUnresolved] = st.Unresolved
	}
	return len(evidence) > 0 || len(st.Timeline) != before
}

// reachedR3 completes the result of a checkpoint passed: the verification
// and the evidence not observed.
func (s *Service) reachedR3(ctx context.Context, sessionID string, c sessionspec.CheckpointDef, call Call, res *Result) {
	if c.Verify != nil {
		if r, ok := s.VerifyResult(ctx, sessionID, c.ID); ok {
			res.Verify = VerifySummary(r)
		}
	}
	if len(call.Evidence) == 0 {
		return
	}
	_, _ = s.update(ctx, sessionID, func(st *domain.CheckpointState) error {
		res.Unobserved = s.recordEvidence(st, c.ID, call.Evidence)
		return nil
	})
}

// CardR3 is what the decision card of a checkpoint shows of R3 (CP-R5):
// the verification run by oh, the evidence checked and the ticket whose
// fix rounds reached the cap.
type CardR3 struct {
	Verify     *domain.CheckpointVerify
	Evidence   []domain.CheckpointEvidence
	Unresolved string
}

// Card returns the R3 part of the card of checkpoint id of a session.
func (s *Service) Card(ctx context.Context, sessionID, id string) CardR3 {
	st, err := s.state(ctx, sessionID)
	if err != nil {
		return CardR3{}
	}
	var c CardR3
	if r, ok := st.Verify[id]; ok {
		c.Verify = &r
	}
	c.Evidence = st.Evidence[id]
	if _, wf, err := s.session(ctx, sessionID); err == nil {
		if cp, ok := wf.Checkpoint(id); ok && slices.Contains(cp.Unlocks, sessionspec.UnlockCommit) {
			c.Unresolved = st.Unresolved
		}
	}
	return c
}
