package shell

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// newListSim is a shell on a simulation screen with a home view.
func newListSim(t *testing.T, cols, rows int) *panicSim {
	t.Helper()
	p := runPanicSim(t, cols, rows, Config{ProjectName: "test", Views: []views.View{&keyView{id: "home", focusList: true}}, HomeViewID: "home"})
	p.loopSafe(func() { p.NavigateHome("home", views.ModeHub) })
	return p
}

// find returns the cell of the first rune of text on the screen, read on the
// event loop (-1, -1 when the screen has no such text).
func (p *panicSim) find(text string) (x, y int) {
	p.t.Helper()
	x, y = -1, -1
	p.loopSafe(func() {
		lines := strings.Split(p.screenText(), "\n")
		for row, line := range lines {
			if i := strings.Index(line, text); i >= 0 {
				x, y = utf8.RuneCountInString(line[:i]), row
				return
			}
		}
	})
	return x, y
}

// background is the background of a cell, read on the event loop.
func (p *panicSim) background(x, y int) (bg tcell.Color) {
	p.t.Helper()
	p.loopSafe(func() {
		cells, w, _ := p.sim.GetContents()
		_, bg, _ = cells[y*w+x].Style.Decompose()
	})
	return bg
}

// The labels of a list are drawn on the background of the window (BgModal),
// not on the BgPanel that tview freezes in a new list: a darker block behind
// each text.
func TestSelectListLabelsAreOnTheWindowBackground(t *testing.T) {
	require.NotEqual(t, theme.BgPanel, theme.BgModal, "the theme has two surfaces")

	open := map[string]func(p *panicSim){
		"select": func(p *panicSim) {
			p.ShowSelectModal("pick", []views.SelectOption{{Label: "first row", Value: "a"}, {Label: "second row", Value: "b"}, {Label: "[#ff0000]red[-:-:-] after reset", Value: "c"}}, "", nil)
		},
		"multi-select": func(p *panicSim) {
			p.ShowMultiSelectModal("pick", []views.SelectOption{{Label: "first row", Value: "a"}, {Label: "second row", Value: "b"}, {Label: "[#ff0000]red[-:-:-] after reset", Value: "c"}}, []string{"b"}, nil)
		},
	}
	for name, show := range open {
		t.Run(name, func(t *testing.T) {
			p := newListSim(t, 120, 36)
			p.loopSafe(func() { show(p) })

			// The highlighted row is the first one: the others show the plain style.
			for _, text := range []string{"second row", "after reset"} {
				x, y := p.find(text)
				require.GreaterOrEqual(t, x, 0, "%q is on the screen", text)
				for dx := range len([]rune(text)) {
					assert.Equal(t, theme.BgModal, p.background(x+dx, y), "%q, column %d", text, dx)
				}
			}
			// The cell after the reset tag too, not only the text after it.
			x, y := p.find("red")
			require.GreaterOrEqual(t, x, 0)
			assert.Equal(t, theme.BgModal, p.background(x, y), "the coloured text")
		})
	}
}
