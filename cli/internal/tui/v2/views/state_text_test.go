package views

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// The three states of a view without data read the same on every screen:
// loading and empty in grey, an error in red with ✗.
func TestStateText(t *testing.T) {
	frLocale(t)
	muted := theme.ColorTag(theme.TextMutedHex)
	assert.Equal(t, muted+"Chargement…"+theme.TagColor, stateText(stateLoading, ""), "the common words")
	assert.Equal(t, muted+"Rien à afficher."+theme.TagColor, stateText(stateEmpty, ""))
	assert.Equal(t, muted+"Chargement des services MCP..."+theme.TagColor, stateText(stateLoading, i18n.T("tui.mcp.loading")), "the words of the view")
	assert.Equal(t, muted+"Aucun ticket"+theme.TagColor, stateText(stateEmpty, "Aucun ticket"))

	assert.Equal(t, theme.ColorTag(theme.ErrorHex)+theme.SymFailure+" Impossible de charger : disque plein"+theme.TagColor,
		stateText(stateError, "disque plein"))
}

// What comes from outside (an error, a path) cannot be read as a tag.
func TestStateTextEscapesTheReason(t *testing.T) {
	frLocale(t)
	got := stateText(stateError, "bad [red]token[-]")
	assert.Contains(t, got, "bad [red[]token[-[]")
	assert.NotContains(t, got, "bad [red]token")
	assert.Contains(t, stateText(stateEmpty, "a [b] c"), "a [b[] c")
}

func TestStateTextIsTranslated(t *testing.T) {
	prev := i18n.Locale()
	t.Cleanup(func() { i18n.SetLocale(prev) })
	i18n.SetLocale("en")
	assert.Contains(t, stateText(stateLoading, ""), "Loading…")
	assert.Contains(t, stateText(stateEmpty, ""), "Nothing to show.")
	assert.Contains(t, stateText(stateError, "x"), "Could not load: x")
}

// A placeholder shows the text, and takes another state in place.
func TestStateView(t *testing.T) {
	frLocale(t)
	tv := stateView(stateLoading, "Chargement des briques…")
	assert.Equal(t, "\n  Chargement des briques…", tv.GetText(true))
	setState(tv, stateError, "boom")
	assert.Equal(t, "\n  "+theme.SymFailure+" Impossible de charger : boom", tv.GetText(true))
}
