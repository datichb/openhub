package shell

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// responsive breakpoints
const (
	breakpointFull     = 120 // full layout
	breakpointCompact  = 100 // truncated menu labels
	breakpointMinimal  = 80  // hidden menu
)

// handleResize checks terminal dimensions and adjusts layout.
// Called from SetBeforeDrawFunc or manually on window resize events.
func (s *Shell) handleResize(screen tcell.Screen) {
	w, _ := screen.Size()

	switch {
	case w >= breakpointFull:
		s.setMenuVisible(true)
		s.header.SetBreadcrumb(s.currentBreadcrumb())
	case w >= breakpointCompact:
		s.setMenuVisible(true)
		s.header.SetBreadcrumb(s.currentBreadcrumbShort())
	case w >= breakpointMinimal:
		s.setMenuVisible(false)
		s.header.SetBreadcrumb(s.currentBreadcrumbShort())
	default:
		s.setMenuVisible(false)
		s.header.SetBreadcrumb("")
	}
}

func (s *Shell) setMenuVisible(visible bool) {
	// The menu visibility is controlled by the middle flex item proportions.
	// When hidden, we use Ctrl+N to show as overlay (already handled via toggle focus).
	// For now, we adjust the border to indicate hidden state.
	if !visible {
		s.menu.Primitive().(*tview.TreeView).SetBorderColor(theme.BgPanel)
	} else {
		s.menu.Primitive().(*tview.TreeView).SetBorderColor(theme.BorderNormal)
	}
}

func (s *Shell) currentBreadcrumb() string {
	if cur := s.router.Current(); cur != nil {
		return cur.Title()
	}
	return ""
}

func (s *Shell) currentBreadcrumbShort() string {
	if cur := s.router.Current(); cur != nil {
		title := cur.Title()
		if len(title) > 15 {
			return title[:12] + "..."
		}
		return title
	}
	return ""
}
