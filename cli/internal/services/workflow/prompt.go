package workflow

import (
	"strings"

	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// Prompt rendering: the engine is internal/workflow (render_prompt.go:
// inputs at the top level, session context under .oh, `data` for free
// text, O11). The service gives it the session context and the template
// source of the catalogue.

// ModeLinePrefix starts the line giving the workflow mode to the agents.
const ModeLinePrefix = "Mode de workflow : "

// PromptContext is the session context of a prompt (mode, runtime and
// workflow come from the resolution).
type PromptContext struct {
	Project  string
	Location string
	Lang     string
}

func (r *Resolution) promptContext(pc PromptContext) wf.PromptContext {
	return wf.PromptContext{Project: pc.Project, Location: pc.Location, Mode: r.Mode,
		Runtime: string(r.Runtime), Lang: pc.Lang, Workflow: r.Spec.ID}
}

// Values returns the effective inputs (session values, else defaults, typed;
// defaults may refer to other inputs).
func (r *Resolution) Values(pc PromptContext) (map[string]any, error) {
	vals, err := r.PromptValues(r.promptContext(pc))
	if err != nil {
		return nil, err
	}
	delete(vals, wf.ReservedInputID)
	return vals, nil
}

// HasPrompt reports whether the workflow declares an initial prompt.
func (r *Resolution) HasPrompt() bool {
	p := r.Spec.Prompt
	return p != nil && (p.Text != "" || p.Template != "")
}

// RenderPrompt renders the initial prompt ("" without `prompt:`). The line
// « Mode de workflow : <mode> » read by the generated workflow skills is
// added when the template does not write it.
func (r *Resolution) RenderPrompt(pc PromptContext) (string, error) {
	out, err := wf.RenderPrompt(r.Resolved, r.env.Prompts, r.promptContext(pc))
	if err != nil || out == "" {
		return out, err
	}
	if !strings.Contains(out, ModeLinePrefix) {
		out = ModeLinePrefix + r.Mode + "\n\n" + out
	}
	if g := r.gateReminder(); g != "" {
		out = strings.TrimRight(out, "\n") + "\n\n" + g + "\n"
	}
	return out, nil
}

// gateReminder tells the entry agent which checkpoints it must report with
// `workflow_checkpoint` before calling a locked agent: oh refuses the
// delegation until then, and opencode does not pass the refusal reason to
// the agent (lot 0 of track 3.E: orchestrator-dev never called cp-1 and
// stayed locked). Checkpoints skipped in the mode release their lock.
func (r *Resolution) gateReminder() string {
	sp := r.Spec
	locks := map[string][]string{}
	var order []string
	for _, id := range sp.Agents.Keys() {
		a, _ := sp.Agents.Get(id)
		if a.After == "" || a.Role == wf.RoleDisabled {
			continue
		}
		cp, ok := sp.Checkpoints.Get(a.After)
		if !ok || cp.Disabled {
			continue
		}
		if cp.Mode[r.Mode] == wf.BehaviorSkip && (cp.Mandatory == nil || !*cp.Mandatory) {
			continue
		}
		if _, seen := locks[a.After]; !seen {
			order = append(order, a.After)
		}
		locks[a.After] = append(locks[a.After], id)
	}
	if len(order) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(GateReminderTitle)
	for _, cp := range order {
		b.WriteString("\n- `" + cp + "` avant " + strings.Join(locks[cp], ", "))
	}
	return b.String()
}

// GateReminderTitle starts the checkpoint reminder added to the prompt.
const GateReminderTitle = "Checkpoints à signaler avec l'outil `workflow_checkpoint` (id, résumé) avant de déléguer : sans cet appel, oh refuse la délégation."

// WithInputs returns a copy of the resolution whose session inputs are
// replaced by inputs (one session of a multi-ticket launch). The values are
// not re-validated: use it with values of the same types.
func (r *Resolution) WithInputs(inputs map[string]any) *Resolution {
	cp := *r
	resolved := *r.Resolved
	resolved.Inputs = inputs
	cp.Resolved = &resolved
	return &cp
}

// FirstInput returns the first input of a type ("" when none).
func FirstInput(sp *wf.Spec, t wf.InputType) string {
	for _, k := range sp.Inputs.Keys() {
		if in, _ := sp.Inputs.Get(k); in.Type == t {
			return k
		}
	}
	return ""
}
