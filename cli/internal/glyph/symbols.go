package glyph

// The table of the symbols that mean the same thing on every screen and in
// every command, besides the states of a run and the kinds of decision. The
// theme (internal/tui/theme, Sym* and Icon*) takes its values from here: a
// symbol is written once, and a screen reaches it through the theme.
//
// Colors are not part of the meaning of a symbol, but the table assumes
// them: ✔ is green, ✗ red, ⚠ yellow, ○ and – grey.
const (
	// Success is a thing that worked, passed or is valid (green).
	Success = "✔"
	// Failure is a thing that failed or is invalid (red).
	Failure = "✗"
	// Inactive is a thing that is off or at rest: a disabled option, an idle
	// session (grey). An empty or absent value is Absent, not Inactive.
	Inactive = "○"
	// Absent is a value that is not there: not configured, not set (grey).
	Absent = "–"
	// Warning is a thing that works but needs attention (yellow).
	Warning = "⚠"
	// Active is a thing that is running right now, and nothing else: a live
	// session, a running step. A completed step is Success, not Active.
	Active = "●"
	// Current is the element the user is in, or the one in use: the current
	// view, the current step, the current team.
	Current = "◆"
	// Modified is an element changed and not saved or published: a draft, a
	// local edit.
	Modified = "✎"
	// Default is the element chosen when nothing is chosen: the default
	// provider, the default workflow, a pinned entry.
	Default = "★"
	// Cursor is the row under the cursor of a list, and the start of a
	// choice or an action (never a state).
	Cursor = "▸"
	// Selected is an element picked in a choice, single (radio, paired with
	// Inactive for the others) or multiple.
	Selected = "◉"
)

// Symbol is one row of the table of meanings.
type Symbol struct {
	Name  string
	Glyph string
}

// Symbols lists the symbols of the table of meanings, in the order of the
// legend. A symbol stands for one thing (see TestSymbolsAreUnique).
var Symbols = []Symbol{
	{"success", Success},
	{"failure", Failure},
	{"inactive", Inactive},
	{"absent", Absent},
	{"warning", Warning},
	{"active", Active},
	{"current", Current},
	{"modified", Modified},
	{"default", Default},
	{"cursor", Cursor},
	{"selected", Selected},
}
