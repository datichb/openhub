package checkpoint

import (
	"strconv"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Checkpoint timeline of a session (session detail, 10 §7.1):
//
//	✔ cp-0 10:02 → ✔ cp-1 10:03 → developer (3) → reviewer → ⏸ cp-2 → ○ cp-3

// Step kinds of a timeline (domain.Checkpoint* plus the steps still to come).
const (
	StepPassed   = domain.CheckpointPassed
	StepWaiting  = domain.CheckpointWaiting
	StepRefused  = domain.CheckpointRefused
	StepDelegate = domain.CheckpointDelegate
	StepBreaker  = domain.CheckpointBreaker
	StepTodo     = "todo"
)

// Step is one step of a session timeline.
type Step struct {
	Kind  string
	ID    string // checkpoint or agent
	Label string // checkpoint label
	At    time.Time
	Count int // delegations of the same agent in a row
}

// Timeline merges the history of a session with the checkpoints still to
// come (in declaration order; checkpoints skipped in the mode are left out).
func Timeline(wf *sessionspec.WorkflowRuntime, mode, lang string, st domain.CheckpointState) []Step {
	if wf == nil {
		return nil
	}
	label := func(id string) string {
		if c, ok := wf.Checkpoint(id); ok {
			return c.LabelFor(lang)
		}
		return ""
	}
	var out []Step
	for _, e := range st.Timeline {
		n := len(out)
		switch e.Kind {
		case domain.CheckpointDelegate:
			if n > 0 && out[n-1].Kind == StepDelegate && out[n-1].ID == e.ID {
				out[n-1].Count++
				continue
			}
			out = append(out, Step{Kind: StepDelegate, ID: e.ID, At: e.At, Count: 1})
		case domain.CheckpointWaiting:
			// Asked again after a refusal: keep one step per round.
			if n > 0 && out[n-1].Kind == StepWaiting && out[n-1].ID == e.ID {
				continue
			}
			out = append(out, Step{Kind: StepWaiting, ID: e.ID, Label: label(e.ID), At: e.At})
		case domain.CheckpointPassed, domain.CheckpointRefused:
			// The decision replaces its waiting step.
			if n > 0 && out[n-1].Kind == StepWaiting && out[n-1].ID == e.ID {
				out = out[:n-1]
			}
			out = append(out, Step{Kind: e.Kind, ID: e.ID, Label: label(e.ID), At: e.At})
		case domain.CheckpointBreaker:
			out = append(out, Step{Kind: StepBreaker, At: e.At})
		}
	}
	// A waiting step is current only if it is still the one waited for.
	for i := range out {
		if out[i].Kind == StepWaiting && out[i].ID != st.Waiting {
			out[i].Kind = StepRefused
		}
	}
	for _, c := range wf.Checkpoints {
		if st.HasPassed(c.ID) || st.Waiting == c.ID || c.Behavior(mode) == sessionspec.CheckpointSkip {
			continue
		}
		out = append(out, Step{Kind: StepTodo, ID: c.ID, Label: c.LabelFor(lang)})
	}
	return out
}

// StepText renders a step in plain text ("✔ cp-1 10:03", "developer (3)").
func StepText(s Step) string {
	switch s.Kind {
	case StepPassed:
		if s.At.IsZero() {
			return "✔ " + s.ID
		}
		return "✔ " + s.ID + " " + s.At.Local().Format("15:04")
	case StepWaiting:
		return "⏸ " + s.ID
	case StepRefused:
		return "↺ " + s.ID
	case StepDelegate:
		if s.Count > 1 {
			return s.ID + " (" + strconv.Itoa(s.Count) + ")"
		}
		return s.ID
	case StepBreaker:
		return "✗"
	}
	return "○ " + s.ID
}

// TimelineText renders a timeline on one line (the last steps when long).
func TimelineText(steps []Step, limit int) string {
	parts := make([]string, 0, len(steps))
	for _, s := range steps {
		parts = append(parts, StepText(s))
	}
	if limit > 0 && len(parts) > limit {
		parts = append([]string{"…"}, parts[len(parts)-limit:]...)
	}
	return strings.Join(parts, " → ")
}
