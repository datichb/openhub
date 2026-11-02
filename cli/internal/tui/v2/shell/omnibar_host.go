package shell

import (
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The shell's side of the omnibar: the suggestions overlay, its visibility,
// the mode bar and the hints line.

// showSuggestionsInLayout adds the suggestions list as a partial overlay
// (resize=false) so it floats just above the omnibar without pushing the content.
func (s *Shell) showSuggestionsInLayout() {
	if s.suggestionsShown {
		return
	}
	s.suggestionsShown = true
	// resize=false: tview will not auto-resize this page — we control placement via SetRect.
	s.pages.AddPage("suggestions-overlay", s.omnibar.SuggestionsPrimitive(), false, true)
	s.repositionSuggestions()
}

// hideSuggestionsFromLayout removes the suggestions overlay.
func (s *Shell) hideSuggestionsFromLayout() {
	if !s.suggestionsShown {
		return
	}
	s.suggestionsShown = false
	s.pages.RemovePage("suggestions-overlay")
}

// updateSuggestionsHeight recalculates and repositions the suggestions overlay.
func (s *Shell) updateSuggestionsHeight() {
	if !s.suggestionsShown {
		return
	}
	s.repositionSuggestions()
}

// repositionSuggestions calculates and applies the exact screen position of
// the suggestions widget. It spans the full width, sits just above the omnibar,
// and never covers more than (screenH - omnibarHeight) rows.
func (s *Shell) repositionSuggestions() {
	// GetInnerRect returns (x, y, width, height) of the pages container.
	_, _, w, screenH := s.pages.GetInnerRect()
	if w == 0 || screenH == 0 {
		// Layout not yet computed (first draw) — let tview call us again.
		return
	}

	height := s.omnibar.SuggestionsHeight()
	omnibarHeight := s.omnibar.Height()

	y := screenH - omnibarHeight - height
	if y < 0 {
		y = 0
		height = screenH - omnibarHeight
	}
	if height < 1 {
		return
	}

	s.omnibar.SuggestionsList().SetRect(0, y, w, height)
}

// SetOmnibarVisible shows or hides the bottom omnibar (including mode bar
// and gutter). When hidden, the 6 rows are reclaimed for the content area.
// The omnibar is deactivated before hiding to ensure suggestions are
// dismissed and focus is properly restored.
func (s *Shell) SetOmnibarVisible(visible bool) {
	if !visible {
		s.omnibar.Deactivate()
	}
	s.omnibarHidden = !visible
	if visible {
		s.root.ResizeItem(s.omnibar.Primitive(), s.omnibar.Height(), 0)
	} else {
		s.root.ResizeItem(s.omnibar.Primitive(), 0, 0)
	}
}

// updateModeBar refreshes the mode bar content based on the current view and
// active mode. If the view implements ModeInfoProvider, its ModeInfo() is used;
// otherwise a default is built from the shell's mode/project/team state.
func (s *Shell) updateModeBar(v views.View) {
	// If the view supplies its own mode info, use it.
	if mp, ok := v.(views.ModeInfoProvider); ok {
		info := mp.ModeInfo()
		info.Badge, info.BadgeAlert = s.badge, s.badgeAlert
		s.omnibar.UpdateModeBar(s.activeMode, info)
		return
	}

	// Fallback: build default info from shell state.
	var info views.ModeBarInfo
	switch s.activeMode {
	case views.ModeHub:
		info = views.ModeBarInfo{Icon: "🏠", Label: "Hub"}
	case views.ModeTeam:
		label := "Team"
		if s.activeTeam != nil {
			label = s.activeTeam.Name
		}
		info = views.ModeBarInfo{Icon: "👥", Label: label}
	case views.ModeProject:
		label := "Project"
		if s.activeProject != nil {
			label = s.activeProject.Name
		}
		info = views.ModeBarInfo{Icon: "💻", Label: label}
	}
	info.Badge, info.BadgeAlert = s.badge, s.badgeAlert
	s.omnibar.UpdateModeBar(s.activeMode, info)
}

// RefreshModeBar redraws the mode bar from the current view (a view that got
// data its ModeInfo shows). Must be called on the event loop.
func (s *Shell) RefreshModeBar() {
	if cur := s.router.Current(); cur != nil {
		s.updateModeBar(cur)
	}
}

// SetSessionsBadge sets the sessions badge of the mode bar ("● 2 ◐ 1", ""
// to hide it). Must be called on the event loop.
func (s *Shell) SetSessionsBadge(badge string, alert bool) {
	s.badge, s.badgeAlert = badge, alert
	if cur := s.router.Current(); cur != nil {
		s.updateModeBar(cur)
	}
}

// RefreshHints shows the hints line of the current view again (its keys
// changed: a tab, the batch).
func (s *Shell) RefreshHints() {
	if cur := s.router.Current(); cur != nil {
		s.omnibar.SetHints(cur.StatusHints())
	}
}
