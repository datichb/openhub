package glyph

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/domain"
)

// U-09: the table of symbols, as documented in docs/reference/cli.*.md.
func TestSymbolTable(t *testing.T) {
	states := map[domain.RunState]string{
		domain.RunPreparing: "◓", domain.RunQueued: "…", domain.RunActive: "●", domain.RunWaiting: "◐",
		domain.RunIdle: "○", domain.RunSleeping: "◌", domain.RunCompleted: "✔", domain.RunFailed: "⊗", domain.RunStopped: "■",
	}
	if len(States) != len(states) {
		t.Fatalf("%d states in the table, %d expected", len(States), len(states))
	}
	for s, want := range states {
		if got := ForState(s); got != want {
			t.Errorf("state %s: %q, want %q", s, got, want)
		}
	}
	kinds := map[domain.DecisionKind]string{
		domain.DecisionCheckpoint: "⏸", domain.DecisionQuestion: "?", domain.DecisionPermission: "!",
		domain.DecisionBudget: "$", domain.DecisionError: "✗", domain.DecisionCircuit: "⊘",
	}
	if len(Kinds) != len(kinds) {
		t.Fatalf("%d kinds in the table, %d expected", len(Kinds), len(kinds))
	}
	for k, want := range kinds {
		if got := ForKind(k); got != want {
			t.Errorf("kind %s: %q, want %q", k, got, want)
		}
	}
	if ForState("nope") != UnknownState || ForKind("nope") != UnknownKind {
		t.Error("unknown state or kind")
	}
}

// U-09: a symbol means one thing (✔ finished and stopped, ⏸ waiting and
// checkpoint, ✗ three meanings were the ambiguities). Since v5.1 the table of
// meanings is part of it: a run state or a decision kind may take a symbol of
// that table only for its own meaning (TestStatesShareOnlyTheirMeaning), so
// ⊗ (a run that stopped on a failure) is not ✗ (a one-off error), ◌ (asleep)
// is not – (a skipped step), and ● (running) is not ✔ (finished).
func TestNoSymbolStandsForTwoThings(t *testing.T) {
	seen := map[string]string{}
	add := func(g, what string) {
		if prev, ok := seen[g]; ok {
			t.Errorf("%q stands for %s and %s", g, prev, what)
		}
		seen[g] = what
	}
	for _, e := range States {
		add(e.Glyph, "state "+string(e.State))
	}
	for _, e := range Kinds {
		add(e.Glyph, "decision "+string(e.Kind))
	}
	for _, p := range [][2]string{
		{ForState(domain.RunFailed), Failure}, {ForState(domain.RunSleeping), Absent},
		{ForState(domain.RunActive), Success}, {ForState(domain.RunCompleted), Active},
		{ForKind(domain.DecisionPermission), Warning}, {Selected, Inactive},
		{SelectedMulti, UnselectedMulti}, {SelectedMulti, Selected},
	} {
		if p[0] == p[1] {
			t.Errorf("%q stands for two things", p[0])
		}
	}
	if ForState(domain.RunFailed) != "⊗" || ForState(domain.RunSleeping) != "◌" {
		t.Error("⊗ is the run that stopped on a failure and ◌ the sleeping run")
	}
}

// U-09: the badge counters are the glyphs of the active and waiting states.
func TestBadgeUsesStateGlyphs(t *testing.T) {
	if BadgeRunning != ForState(domain.RunActive) || BadgeWaiting != ForState(domain.RunWaiting) {
		t.Errorf("badge %q %q, states %q %q", BadgeRunning, BadgeWaiting, ForState(domain.RunActive), ForState(domain.RunWaiting))
	}
	if Checkpoint != ForKind(domain.DecisionCheckpoint) {
		t.Error("Checkpoint is the glyph of the checkpoint decision")
	}
}

