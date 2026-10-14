// Package checkpoint is the CheckpointService (03 §2.4, D8): the workflow
// state of each session (checkpoints passed, current one, gates, circuit
// breaker) and the backend of the oh MCP server `workflow`. It runs in the
// oh daemon, fed by the tool event stream; the CLI and the TUI read it.
package checkpoint

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Service is the CheckpointService.
type Service struct {
	Sessions domain.SessionStore
	// States keeps the workflow state of the sessions (sessions.checkpoint_state).
	States domain.CheckpointStore
	// BundlesDir locates the session bundles (~/.oh/bundles).
	BundlesDir string
	// SessionsDir keeps per-session files (~/.oh/sessions/<id>).
	SessionsDir string
	// SessionOutputs records the outputs declared by workflow_outputs
	// (sessions.outputs).
	SessionOutputs domain.SessionOutputStore
	// Head returns the commit of a directory and whether it has changes not
	// committed outside the Beads data (nil = git).
	Head func(ctx context.Context, dir string) (head string, dirty bool, err error)
	// Work tells what a directory committed since a commit (nil = git,
	// GitWork): the closing guard (S-09).
	Work func(ctx context.Context, dir, since string, at time.Time) (Work, error)
	// Tags returns a fingerprint of the local tags of a directory (nil =
	// git, GitTags): the tag window closes after the first tag (CP-R8).
	Tags func(ctx context.Context, dir string) (string, error)
	Now  func() time.Time
	// ApprovalWait bounds the wait for the tool approval of a paused
	// checkpoint (0 = 3 s).
	ApprovalWait time.Duration

	// R3 (verify.go, when.go): verification run by oh and conditions.
	// Exec runs a verification command (nil = exec); Fingerprint and
	// Changed read the state of the work (nil = git); Labels reads the
	// labels of a ticket (nil = bd show); VerifyWait bounds the wait of a
	// call for a verification still running (0 = 20 s).
	Exec        ExecFunc
	Fingerprint func(ctx context.Context, dir string) string
	Changed     func(ctx context.Context, dir, since string) ([]string, error)
	Labels      func(ctx context.Context, dir, ticket string) ([]string, error)
	VerifyWait  time.Duration
	// Account runs the verifications and their git reads as the account
	// of the agents (remote job: the oh account of the image, never root;
	// R3-remote); nil = the account of oh.
	Account  *Account
	verifier verifier

	// CommitPassed runs when a checkpoint that unlocks the commit is
	// passed, before the call returns to the agent (LD-J: oh applies the
	// wiki proposals the user accepted on the card, so that they are
	// committed with the code). It returns the line for the agent ("" =
	// nothing applied) and the proposals applied, for the timeline. sync:
	// the checkpoint synchronizes the wiki (`wiki: sync`, doc-sync, LD-D).
	CommitPassed func(ctx context.Context, sess *domain.Session, checkpoint string, sync bool) (message string, applied []string)

	// R6 (fanout.go): Integrate merges the integration branch of the
	// fan-out of sess into the branch of the session and closes the tickets
	// merged, when its merge checkpoint is validated and verified, before
	// the checkpoint is recorded as passed (nil: the merge checkpoint never
	// passes). It returns the line for the agent.
	Integrate func(ctx context.Context, sess *domain.Session, checkpoint string) (message string, err error)
}

// Errors.
var (
	ErrNoWorkflow = errors.New("this session was not started from a workflow")
	ErrUnknownCP  = errors.New("unknown checkpoint")
)

// Checkpoint states in Status.
const (
	StateTodo    = "todo"
	StatePassed  = "passed"
	StateWaiting = "waiting"
	StateSkipped = "skipped"
)

