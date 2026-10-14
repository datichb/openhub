package sessionspec

// Fan-out of a session (R6, E3, CP-R3), copied from the oh/v1 schema: the
// bundle of a fan-out session carries it; the daemon reads it from the
// bundle of the parent before it lets a delegated checkpoint of a child
// pass without a pause.

// FanoutDef is the fan-out of a fan-out session.
type FanoutDef struct {
	// Child is the workflow of the child sessions; Tickets the input that
	// holds their tickets.
	Child   string `json:"child"`
	Tickets string `json:"tickets"`
	// Parallel is the number of children at the same time (1 to 3).
	Parallel int `json:"parallel"`
	// Merge is the merge checkpoint of the parent.
	Merge string `json:"merge"`
	// Delegates are the checkpoints of the children passed without pause
	// (CP-R3), valid only while Merge substitutes them (Substitutes).
	Delegates []string `json:"delegates,omitempty"`
	// Merger is the workflow of the conflict resolutions ("" = none).
	Merger string `json:"merger,omitempty"`
}

// VerifyOnIntegration is VerifyDef.On for the integration branch.
const VerifyOnIntegration = "integration"

// FanoutMerge returns the merge checkpoint of the fan-out of w.
func (w *WorkflowRuntime) FanoutMerge() (CheckpointDef, bool) {
	if w == nil || w.Fanout == nil || w.Fanout.Merge == "" {
		return CheckpointDef{}, false
	}
	return w.Checkpoint(w.Fanout.Merge)
}

// Substitutes reports whether the merge checkpoint of the fan-out of w
// replaces the checkpoint id of the children (CP-R3, checked again at run
// time on the bundle of the parent): id is delegated, and the merge
// checkpoint is mandatory, pauses in every mode, unlocks the merge and is
// verified by oh on the integration branch with an expected success.
func (w *WorkflowRuntime) Substitutes(id string) bool {
	if w == nil || w.Fanout == nil {
		return false
	}
	delegated := false
	for _, d := range w.Fanout.Delegates {
		delegated = delegated || d == id
	}
	if !delegated {
		return false
	}
	m, ok := w.FanoutMerge()
	if !ok || !m.Mandatory || m.Verify == nil || m.Verify.On != VerifyOnIntegration || m.Verify.Expect != "pass" {
		return false
	}
	unlocks := false
	for _, op := range m.Unlocks {
		unlocks = unlocks || op == UnlockMerge
	}
	if !unlocks {
		return false
	}
	for mode := range m.Behaviors {
		if m.Behavior(mode) != CheckpointPause {
			return false
		}
	}
	return len(m.Behaviors) > 0
}
