// Package keys is the key convention of the TUI (modernisation tui-keys):
// one key per action, the same in every view, with its translated label.
// Views match events with Action.Matches and build their hints line and
// their help from the same values (Hint, Hints, HelpRows), so what a view
// does and what it shows cannot drift apart.
//
// Contexts:
//   - Everywhere: ↑↓/jk move, g/G first/last, / filter, r refresh, ? help,
//     Enter open, Esc back;
//   - List: n create, e edit, d delete (always confirmed), x archive or file
//     away (never destructive), R resume or restore, c compare (diff);
//   - Decision (checkpoint card, wiki proposal, element of a batch): y
//     accept, n refuse (confirmed, with an instruction), e modify, s leave
//     pending, A accept all;
//   - Reader (trace, reports): / search, n/N next/previous match.
//
// A view may bind other letters to its own actions (a attach, t transfer…)
// but never gives a key of the convention another meaning, nor an action of
// the convention another key. TestKeyConvention checks the hints lines.
// See docs/dev/tui-conventions.md.
package keys

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// Context groups the actions of the convention.
type Context string

const (
	// Everywhere: the keys of every view.
	Everywhere Context = "everywhere"
	// List: the lists of objects (sessions, workflows, projects…).
	List Context = "list"
	// Decision: something to accept or refuse (checkpoint card, wiki
	// proposal, element of a batch).
	Decision Context = "decision"
	// Reader: the text readers (trace, reports).
	Reader Context = "reader"
)

// Contexts are the contexts in display order (help, docs, notice).
func Contexts() []Context { return []Context{Everywhere, List, Decision, Reader} }

// Title is the translated name of the context.
func (c Context) Title() string { return i18n.T("tui.keys.context." + string(c)) }

// Binding is one key of an action: a letter (Key = tcell.KeyRune) or a
// special key (Enter, Esc, arrows…).
type Binding struct {
	Key  tcell.Key
	Rune rune
}

// Rune is the binding of a letter.
func Rune(r rune) Binding { return Binding{Key: tcell.KeyRune, Rune: r} }

// Special is the binding of a special key.
func Special(k tcell.Key) Binding { return Binding{Key: k} }

// Action is an action of the convention.
type Action struct {
	// ID identifies the action (stable, used by the tests and the docs).
	ID string
	// Context is the context of the action.
	Context Context
	// Bindings are the keys of the action; the first one is the main one.
	Bindings []Binding
	// display is the key as shown in hints ("d", "g/G", "↑↓/jk"); nameKey,
	// when set, is the i18n key of a key name (Entrée / Enter).
	display string
	nameKey string
	// labelKey is the i18n key of the short label ("supprimer").
	labelKey string
	// Confirm: the action always asks a confirmation (d delete, n refuse:
	// the instruction typed then validated is the confirmation).
	Confirm bool
	// Reason: the action asks an instruction (n refuse).
	Reason bool
}

// Matches reports whether ev is one of the keys of the action. Letters
// match without Ctrl or Alt (Shift is the case of the letter).
func (a Action) Matches(ev *tcell.EventKey) bool {
	if ev == nil {
		return false
	}
	for _, b := range a.Bindings {
		if b.Key != ev.Key() {
			continue
		}
		if b.Key != tcell.KeyRune {
			return true
		}
		if ev.Rune() == b.Rune && ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) == 0 {
			return true
		}
	}
	return false
}

// Keys is the key as displayed ("d", "Entrée", "g/G").
func (a Action) Keys() string {
	if a.nameKey != "" {
		return i18n.T(a.nameKey)
	}
	return a.display
}

// Label is the translated short label of the action ("supprimer").
func (a Action) Label() string { return i18n.T(a.labelKey) }

// LabelKey is the i18n key of the label.
func (a Action) LabelKey() string { return a.labelKey }

// Hint is the action in a hints line: "d supprimer".
func (a Action) Hint() string { return a.Keys() + " " + a.Label() }

// Description is the translated description of the action for the help
// ("supprimer (toujours confirmé)").
func (a Action) Description() string { return i18n.T("tui.keys.describe." + a.ID) }

// HintAs is the action with a label of the view (already translated), for
// a more precise wording of the same action: "d supprimer la session",
// "R récupérer". The label must keep the meaning of the action.
func (a Action) HintAs(label string) string { return a.Keys() + " " + label }

// Rune is the letter of the main binding (0 for a special key).
func (a Action) Rune() rune {
	if len(a.Bindings) == 0 || a.Bindings[0].Key != tcell.KeyRune {
		return 0
	}
	return a.Bindings[0].Rune
}

// Hints joins the parts of a hints line with " · ", skipping empty ones.
// Parts are Action.Hint/HintAs values or texts of the view.
func Hints(parts ...string) string {
	out := parts[:0:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, Separator)
}

// HintsOf is the hints line of the actions, in order.
func HintsOf(actions ...Action) string {
	parts := make([]string, len(actions))
	for i, a := range actions {
		parts[i] = a.Hint()
	}
	return Hints(parts...)
}

// Separator separates the parts of a hints line.
const Separator = " · "

// HelpRow is a line of the help: the keys and what they do.
type HelpRow struct {
	Keys  string
	Label string
}

// HelpRows are the lines of the help for a context. Composite entries
// (Navigate, Ends) replace the single moves in the Everywhere context.
func HelpRows(c Context) []HelpRow {
	var out []HelpRow
	for _, a := range helpActions(c) {
		out = append(out, HelpRow{Keys: a.Keys(), Label: a.Description()})
	}
	return out
}

