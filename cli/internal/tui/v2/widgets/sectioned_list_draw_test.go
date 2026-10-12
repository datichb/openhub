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
