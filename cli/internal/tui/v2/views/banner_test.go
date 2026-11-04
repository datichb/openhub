package views

import (
	"strings"
	"testing"

	"github.com/common-nighthawk/go-figure"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An accented name is drawn with the letter without its accent (go-figure
// draws « ? » for a character outside ASCII); a name that cannot be
// transliterated (Chinese…) is shown as a plain line, not as question marks.
func TestRenderBannerTransliteratesAccents(t *testing.T) {
	want, wantLines := renderBanner("Equipe Ete", 100, "#ffffff")
	got, gotLines := renderBanner("Équipe Été", 100, "#ffffff")
	assert.Equal(t, want, got)
	assert.Equal(t, wantLines, gotLines)

	for in, plain := range map[string]string{"Straße": "Strasse", "Œuvre": "OEuvre", "Łódź": "Lodz", "Ñandú": "Nandu"} {
		w, _ := renderBanner(plain, 100, "#ffffff")
		g, _ := renderBanner(in, 100, "#ffffff")
		assert.Equal(t, w, g, in)
	}

	cjk, lines := renderBanner("日本語", 100, "#ffffff")
	assert.Equal(t, 1, lines)
	assert.Contains(t, cjk, "日本語", "the name as it is")
	assert.False(t, strings.Contains(cjk, "?"))
}

// The banner is shown in a centered text view: its lines keep the columns of
// the font (each line used to be centered on its own width, which shifted the
// letters of one line against the next).
func TestRenderBannerLinesStayAlignedWhenCentered(t *testing.T) {
	for _, name := range []string{"OPENHUB", "transparence-sru", "t-sru-team"} {
		banner, _ := renderBanner(name, 120, "#ffffff")
		font := trimEmptyLines(figure.NewFigure(name, bannerFont, false).Slicify())

		screen := tcell.NewSimulationScreen("UTF-8")
		require.NoError(t, screen.Init())
		screen.SetSize(140, 20)
		tv := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)
		tv.SetText(banner)
		tv.SetRect(0, 0, 140, 20)
		tv.Draw(screen)
		screen.Show()

		cells, w, _ := screen.GetContents()
		shift := -1
		for row, line := range font {
			first := strings.IndexFunc(line, func(r rune) bool { return r != ' ' })
			x := -1
			for col := 0; col < w; col++ {
				if c := cells[row*w+col]; len(c.Runes) > 0 && c.Runes[0] != ' ' && c.Runes[0] != '\u00a0' {
					x = col
					break
				}
			}
			require.NotEqual(t, -1, x, "%s: line %d drawn", name, row)
			if shift == -1 {
				shift = x - first
			}
			assert.Equal(t, shift, x-first, "%s: line %d starts at the column of the font", name, row)
		}
	}
}