func helpActions(c Context) []Action {
	if c == Everywhere {
		return []Action{Navigate, Ends, Filter, Refresh, Help, Open, Back}
	}
	return ByContext(c)
}

// ByContext lists the actions of a context, in display order.
func ByContext(c Context) []Action {
	var out []Action
	for _, a := range All() {
		if a.Context == c {
			out = append(out, a)
		}
	}
	return out
}

// All lists every action of the convention (single moves included), in
// display order.
func All() []Action {
	return []Action{
		Up, Down, First, Last, Filter, Refresh, Help, Open, Back,
		Create, Edit, Delete, Archive, Resume, Compare,
		Accept, Refuse, Modify, Leave, AcceptAll,
		Search, NextMatch, PrevMatch,
	}
}

func letter(id string, c Context, r rune) Action {
	return Action{ID: id, Context: c, Bindings: []Binding{Rune(r)}, display: string(r), labelKey: "tui.keys.action." + id}
}

// Everywhere.
var (
	Up    = Action{ID: "up", Context: Everywhere, Bindings: []Binding{Special(tcell.KeyUp), Rune('k')}, display: "↑/k", labelKey: "tui.keys.action.up"}
	Down  = Action{ID: "down", Context: Everywhere, Bindings: []Binding{Special(tcell.KeyDown), Rune('j')}, display: "↓/j", labelKey: "tui.keys.action.down"}
	First = Action{ID: "first", Context: Everywhere, Bindings: []Binding{Rune('g'), Special(tcell.KeyHome)}, display: "g", labelKey: "tui.keys.action.first"}
	Last  = Action{ID: "last", Context: Everywhere, Bindings: []Binding{Rune('G'), Special(tcell.KeyEnd)}, display: "G", labelKey: "tui.keys.action.last"}
	// Filter filters the list of the view; without a filter, / opens the
	// palette (shell fallback).
	Filter  = letter("filter", Everywhere, '/')
	Refresh = letter("refresh", Everywhere, 'r')
	Help    = Action{ID: "help", Context: Everywhere, Bindings: []Binding{Rune('?'), Special(tcell.KeyF1)}, display: "?", labelKey: "tui.keys.action.help"}
	Open    = Action{ID: "open", Context: Everywhere, Bindings: []Binding{Special(tcell.KeyEnter)}, nameKey: "tui.keys.name.enter", labelKey: "tui.keys.action.open"}
	Back    = Action{ID: "back", Context: Everywhere, Bindings: []Binding{Special(tcell.KeyEscape)}, nameKey: "tui.keys.name.esc", labelKey: "tui.keys.action.back"}

	// Navigate (↑↓/jk) and Ends (g/G) are the composite entries of the
	// hints and the help.
	Navigate = Action{ID: "navigate", Context: Everywhere, Bindings: append(append([]Binding(nil), Up.Bindings...), Down.Bindings...), display: "↑↓/jk", labelKey: "tui.keys.action.navigate"}
	Ends     = Action{ID: "ends", Context: Everywhere, Bindings: append(append([]Binding(nil), First.Bindings...), Last.Bindings...), display: "g/G", labelKey: "tui.keys.action.ends"}
)

// List.
var (
	Create = letter("create", List, 'n')
	Edit   = letter("edit", List, 'e')
	// Delete always asks a confirmation (Shell.Confirm, danger).
	Delete = Action{ID: "delete", Context: List, Bindings: []Binding{Rune('d')}, display: "d", labelKey: "tui.keys.action.delete", Confirm: true}
	// Archive files the element away; never destructive (it can be
	// restored with R).
	Archive = letter("archive", List, 'x')
	// Resume resumes a session or restores a version, a remote session…
	Resume = letter("resume", List, 'R')
	// Compare shows a diff (draft of a workflow, version of the history).
	Compare = letter("compare", List, 'c')
)

// Decision.
var (
	Accept = letter("accept", Decision, 'y')
	// Refuse asks an instruction; validating it is the confirmation.
	Refuse    = Action{ID: "refuse", Context: Decision, Bindings: []Binding{Rune('n')}, display: "n", labelKey: "tui.keys.action.refuse", Confirm: true, Reason: true}
	Modify    = letter("modify", Decision, 'e')
	Leave     = letter("leave", Decision, 's')
	AcceptAll = letter("accept_all", Decision, 'A')
)

// Reader.
var (
	Search    = letter("search", Reader, '/')
	NextMatch = letter("next_match", Reader, 'n')
	PrevMatch = letter("prev_match", Reader, 'N')
)

// Change is a key changed by the convention (first-launch notice, help,
// docs).
type Change struct {
	From, To string
	// labelKey is the i18n key of what changed ("Sessions : supprimer").
	labelKey string
}

// Label is the translated description of the change.
func (c Change) Label() string { return i18n.T(c.labelKey) }

// LabelKey is the i18n key of the description.
func (c Change) LabelKey() string { return c.labelKey }

// Changes are the keys changed by the convention.
func Changes() []Change {
	return []Change{
		{From: "D", To: Delete.Keys(), labelKey: "tui.keys.change.sessions_delete"},
		{From: "c", To: Resume.Keys(), labelKey: "tui.keys.change.sessions_resume"},
		{From: "g", To: Resume.Keys(), labelKey: "tui.keys.change.sessions_recover"},
		{From: "r", To: Resume.Keys(), labelKey: "tui.keys.change.history_restore"},
		{From: "a", To: Accept.Keys(), labelKey: "tui.keys.change.wiki_accept"},
		{From: "x", To: Refuse.Keys(), labelKey: "tui.keys.change.wiki_refuse"},
		{From: "D", To: Compare.Keys(), labelKey: "tui.keys.change.workflows_diff"},
	}
}
