package domain

import "time"

// Checkpoint timeline entries of the verification run by oh (R3).
const (
	CheckpointVerified   = "verified"    // the verification passed
	CheckpointVerifyFail = "verify_fail" // the verification failed: nothing unlocked
	CheckpointOverridden = "overridden"  // validated despite a failed verification
	CheckpointUnobserved = "unobserved"  // an evidence of the agent was not observed
	CheckpointFixLimit   = "fix_limit"   // fix rounds reached the cap
	CheckpointWindow     = "window"      // a window closed (create, commit)
)

// Verification statuses.
const (
	VerifyRunning = "running"
	VerifyPassed  = "pass"
	VerifyFailed  = "fail"
	// VerifyNone: no verification command for this project (passes, shown).
	VerifyNone = "none"
)

// CheckpointVerify is the result of the verification of a checkpoint.
type CheckpointVerify struct {
	Status string `json:"status"`
	// Expect is pass or fail (the expected result of the commands).
	Expect string `json:"expect,omitempty"`
	// Fingerprint identifies the state of the work verified (HEAD and the
	// uncommitted changes): a result is reused only for the same state.
	Fingerprint string                `json:"fingerprint,omitempty"`
	Started     time.Time             `json:"started"`
	Finished    *time.Time            `json:"finished,omitempty"`
	Commands    []VerifyCommandResult `json:"commands,omitempty"`
	// Reason explains a failure that is not a command (must_touch, timeout,
	// no folder).
	Reason string `json:"reason,omitempty"`
	// Excerpt is the end of the output of the first command that decided
	// the result (bounded).
	Excerpt string `json:"excerpt,omitempty"`
}

// Done reports whether the verification finished.
func (v CheckpointVerify) Done() bool { return v.Status != "" && v.Status != VerifyRunning }

// OK reports whether the verification lets the checkpoint pass.
func (v CheckpointVerify) OK() bool { return v.Status == VerifyPassed || v.Status == VerifyNone }

// VerifyCommandResult is a command run by the verification.
type VerifyCommandResult struct {
	Kind     string `json:"kind,omitempty"`
	Cmd      string `json:"cmd"`
	Exit     int    `json:"exit"`
	TimedOut bool   `json:"timed_out,omitempty"`
	Millis   int64  `json:"ms"`
}

// CheckpointEvidence is a command the agent reports with a checkpoint
// (workflow_checkpoint evidence), checked against the shell calls observed.
type CheckpointEvidence struct {
	Cmd  string `json:"cmd"`
	Exit int    `json:"exit"`
	// Observed: a shell call of the session ran this command.
	Observed bool `json:"observed"`
	// Contradicted: it was observed, but failed while the agent says 0.
	Contradicted bool `json:"contradicted,omitempty"`
}

// ShellCall is a shell command observed in the session.
type ShellCall struct {
	ID      string `json:"id"`
	Command string `json:"cmd"`
	// Status: called | ok | failed.
	Status string    `json:"status"`
	At     time.Time `json:"at"`
}
