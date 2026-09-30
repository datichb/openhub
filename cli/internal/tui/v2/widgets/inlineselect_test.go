package widgets

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
)

func TestInlineSelect_GetFieldHeight_NoDescriptions(t *testing.T) {
	s := NewInlineSelect("Label", []string{"a", "b", "c"}, 0, nil)
	assert.Equal(t, 4, s.GetFieldHeight(), "without descriptions, height = len(options) + 1 label")
}

func TestInlineSelect_GetFieldHeight_WithDescriptions(t *testing.T) {
	s := NewInlineSelect("Label", []string{"a", "b", "c"}, 0, nil)
	s.SetDescriptions([]string{"desc a", "desc b", "desc c"})
	// 3 options * 2 lines + 2 separators + 1 label = 9
	assert.Equal(t, 9, s.GetFieldHeight(), "with descriptions, height = n*2 + (n-1) + 1 label")
}

func TestInlineSelect_GetSelectedIndex(t *testing.T) {
	s := NewInlineSelect("", []string{"a", "b", "c"}, 1, nil)
	assert.Equal(t, 1, s.GetSelectedIndex())
	assert.Equal(t, "b", s.GetSelectedValue())
}

func TestInlineSelect_GetSelectedValue_OutOfRange(t *testing.T) {
	s := NewInlineSelect("", []string{}, 0, nil)
	assert.Equal(t, "", s.GetSelectedValue())
}

func TestInlineSelect_DefaultIdxClamped(t *testing.T) {
	s := NewInlineSelect("", []string{"a", "b"}, 10, nil)
	assert.Equal(t, 0, s.GetSelectedIndex(), "out-of-range defaultIdx should clamp to 0")
}

func TestInlineSelect_DefaultIdxNegative(t *testing.T) {
	s := NewInlineSelect("", []string{"a", "b"}, -1, nil)
	assert.Equal(t, 0, s.GetSelectedIndex(), "negative defaultIdx should clamp to 0")
}

func TestInlineSelect_SetDescriptions(t *testing.T) {
	s := NewInlineSelect("", []string{"a", "b"}, 0, nil)
	assert.False(t, s.hasDescriptions(), "no descriptions initially")

	s.SetDescriptions([]string{"d1", "d2"})
	assert.True(t, s.hasDescriptions(), "should have descriptions after set")

	// Fewer descriptions than options → hasDescriptions false
	s.SetDescriptions([]string{"d1"})
	assert.False(t, s.hasDescriptions(), "fewer descriptions than options should return false")
}

func TestInlineSelect_GetLabel(t *testing.T) {
	s := NewInlineSelect("My Label", []string{"a"}, 0, nil)
	assert.Equal(t, "My Label", s.GetLabel())
}

func TestInlineSelect_FocusBlur(t *testing.T) {
	s := NewInlineSelect("", []string{"a"}, 0, nil)
	assert.False(t, s.HasFocus())
	s.Focus(func(p tview.Primitive) {})
	assert.True(t, s.HasFocus())
	s.Blur()
	assert.False(t, s.HasFocus())
}
