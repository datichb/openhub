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
// checkpoint, ✗ three meanings were the ambiguities).
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
