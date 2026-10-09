package shell

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// screenText returns the simulation screen as lines of text.
func (s *simShell) screenText() string {
	cells, w, h := s.sim.GetContents()
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := cells[y*w+x]
			if len(c.Runes) > 0 {
				b.WriteRune(c.Runes[0])
			} else {
				b.WriteByte(' ')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func (s *simShell) resize(w, h int) {
	s.sim.SetSize(w, h)
	s.sim.PostEvent(tcell.NewEventResize(w, h))
}

func (s *simShell) waitLevel(t *testing.T, want views.LayoutLevel) {
	t.Helper()
	require.Eventually(t, func() bool {
		var l views.LayoutLevel
		s.loop(func() { l = s.level })
		return l == want
	}, 2*time.Second, 5*time.Millisecond)
}

// focusedList returns the focused landing list once rebuilt for the level.
func (s *simShell) focusedList(t *testing.T) *widgets.SectionedList {
	t.Helper()
	var list *widgets.SectionedList
	require.Eventually(t, func() bool {
		s.loop(func() { list, _ = s.app.GetFocus().(*widgets.SectionedList) })
		return list != nil
	}, 2*time.Second, 5*time.Millisecond)
	return list
}

func runHome(t *testing.T, cols, rows int, compact string) *simShell {
	t.Helper()
	views.SetLayoutLevel(views.LevelFull)
	t.Cleanup(func() { views.SetLayoutLevel(views.LevelFull) })
	home := views.NewHomeView(views.HomeViewConfig{})
	s := runSimShell(t, cols, rows, Config{ProjectName: "test", Views: []views.View{home}, HomeViewID: "home",
		Compact: func() string { return compact }})
	s.loop(func() { s.NavigateHome("home", views.ModeHub) })
	return s
}

// 80×24: compact level — 3-row palette, one-line banner, no footer: the
// list gets at least 10 rows and its first item is visible (N20).
func TestLayoutCompact80x24(t *testing.T) {
	s := runHome(t, 80, 24, "")
	s.waitLevel(t, views.LevelCompact)
	var height int
	require.Eventually(t, func() bool {
		list := s.focusedList(t)
		s.loop(func() { _, _, _, height = list.GetRect() })
		return height >= 10
	}, 2*time.Second, 10*time.Millisecond)
	var omnibarRows int
	s.loop(func() { _, _, _, omnibarRows = s.omnibar.Primitive().GetRect() })
	assert.Equal(t, 3, omnibarRows)
	require.Eventually(t, func() bool {
		txt := s.screenText()
		return strings.Contains(txt, i18n.T("tui.home.section.system")) && strings.Contains(txt, "OPENHUB")
	}, 2*time.Second, 10*time.Millisecond)
	assert.NotContains(t, s.screenText(), "___", "no figlet banner")
}

// 120×40: full level — figlet banner, 8-row palette.
func TestLayoutFull120x40(t *testing.T) {
	s := runHome(t, 120, 40, "")
	time.Sleep(50 * time.Millisecond)
	s.waitLevel(t, views.LevelFull)
	require.Eventually(t, func() bool { return strings.Contains(s.screenText(), "___") }, 2*time.Second, 10*time.Millisecond)
	var omnibarRows int
	s.loop(func() { _, _, _, omnibarRows = s.omnibar.Primitive().GetRect() })
	assert.Equal(t, 8, omnibarRows)
}

// [cli] compact = always / never.
func TestLayoutCompactSetting(t *testing.T) {
	// One shell at a time (shell.New sets the global tview styles).
	t.Run("always", func(t *testing.T) {
		s := runHome(t, 120, 40, views.CompactAlways)
		s.waitLevel(t, views.LevelCompact)
	})
	t.Run("never", func(t *testing.T) {
		n := runHome(t, 80, 24, views.CompactNever)
		time.Sleep(50 * time.Millisecond)
		n.waitLevel(t, views.LevelFull)
		require.Eventually(t, func() bool { return strings.Contains(n.screenText(), "___") }, 2*time.Second, 10*time.Millisecond)
	})
}

// 50×12: only the « too small » message; keys do nothing, Ctrl+Q quits.
func TestLayoutTooSmall50x12(t *testing.T) {
	s := runHome(t, 50, 12, "")
	s.waitLevel(t, views.LevelTooSmall)
	require.Eventually(t, func() bool {
		return strings.Contains(s.screenText(), "50×12")
	}, 2*time.Second, 10*time.Millisecond)
	s.key(tcell.KeyCtrlP, 0, tcell.ModNone)
	assert.False(t, s.omnibarActive(), "keys blocked")
	s.sim.InjectKey(tcell.KeyCtrlQ, 0, tcell.ModNone)
	require.Eventually(t, func() bool { return !s.IsRunning() }, 2*time.Second, 10*time.Millisecond)
}

// 120×40 → 80×24: the level changes, the focus stays on the same item.
func TestLayoutResizeKeepsTheFocus(t *testing.T) {
	s := runHome(t, 120, 40, "")
	time.Sleep(50 * time.Millisecond)
	s.waitLevel(t, views.LevelFull)
	list := s.focusedList(t)
	var want widgets.SectionItem
	s.loop(func() {
		list.SelectIndex(len(list.GetItems()) - 1)
		_, want, _ = list.CurrentItem()
	})
	require.NotEmpty(t, want.MainText)
	s.resize(80, 24)
	s.waitLevel(t, views.LevelCompact)
	require.Eventually(t, func() bool {
		var got widgets.SectionItem
		ok := false
		s.loop(func() {
			if l, isList := s.app.GetFocus().(*widgets.SectionedList); isList && l != list {
				_, got, ok = l.CurrentItem()
			}
		})
		return ok && got.MainText == want.MainText
	}, 2*time.Second, 10*time.Millisecond)
	// and back
	s.resize(120, 40)
	s.waitLevel(t, views.LevelFull)
}
