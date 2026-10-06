package workflow

import "sort"

// AgentLister lists the agents of the brick catalogue (implemented by the
// hub catalogue); needed to follow the delegations of a selected entry.
type AgentLister interface {
	AgentIDs() []string
}

// SelectEntry makes id the entry agent of a workflow whose entry is
// selectable: the members become id (primary) and every agent it may
// delegate to according to the task permissions of the catalogue, followed
// transitively (closed world: nothing else is visible).
func (r *Resolved) SelectEntry(id string, agents AgentCatalog) Diagnostics {
	var diags Diagnostics
	add := func(d Diagnostic) {
		d.Source = string(LayerSession)
		diags = append(diags, d)
	}
	s := r.Spec
	if s.Entry == nil || !s.Entry.Selectable {
		add(errDiag("session_entry_not_selectable", "session.agent", s.ID))
		return diags
	}
	if agents == nil {
		add(errDiag("session_entry_unknown", "session.agent", id))
		return diags
	}
	if _, ok := agents.Agent(id); !ok {
		add(errDiag("session_entry_unknown", "session.agent", id))
		return diags
	}
	var all []string
	if l, ok := agents.(AgentLister); ok {
		all = append(all, l.AgentIDs()...)
		sort.Strings(all)
	}
	members := OrderedMap[AgentRef]{}
	members.Set(id, AgentRef{Role: RoleWorkflow, Mode: ModePrimary})
	for queue := []string{id}; len(queue) > 0; queue = queue[1:] {
		info, ok := agents.Agent(queue[0])
		if !ok {
			continue
		}
		for _, other := range all {
			if _, seen := members.Get(other); seen || !matchAny(info.Tasks, other) {
				continue
			}
			members.Set(other, AgentRef{Role: RoleWorkflow})
			queue = append(queue, other)
		}
	}
	entry := *s.Entry
	entry.Agent = id
	s.Entry = &entry
	s.Agents = members
	return diags
}