// Status is what workflow_status returns to the agent.
type Status struct {
	SessionID   string             `json:"session_id"`
	Workflow    string             `json:"workflow"`
	Mode        string             `json:"mode"`
	Checkpoints []CheckpointStatus `json:"checkpoints"`
	// Next is the first checkpoint not passed yet (declaration order).
	Next string `json:"next,omitempty"`
	// Locked lists the agents that cannot be called yet, with their lock.
	Locked []sessionspec.AgentGate `json:"locked_agents,omitempty"`
	// ExpectedOutputs are the outputs the agents declare (workflow_outputs);
	// the outputs recorded by oh (R5, `record:`) are left out.
	ExpectedOutputs []sessionspec.OutputDef `json:"expected_outputs,omitempty"`
	Outputs         []Output                `json:"outputs,omitempty"`
	// Breaker: delegation is held by the circuit breaker until the user steps in.
	Breaker bool `json:"circuit_breaker,omitempty"`
	// LockedOps lists the operations refused until their checkpoint is
	// passed (commit, push, close).
	LockedOps []string `json:"locked_operations,omitempty"`
	// Fanout is the fan-out of the session (R6): its children for the
	// parent, its parent for a child.
	Fanout *FanoutStatus `json:"fanout,omitempty"`
}

// CheckpointStatus is a checkpoint of the workflow in the session mode.
type CheckpointStatus struct {
	ID       string     `json:"id"`
	Label    string     `json:"label"`
	Behavior string     `json:"behavior"` // pause | auto | skip | conditional
	State    string     `json:"state"`
	At       *time.Time `json:"at,omitempty"`
}

// Call is a workflow_checkpoint call.
type Call struct {
	ID      string `json:"id"`
	Summary string `json:"summary,omitempty"`
	// Evidence are the commands the agent ran for this checkpoint (R3, E2),
	// checked against the shell calls oh observed.
	Evidence []Evidence `json:"evidence,omitempty"`
	// Reports are the reports (rp-…) the agent cites; those filed since the
	// last checkpoint are attached to its card anyway.
	Reports []string `json:"reports,omitempty"`
	// Agent is the agent oh saw make the call (set by the daemon, never
	// read from the request; "" = unknown).
	Agent string `json:"-"`
}

// Result answers a workflow_checkpoint call (the tool already let it run:
// the checkpoint was validated or did not need a validation).
type Result struct {
	ID       string `json:"id"`
	Label    string `json:"label,omitempty"`
	Behavior string `json:"behavior,omitempty"`
	// Message is the instruction given with the validation (may be empty).
	Message string `json:"message,omitempty"`
	Next    string `json:"next,omitempty"`
	// Unlocked lists the operations the checkpoint unlocked (commit, push,
	// close), until a ticket is closed or the checkpoint is asked again.
	Unlocked []string `json:"unlocked,omitempty"`
	// Verify is the result of the verification run by oh (R3).
	Verify string `json:"verify,omitempty"`
	// Unobserved lists the evidence oh did not observe (R3, E2).
	Unobserved []string `json:"unobserved,omitempty"`
	// Warning tells the agent it passed a checkpoint of another agent (L3,
	// `by:`).
	Warning string `json:"warning,omitempty"`
	// Wiki tells the agent the wiki proposals oh applied at the checkpoint
	// of the commit (LD-J).
	Wiki string `json:"wiki,omitempty"`
}

// Output is a typed value declared by workflow_outputs (O7).
type Output struct {
	ID    string    `json:"id,omitempty"`
	Type  string    `json:"type"`
	Value string    `json:"value"`
	At    time.Time `json:"at,omitzero"`
}

// session loads a session and the workflow runtime of its bundle.
func (s *Service) session(ctx context.Context, id string) (*domain.Session, *sessionspec.WorkflowRuntime, error) {
	if s.Sessions == nil {
		return nil, nil, errors.New("checkpoint: no session store")
	}
	sess, err := s.Sessions.Get(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("session %s: %w", id, err)
	}
	if sess.BundleHash == "" || s.BundlesDir == "" {
		return sess, nil, ErrNoWorkflow
	}
	b, err := bundle.Load(s.BundlesDir, sess.BundleHash)
	if err != nil {
		return sess, nil, fmt.Errorf("session %s bundle: %w", id, err)
	}
	if b.Spec.Workflow == nil {
		return sess, nil, ErrNoWorkflow
	}
	return sess, b.Spec.Workflow, nil
}

