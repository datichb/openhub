package sessionctx

import "github.com/datichb/openhub/cli/internal/sessionspec"

// CheckpointRef is a checkpoint named in the oh.checkpoints entry.
type CheckpointRef struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
}

// Checkpoints is the workflow state of the oh.checkpoints entry.
type Checkpoints struct {
	Workflow string
	Mode     string
	Passed   []CheckpointRef // passed or skipped in this mode
	Current  string          // waiting for its decision
	Next     string
	Breaker  bool // circuit breaker tripped
}

// Value is the entry value (same shape whoever writes it: the RunService
// at creation, the daemon at each transition — an unchanged value is not
// written again).
func (c Checkpoints) Value() map[string]any {
	passed := c.Passed
	if passed == nil {
		passed = []CheckpointRef{}
	}
	v := map[string]any{"workflow": c.Workflow, "mode": c.Mode, "passed": passed}
	if c.Current != "" {
		v["current"] = c.Current
	}
	if c.Next != "" {
		v["next"] = c.Next
	}
	if c.Breaker {
		v["circuit_breaker"] = true
	}
	return v
}

// InitialCheckpoints is the state of a session that has not started yet:
// the checkpoints skipped in its mode count as passed, the next one is the
// first other. ok=false for a workflow without checkpoint (no entry).
func InitialCheckpoints(wf *sessionspec.WorkflowRuntime, mode, lang string) (Checkpoints, bool) {
	if wf == nil || len(wf.Checkpoints) == 0 {
		return Checkpoints{}, false
	}
	c := Checkpoints{Workflow: wf.ID, Mode: mode}
	for _, cp := range wf.Checkpoints {
		if cp.Behavior(mode) == sessionspec.CheckpointSkip {
			c.Passed = append(c.Passed, CheckpointRef{ID: cp.ID, Label: cp.LabelFor(lang)})
			continue
		}
		if c.Next == "" {
			c.Next = cp.ID
		}
	}
	return c, true
}
