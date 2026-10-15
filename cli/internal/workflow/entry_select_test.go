package workflow

import (
	"reflect"
	"sort"
	"testing"
)

// listedAgents is a catalogue that lists its agents (SelectEntry follows the
// delegations through them).
type listedAgents struct{ fakeAgents }

func (l listedAgents) AgentIDs() []string {
	ids := make([]string, 0, len(l.fakeAgents))
	for id := range l.fakeAgents {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// A selected entry reaches the agents its task permissions name; an agent
// whose task permission names itself (reviewer) never calls itself by
// derivation (libre: depth 2, not 3).
func TestSelectEntryNoSelfDelegation(t *testing.T) {
	cat := listedAgents{fakeAgents{
		"orchestrator":     {Tasks: []string{"orchestrator-dev", "reviewer"}},
		"orchestrator-dev": {Tasks: []string{"developer", "reviewer"}},
		"developer":        {},
		"reviewer":         {SelfTask: true},
		"designer":         {},
	}}
	r := &Resolved{Spec: &Spec{ID: "libre", Entry: &Entry{Agent: "orchestrator", Selectable: true}}}
	if d := r.SelectEntry("orchestrator", cat); len(d) > 0 {
		t.Fatal(d)
	}
	if got, want := r.Spec.Agents.Keys(), []string{"orchestrator", "orchestrator-dev", "reviewer", "developer"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("members = %v, want %v", got, want)
	}
	for _, id := range r.Spec.Agents.Keys() {
		if ref, _ := r.Spec.Agents.Get(id); ref.Calls != nil {
			t.Errorf("%s: calls %v derived", id, ref.Calls)
		}
	}
	graph := DelegationGraph(r.Spec, cat)
	if targets := graph["reviewer"]; len(targets) > 0 {
		t.Errorf("reviewer delegates to %v", targets)
	}
	if d := MaxDepth("orchestrator", graph); d != 2 {
		t.Errorf("depth = %d, want 2 (graph %v)", d, graph)
	}

	// The reviewer as entry: alone, without calling itself.
	r = &Resolved{Spec: &Spec{ID: "libre", Entry: &Entry{Agent: "reviewer", Selectable: true}}}
	if d := r.SelectEntry("reviewer", cat); len(d) > 0 {
		t.Fatal(d)
	}
	if graph := DelegationGraph(r.Spec, cat); r.Spec.Agents.Len() != 1 || len(graph) != 0 {
		t.Fatalf("members %v, graph %v", r.Spec.Agents.Keys(), graph)
	}
}
