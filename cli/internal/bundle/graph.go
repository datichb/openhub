package bundle

import (
	"log/slog"
	"sort"

	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// deriveGraph returns the delegation graph (agent → agents it may launch).
//
// Phase 0 source of truth: each agent's resolved `task` permission (frontmatter
// + permission base), which encodes the intended delegations
// (orchestrator-dev → developer, reviewer…). The workflow-derived task map is
// much broader (any downstream agent) and is only used to drop agents the
// workflow disables. From phase 1 on, the workflow YAML graph replaces this.
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
		for target, v := range task {
			if target == "*" || target == id || disabled[target] {
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
	seen := map[string]bool{entry: true}
	queue := []string{entry}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range graph[cur] {
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		if id != entry {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return append([]string{entry}, out...)
}

// maxDepth returns the length of the longest simple delegation path from entry
// (1 = entry may launch subagents that do not delegate further). Minimum 1.
func maxDepth(entry string, graph map[string][]string) int {
	var walk func(node string, onPath map[string]bool) int
	walk = func(node string, onPath map[string]bool) int {
		best := 0
		onPath[node] = true
		for _, next := range graph[node] {
			if onPath[next] {
				continue
			}
			if d := 1 + walk(next, onPath); d > best {
				best = d
			}
		}
		delete(onPath, node)
		return best
	}
	if d := walk(entry, map[string]bool{}); d > 1 {
		return d
	}
	return 1
}
