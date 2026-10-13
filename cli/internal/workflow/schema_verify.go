package workflow

import (
	"slices"
	"time"
)

// Verification run by oh at a checkpoint and conditions of a checkpoint
// (R3: E1, E6, CP-R7).
//
//	checkpoints:
//	  cp-2:
//	    verify: { from: stack, expect: pass }
//	  cp-spec:
//	    mode: { semi-auto: conditional }
//	    when: { paths: ["docs/specs/**"] }

// VerifySpec is the verification of a checkpoint: the commands come from
// the stack of the project (`from: stack`, bricks.VerifyCommands) or from
// the workflow document itself (`cmd`), never from the agent.
type VerifySpec struct {
	// From names the source of the commands: "stack" (detected by oh).
	From VerifyFrom `yaml:"from,omitempty"`
	// Kinds restricts the stack commands (build, test, lint, typecheck);
	// empty = build, test and typecheck.
	Kinds []VerifyKind `yaml:"kinds,omitempty"`
	// Cmd lists commands written in the workflow document (never taken
	// from the agent nor from an input: no template), run without a shell
	// (split on spaces), after the stack commands.
	Cmd []string `yaml:"cmd,omitempty"`
	// Expect is the expected result: pass (default) or fail (a test written
	// first must fail).
	Expect VerifyExpect `yaml:"expect,omitempty"`
	// In is the folder the commands run in, relative to the session
	// location ("" = the location).
	In string `yaml:"in,omitempty"`
	// Timeout bounds the whole verification (Go duration, default 10m, at
	// most 30m).
	Timeout string `yaml:"timeout,omitempty"`
	// MustTouch requires the changes of the session to touch at least one
	// of these paths (wildcards: *, **).
	MustTouch []string `yaml:"must_touch,omitempty"`

	// R7 (TDD): the changes and the test files.

	// MustTouchTests requires the changes to touch a test file of the
	// stack (bricks.TestFilePatterns).
	MustTouchTests bool `yaml:"must_touch_tests,omitempty"`
	// OutsideTests is what a change outside the test files does (with
	// must_touch_tests): "" nothing, warn (noted on the result), refuse.
	OutsideTests OutsideTests `yaml:"outside_tests,omitempty"`
	// Since counts the changes from the commit of the location when this
	// checkpoint was last passed ("" = from the start of the session).
	Since string `yaml:"since,omitempty"`
}

// OutsideTests is what a change outside the test files does.
type OutsideTests string

const (
	OutsideTestsWarn   OutsideTests = "warn"
	OutsideTestsRefuse OutsideTests = "refuse"
)

// rank orders the values by strictness ("" < warn < refuse).
func (o OutsideTests) rank() int {
	switch o {
	case OutsideTestsWarn:
		return 1
	case OutsideTestsRefuse:
		return 2
	}
	return 0
}

// VerifyFrom is the source of the verification commands.
type VerifyFrom string

// VerifyFromStack takes the commands of the stack detected by oh.
const VerifyFromStack VerifyFrom = "stack"

// VerifyKind is a kind of verification command.
type VerifyKind string

const (
	VerifyBuild     VerifyKind = "build"
	VerifyTest      VerifyKind = "test"
	VerifyLint      VerifyKind = "lint"
	VerifyTypecheck VerifyKind = "typecheck"
)

// VerifyKinds lists the valid kinds; DefaultVerifyKinds the ones run when
// `kinds` is not set (lint is opt-in: many projects are not lint clean).
var (
	VerifyKinds        = []VerifyKind{VerifyBuild, VerifyTest, VerifyLint, VerifyTypecheck}
	DefaultVerifyKinds = []VerifyKind{VerifyBuild, VerifyTest, VerifyTypecheck}
)

// VerifyExpect is the expected result of a verification.
type VerifyExpect string

const (
	VerifyPass VerifyExpect = "pass"
	VerifyFail VerifyExpect = "fail"
)

// Verification timeouts.
const (
	DefaultVerifyTimeout = 10 * time.Minute
	MaxVerifyTimeout     = 30 * time.Minute
)

// Valid reports whether the kind is known.
func (k VerifyKind) Valid() bool { return slices.Contains(VerifyKinds, k) }