// The table of meanings: the symbols agreed on for the whole product.
func TestMeaningTable(t *testing.T) {
	want := map[string]string{
		"success": "✔", "failure": "✗", "inactive": "○", "absent": "–", "warning": "⚠", "active": "●",
		"current": "◆", "modified": "✎", "default": "★", "cursor": "▸", "selected": "◉",
		"selected-multi": "☑", "unselected-multi": "☐",
	}
	if len(Symbols) != len(want) {
		t.Fatalf("%d symbols in the table, %d expected", len(Symbols), len(want))
	}
	seen := map[string]string{}
	for _, s := range Symbols {
		if want[s.Name] != s.Glyph {
			t.Errorf("symbol %s: %q, want %q", s.Name, s.Glyph, want[s.Name])
		}
		if prev, ok := seen[s.Glyph]; ok {
			t.Errorf("%q stands for %s and %s", s.Glyph, prev, s.Name)
		}
		seen[s.Glyph] = s.Name
	}
}

// A state or a kind shares a symbol with the table of meanings only when it
// is that meaning: a finished run is a success, a failed decision a failure,
// a live run is active, an idle one is inactive. Nothing else (a state may
// not take ◆ or ⚠ for itself).
func TestStatesShareOnlyTheirMeaning(t *testing.T) {
	shared := map[string]string{}
	for _, s := range Symbols {
		shared[s.Glyph] = s.Name
	}
	allowed := map[string]string{
		string(domain.RunActive): "active", string(domain.RunIdle): "inactive",
		string(domain.RunCompleted): "success", string(domain.DecisionError): "failure",
	}
	for _, e := range States {
		if name, ok := shared[e.Glyph]; ok && allowed[string(e.State)] != name {
			t.Errorf("state %s uses %q, the symbol of %q", e.State, e.Glyph, name)
		}
	}
	for _, e := range Kinds {
		if name, ok := shared[e.Glyph]; ok && allowed[string(e.Kind)] != name {
			t.Errorf("decision %s uses %q, the symbol of %q", e.Kind, e.Glyph, name)
		}
	}
}

// The events of a team: one row per type, a symbol of the table of meanings
// only for its own meaning (success ✔, failure ✗, inactive ○, warning ⚠,
// active ●, modified ✎), no symbol shared with a run state or a decision kind
// except the ones that mean the same thing.
func TestEventsTable(t *testing.T) {
	want := map[string]string{
		"claim.taken": "●", "claim.released": "○", "claim.transferred": "→", "claim.conflict": "✗",
		"session.complete": "✔", "review.ready": "◐", "review.approved": "✔", "review.rejected": "✗",
		"audit.finding": "⚠", "wiki.proposal": "✎", "wiki.accepted": "✔", "wiki.rejected": "✗",
		"workflow.published": "✔", "workflow.restored": "↺", "workflow.archived": "○",
	}
	if len(Events) != len(want) {
		t.Fatalf("%d events in the table, %d expected", len(Events), len(want))
	}
	seen := map[string]bool{}
	for _, e := range Events {
		if seen[e.Type] {
			t.Errorf("event %s listed twice", e.Type)
		}
		seen[e.Type] = true
		if want[e.Type] != e.Glyph {
			t.Errorf("event %s: %q, want %q", e.Type, e.Glyph, want[e.Type])
		}
		if e.Tone == "" {
			t.Errorf("event %s has no tone", e.Type)
		}
		if got := ForEvent(e.Type); got != e {
			t.Errorf("ForEvent(%s) = %v", e.Type, got)
		}
	}

	// Only ◐ is shared with a state, and it means the same: a person must decide.
	states := map[string]string{}
	for _, s := range States {
		states[s.Glyph] = string(s.State)
	}
	for _, e := range Events {
		if st, ok := states[e.Glyph]; ok && e.Glyph != Active && e.Glyph != Inactive && e.Glyph != Success && e.Glyph != BadgeWaiting {
			t.Errorf("event %s uses %q, the glyph of the run state %s", e.Type, e.Glyph, st)
		}
	}
	for _, k := range Kinds {
		for _, e := range Events {
			if e.Glyph == k.Glyph && e.Glyph != Failure {
				t.Errorf("event %s uses %q, the glyph of the decision %s", e.Type, e.Glyph, k.Kind)
			}
		}
	}
	// An event that is not in the table keeps its type and gets a neutral dot.
	u := ForEvent("nope.nothing")
	if u.Glyph != UnknownEvent || u.Type != "nope.nothing" || u.Tone != ToneMuted {
		t.Errorf("unknown event: %v", u)
	}
}
