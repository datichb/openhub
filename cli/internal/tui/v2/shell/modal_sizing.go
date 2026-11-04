package shell

import (
	"strings"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Modal sizing — content-aware dimension computation
// ─────────────────────────────────────────────────────────────────────────────

// modalSize holds the computed grid dimensions for an overlay.
type modalSize struct {
	cols []int // 3 values for the 3x3 grid columns
	rows []int // 3 values for the 3x3 grid rows
}

// modalSizeHint tells the sizing algorithm what content shape to expect.
type modalSizeHint struct {
	// Content metrics — set whichever is relevant.
	ContentLines int // estimated lines of text content (scrollable modals)
	OptionCount  int // number of list options (select modals)
	FieldCount   int // number of form fields (form modals)
	HasButtons   bool
	ButtonCount  int

	// ContentWidth is the width of the widest row of the content (a list:
	// its longest label), border and padding excluded. The window is as wide as
	// the rows need, from ModalMinWidth to the width of the screen less 4: a
	// row is only ellipsized on a screen too narrow for it.
	ContentWidth int
	// FrameWidth is the width that the title and the hints line need, border
	// included. It widens a window sized by ContentWidth or WidthPct.
	FrameWidth int
	// WidthPct sizes the window as a share of the screen (forms and inputs),
	// from ModalMaxWidth to ModalFormMaxWidth, then to the width of the screen
	// less 4.
	WidthPct int

	// Fixed dimensions — set to override auto-sizing.
	FixedWidth  int // >0 = fixed column count, 0 = auto
	FixedHeight int // >0 = fixed row count, 0 = auto

	// Proportional width weight — set for proportional sizing (e.g. -3 = 60%, -4 = 67%).
	// 0 = use auto fixed-width logic. Negative = proportional grid weight.
	ProportionalWidth int
}

// termSize returns the terminal dimensions from the shell's pages root.
// Falls back to 80x24 if dimensions cannot be determined.
func (s *Shell) termSize() (width, height int) {
	_, _, w, h := s.pages.GetInnerRect()
	if w <= 0 || h <= 0 {
		return 80, 24
	}
	return w, h
}

// computeModalSize calculates the optimal grid dimensions for a modal overlay.
func (s *Shell) computeModalSize(hint modalSizeHint) modalSize {
	termW, termH := s.termSize()

	width := computeWidth(hint, termW)
	height := computeHeight(hint, termH)

	return modalSize{
		cols: buildGridCols(width, termW),
		rows: buildGridRows(height, termH),
	}
}

// ── Width computation ────────────────────────────────────────────────────────

func computeWidth(hint modalSizeHint, termW int) int {
	// Fixed override
	if hint.FixedWidth > 0 {
		return clamp(hint.FixedWidth, theme.ModalMinWidth, termW-4)
	}

	// Rows: as wide as the widest row, plus the border and the padding
	if hint.ContentWidth > 0 {
		return fitWidth(max(hint.ContentWidth+modalChromeWidth, hint.FrameWidth), termW)
	}

	// Share of the screen, bounded: forms and inputs
	if hint.WidthPct > 0 {
		w := clamp(termW*hint.WidthPct/100, theme.ModalMaxWidth, theme.ModalFormMaxWidth)
		return fitWidth(max(w, hint.FrameWidth), termW)
	}

	// Proportional — returns negative value for grid weight
	if hint.ProportionalWidth < 0 {
		return hint.ProportionalWidth
	}

	// Auto: use MaxWidth but respect terminal
	w := theme.ModalMaxWidth
	if w > termW-4 {
		w = termW - 4
	}
	if w < theme.ModalMinWidth {
		w = theme.ModalMinWidth
	}
	return w
}

// modalChromeWidth is what a window adds to its rows: the two borders and the
// padding of each side.
const modalChromeWidth = 2 + 2*theme.ModalContentPadX

// fitWidth bounds a window width to [ModalMinWidth, termW-4]; the screen wins
// when it is narrower than the minimum.
func fitWidth(w, termW int) int {
	w = max(w, theme.ModalMinWidth)
	return max(min(w, termW-4), 1)
}

// frameWidth is the width that a window needs to show its title (padded) and
// its hints line (indented by two), borders included.
func frameWidth(title, hints string) int {
	w := 0
	if title != "" {
		w = tview.TaggedStringWidth(title) + 2*theme.ModalTitlePad
	}
	if hints != "" {
		w = max(w, tview.TaggedStringWidth(hints)+2)
	}
	return w + 2
}

// fitTitle ellipsizes a title that the border of the window is too narrow for
// (a screen narrower than the title); a window sized in proportion is left
// alone.
func fitTitle(title string, size modalSize) string {
	if w := size.cols[1]; w > 0 && title != "" {
		return widgets.TruncateTagged(title, w-2-2*theme.ModalTitlePad)
	}
	return title
}

// ── Height computation ───────────────────────────────────────────────────────

func computeHeight(hint modalSizeHint, termH int) int {
	maxH := termH * theme.ModalMaxHeightPct / 100
	if maxH < theme.ModalMinHeight {
		maxH = theme.ModalMinHeight
	}

	// Fixed override
	if hint.FixedHeight > 0 {
		return clamp(hint.FixedHeight, theme.ModalMinHeight, maxH)
	}

	// Select modal: content-based with cap
	if hint.OptionCount > 0 {
		h := hint.OptionCount + 2 // +2 for border
		if hint.HasButtons {
			h += theme.ModalButtonBarHeight
		}
		maxItems := theme.ModalSelectMaxItems + 2
		if h > maxItems {
			h = maxItems
		}
		return clamp(h, theme.ModalMinHeight, maxH)
	}

	// Scrollable modal: adapt to content
	if hint.ContentLines > 0 {
		h := hint.ContentLines + 2 // +2 for border
		if hint.HasButtons {
			h += theme.ModalButtonBarHeight
		}
		h += 1 // hints line
		// Minimum readable size
		if h < 10 {
			h = 10
		}
		return clamp(h, theme.ModalMinHeight, maxH)
	}

	// Form modal: estimate height from field count
	if hint.FieldCount > 0 {
		h := hint.FieldCount*3 + 4 + theme.ModalHintsHeight // ~3 rows per field + border + buttons + hints
		return clamp(h, theme.ModalMinHeight, maxH)
	}

	// Fallback: proportional (for scrollable with unknown content)
	return -4 // proportional weight
}

// ── Grid construction ────────────────────────────────────────────────────────

// buildGridCols constructs the 3-element column spec for overlayGrid.
func buildGridCols(width, termW int) []int {
	if width < 0 {
		// Proportional: e.g. {0, -4, 0}
		return []int{0, width, 0}
	}
	// Fixed: center the modal
	return []int{0, width, 0}
}

// buildGridRows constructs the 3-element row spec for overlayGrid.
func buildGridRows(height, termH int) []int {
	if height < 0 {
		// Proportional: e.g. {1, -4, 1}
		return []int{1, height, 1}
	}
	// Fixed height: center vertically
	return []int{0, height, 0}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// countLines counts the number of visual lines in a string.
func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}
