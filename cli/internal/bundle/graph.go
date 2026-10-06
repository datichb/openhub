package bundle

import (
	"fmt"
	"sort"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/workflow/hubcat"
)

// selectAgents returns the delegation graph and the agents of the bundle
// (entry first). The workflow is the source of truth (P1-T06): every member
// is shipped (workflow and independent agents) and the graph is the workflow
// delegation graph (`calls`, or each agent's task permission restricted to
// the members).
func selectAgents(req Request, files map[string]string) (graph map[string][]string, selected []string, err error) {
	cat, err := hubcat.New(req.HubDir)
	if err != nil {
		return nil, nil, err
	}
	members, graph := workflow.SubagentGraph(req.Spec, cat)
	for _, id := range members {
		if _, ok := files[id]; !ok {
			return nil, nil, fmt.Errorf("bundle: workflow %s: unknown agent %q", req.Spec.ID, id)
		}
	}
	return graph, members, nil
}

// applySpec fills the request from the workflow where the caller left it
// empty; `code_mode`, when written, always comes from the workflow.
func (req *Request) applySpec() error {
	s := req.Spec
	if req.EntryAgent == "" {
		req.EntryAgent = s.EntryAgent()
	} else if req.EntryAgent != s.EntryAgent() {
		return fmt.Errorf("bundle: entry agent %q differs from the workflow entry %q", req.EntryAgent, s.EntryAgent())
	}
	if s.Isolation == workflow.IsolationStrict {
		req.StrictIsolation = true
	}
	if req.WorkflowModels == nil {
		req.WorkflowModels = WorkflowModels(s.Models)
	}
	if req.Plugins == nil && len(s.Plugins) > 0 {
		req.Plugins = WorkflowPlugins(s.Plugins)
	}
	if s.CodeMode != nil {
		req.CodeMode = *s.CodeMode
	}
	if s.Skills != nil {
		if req.ExtraSkills == nil {
			req.ExtraSkills = s.Skills.Extra
		}
		if req.DenySkills == nil {
			req.DenySkills = s.Skills.Deny
		}
	}
	return nil
}

// specSkills returns the chain skills generated from the workflow YAML
// (P1-T11), by skill ref.
func specSkills(hubDir string, spec *workflow.Spec) (map[string]string, error) {
	cat, err := hubcat.New(hubDir)
	if err != nil {
		return nil, err
	}
	_, graph := workflow.SubagentGraph(spec, cat)
	gen, err := GenerateWorkflowSkills(spec, graph, func(id string) workflow.AgentMode {
		info, _ := cat.Agent(id)
		return info.Mode
	})
	if err != nil {
		return nil, err
	}
	return gen, nil
}

// bundleGraph restricts graph to the shipped agents (sorted targets).
func bundleGraph(graph map[string][]string, selected []string) map[string][]string {
	inBundle := map[string]bool{}
	for _, id := range selected {
		inBundle[id] = true
	}
	out := map[string][]string{}
	for _, id := range selected {
		var targets []string
		for _, t := range graph[id] {
			if inBundle[t] {
				targets = append(targets, t)
			}
		}
		if len(targets) > 0 {
			sort.Strings(targets)
			out[id] = targets
		}
	}
	return out
}

// applyWorkflowModes applies `agents.<id>.mode` of the workflow.
func applyWorkflowModes(agents []sessionspec.AgentDef, spec *workflow.Spec) {
	if spec == nil {
		return
	}
	for i := range agents {
		if ref, ok := spec.Agents.Get(agents[i].ID); ok && ref.Mode != "" {
			agents[i].Mode = string(ref.Mode)
		}
	}
}
