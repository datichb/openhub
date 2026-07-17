package shell

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/tui/v2/menu"
)

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		pattern  string
		str      string
		expected bool
	}{
		{"st", "start", true},
		{"srt", "start", true},
		{"art", "start", true},
		{"xyz", "start", false},
		{"", "anything", true},
		{"a", "", false},
		{"brd", "board", true},
		{"bxd", "board", false}, // 'x' not in 'board'
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.str, func(t *testing.T) {
			assert.Equal(t, tt.expected, fuzzyMatch(tt.pattern, tt.str))
		})
	}
}

func TestBuildPaletteEntries(t *testing.T) {
	s := &Shell{
		menuItems: []*menu.MenuItem{
			{ID: "home", Label: "Home", ViewID: "home"},
			{
				ID: "sessions", Label: "Sessions",
				Children: []*menu.MenuItem{
					{ID: "sessions.start", Label: "Start", Action: func() {}},
					{ID: "sessions.quick", Label: "Quick", ViewID: "quick"},
				},
			},
		},
	}

	entries := s.buildPaletteEntries()
	assert.Equal(t, 3, len(entries))
	assert.Equal(t, "Home", entries[0].Label)
	assert.Equal(t, "", entries[0].Category)
	assert.Equal(t, "Start", entries[1].Label)
	assert.Equal(t, "Sessions", entries[1].Category)
	assert.Equal(t, "Quick", entries[2].Label)
	assert.Equal(t, "Sessions", entries[2].Category)
}

func TestFlashStyle(t *testing.T) {
	icon, hex := flashStyle(FlashSuccess)
	assert.NotEmpty(t, icon)
	assert.NotEmpty(t, hex)

	icon, hex = flashStyle(FlashError)
	assert.NotEmpty(t, icon)
	assert.NotEmpty(t, hex)

	icon, hex = flashStyle(FlashWarning)
	assert.NotEmpty(t, icon)
	assert.NotEmpty(t, hex)
}