// ExpectOrPass returns `expect`, pass when unset.
func (v *VerifySpec) ExpectOrPass() VerifyExpect {
	if v == nil || v.Expect == "" {
		return VerifyPass
	}
	return v.Expect
}

// EffectiveKinds returns `kinds`, DefaultVerifyKinds when unset.
func (v *VerifySpec) EffectiveKinds() []VerifyKind {
	if v == nil || len(v.Kinds) == 0 {
		return DefaultVerifyKinds
	}
	return v.Kinds
}

// TimeoutOrDefault returns the timeout (default when unset or invalid,
// capped at MaxVerifyTimeout).
func (v *VerifySpec) TimeoutOrDefault() time.Duration {
	if v == nil || v.Timeout == "" {
		return DefaultVerifyTimeout
	}
	d, err := time.ParseDuration(v.Timeout)
	switch {
	case err != nil || d <= 0:
		return DefaultVerifyTimeout
	case d > MaxVerifyTimeout:
		return MaxVerifyTimeout
	}
	return d
}

// Covers reports whether v is at least as strict as parent: a layer may
// add to an inherited verification, never remove from it (same source,
// expectation and folder, every kind, command and path kept, a timeout not
// shorter).
func (v *VerifySpec) Covers(parent *VerifySpec) bool {
	if parent == nil {
		return true
	}
	if v == nil || v.From != parent.From || v.ExpectOrPass() != parent.ExpectOrPass() || v.In != parent.In {
		return false
	}
	if parent.From != "" && !containsAll(v.EffectiveKinds(), parent.EffectiveKinds()) {
		return false
	}
	if (parent.MustTouchTests && !v.MustTouchTests) || v.OutsideTests.rank() < parent.OutsideTests.rank() || v.Since != parent.Since {
		return false
	}
	return containsAll(v.Cmd, parent.Cmd) && containsAll(v.MustTouch, parent.MustTouch) &&
		v.TimeoutOrDefault() >= parent.TimeoutOrDefault()
}

func (v *VerifySpec) clone() *VerifySpec {
	if v == nil {
		return nil
	}
	c := *v
	c.Kinds = cloneSlice(v.Kinds)
	c.Cmd = cloneSlice(v.Cmd)
	c.MustTouch = cloneSlice(v.MustTouch)
	return &c
}

// CheckpointWhen are the conditions, evaluated by oh, that make a
// checkpoint pause: one of them is enough.
type CheckpointWhen struct {
	// Entry: the session started on one of these agents.
	Entry []string `yaml:"entry,omitempty"`
	// Labels: a ticket of the session has one of these labels.
	Labels []string `yaml:"labels,omitempty"`
	// Paths: the changes of the session touch one of these paths
	// (wildcards: *, **).
	Paths []string `yaml:"paths,omitempty"`
	// Outputs: the session declared one of these outputs (workflow_outputs).
	Outputs []string `yaml:"outputs,omitempty"`
}

// IsZero reports whether no condition is set.
func (w *CheckpointWhen) IsZero() bool {
	return w == nil || len(w.Entry)+len(w.Labels)+len(w.Paths)+len(w.Outputs) == 0
}

// Covers reports whether w pauses at least whenever parent does (every
// inherited condition kept).
func (w *CheckpointWhen) Covers(parent *CheckpointWhen) bool {
	if parent.IsZero() {
		return true
	}
	if w == nil {
		return false
	}
	return containsAll(w.Entry, parent.Entry) && containsAll(w.Labels, parent.Labels) &&
		containsAll(w.Paths, parent.Paths) && containsAll(w.Outputs, parent.Outputs)
}

func (w *CheckpointWhen) clone() *CheckpointWhen {
	if w == nil {
		return nil
	}
	return &CheckpointWhen{Entry: cloneSlice(w.Entry), Labels: cloneSlice(w.Labels), Paths: cloneSlice(w.Paths), Outputs: cloneSlice(w.Outputs)}
}

func containsAll[T comparable](have, want []T) bool {
	for _, x := range want {
		if !slices.Contains(have, x) {
			return false
		}
	}
	return true
}
