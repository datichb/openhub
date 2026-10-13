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
	// WikiMCP: the project wiki server (LD-J), not shipped yet.
	WikiMCP bool
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
func contractFor(req Request, wf *sessionspec.WorkflowRuntime, id string, graph, callers map[string][]string, env whenEnv) contractData {
	d := contractData{
		Entry:    env.roles[roleEntry],
		Subagent: env.roles[roleSubagent],
		Callers:  callers[id],
		Callees:  graph[id],
		Headless: env.flags["headless"],
		Report:   env.mcp[sessionspec.WorkflowMCPServer],
	}
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
	return d
}

// sessionContract renders the contract of agent id ("" when the template
// fails, which a test prevents).
func sessionContract(req Request, wf *sessionspec.WorkflowRuntime, id string, graph, callers map[string][]string, env whenEnv) string {
	t, err := contractTemplate()
	if err != nil {
		return ""
	}
	var b bytes.Buffer
	if err := t.Execute(&b, contractFor(req, wf, id, graph, callers, env)); err != nil {
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
