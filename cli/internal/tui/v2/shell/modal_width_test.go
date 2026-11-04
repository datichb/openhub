package shell

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// drawnWindow is the frame drawn on the screen: the columns of its corners and the
// text of the screen, read on the event loop.
type drawnWindow struct {
	left, right int // columns of ╭ and ╮ (-1 when no window is drawn)
	text        string
}

func (w drawnWindow) width() int { return w.right - w.left + 1 }

func (p *panicSim) frame() drawnWindow {
	p.t.Helper()
	w := drawnWindow{left: -1, right: -1, text: p.text()}
	for _, line := range strings.Split(w.text, "\n") {
		runes := []rune(line)
		for x, r := range runes {
			switch {
			case r == '╭' && w.left < 0:
				w.left = x
			case r == '╮' && w.left >= 0 && w.right < 0:
				w.right = x
			}
		}
		if w.right >= 0 {
			break
		}
	}
	return w
}

// row is the line of the screen that holds text, as runes.
func (w drawnWindow) row(text string) []rune {
	for _, line := range strings.Split(w.text, "\n") {
		if strings.Contains(line, text) {
			return []rune(line)
		}
	}
	return nil
}

const longLabel = "ticket  Implémenter un ou plusieurs tickets Beads déjà détaillés, développement puis review et commit  write ⌂ ▣"

func TestComputeWidth(t *testing.T) {
	min, chrome := theme.ModalMinWidth, modalChromeWidth
	tests := []struct {
		name string
		hint modalSizeHint
		term int
		want int
	}{
		{"rows: as wide as the longest", modalSizeHint{ContentWidth: 90}, 200, 90 + chrome},
		{"rows: not above the screen less 4", modalSizeHint{ContentWidth: 90}, 80, 76},
		{"rows: a short list keeps the minimum", modalSizeHint{ContentWidth: 5}, 200, min},
		{"rows: the title widens the window", modalSizeHint{ContentWidth: 5, FrameWidth: 70}, 200, 70},
		{"rows: a screen narrower than the minimum wins", modalSizeHint{ContentWidth: 90}, 30, 26},
		{"share: 70% of a wide screen, capped", modalSizeHint{WidthPct: theme.ModalWidthPct}, 300, theme.ModalFormMaxWidth},
		{"share: 70% of the screen", modalSizeHint{WidthPct: theme.ModalWidthPct}, 140, 98},
		{"share: not under the usual width", modalSizeHint{WidthPct: theme.ModalWidthPct}, 90, theme.ModalMaxWidth},
		{"share: not above the screen less 4", modalSizeHint{WidthPct: theme.ModalWidthPct}, 60, 56},
		{"share: a long title is not cut by the cap", modalSizeHint{WidthPct: theme.ModalWidthPct, FrameWidth: 130}, 300, 130},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, computeWidth(tc.hint, tc.term))
		})
	}
}

// A list as wide as its longest label: no label is cut on a wide screen, and
// the window is not wider than the label needs.
func TestListWindowFitsItsLongestLabel(t *testing.T) {
	opts := []views.SelectOption{{Label: "short", Value: "a"}, {Label: longLabel, Value: "b"}, {Label: "other", Value: "c"}}
	show := map[string]func(p *panicSim){
		"select":       func(p *panicSim) { p.ShowSelectModal("Launch", opts, "", nil) },
		"multi-select": func(p *panicSim) { p.ShowMultiSelectModal("Launch", opts, nil, nil) },
	}
	for name, open := range show {
		t.Run(name, func(t *testing.T) {
			p := newListSim(t, 200, 36)
			p.loopSafe(func() { open(p) })

			w := p.frame()
			require.GreaterOrEqual(t, w.left, 0, "a window is drawn")
			assert.Contains(t, w.text, longLabel, "the label is whole")
			assert.NotContains(t, w.text, "…", "nothing is ellipsized")
			label := w.row(longLabel)
			assert.Contains(t, "│║", string(label[w.right]), "the label ends inside the window")
			assert.Less(t, w.width(), len([]rune(longLabel))+modalChromeWidth+ /* check box */ 2+theme.ModalContentPadX*2, "no wider than needed")
		})
	}
}

