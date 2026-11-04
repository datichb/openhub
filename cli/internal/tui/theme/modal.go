package theme

// ─────────────────────────────────────────────────────────────────────────────
// Modal design constants — shared by all modal types in the TUI shell
// ─────────────────────────────────────────────────────────────────────────────

// Modal width constraints (in columns).
const (
	// ModalMinWidth is the smallest usable modal width.
	ModalMinWidth = 40
	// ModalMaxWidth is the width of a form or an input on a standard screen: the
	// floor of their proportional width (ModalWidthPct), not a cap.
	ModalMaxWidth = 72
	// ModalWidthPct is the share of the screen that a form or an input takes.
	ModalWidthPct = 70
	// ModalFormMaxWidth is the widest a form or an input gets, however wide the
	// screen: the fields have nothing to gain beyond.
	ModalFormMaxWidth = 120
)

// Modal height constraints.
const (
	// ModalMinHeight is the smallest usable modal height (border + padding + 1 content line + hints).
	ModalMinHeight = 9
	// ModalMaxHeightPct is the maximum percentage of terminal height a modal may occupy.
	ModalMaxHeightPct = 85
	// ModalSelectMaxItems is the maximum number of visible items in a select list
	// before scrolling kicks in.
	ModalSelectMaxItems = 14
)

// Modal inner spacing.
const (
	// ModalTitlePad is the number of spaces on each side of the title text in the border.
	ModalTitlePad = 2
	// ModalContentPadX is the horizontal inner padding (left/right) for text content.
	ModalContentPadX = 2
	// ModalContentPadY is the vertical inner padding (top/bottom) for breathing room.
	ModalContentPadY = 1
	// ModalButtonBarHeight is the number of rows reserved for the button bar (separator + buttons + spacing).
	ModalButtonBarHeight = 3
	// ModalButtonPadX is the padding inside each button (each side of the label).
	ModalButtonPadX = 2
	// ModalButtonGap is the gap between adjacent buttons.
	ModalButtonGap = 2
	// ModalHintsHeight is the number of rows for the bottom hints bar.
	ModalHintsHeight = 1
)

// ModalSeparatorRune is the character drawn as a horizontal separator between content and buttons.
const ModalSeparatorRune = '─'
