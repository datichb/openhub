package sessionspec

import (
	"path"
	"strings"
)

// Verification run by oh at a checkpoint and conditions of a checkpoint
// (R3: E1, E6, CP-R7), copied from the oh/v1 workflow.

// Verify kinds and expectations (copied from the oh/v1 schema).
const (
	VerifyFromStack = "stack"
	VerifyPass      = "pass"
	VerifyFail      = "fail"
)

// VerifyDef is the verification of a checkpoint.
type VerifyDef struct {
	// From is "stack" (the commands of the stack detected by oh) or "".
	From string `json:"from,omitempty"`
	// Kinds are the stack commands run (build, test, lint, typecheck).
	Kinds []string `json:"kinds,omitempty"`
	// Cmd are the commands written in the workflow document.
	Cmd []string `json:"cmd,omitempty"`
	// Expect is pass or fail.
	Expect string `json:"expect"`
	// In is the folder of the commands, relative to the session location.
	In string `json:"in,omitempty"`
	// TimeoutSec bounds the whole verification.
	TimeoutSec int `json:"timeout_sec"`
	// MustTouch: the changes of the session touch one of these paths.
	MustTouch []string `json:"must_touch,omitempty"`
	// Ask: the workflow is read-only (risk: read), so the commands only
	// run at a checkpoint the user validates (Behavior returns pause).
	Ask bool `json:"ask,omitempty"`

	// R7 (TDD): MustTouchTests: the changes touch a test file of the
	// stack; OutsideTests: what a change outside the test files does
	// ("", warn, refuse); Since: the changes are counted from the commit
	// recorded when this checkpoint was last passed (CheckpointDef.Base).
	MustTouchTests bool   `json:"must_touch_tests,omitempty"`
	OutsideTests   string `json:"outside_tests,omitempty"`
	Since          string `json:"since,omitempty"`
}

// What a change outside the test files does (copied from the oh/v1 schema).
const (
	OutsideTestsWarn   = "warn"
	OutsideTestsRefuse = "refuse"
)

// WhenDef are the conditions, evaluated by oh, that make a checkpoint that
// does not pause in the mode pause (one is enough).
type WhenDef struct {
	Entry   []string `json:"entry,omitempty"`
	Labels  []string `json:"labels,omitempty"`
	Paths   []string `json:"paths,omitempty"`
	Outputs []string `json:"outputs,omitempty"`
}

// WhenFacts are what oh knows of a session when it evaluates `when:`.
type WhenFacts struct {
	Entry   string
	Labels  []string
	Paths   []string
	Outputs []string
	// PathsUnknown: the changed paths could not be read (git refuses the
	// repository): a paths condition holds (fails closed, a pause added).
	PathsUnknown bool
}

// Holds reports whether one condition holds for facts, and which one.
func (w *WhenDef) Holds(f WhenFacts) (holds bool, why string) {
	if w == nil {
		return false, ""
	}
	for _, e := range w.Entry {
		if e == f.Entry {
			return true, "entry: " + e
		}
	}
	for _, l := range w.Labels {
		for _, have := range f.Labels {
			if strings.EqualFold(l, have) {
				return true, "label: " + l
			}
		}
	}
	if len(w.Paths) > 0 && f.PathsUnknown {
		return true, "paths: unknown"
	}
	for _, p := range w.Paths {
		for _, have := range f.Paths {
			if PathMatch(p, have) {
				return true, "path: " + have
			}
		}
	}
	for _, o := range w.Outputs {
		for _, have := range f.Outputs {
			if o == have {
				return true, "output: " + o
			}
		}
	}
	return false, ""
}

// EffectiveBehavior is the behavior of the checkpoint in mode once its
// conditions are evaluated (CP-R7): when: only adds a pause. An automatic
// checkpoint pauses when a condition holds; a conditional one with when:
// pauses only then (without when:, oh cannot evaluate it: it pauses). A
// pause is never removed.
func (c CheckpointDef) EffectiveBehavior(mode string, f WhenFacts) string {
	b := c.Behavior(mode)
	if c.When == nil || c.Mandatory || len(c.Unlocks) > 0 {
		return b // a mandatory checkpoint keeps its pause
	}
	holds, _ := c.When.Holds(f)
	switch b {
	case CheckpointAuto:
		if holds {
			return CheckpointPause
		}
	case CheckpointConditional:
		if holds {
			return CheckpointPause
		}
		return CheckpointAuto
	}
	return b
}

// PathMatch matches a slash-separated path against a pattern: `*` stands
// for any part of one segment, `**` for any number of segments; a pattern
// without a slash matches the base name too ("*_test.go").
func PathMatch(pattern, name string) bool {
	pattern, name = strings.Trim(pattern, "/"), strings.Trim(name, "/")
	if !strings.Contains(pattern, "/") && !strings.Contains(pattern, "**") {
		if ok, _ := path.Match(pattern, path.Base(name)); ok {
			return true
		}
	}
	return matchSegments(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func matchSegments(pat, name []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			for i := 0; i <= len(name); i++ {
				if matchSegments(pat[1:], name[i:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 {
			return false
		}
		if ok, _ := path.Match(pat[0], name[0]); !ok {
			return false
		}
		pat, name = pat[1:], name[1:]
	}
	return len(name) == 0
}
