// Package glyph is the single table of the symbols of oh sessions: one
// symbol per run state and per decision kind, shared by the CLI
// (oh session list|inbox) and the TUI (Sessions view, badge, home sections).
// The legend is in docs/reference/cli.*.md (U-09).
//
// A symbol never stands for two things: the states use geometric shapes, the
// decision kinds use signs (⏸ ? ! $ ✗ ⊘), and a test checks that no symbol is
// shared between the two tables.
package glyph

import "github.com/datichb/openhub/cli/internal/domain"

// State is one row of the table of run states.
type State struct {
	State domain.RunState
	Glyph string
}

// States lists the glyph of every run state, in lifecycle order.
var States = []State{
	{domain.RunPreparing, "◓"},
	{domain.RunQueued, "…"},
	{domain.RunActive, "●"},
	{domain.RunWaiting, "◐"}, // waits for a decision of the user
	{domain.RunIdle, "○"},
	{domain.RunSleeping, "◌"},
	{domain.RunCompleted, "✔"},
	{domain.RunFailed, "⊗"},
	{domain.RunStopped, "■"},
}

// Kind is one row of the table of decision kinds.
type Kind struct {
	Kind  domain.DecisionKind
	Glyph string
}

// Kinds lists the glyph of every decision kind, in display order.
var Kinds = []Kind{
	{domain.DecisionCheckpoint, "⏸"},
	{domain.DecisionQuestion, "?"},
	{domain.DecisionPermission, "!"},
	{domain.DecisionBudget, "$"},
	{domain.DecisionError, "✗"},
	{domain.DecisionCircuit, "⊘"},
}

const (
	// Unknown is the glyph of a state or kind that is not in the tables.
	UnknownState = "·"
	UnknownKind  = "•"

	// Checkpoint is the glyph of a paused checkpoint, in a decision list or
	// in the timeline of a session.
	Checkpoint = "⏸"
	// BadgeRunning and BadgeWaiting are the two counters of the sessions
	// badge (« ● 2  ◐ 1 »): live sessions, and sessions that wait for a
	// decision. They are the glyphs of the active and waiting states.
	BadgeRunning = "●"
	BadgeWaiting = "◐"
)

// ForState is the glyph of a run state.
func ForState(s domain.RunState) string {
	for _, e := range States {
		if e.State == s {
			return e.Glyph
		}
	}
	return UnknownState
}

// ForKind is the glyph of a decision kind.
func ForKind(k domain.DecisionKind) string {
	for _, e := range Kinds {
		if e.Kind == k {
			return e.Glyph
		}
	}
	return UnknownKind
}