// On a screen too narrow for the label, the window keeps the margin of the
// screen and the label ends with « … ».
func TestListWindowOnANarrowScreen(t *testing.T) {
	p := newListSim(t, 70, 24)
	p.loopSafe(func() {
		p.ShowSelectModal("Launch", []views.SelectOption{{Label: "short", Value: "a"}, {Label: longLabel, Value: "b"}}, "", nil)
	})

	w := p.frame()
	require.GreaterOrEqual(t, w.left, 0)
	assert.Equal(t, 70-4, w.width(), "the screen less 4")
	assert.Equal(t, 2, w.left, "centered")

	label := w.row("ticket  Implémenter")
	require.NotNil(t, label)
	assert.Contains(t, "│║", string(label[w.right]))
	assert.Equal(t, '…', label[w.right-1-theme.ModalContentPadX], "the label ends with an ellipsis at the padding")
	assert.NotContains(t, w.text, longLabel)
}

// A long title is ellipsized in the border of a narrow window, not cut.
func TestWindowTitleOnANarrowScreen(t *testing.T) {
	p := newListSim(t, 70, 24)
	title := "Launch on bd-42 · " + strings.Repeat("a very long title ", 6)
	p.loopSafe(func() {
		p.ShowSelectModal(title, []views.SelectOption{{Label: "one", Value: "a"}}, "", nil)
	})

	w := p.frame()
	require.GreaterOrEqual(t, w.left, 0)
	assert.Equal(t, 70-4, w.width())
	assert.Contains(t, w.text, "Launch on bd-42 ·")
	assert.Contains(t, w.text, "…")
}

// On a wide screen the title is whole and widens a window of short labels.
func TestWindowTitleWidensTheWindow(t *testing.T) {
	p := newListSim(t, 200, 24)
	title := "Launch on bd-42 · " + strings.Repeat("a long title ", 6)
	p.loopSafe(func() {
		p.ShowSelectModal(title, []views.SelectOption{{Label: "one", Value: "a"}}, "", nil)
	})

	w := p.frame()
	assert.Contains(t, w.text, title, "the title is whole")
	assert.NotContains(t, w.text, "…")
	assert.GreaterOrEqual(t, w.width(), len([]rune(title))+2*theme.ModalTitlePad+2)
}

// Inputs and forms take a share of the screen, bounded, instead of 72 columns.
func TestInputWidthFollowsTheScreen(t *testing.T) {
	for _, tc := range []struct{ cols, want int }{
		{200, theme.ModalFormMaxWidth},
		{120, 84},
		{80, theme.ModalMaxWidth},
		{60, 56},
	} {
		p := newListSim(t, tc.cols, 30)
		p.loopSafe(func() { p.ShowInputModal("Name", "", nil) })
		assert.Equal(t, tc.want, p.frame().width(), "input on %d columns", tc.cols)
	}
}

func TestFormWidthFollowsTheScreen(t *testing.T) {
	hint := "A hint of a hundred characters or so, that a form as wide as the screen shows on one line, not two or three."
	form := views.InlineFormConfig{Title: "Form", Fields: []views.FormField{
		{Key: "name", Label: "Name", Type: views.FieldText, Hint: hint},
		{Key: "kind", Label: "Kind", Type: views.FieldSelect, Options: []views.SelectOption{{Label: "one", Value: "1"}, {Label: "two", Value: "2"}}},
	}}

	wide := newListSim(t, 200, 40)
	wide.loopSafe(func() { wide.ShowInlineForm(form) })
	w := wide.frame()
	assert.Equal(t, theme.ModalFormMaxWidth, w.width())
	assert.Contains(t, w.text, hint, "the hint is on one line")
	assert.Contains(t, w.text, "Kind", "the field under the hint is shown")

	narrow := newListSim(t, 80, 40)
	narrow.loopSafe(func() { narrow.ShowInlineForm(form) })
	n := narrow.frame()
	assert.Equal(t, theme.ModalMaxWidth, n.width())
	assert.NotContains(t, n.text, hint, "the hint wraps on a narrower form")
	assert.Contains(t, n.text, "A hint of a hundred", "and still shows")
	assert.Contains(t, n.text, "two or three.", "to its end")
	assert.Contains(t, n.text, "Kind", "and the field under it")
}
