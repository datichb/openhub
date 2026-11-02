package glyph

// The table of the events of a team: the symbol and the tone of each event
// type of the team-state (internal/teamstate, Event.Type), the same in the CLI
// (oh team activity) and in the TUI (Activity, Team Status). A screen reaches
// it through ForEvent and paints the tone with its own palette; the phrase of
// the event is teamstate.Event.Describe.
//
// The types are the strings written in the events files, not the constants of
// teamstate, so that this package stays below it; a test of teamstate checks
// that every event it defines has its row.

// Tone is how an event is painted: the meaning of its color, not the color.
type Tone string

const (
	// ToneSuccess is something that went well (green).
	ToneSuccess Tone = "success"
	// ToneError is something that failed or was refused (red).
	ToneError Tone = "error"
	// ToneWarning is something that needs attention (yellow).
	ToneWarning Tone = "warning"
	// ToneInfo is a piece of news (blue).
	ToneInfo Tone = "info"
	// ToneMuted is something that ended or went away (grey).
	ToneMuted Tone = "muted"
)

// Symbols of the events that are not in the table of meanings (symbols.go).
const (
	// Transferred is a claim that went from a member to another.
	Transferred = "→"
	// Restored is a version put back (a workflow restored).
	Restored = "↺"
	// waiting is something that waits for a person to decide (a review that
	// is ready): the glyph of the waiting run state.
	waiting = BadgeWaiting
)

// EventGlyph is one row of the table of events.
type EventGlyph struct {
	// Type is the type of the event as written in the events files.
	Type  string
	Glyph string
	Tone  Tone
}

// UnknownEvent is the glyph of an event type that is not in the table.
const UnknownEvent = UnknownState

// Events lists the symbol of every event type of a team, in display order.
var Events = []EventGlyph{
	{"claim.taken", Active, ToneSuccess},
	{"claim.released", Inactive, ToneMuted},
	{"claim.transferred", Transferred, ToneInfo},
	{"claim.conflict", Failure, ToneError},
	{"session.complete", Success, ToneSuccess},
	{"review.ready", waiting, ToneInfo},
	{"review.approved", Success, ToneSuccess},
	{"review.rejected", Failure, ToneError},
	{"audit.finding", Warning, ToneWarning},
	{"wiki.proposal", Modified, ToneInfo},
	{"wiki.accepted", Success, ToneSuccess},
	{"wiki.rejected", Failure, ToneError},
	{"workflow.published", Success, ToneInfo},
	{"workflow.restored", Restored, ToneInfo},
	{"workflow.archived", Inactive, ToneMuted},
}

// ForEvent is the row of an event type; an unknown type gets a neutral dot
// (UnknownEvent, ToneMuted) and keeps its type.
func ForEvent(eventType string) EventGlyph {
	for _, e := range Events {
		if e.Type == eventType {
			return e
		}
	}
	return EventGlyph{Type: eventType, Glyph: UnknownEvent, Tone: ToneMuted}
}
