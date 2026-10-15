package workflow

import (
	"sort"

	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// EntryChoices lists the entry agents a session of the workflow may start
// on when its entry is selectable (`entry.selectable`, `oh run libre
// --agent <id>`): the default entry agent first, then the primary agents of
// the brick catalogue, then the others, each group by id. nil when the
// entry is not selectable or the catalogue is unknown.
func (r *Resolution) EntryChoices() []string {
	if r == nil || r.Spec == nil || r.Spec.Entry == nil || !r.Spec.Entry.Selectable || r.env.Agents == nil {
		return nil
	}
	l, ok := r.env.Agents.(interface{ AgentIDs() []string })
	if !ok {
		return nil
	}
	def := r.Spec.EntryAgent()
	var primary, others []string
	for _, id := range l.AgentIDs() {
		if id == def {
			continue
		}
		if info, ok := r.env.Agents.Agent(id); ok && info.Mode == wf.ModePrimary {
			primary = append(primary, id)
		} else {
			others = append(others, id)
		}
	}
	sort.Strings(primary)
	sort.Strings(others)
	return append(append([]string{def}, primary...), others...)
}