// Status returns the workflow status of a session.
func (s *Service) Status(ctx context.Context, sessionID string) (Status, error) {
	sess, wf, err := s.session(ctx, sessionID)
	if err != nil {
		return Status{}, err
	}
	cs, err := s.state(ctx, sessionID)
	if err != nil {
		return Status{}, err
	}
	lang := i18n.Locale()
	st := Status{SessionID: sess.ID, Workflow: wf.ID, Mode: sess.Mode}
	for _, o := range wf.Outputs {
		if o.Record == "" { // R5: a recorded output is not for the agents to declare
			st.ExpectedOutputs = append(st.ExpectedOutputs, o)
		}
	}
	for _, c := range wf.Checkpoints {
		row := CheckpointStatus{ID: c.ID, Label: c.LabelFor(lang), Behavior: c.Behavior(sess.Mode), State: StateTodo}
		if _, _, ok := s.delegation(ctx, sess, c); ok {
			row.Behavior = sessionspec.CheckpointAuto // R6, CP-R3
		}
		switch {
		case cs.HasPassed(c.ID):
			at := cs.Passed[c.ID]
			row.State, row.At = StatePassed, &at
		case cs.Waiting == c.ID:
			row.State = StateWaiting
		case row.Behavior == sessionspec.CheckpointSkip:
			row.State = StateSkipped
		}
		if st.Next == "" && (row.State == StateTodo || row.State == StateWaiting) {
			st.Next = c.ID
		}
		st.Checkpoints = append(st.Checkpoints, row)
	}
	locked := map[string]bool{}
	for _, a := range bundle.LockedAgents(wf, sess.Mode, &cs) {
		locked[a] = true
	}
	for _, g := range wf.Gates {
		if locked[g.Agent] {
			st.Locked = append(st.Locked, g)
		}
	}
	st.Breaker = cs.Breaker
	st.LockedOps = fanoutLockedOps(sess, bundle.LockedOps(wf, sess.Mode, &cs))
	st.Fanout = fanoutStatus(sess)
	st.Outputs, _ = s.Outputs(ctx, sessionID)
	return st, nil
}

// Reached records a workflow_checkpoint call that the tool let run: the
// checkpoint is passed; the result carries the instruction given with the
// validation. The caller re-applies the session rules (locks released).
func (s *Service) Reached(ctx context.Context, sessionID string, call Call) (Result, error) {
	sess, wf, err := s.session(ctx, sessionID)
	if err != nil {
		return Result{}, err
	}
	c, ok := wf.Checkpoint(call.ID)
	if !ok {
		return Result{}, fmt.Errorf("%w %q (%s)", ErrUnknownCP, call.ID, strings.Join(checkpointIDs(wf), ", "))
	}
	res := Result{ID: c.ID, Label: c.LabelFor(i18n.Locale()), Behavior: c.Behavior(sess.Mode)}
	res.Warning = byWarning(sess.ID, c, call.Agent)
	if s.States != nil {
		ap, err := s.pass(ctx, sess, c)
		if err != nil {
			return res, err
		}
		res.Message = ap.Message
		res.Unlocked = c.Unlocks
		s.reachedR3(ctx, sess.ID, c, call, &res)
		res.Wiki = s.commitPassed(ctx, sess, c)
	}
	for i, cp := range wf.Checkpoints {
		if cp.ID == c.ID && i+1 < len(wf.Checkpoints) {
			res.Next = wf.Checkpoints[i+1].ID
		}
	}
	return res, nil
}

// commitPassed runs CommitPassed for a checkpoint that unlocks the commit
// and records what it applied in the timeline.
func (s *Service) commitPassed(ctx context.Context, sess *domain.Session, c sessionspec.CheckpointDef) string {
	if s.CommitPassed == nil || !slices.Contains(c.Unlocks, sessionspec.UnlockCommit) {
		return ""
	}
	msg, applied := s.CommitPassed(ctx, sess, c.ID, c.Wiki == sessionspec.WikiSync)
	if len(applied) > 0 {
		_, _ = s.update(ctx, sess.ID, func(st *domain.CheckpointState) error {
			s.record(st, domain.CheckpointWiki, strings.Join(applied, " "), "oh", c.ID)
			return nil
		})
	}
	return msg
}

