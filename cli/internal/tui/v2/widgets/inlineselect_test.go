package widgets

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInlineSelect(t *testing.T) {
	t.Run("selection", func(t *testing.T) {
		tests := []struct {
			name      string
			options   []string
			defaultAt int
			wantIdx   int
			wantValue string
		}{
			{"default index kept", []string{"a", "b", "c"}, 1, 1, "b"},
			{"index past the end clamps to 0", []string{"a", "b"}, 10, 0, "a"},
			{"negative index clamps to 0", []string{"a", "b"}, -1, 0, "a"},
			{"no option: empty value", nil, 0, 0, ""},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				s := NewInlineSelect("", tc.options, tc.defaultAt, nil)
				assert.Equal(t, tc.wantIdx, s.GetSelectedIndex(), "selected index")
				assert.Equal(t, tc.wantValue, s.GetSelectedValue(), "selected value")
			})
		}
	})

	t.Run("descriptions drive the height", func(t *testing.T) {
		s := NewInlineSelect("My Label", []string{"a", "b", "c"}, 0, nil)
		assert.Equal(t, "My Label", s.GetLabel(), "label")
		assert.False(t, s.hasDescriptions(), "no descriptions initially")
		assert.Equal(t, 4, s.GetFieldHeight(), "without descriptions, height = len(options) + 1 label")

		s.SetDescriptions([]string{"desc a", "desc b", "desc c"})
		assert.True(t, s.hasDescriptions(), "descriptions after set")
		assert.Equal(t, 9, s.GetFieldHeight(), "with descriptions, height = n*2 + (n-1) + 1 label")

		s.SetDescriptions([]string{"d1"})
		assert.False(t, s.hasDescriptions(), "fewer descriptions than options are ignored")
	})

	t.Run("focus and blur", func(t *testing.T) {
		s := NewInlineSelect("", []string{"a"}, 0, nil)
		assert.False(t, s.HasFocus(), "no focus initially")
		s.Focus(func(tview.Primitive) {})
		assert.True(t, s.HasFocus(), "focused after Focus")
		s.Blur()
		assert.False(t, s.HasFocus(), "blurred after Blur")
	})
}

// screenText is what the simulation screen shows, one string per row.
func screenText(screen tcell.SimulationScreen, w, h int) []string {
	cells, _, _ := screen.GetContents()
	rows := make([]string, h)
	for y := 0; y < h; y++ {
		var b strings.Builder
		for x := 0; x < w; x++ {
			c := cells[y*w+x]
			if len(c.Runes) == 0 {
				b.WriteRune(' ')
				continue
			}
			b.WriteRune(c.Runes[0])
		}
		rows[y] = strings.TrimRight(b.String(), " ")
	}
	return rows
}

func TestInlineSelect_CompactDescription(t *testing.T) {
	s := NewInlineSelect("Type", []string{"solo", "team", "full"}, 0, nil)
	s.SetDescriptions([]string{"alone", "with a team\nof people", "everything"})
	listed := s.GetFieldHeight()
	s.SetCompactDescription(true)
	assert.Equal(t, 1+3+1, s.GetFieldHeight(), "compact: label + options + one description line, whatever the descriptions hold")
	assert.Less(t, s.GetFieldHeight(), listed, "compact is shorter than the listed descriptions")

	const w, h = 40, 8
	screen := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, screen.Init())
	screen.SetSize(w, h)
	s.SetRect(0, 0, w, h)

	s.Focus(func(tview.Primitive) {})
	s.Draw(screen)
	screen.Show()
	rows := screenText(screen, w, h)
	assert.Contains(t, rows[4], "alone", "the description of the highlighted option shows under the options")
	assert.NotContains(t, strings.Join(rows, "\n"), "everything", "the other descriptions do not")

	handler := s.InputHandler()
	handler(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), func(tview.Primitive) {})
	s.Draw(screen)
	screen.Show()
	rows = screenText(screen, w, h)
	assert.Contains(t, rows[4], "with a team of people", "the description follows the cursor, newlines folded into one line")
	assert.Equal(t, 0, s.GetSelectedIndex(), "moving the cursor does not change the selection")
}
