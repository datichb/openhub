package shell

import (
	"strings"

	"github.com/datichb/openhub/cli/internal/tui/theme"
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

	// Fixed dimensions — set to override auto-sizing.
	FixedWidth  int // >0 = fixed column count, 0 = auto
	FixedHeight int // >0 = fixed row count, 0 = auto

	// Proportional width weight — set for proportional sizing (e.g. -3 = 60%, -4 = 67%).
	// 0 = use auto fixed-width logic. Negative = proportional grid weight.
	ProportionalWidth int
}

// termSize returns the terminal dimensions from the shell's pages root.
// Falls back to 80x24 if dimensions cannot be determined.
func (s *Shell) termSize() (int, int) {
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
		h := hint.FieldCount*3 + 4 // ~3 rows per field + border + buttons
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

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
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
