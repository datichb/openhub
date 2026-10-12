package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLevelFor(t *testing.T) {
	cases := []struct {
		cols, rows int
		pref       string
		want       LayoutLevel
	}{
		{120, 40, "", LevelFull},
		{120, 32, CompactAuto, LevelFull},
		{120, 31, "", LevelCompact},
		{80, 24, "", LevelCompact},
		{60, 16, "", LevelCompact},
		{59, 40, "", LevelTooSmall},
		{80, 15, "", LevelTooSmall},
		{50, 12, CompactNever, LevelTooSmall},
		{120, 40, CompactAlways, LevelCompact},
		{80, 24, CompactNever, LevelFull},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, LevelFor(c.cols, c.rows, c.pref), "%d×%d %q", c.cols, c.rows, c.pref)
	}
}

// The narrow landing is min(72, width) wide (N20).
func TestContentWidthFor(t *testing.T) {
	assert.Equal(t, 72, contentWidthFor(layoutNarrow, 80))
	assert.Equal(t, 64, contentWidthFor(layoutNarrow, 64))
	assert.Equal(t, 90, contentWidthFor(layoutWide, 120))
	assert.Equal(t, 72, contentWidthFor(layoutNarrow, 0), "unknown width")
	// The dual layout grows with the terminal, capped at 180.
	assert.Equal(t, 134, contentWidthFor(layoutDual, 140))
	assert.Equal(t, 174, contentWidthFor(layoutDual, 180))
	assert.Equal(t, 180, contentWidthFor(layoutDual, 250))
}
