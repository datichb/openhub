package bundle

import (
	"bytes"
	"slices"
	"strings"
	"sync"
	"text/template"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Session contract (modernisation L3): what the workflow gives each agent,
// generated from the workflow and its delegation graph, appended to the
// prompt of the agent (templates/session-contract.md.tmpl). It replaces
// the role note of C0.1: role, callers and return heading, agents it may
// launch, its own checkpoints only (CP-R11), fix rounds, whether it may ask
// the user, reports, headless session.

// contractHeading opens the contract in a composed prompt.
const contractHeading = "\n\n## Contrat de session\n\n> Généré par oh"

type contractCheckpoint struct{ ID, Label string }

type contractData struct {
	Workflow         string
	Entry, Subagent  bool
	Callers, Callees []string
	HasCheckpoints   bool
	Own              []contractCheckpoint
	MaxFixRounds     int
	Headless         bool
	Report           bool
	// WikiMCP: the wiki MCP server of the project (LD-J); WikiApply: the
	// agent may apply proposals (wiki_apply).
	WikiMCP, WikiApply bool
	// Statuses: the return statuses of the agents it may launch (L5).
	Statuses []contractStatuses
	// Method: the method of the session (R7) and its steps, in order.
	Method contractMethod
	// Writes: the documentation paths the workflow opens to the agent (R4);
	// WritesOnly: they are the only files it may edit (its own rules edit
	// none).
	Writes     []string
	WritesOnly bool
	// WritesAfter: the checkpoint that opens them (r5-fixes).
	WritesAfter string
	// RefusedShell: the helpers its shell refuses (p5-fixes).
	RefusedShell []string
}

var (
	contractOnce sync.Once
	contractTmpl *template.Template
	errContract  error
)

func contractTemplate() (*template.Template, error) {
	contractOnce.Do(func() {
		contractTmpl, errContract = template.New("session-contract.md.tmpl").Funcs(template.FuncMap{
			"codes": func(ss []string) string {
				out := make([]string, len(ss))
				for i, s := range ss {
					out[i] = "`" + s + "`"
				}
				return strings.Join(out, ", ")
			},
		}).ParseFS(templateFS, "templates/session-contract.md.tmpl")
	})
	return contractTmpl, errContract
}

// contractFor returns the contract data of agent id.
func contractFor(req Request, wf *sessionspec.WorkflowRuntime, id string, graph, callers map[string][]string, env whenEnv, statuses payloadStatuses) contractData {
	d := contractData{
		Entry:        env.roles[roleEntry],
		Subagent:     env.roles[roleSubagent],
		Callers:      callers[id],
		Callees:      graph[id],
		Headless:     env.flags["headless"],
		Report:       env.mcp[sessionspec.WorkflowMCPServer],
		WikiMCP:      env.mcp[sessionspec.WikiMCPServer],
		RefusedShell: env.refusedShell,
	}
	d.WikiApply = d.WikiMCP && id == sessionspec.WikiApplier
	if req.Spec != nil {
		d.Workflow = req.Spec.ID
	}
	commits := false // passes the checkpoint of the commit
	if wf != nil {
		d.HasCheckpoints = len(wf.Checkpoints) > 0
		for _, c := range wf.Checkpoints {
			if slices.Contains(c.By, id) {
				d.Own = append(d.Own, contractCheckpoint{ID: c.ID, Label: c.LabelFor(generatedLang)})
				commits = commits || slices.Contains(c.Unlocks, sessionspec.UnlockCommit)
			}
		}
	}
	if req.Spec != nil && commits && req.Spec.Limits != nil && req.Spec.Limits.MaxFixRounds != nil {
		l := req.Spec.Limits
		d.MaxFixRounds = *l.MaxFixRounds
	}
	d.Statuses = calleeStatuses(id, d.Callees, statuses)
	d.Method = methodContract(req, wf, id, d.Callees)
	d.Writes = req.writesOf(id)
	d.WritesOnly = len(d.Writes) > 0 && !env.ownEdits
	if len(d.Writes) > 0 && req.Spec != nil {
		if a, ok := req.Spec.Agents.Get(id); ok {
			d.WritesAfter = a.WritesAfter
		}
	}
	return d
}

// sessionContract renders the contract of agent id ("" when the template
// fails, which a test prevents).
func sessionContract(req Request, wf *sessionspec.WorkflowRuntime, id string, graph, callers map[string][]string, env whenEnv, statuses payloadStatuses) string {
	t, err := contractTemplate()
	if err != nil {
		return ""
	}
	var b bytes.Buffer
	if err := t.Execute(&b, contractFor(req, wf, id, graph, callers, env, statuses)); err != nil {
		return ""
	}
	return "\n\n" + strings.TrimSpace(b.String()) + "\n"
}

// fillCheckpointBy sets the agents that pass each checkpoint of the runtime:
// `by:` of the workflow, or deduced from the delegation graph.
func fillCheckpointBy(wf *sessionspec.WorkflowRuntime, spec *workflow.Spec, graph map[string][]string) {
	if wf == nil || spec == nil {
		return
	}
	for i := range wf.Checkpoints {
		wf.Checkpoints[i].By = spec.CheckpointBy(wf.Checkpoints[i].ID, graph)
	}
}

// Return statuses of the callees (modernisation L5). Each agent that returns
// to a caller has a payload skill <agent>-payload whose frontmatter lists
// the statuses of its envelope (`statuses:`); the contract of a caller gives
// them for the agents it may launch, so that the caller reads a status
// without the format of the callee in its prompt.

// payloadSuffix names the payload skill of an agent: <agent>-payload.
const payloadSuffix = "-payload"

// payloadStatuses returns the statuses of the payload of an agent (nil when
// it has none).
type payloadStatuses func(agent string) []string

type contractStatuses struct {
	Agent    string
	Statuses []string
}

// calleeStatuses lists the statuses of the callees of id that have a
// payload, in the order of callees (the agent itself, when it may launch
// itself, is left out: it knows its own payload).
func calleeStatuses(id string, callees []string, statuses payloadStatuses) []contractStatuses {
	if statuses == nil {
		return nil
	}
	var out []contractStatuses
	for _, c := range callees {
		if c == id {
			continue
		}
		if s := statuses(c); len(s) > 0 {
			out = append(out, contractStatuses{Agent: c, Statuses: s})
		}
	}
	return out
}

// hubPayloadStatuses reads the statuses from the payload skills of the hub
// (index: skill id → ref, as hubSkillIndex).
func hubPayloadStatuses(loader *skillLoader, index map[string]string) payloadStatuses {
	return func(agent string) []string {
		ref, ok := index[agent+payloadSuffix]
		if !ok {
			return nil
		}
		d, err := loader.load(ref)
		if err != nil {
			return nil
		}
		return d.Front.Statuses
	}
}

// Restricted shell (p5-fixes): the tool checks every command of a line on
// its own (pipes, lists, substitutions) and refuses the whole line when
// one is refused (shellsim.go). Agents allowed only a list of commands
// chained a refused helper (`| head`, `echo`, `| jq`, `which`) and took
// the refusal for a missing shell; the contract of an agent whose rules
// refuse one of these helpers names those it is refused and tells it to
// run each command alone. The refusal message is the tool's own: a line
// denied by the rules of the agent never reaches the oh plugin.

// restrictedShellProbes are the helpers agents chain to a command, in the
// order the contract names them.
var restrictedShellProbes = []string{"head", "tail", "jq", "echo", "which", "cat"}

// refusedShellHelpers returns the helpers of restrictedShellProbes the
// rules perms refuse.
func refusedShellHelpers(perms []sessionspec.PermissionRule) []string {
	var out []string
	for _, h := range restrictedShellProbes {
		if ShellEffect(perms, h+" x") == sessionspec.EffectDeny {
			out = append(out, h)
		}
	}
	return out
}
