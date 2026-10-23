package theme

import "github.com/datichb/openhub/cli/internal/glyph"

// ─────────────────────────────────────────────────────────────────────────────
// Table of meanings — the symbols that mean the same thing on every screen
// ─────────────────────────────────────────────────────────────────────────────
//
// The values come from internal/glyph (the table shared with the CLI and the
// sessions): the theme names them, glyph owns them. A screen writes
// theme.SymSuccess, never "✔". The colors go with the meaning (✔ green, ✗ red,
// ⚠ yellow, ○ and – grey) and come from the theme: Success, Error, Warning,
// FgMuted.
//
// A symbol stands for one thing. Where an Icon* below still has another
// value for the same meaning, it says so (the screens are moved to the table
// group by group).
//
// Two failures: SymFailure ✗ is a one-off error (a refused input, a failed
// check, glyph.Kinds DecisionError); a session that stopped on a failure is
// ⊗ (glyph.ForState(domain.RunFailed)). The table of states is in glyph.

const (
	// SymSuccess: it worked, it passed, it is valid (green).
	SymSuccess = glyph.Success
	// SymFailure: it failed, it is invalid (red).
	SymFailure = glyph.Failure
	// SymInactive: off or at rest — a disabled option, an idle session, a
	// step not started (grey). A missing value is SymAbsent.
	SymInactive = glyph.Inactive
	// SymAbsent: the value is not there — not configured, not set (grey).
	SymAbsent = glyph.Absent
	// SymWarning: it works but needs attention (yellow).
	SymWarning = glyph.Warning
	// SymLive: running right now, and nothing else — a live session. A
	// completed step is SymSuccess.
	SymLive = glyph.Active
	// SymCurrent: the element the user is in or that is in use — current
	// view, current step, current team.
	SymCurrent = glyph.Current
	// SymModified: changed and not saved or published — a draft, a local edit.
	SymModified = glyph.Modified
	// SymDefault: what is chosen when nothing is — the default provider or
	// workflow, a pinned entry.
	SymDefault = glyph.Default
	// SymCursor: the row under the cursor of a list, the start of a choice or
	// of an action. Never a state.
	SymCursor = glyph.Cursor
	// SymSelected: the element picked in a single choice (radio); the others
	// are SymUnselected.
	SymSelected = glyph.Selected
	// SymUnselected: an element not picked in a single choice (radio). It is
	// the symbol of SymInactive: ○ is the same "not on" in a choice.
	SymUnselected = glyph.Inactive
	// SymSelectedMulti and SymUnselectedMulti: an element ticked / not ticked
	// in a multiple choice (checkbox).
	SymSelectedMulti   = glyph.SelectedMulti
	SymUnselectedMulti = glyph.UnselectedMulti
)

// ─────────────────────────────────────────────────────────────────────────────
// Icons (deduplicated, single source of truth for the screens)
// ─────────────────────────────────────────────────────────────────────────────

const (
	// IconActive marks the currently active view/item (SymCurrent).
	IconActive = SymCurrent
	// IconDone marks a completed step (SymSuccess). ● is SymLive: running
	// now, never completed.
	IconDone = SymSuccess
	// IconPending marks a future/pending step (SymInactive).
	IconPending = SymInactive
	// IconSkipped marks a skipped step (SymAbsent). ◌ is a sleeping session.
	IconSkipped = SymAbsent
	// IconSuccess is a checkmark for success feedback (SymSuccess).
	IconSuccess = SymSuccess
	// IconError is a cross for error feedback (SymFailure).
	IconError = SymFailure
	// IconWarning marks a warning (SymWarning). ⚠ has an ambiguous width in
	// East Asian terminals: measure a column with runewidth or
	// textfmt.ClipWidth, never len.
	IconWarning = SymWarning
	// IconInfo is an info indicator.
	IconInfo = "ℹ"
	// IconArrow is a pointer/expanded indicator (SymCursor).
	IconArrow = SymCursor
	// IconCollapsed is a collapsed category indicator.
	IconCollapsed = "▾"
	// IconDot is a bullet for menu items.
	IconDot = "·"
	// IconConnector is a step bar connector.
	IconConnector = "───"
	// IconGutter is the active item left gutter.
	IconGutter = "▎"
	// IconStepActive marks the active step in a wizard.
	IconStepActive = SymCurrent
	// IconSepBold is a bold separator.
	IconSepBold = "━"
	// IconSepNormal is a normal separator.
	IconSepNormal = "─"
	// IconSepConn is a connector separator.
	IconSepConn = "┃"
)

// Workflow catalogue pictograms (catalogue list, legend, sessions, board).
const (
	// IconDraft marks a draft (leading) or a published workflow that has a
	// draft (trailing).
	IconDraft = SymModified
	// IconPinned marks a pinned workflow.
	IconPinned = SymDefault
	// IconRuntimeLocal, IconRuntimeContainer and IconRuntimeRemote are the
	// runtimes (local, container, remote).
	IconRuntimeLocal     = "⌂"
	IconRuntimeContainer = "▣"
	IconRuntimeRemote    = "☁"
	// IconValid marks a valid workflow.
	IconValid = SymSuccess
	// IconInvalid marks a workflow with errors.
	IconInvalid = SymFailure
	// IconWarn marks validation or integrity warnings.
	IconWarn = SymWarning
	// IconQueued marks an operation waiting for the network.
	IconQueued = "⏳"
	// IconNewBrick marks a draft that introduces a team brick.
	IconNewBrick = "+"
	// IconExtends introduces the `extends` chain of a workflow.
	IconExtends = "↳"
)

// RuntimeIcon returns the pictogram of a runtime ("" = local); an unknown
// runtime is returned as is.
func RuntimeIcon(rt string) string {
	switch rt {
	case "local", "":
		return IconRuntimeLocal
	case "container":
		return IconRuntimeContainer
	case "remote":
		return IconRuntimeRemote
	}
	return rt
}
