// Package checkpoint is the CheckpointService (03 §2.4, D8): the workflow
// state of each session (checkpoints passed, current one, gates, circuit
// breaker) and the backend of the oh MCP server `workflow`. It runs in the
// oh daemon, fed by the tool event stream; the CLI and the TUI read it.
package checkpoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	// BundlesDir locates the session bundles (~/.oh/bundles).
	BundlesDir string
	// SessionsDir keeps per-session files (~/.oh/sessions/<id>/outputs.json).
	SessionsDir string
	Now         func() time.Time
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
	Locked          []sessionspec.AgentGate `json:"locked_agents,omitempty"`
	ExpectedOutputs []sessionspec.OutputDef `json:"expected_outputs,omitempty"`
	Outputs         []Output                `json:"outputs,omitempty"`
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
}

// Output is a typed value declared by workflow_outputs (O7).
type Output struct {
	ID    string    `json:"id,omitempty"`
	Type  string    `json:"type"`
	Value string    `json:"value"`
	At    time.Time `json:"at"`
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
	lang := i18n.Locale()
	st := Status{SessionID: sess.ID, Workflow: wf.ID, Mode: sess.Mode, ExpectedOutputs: wf.Outputs}
	for _, c := range wf.Checkpoints {
		cs := CheckpointStatus{ID: c.ID, Label: c.LabelFor(lang), Behavior: c.Behavior(sess.Mode), State: StateTodo}
		if cs.Behavior == sessionspec.CheckpointSkip {
			cs.State = StateSkipped
		} else if st.Next == "" {
			st.Next = c.ID
		}
		st.Checkpoints = append(st.Checkpoints, cs)
	}
	st.Locked = append(st.Locked, wf.Gates...)
	st.Outputs, _ = s.Outputs(sessionID)
	return st, nil
}

// Reached records a workflow_checkpoint call that the tool let run.
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
	for i, cp := range wf.Checkpoints {
		if cp.ID == c.ID && i+1 < len(wf.Checkpoints) {
			res.Next = wf.Checkpoints[i+1].ID
		}
	}
	return res, nil
}

func checkpointIDs(wf *sessionspec.WorkflowRuntime) []string {
	ids := make([]string, 0, len(wf.Checkpoints))
	for _, c := range wf.Checkpoints {
		ids = append(ids, c.ID)
	}
	return ids
}

// outputsFile is ~/.oh/sessions/<id>/outputs.json. It stands in for the
// sessions.outputs column (migration v34, launch track) until it exists.
func (s *Service) outputsFile(id string) string {
	return filepath.Join(s.SessionsDir, id, "outputs.json")
}

// Outputs returns the outputs declared by a session.
func (s *Service) Outputs(sessionID string) ([]Output, error) {
	if s.SessionsDir == "" {
		return nil, nil
	}
	data, err := os.ReadFile(s.outputsFile(sessionID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Output
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Declare records an output of a session. An output with the same id (or,
// without id, the same type) replaces the previous one. A workflow that
// declares its outputs only accepts those (by id or type).
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
		o.ID = d.ID
	}
	if s.SessionsDir == "" {
		return errors.New("checkpoint: no sessions directory")
	}
	list, err := s.Outputs(sessionID)
	if err != nil {
		return err
	}
	if o.At.IsZero() {
		o.At = s.now()
	}
	kept := list[:0]
	for _, prev := range list {
		if (o.ID != "" && prev.ID == o.ID) || (o.ID == "" && prev.ID == "" && prev.Type == o.Type) {
			continue
		}
		kept = append(kept, prev)
	}
	kept = append(kept, o)
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].At.Before(kept[j].At) })
	data, err := json.MarshalIndent(kept, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.outputsFile(sessionID)), 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.outputsFile(sessionID), data, 0o600)
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
