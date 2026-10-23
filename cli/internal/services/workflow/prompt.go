package workflow

import (
	"fmt"
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
	Branch   string // dedicated work branch, "" on the base branch
	Lang     string
	Memories string // persistent memories of the project's Beads
	// Tracker, TrackerProject and GitLabMCP: where the tickets of the project
	// live and whether the session can read them (see workflow.PromptContext).
	// GitLabRefs are the GitLab tickets the inputs name: RenderPrompt lists
	// them at the end of the prompt, whatever the template.
	Tracker        string
	TrackerProject string
	GitLabMCP      bool
	GitLabRefs     []GitLabRef
}

// GitLabRef is a GitLab issue or merge request named by an input, with the
// MCP tool that reads it (see GitLabRefsTitle).
type GitLabRef struct {
	Text    string // as written (#623, !12, a URL)
	Kind    string // "issue" or "mr"
	IID     int
	Project string // "" when unknown
	Tool    string // gitlab_get_issue, gitlab_get_mr
}

func (r *Resolution) promptContext(pc PromptContext) wf.PromptContext {
	return wf.PromptContext{Project: pc.Project, Location: pc.Location, Branch: pc.Branch, Mode: r.Mode,
		Runtime: string(r.Runtime), Lang: pc.Lang, Workflow: r.Spec.ID, Memories: pc.Memories,
		Tracker: pc.Tracker, TrackerProject: pc.TrackerProject, GitLabMCP: pc.GitLabMCP}
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

// PromptTemplate returns the template of the initial prompt, before
// rendering ("" without `prompt:`): the bundle gives the skills it names to
// the entry agent.
func (r *Resolution) PromptTemplate() (string, error) {
	return r.ReadPromptSource(r.env.Prompts)
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
	if refs := gitlabRefsReminder(pc); refs != "" {
		out = strings.TrimRight(out, "\n") + "\n\n" + refs + "\n"
	}
	if g := r.gateReminder(); g != "" {
		out = strings.TrimRight(out, "\n") + "\n\n" + g + "\n"
	}
	return out, nil
}

// GitLabRefsTitle starts the list of the GitLab tickets that oh found in the
// inputs of the launch.
const GitLabRefsTitle = "Tickets GitLab relevés par oh dans la demande (des tickets GitLab, pas des identifiants Beads : pas de `bd show`) :"

// gitlabRefsReminder tells the session which GitLab tickets the launch names
// and how to read each one; without the GitLab MCP server it tells that they
// cannot be read, so that the session says so instead of looking in Beads.
func gitlabRefsReminder(pc PromptContext) string {
	if len(pc.GitLabRefs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(GitLabRefsTitle)
	for _, r := range pc.GitLabRefs {
		project, iid := "`project_id` = celui du projet GitLab de l'équipe (demande-le à l'utilisateur s'il est inconnu)", "`iid`"
		if r.Project != "" {
			project = "`project_id` = `" + r.Project + "`"
		}
		if r.Kind == "mr" {
			iid = "`mr_iid`"
		}
		fmt.Fprintf(&b, "\n- `%s` : outil `%s` (%s, %s = %d)", r.Text, r.Tool, project, iid, r.IID)
	}
	if !pc.GitLabMCP {
		b.WriteString("\nLe MCP GitLab n'est pas activé dans cette session : tu ne peux pas lire ces tickets. Dis-le à l'utilisateur (il l'active avec `oh mcp enable gitlab`) plutôt que de les chercher avec `bd`.")
	}
	return b.String()
}

// gateReminder tells the entry agent which checkpoints it must report with
// `workflow_checkpoint` before calling a locked agent: oh refuses the
// delegation until then, and the tool does not pass the refusal reason to
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
