package widgets

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// The text of a card line is drawn like tview does: style tags change the
// colors and are not shown, "[x[]" is a literal "[x]", other brackets are
// plain text, and the line never goes past its width (wide runes count 2).
func TestCardColumnDrawColoredText(t *testing.T) {
	base := tcell.StyleDefault.Foreground(tcell.ColorSilver).Background(theme.BgCard)
	type cell struct {
		x    int
		fg   tcell.Color
		bold bool
	}
	for _, tc := range []struct {
		name  string
		text  string
		width int
		want  string // the line, without its padding
		cells []cell // the style of some cells
	}{
		{"plain text", "hello", 10, "hello", []cell{{0, tcell.ColorSilver, false}}},
		{"escaped bracket", "tag [AI[] ok", 20, "tag [AI] ok", []cell{{4, tcell.ColorSilver, false}, {7, tcell.ColorSilver, false}}},
		{"exclamation mark", "[!] warn", 20, "[!] warn", []cell{{0, tcell.ColorSilver, false}}},
		{"counter", "done [2/3]", 20, "done [2/3]", []cell{{5, tcell.ColorSilver, false}}},
		{"unknown one-word tag is a tag", "[bug]x", 20, "x", nil},
		{"unterminated bracket", "a [b", 20, "a [b", nil},
		{"color then reset", "[#a6e3a1]vert[-].", 20, "vert.", []cell{
			{0, tcell.GetColor("#a6e3a1"), false}, {3, tcell.GetColor("#a6e3a1"), false}, {4, tcell.ColorSilver, false}}},
		{"foreground, background and attribute", "[red:blue:b]x", 20, "x", []cell{{0, tcell.ColorRed, true}}},
		{"invalid tag is text", "[a:b:c:d]x", 20, "[a:b:c:d]x", nil},
		{"truncated to the width", "abcdefgh", 5, "abcde", nil},
		{"escape cut by the width", "ab[cd[] ef", 4, "ab[c", nil},
		{"wide runes", "日本語", 5, "日本", nil},
		{"wide rune after a tag", "[red]日本", 3, "日", []cell{{0, tcell.ColorRed, false}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := tcell.NewSimulationScreen("UTF-8")
			require.NoError(t, s.Init())
			defer s.Fini()
			s.SetSize(30, 2)
			// Leftovers show if the line does not pad its width.
			for x := 0; x < 30; x++ {
				s.SetContent(x, 0, '#', nil, tcell.StyleDefault)
			}
			c := NewCardColumn("col", tcell.ColorWhite)
			c.drawColoredText(s, 0, 0, tc.width, tc.text, base)
			s.Show()

			var line strings.Builder
			for x := 0; x < tc.width; {
				str, style, w := s.Get(x, 0)
				line.WriteString(str)
				_, bg, _ := style.Decompose()
				assert.Equal(t, theme.BgCard, bg, "x=%d: the card background", x)
				x += max(w, 1)
			}
			assert.Equal(t, tc.want, strings.TrimRight(line.String(), " "), "text of %q", tc.text)
			str, _, _ := s.Get(tc.width, 0)
			assert.Equal(t, "#", str, "nothing is written past the width")

			for _, want := range tc.cells {
				_, style, _ := s.Get(want.x, 0)
				fg, _, attrs := style.Decompose()
				assert.Equal(t, want.fg, fg, "x=%d: foreground", want.x)
				assert.Equal(t, want.bold, attrs&tcell.AttrBold != 0, "x=%d: bold", want.x)
			}
		})
	}
}

func refCards(ids ...string) []Card {
	cards := make([]Card, len(ids))
	for i, id := range ids {
		cards[i] = Card{MainText: id, SecondaryText: id, Reference: id}
	}
	return cards
}

func drawColumn(t *testing.T, c *CardColumn, w, h int) {
	t.Helper()
	s := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, s.Init())
	defer s.Fini()
	s.SetSize(w, h)
	c.SetRect(0, 0, w, h)
	c.Draw(s)
}

// ReplaceCards follows the selected card and the first visible card by their
// Reference, and keeps the nearest index when the selected card is gone.
func TestCardColumnReplaceCardsKeepsTheUserWhereTheyAre(t *testing.T) {
	ids := func(from, to int) []string {
		var out []string
		for i := from; i < to; i++ {
			out = append(out, string(rune('a'+i)))
		}
		return out
	}
	c := NewCardColumn("X", tcell.ColorWhite).SetCards(refCards(ids(0, 20)...))
	c.SetFocused(true)
	for range 12 {
		c.MoveSelection(1)
		drawColumn(t, c, 30, 20)
	}
	require.Equal(t, 12, c.selected)
	offset := c.offset
	require.Positive(t, offset)

	t.Run("same cards", func(t *testing.T) {
		c.ReplaceCards(refCards(ids(0, 20)...))
		assert.Equal(t, 12, c.selected)
		assert.Equal(t, offset, c.offset)
	})
	t.Run("cards inserted above", func(t *testing.T) {
		c.ReplaceCards(refCards(append([]string{"x", "y"}, ids(0, 20)...)...))
		assert.Equal(t, 14, c.selected, "still the card 'm'")
		assert.Equal(t, offset+2, c.offset, "still the same first visible card")
		drawColumn(t, c, 30, 20)
		assert.Equal(t, offset+2, c.offset)
	})
	t.Run("selected card gone", func(t *testing.T) {
		c.ReplaceCards(refCards(append(append(ids(0, 12), ids(13, 20)...), "x")...))
		assert.Equal(t, 14, c.selected, "the nearest index")
	})
	t.Run("column shorter than the selection", func(t *testing.T) {
		c.ReplaceCards(refCards(ids(0, 5)...))
		assert.Equal(t, 4, c.selected)
		assert.LessOrEqual(t, c.offset, c.selected)
		drawColumn(t, c, 30, 40)
		assert.Zero(t, c.offset, "all the cards fit: no blank under them while others are hidden above")
	})
	t.Run("empty", func(t *testing.T) {
		c.ReplaceCards(nil)
		assert.Equal(t, -1, c.GetCurrentItem())
		assert.Zero(t, c.offset)
	})
}

// Cards without a usable Reference are followed by index; SetCards starts
// again from the top.
func TestCardColumnReplaceCardsWithoutIdentity(t *testing.T) {
	c := NewCardColumn("X", tcell.ColorWhite).SetCards([]Card{
		{MainText: "a"}, {MainText: "b", Reference: []int{1}}, {MainText: "c", Reference: []int{2}},
	})
	c.SetCurrentItem(2)
	assert.NotPanics(t, func() {
		c.ReplaceCards([]Card{{MainText: "a"}, {MainText: "b", Reference: []int{1}}, {MainText: "c", Reference: []int{2}}, {MainText: "d"}})
	})
	assert.Equal(t, 2, c.selected)

	c.SetCards(refCards("a", "b", "c"))
	assert.Zero(t, c.selected)
	assert.Zero(t, c.offset)
}
