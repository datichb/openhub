package workflow

import (
	"path"
	"sort"
)

// Delegation graph of an oh/v1 workflow (who may launch whom).

// AgentInfo describes an agent of the brick catalogue, as far as workflow
// validation and graph derivation need it.
type AgentInfo struct {
	ID   string
	Mode AgentMode
	// Edits reports that the agent may modify files (edit/write/patch not denied).
	Edits bool
	// Shell reports an unrestricted shell ("*" not denied).
	Shell bool
	// Tasks lists the agents the agent may launch according to its own task
	// permission, as glob patterns ("*" means any agent).
	Tasks []string
	// SelfTask reports an explicit permission to launch itself (reviewer →
	// parallel reviewer sessions); never part of Tasks.
	SelfTask bool
	// Skills lists the skill refs the agent loads (inlined and on demand).
	Skills []string
}

// AgentCatalog gives access to the agents of the brick catalogue.
type AgentCatalog interface {
	Agent(id string) (AgentInfo, bool)
}

// Members returns the agents of the workflow in declaration order, with the
// entry agent first when it is not declared (implicit conductor).
func (s *Spec) Members() []string {
	keys := s.Agents.Keys()
	entry := s.EntryAgent()
	if _, ok := s.Agents.Get(entry); ok {
		return keys
	}
	return append([]string{entry}, keys...)
}

// DelegationGraph returns, for every member of the workflow, the members it
// may launch: `calls` when written, otherwise the agent's own task
// permission restricted to the workflow. agents may be nil (only explicit
// calls are then known). A self-delegation is kept only when `calls` names
// the agent itself (e.g. a reviewer launching parallel reviewer sessions);
// derived delegations never include it.
func DelegationGraph(s *Spec, agents AgentCatalog) map[string][]string {
	members := s.Members()
	isMember := make(map[string]bool, len(members))
	for _, m := range members {
		isMember[m] = true
	}
	graph := map[string][]string{}
	for _, m := range members {
		var targets []string
		if ref, ok := s.Agents.Get(m); ok && ref.Calls != nil {
			for _, c := range ref.Calls {
				if isMember[c] && !containsStr(targets, c) {
					targets = append(targets, c)
				}
			}
		} else if agents != nil {
			info, ok := agents.Agent(m)
			if !ok {
				continue
			}
			for _, other := range members {
				if other != m && matchAny(info.Tasks, other) {
					targets = append(targets, other)
				}
			}
		}
		if len(targets) > 0 {
			graph[m] = targets
		}
	}
	return graph
}

func matchAny(patterns []string, name string) bool {
	for _, p := range patterns {
		if ok, err := path.Match(p, name); ok && err == nil {
			return true
		}
	}
	return false
}

// graphKnown reports whether the graph can be derived without the catalogue.
func graphKnown(s *Spec, agents AgentCatalog) bool {
	if agents != nil {
		return true
	}
	for _, m := range s.Members() {
		if ref, ok := s.Agents.Get(m); !ok || ref.Calls == nil {
			return false
		}
	}
	return true
}

// SubagentGraph is the delegation graph compiled into a bundle (O1): the
// workflow members (entry first, then declaration order) and, for each
// member, the members it may launch (sorted).
func SubagentGraph(s *Spec, agents AgentCatalog) (members []string, graph map[string][]string) {
	graph = DelegationGraph(s, agents)
	for id := range graph {
		sort.Strings(graph[id])
	}
	return s.Members(), graph
}

// MaxDepth is the length of the longest delegation chain from entry without
// revisiting an agent (1 = the entry launches subagents that do not delegate
// further). A self-delegation adds one level (the agent launches itself
// once, then goes on). It is at least 1: the tool needs a positive subagent
// depth.
func MaxDepth(entry string, graph map[string][]string) int {
	var walk func(node string, onPath map[string]bool) int
	walk = func(node string, onPath map[string]bool) int {
		best, self := 0, false
		onPath[node] = true
		for _, next := range graph[node] {
			if next == node {
				self = true
				continue
			}
			if onPath[next] {
				continue
			}
			if d := 1 + walk(next, onPath); d > best {
				best = d
			}
		}
		delete(onPath, node)
		if self {
			best++
		}
		return best
	}
	if d := walk(entry, map[string]bool{}); d > 1 {
		return d
	}
	return 1
}

// Reachable returns the members reachable from entry (entry included).
func Reachable(entry string, graph map[string][]string) map[string]bool {
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
	return seen
}

// findCycle returns one cycle of graph (first node repeated at the end), in
// the order of nodes, or nil. Explicit self-delegations are not cycles.
func findCycle(nodes []string, graph map[string][]string) []string {
	const (
		white = iota
		grey
		black
	)
	color := map[string]int{}
	var stack []string
	var cycle []string
	var visit func(n string) bool
	visit = func(n string) bool {
		color[n] = grey
		stack = append(stack, n)
		for _, next := range graph[n] {
			if next == n {
				continue
			}
			switch color[next] {
			case grey:
				for i, s := range stack {
					if s == next {
						cycle = append(append([]string(nil), stack[i:]...), next)
						return true
					}
				}
			case white:
				if visit(next) {
					return true
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[n] = black
		return false
	}
	for _, n := range nodes {
		if color[n] == white && visit(n) {
			return cycle
		}
	}
	return nil
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
