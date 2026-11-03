package shell

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

func (s *simShell) contentFocused() (is bool) {
	s.loop(func() { is = s.content.HasFocus() })
	return is
}

// Esc closes a window whether it has buttons or not: a reader (no button) used
// to trap the user, only a click outside closed it.
func TestEscClosesEveryModal(t *testing.T) {
	cases := map[string][]views.ModalAction{
		"without button": nil,
		"with buttons":   {{Label: "ok"}, {Label: "other"}},
		"one button":     {{Label: "ok"}},
	}
	for name, actions := range cases {
		t.Run(name, func(t *testing.T) {
			s := newKeyShell(t, &keyView{id: "home", focusList: true})
			s.loop(func() { s.ShowScrollableModal("reader", "body", actions) })
			require.True(t, s.overlayShown())

			s.key(tcell.KeyEscape, 0, tcell.ModNone)
			assert.False(t, s.overlayShown(), "Esc closed the window")
			assert.True(t, s.contentFocused(), "the focus is back on the content")
		})
	}
}
