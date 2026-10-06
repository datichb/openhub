package bundle

import (
	"fmt"
	"log/slog"
	"sort"

	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/workflow/hubcat"
)

// selectAgents returns the delegation graph and the agents of the bundle
// (entry first).
//
// With an oh/v1 workflow (req.Spec), the workflow is the source of truth
// (P1-T06): every member is shipped (workflow and independent agents) and the
// graph is the workflow delegation graph (`calls`, or each agent's task
// permission restricted to the members). Otherwise the phase 0 selection is
// used: agents reachable from the entry through their own task permissions.
func selectAgents(req Request, wf *workflow.WorkflowDefinition, files map[string]string) (graph map[string][]string, selected []string, err error) {
	if req.Spec == nil {
		graph = deriveGraph(req.HubDir, wf, files)
		return graph, reachable(req.EntryAgent, graph), nil
	}
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

// applySpec fills the request from the workflow where the caller left it empty.
func (req *Request) applySpec() error {
	s := req.Spec
	if req.EntryAgent == "" {
		req.EntryAgent = s.EntryAgent()
	} else if req.EntryAgent != s.EntryAgent() {
		return fmt.Errorf("bundle: entry agent %q differs from the workflow entry %q", req.EntryAgent, s.EntryAgent())
	}
	if req.WorkflowModels == nil {
		req.WorkflowModels = WorkflowModels(s.Models)
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

// specWorkflow replaces the legacy workflow by the oh/v1 one: no legacy
// agent slots (modes come from the agents and the workflow), and chain
// skills generated from the YAML (P1-T11).
func specWorkflow(hubDir string, spec *workflow.Spec) (*deploy.WorkflowDeployResult, error) {
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
	return &deploy.WorkflowDeployResult{GeneratedSkills: gen}, nil
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

// deriveGraph returns the delegation graph (agent → agents it may launch).
//
// Phase 0 source of truth: each agent's resolved `task` permission (frontmatter
// + permission base), which encodes the intended delegations
// (orchestrator-dev → developer, reviewer…). The workflow-derived task map is
// much broader (any downstream agent) and is only used to drop agents the
// workflow disables. oh/v1 workflows use selectAgents instead.
func deriveGraph(hubDir string, wf *workflow.WorkflowDefinition, files map[string]string) map[string][]string {
	disabled := map[string]bool{}
	for _, id := range deploy.DisabledAgentIDs(wf) {
		disabled[id] = true
	}
	graph := map[string][]string{}
	for id, path := range files {
		if disabled[id] {
			continue
		}
		fm, err := deploy.ParseAgentFrontmatter(path)
		if err != nil {
			slog.Warn("bundle: skipping agent with malformed frontmatter", "agent", id, "error", err)
			continue
		}
		perms, err := deploy.ResolvePermissions(hubDir, fm)
		if err != nil {
			perms = fm.Permission
		}
		task, _ := perms["task"].(map[string]interface{})
		var targets []string
		// A key naming the agent itself is an explicit self-delegation
		// (reviewer → parallel reviewer sessions); "*" never includes it.
		for target, v := range task {
			if target == "*" || disabled[target] {
				continue
			}
			if _, ok := files[target]; !ok {
				continue
			}
			if eff, ok := toEffect(v); ok && eff != "deny" {
				targets = append(targets, target)
			}
		}
		if len(targets) > 0 {
			sort.Strings(targets)
			graph[id] = targets
		}
	}
	return graph
}

// reachable returns entry plus every agent reachable through the graph (sorted, entry first).
func reachable(entry string, graph map[string][]string) []string {
	seen := workflow.Reachable(entry, graph)
	out := make([]string, 0, len(seen))
	for id := range seen {
		if id != entry {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return append([]string{entry}, out...)
}

// maxDepth is kept for the phase 0 tests; see workflow.MaxDepth.
func maxDepth(entry string, graph map[string][]string) int { return workflow.MaxDepth(entry, graph) }
