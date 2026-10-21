package widgets

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
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
