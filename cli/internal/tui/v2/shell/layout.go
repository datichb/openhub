package shell

import (
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// Layout levels (N20): full ≥ 32 rows, compact 16–31 rows, too small under
// 60×16 (views.LevelFor). The level follows the terminal size at every draw;
// it is applied on the event loop, the landings rebuild themselves.
// ─────────────────────────────────────────────────────────────────────────────

// compactPref is the `[cli] compact` setting.
func (s *Shell) compactPref() string {
	if s.cfg.Compact == nil {
		return views.CompactAuto
	}
	return s.cfg.Compact()
}

// hasOverlay reports whether a window (modal, form, quit dialog) is shown.
func (s *Shell) hasOverlay() bool {
	return s.pages.HasPage("inline-overlay") || s.pages.HasPage("sub-overlay")
}

// beforeDraw computes the level of the terminal; a new level is applied on
// the next loop tick (the draw holds the application lock). Too small and
// no window open: only the message is drawn.
func (s *Shell) beforeDraw(screen tcell.Screen) bool {
	w, h := screen.Size()
	level := views.LevelFor(w, h, s.compactPref())
	if int32(level) != s.wantLevel.Load() {
		s.wantLevel.Store(int32(level))
		go s.app.QueueUpdateDraw(func() { s.applyLevel(views.LayoutLevel(s.wantLevel.Load())) })
	}
	s.tooSmall.Store(level == views.LevelTooSmall)
	if level == views.LevelTooSmall && !s.hasOverlay() {
		drawTooSmall(screen, w, h)
		return true
	}
	return false
}

// applyLevel sizes the omnibar and the content margin for level. Event
// loop only.
func (s *Shell) applyLevel(level views.LayoutLevel) {
	if level == s.level {
		return
	}
	s.level = level
	views.SetLayoutLevel(level)
	compact := level != views.LevelFull
	s.omnibar.SetCompact(compact)
	if !s.omnibarHidden {
		s.root.ResizeItem(s.omnibar.Primitive(), s.omnibar.Height(), 0)
	}
	top := 1
	if compact {
		top = 0
	}
	s.content.SetBorderPadding(top, 0, 2, 2)
	s.repositionSuggestions()
}

// Level returns the layout level applied to the shell.
func (s *Shell) Level() views.LayoutLevel { return s.level }

// drawTooSmall draws « terminal too small » centered on the screen.
func drawTooSmall(screen tcell.Screen, w, h int) {
	bg := tcell.StyleDefault.Background(theme.BgPanel)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			screen.SetContent(x, y, ' ', nil, bg)
		}
	}
	lines := []struct {
		text  string
		style tcell.Style
	}{
		{i18n.Tf("tui.compact.too_small", w, h, views.MinCols, views.MinRows), bg.Foreground(theme.Warning).Bold(true)},
		{i18n.T("tui.compact.quit_hint"), bg.Foreground(theme.FgSecondary)},
	}
	y := h/2 - len(lines)/2
	for i, l := range lines {
		row := y + i
		if row < 0 || row >= h {
			continue
		}
		width := runewidth.StringWidth(l.text)
		x := (w - width) / 2
		if x < 0 {
			x = 0
		}
		for _, r := range l.text {
			if x >= w {
				break
			}
			screen.SetContent(x, row, r, nil, l.style)
			x += runewidth.RuneWidth(r)
		}
	}
}
