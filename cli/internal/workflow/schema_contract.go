package workflow

import "sort"

// Session contract (modernisation L3): who passes each checkpoint, and
// whether anyone follows the session.

// CheckpointBy returns the agents that pass checkpoint id: its `by` when
// written; otherwise the agents that may launch (graph) an agent the
// checkpoint gates (`after: id`), as they wait for it; otherwise the entry
// agent.
func (s *Spec) CheckpointBy(id string, graph map[string][]string) []string {
	if cp, ok := s.Checkpoints.Get(id); ok && len(cp.By) > 0 {
		return append([]string(nil), cp.By...)
	}
	gated := map[string]bool{}
	for _, k := range s.Agents.Keys() {
		if a, _ := s.Agents.Get(k); a.After == id && a.Role != RoleDisabled {
			gated[k] = true
		}
	}
	seen := map[string]bool{}
	var out []string
	for caller, targets := range graph {
		for _, t := range targets {
			if gated[t] && !gated[caller] && !seen[caller] {
				seen[caller] = true
				out = append(out, caller)
			}
		}
	}
	if len(out) == 0 {
		return []string{s.EntryAgent()}
	}
	sort.Strings(out)
	return out
}
