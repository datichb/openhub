package widgets

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

func TestDecorateButton(t *testing.T) {
	primary := DecorateButton(tview.NewButton("Oui"), ButtonPrimary)
	other := DecorateButton(tview.NewButton("Annuler"), ButtonNormal)
	assert.Equal(t, "Oui", ButtonLabel(primary))
	assert.Equal(t, "Annuler", ButtonLabel(other))
	// Same width focused or not: the layout does not move.
	assert.Equal(t, tview.TaggedStringWidth(decorateLabel("Oui", ButtonPrimary, true)),
		tview.TaggedStringWidth(decorateLabel("Oui", ButtonPrimary, false)))

	sc := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, sc.Init())
	sc.SetSize(20, 2)
	primary.SetRect(0, 0, 7, 1)
	primary.Draw(sc)
	other.Focus(nil)
	other.SetRect(0, 1, 11, 1)
	other.Draw(sc)
	sc.Show()
	cells, w, _ := sc.GetContents()

	// Primary at rest: neutral background, accent bold text.
	fg, bg, attr := cells[2].Style.Decompose()
	assert.Equal(t, "O", string(cells[2].Runes))
	assert.Equal(t, theme.BgModalHighlight, bg)
	assert.Equal(t, theme.ActiveMode.Accent, fg)
	assert.NotZero(t, attr&tcell.AttrBold)

	// Focused: markers and the shared focused style.
	assert.Equal(t, "▸", string(cells[w].Runes))
	wantFg, wantBg, _ := theme.ButtonFocusedStyle().Decompose()
	fg, bg, _ = cells[w+2].Style.Decompose()
	assert.Equal(t, wantBg, bg)
	assert.Equal(t, wantFg, fg)

	// A later SetLabel is picked up.
	other.SetLabel("Fermer")
	other.Draw(sc)
	assert.Equal(t, "Fermer", ButtonLabel(other))
}

func TestStyleFormButtonsLate(t *testing.T) {
	f := tview.NewForm()
	StyleFormButtons(f, 0)
	f.AddButton("Suivant", nil)
	sc := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, sc.Init())
	sc.SetSize(40, 3)
	f.SetRect(0, 0, 40, 3)
	f.Draw(sc)
	assert.Equal(t, "Suivant", ButtonLabel(f.GetButton(0)))
	assert.NotEqual(t, "Suivant", f.GetButton(0).GetLabel(), "decorated on draw")
}