func checkpointIDs(wf *sessionspec.WorkflowRuntime) []string {
	ids := make([]string, 0, len(wf.Checkpoints))
	for _, c := range wf.Checkpoints {
		ids = append(ids, c.ID)
	}
	return ids
}

// Outputs returns the outputs declared by a session (sessions.outputs, by
// output id), typed after the workflow outputs.
func (s *Service) Outputs(ctx context.Context, sessionID string) ([]Output, error) {
	sess, wf, err := s.session(ctx, sessionID)
	if err != nil && !errors.Is(err, ErrNoWorkflow) {
		return nil, err
	}
	if sess == nil || len(sess.Outputs) == 0 {
		return nil, nil
	}
	types := map[string]string{}
	if wf != nil {
		for _, d := range wf.Outputs {
			types[d.ID] = d.Type
		}
	}
	keys := make([]string, 0, len(sess.Outputs))
	for k := range sess.Outputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]Output, 0, len(keys))
	for _, k := range keys {
		o := Output{ID: k, Type: firstNonEmpty(types[k], k)}
		switch v := sess.Outputs[k].(type) {
		case []any:
			parts := make([]string, 0, len(v))
			for _, e := range v {
				parts = append(parts, fmt.Sprint(e))
			}
			o.Value = strings.Join(parts, ",")
		default:
			o.Value = fmt.Sprint(v)
		}
		out = append(out, o)
	}
	return out, nil
}

// Declare records an output of a session in sessions.outputs (key: the
// workflow output id, else the type), replacing a previous value. A workflow
// that declares its outputs only accepts those (by id or type). Beads ids
// are stored as a list.
func (s *Service) Declare(ctx context.Context, sessionID string, o Output) error {
	_, wf, err := s.session(ctx, sessionID)
	if err != nil {
		return err
	}
	o.Type, o.ID, o.Value = strings.TrimSpace(o.Type), strings.TrimSpace(o.ID), strings.TrimSpace(o.Value)
	if o.Type == "" || o.Value == "" {
		return errors.New("an output needs a type and a value")
	}
	if len(wf.Outputs) > 0 {
		d, ok := match(wf.Outputs, o)
		if !ok {
			return fmt.Errorf("output %q is not declared by the workflow (%s)", firstNonEmpty(o.ID, o.Type), outputList(wf.Outputs))
		}
		if d.Record != "" {
			return fmt.Errorf("output %q is recorded by oh from what the session does (%s): do not declare it", d.ID, d.Record)
		}
		o.ID = d.ID
	}
	if s.SessionOutputs == nil {
		return errors.New("checkpoint: no output store")
	}
	var value any = o.Value
	if o.Type == "beads-ids" {
		var ids []string
		for _, p := range strings.Split(o.Value, ",") {
			if p = strings.TrimSpace(p); p != "" {
				ids = append(ids, p)
			}
		}
		value = ids
	}
	return s.SessionOutputs.SetSessionOutput(ctx, sessionID, firstNonEmpty(o.ID, o.Type), value)
}

// match finds the workflow output o stands for: same id and type, or the
// first output of that type when o has no id.
func match(defs []sessionspec.OutputDef, o Output) (sessionspec.OutputDef, bool) {
	for _, d := range defs {
		if (o.ID != "" && d.ID == o.ID && d.Type == o.Type) || (o.ID == "" && d.Type == o.Type) {
			return d, true
		}
	}
	return sessionspec.OutputDef{}, false
}

func outputList(defs []sessionspec.OutputDef) string {
	parts := make([]string, 0, len(defs))
	for _, d := range defs {
		parts = append(parts, d.ID+":"+d.Type)
	}
	return strings.Join(parts, ", ")
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}
