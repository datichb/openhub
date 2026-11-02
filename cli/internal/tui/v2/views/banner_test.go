package views

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
