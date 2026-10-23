package widgets

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTruncateTagged(t *testing.T) {
	assert.Equal(t, "hello", TruncateTagged("hello", 5))
	assert.Equal(t, "hel…", TruncateTagged("hello", 4))
	assert.Equal(t, "", TruncateTagged("hello", 0))
	// Style tags have no width and are never split.
	assert.Equal(t, "[#ff0000]abcdef[-]", TruncateTagged("[#ff0000]abcdef[-]", 6))
	got := TruncateTagged("[#ff0000]abcdef[-]", 4)
	assert.Equal(t, "[#ff0000]abc…", got)
	assert.Equal(t, 4, tview.TaggedStringWidth(got))
	// Wide runes count two cells.
	got = TruncateTagged("日本語テキスト", 7)
	assert.LessOrEqual(t, tview.TaggedStringWidth(got), 7)
	assert.True(t, strings.HasSuffix(got, "…"))
}

func drawList(t *testing.T, sl *SectionedList, w, h int) tcell.SimulationScreen {
	t.Helper()
	s := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, s.Init())
	s.SetSize(w, h)
	sl.SetRect(0, 0, w, h)
	sl.Draw(s)
	s.Show()
	return s
}

func rowText(s tcell.SimulationScreen, y, w int) string {
	var b strings.Builder
	for x := 0; x < w; x++ {
		str, _, _ := s.Get(x, y)
		b.WriteString(str)
	}
	return b.String()
}

// SetBorderPadding is honoured (the draw func returns the padded rect) and
// long texts are ellipsized to the real width.
func TestSectionedList_PaddingAndEllipsis(t *testing.T) {
	sl := NewSectionedList()
	sl.SetBorderPadding(0, 0, 3, 3)
	sl.SetItems([]SectionItem{
		{MainText: "Section", IsHeader: true},
		{MainText: strings.Repeat("m", 60), SecondaryText: strings.Repeat("s", 60)},
	})
	const w = 30
	s := drawList(t, sl, w, 10)

	x, _, iw, _ := sl.GetInnerRect()
	assert.Equal(t, 3, x)
	assert.Equal(t, w-6, iw)

	main := rowText(s, 2, w)
	sec := rowText(s, 3, w)
	assert.Equal(t, "   ", main[:3], "left padding kept free")
	assert.Contains(t, main, "…")
	assert.Contains(t, sec, "…")
	// Nothing printed in the right padding.
	for _, line := range []string{main, sec} {
		runes := []rune(line)
		assert.Equal(t, "   ", string(runes[len(runes)-3:]), "right padding kept free: %q", line)
	}
}

func sectionFixture() []SectionItem {
	return []SectionItem{
		{MainText: "MCP Gitlab", IsHeader: true},
		{MainText: "Team", IsHeader: true},
		{MainText: "enabled", SecondaryText: "desc 1"},
		{MainText: "url", SecondaryText: "desc 2"},
		{MainText: "MCP Jira", IsHeader: true},
		{MainText: "Team", IsHeader: true},
		{MainText: "enabled", SecondaryText: "desc 3"},
		{MainText: "url", SecondaryText: "desc 4"},
		{MainText: "write", SecondaryText: "desc 5"},
	}
}

func screenOf(s tcell.SimulationScreen, w, h int) string {
	var rows []string
	for y := 0; y < h; y++ {
		rows = append(rows, rowText(s, y, w))
	}
	return strings.Join(rows, "\n")
}

func press(sl *SectionedList, r rune) {
	sl.InputHandler()(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone), func(tview.Primitive) {})
}

// The title of a section stays on screen when the cursor comes back to its
// first field: tview scrolls to put the cursor on the first row, which would
// hide the title and the sub-title above it.
func TestSectionedList_TitleStaysWhenTheCursorComesBackUp(t *testing.T) {
	sl := NewSectionedList()
	sl.SetItems(sectionFixture())
	const w, h = 40, 10
	drawList(t, sl, w, h)

	press(sl, 'G')
	assert.NotContains(t, screenOf(drawList(t, sl, w, h), w, h), "MCP Gitlab", "scrolled down")
	press(sl, 'g')
	screen := screenOf(drawList(t, sl, w, h), w, h)
	assert.Contains(t, screen, "── MCP Gitlab ──", "screen:\n%s", screen)
	assert.Contains(t, screen, "Team", "the sub-title too")

	// second section: moving up to its first field keeps its title
	press(sl, 'G')
	drawList(t, sl, w, h)
	press(sl, 'k')
	press(sl, 'k')
	screen = screenOf(drawList(t, sl, w, h), w, h)
	assert.Contains(t, screen, "── MCP Jira ──", "screen:\n%s", screen)
}

// A first draw in a rect too small to hold the cursor (before the layout is
// final) must not leave the first title scrolled out.
func TestSectionedList_TitleSurvivesATooSmallFirstDraw(t *testing.T) {
	sl := NewSectionedList()
	sl.SetItems(sectionFixture())
	drawList(t, sl, 40, 4)
	screen := screenOf(drawList(t, sl, 40, 12), 40, 12)
	assert.Contains(t, screen, "── MCP Gitlab ──", "screen:\n%s", screen)
}
