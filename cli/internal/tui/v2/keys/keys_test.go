package keys

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/i18n"
)

func withLocale(t *testing.T, l string) {
	t.Helper()
	prev := i18n.Locale()
	i18n.SetLocale(l)
	t.Cleanup(func() { i18n.SetLocale(prev) })
}

// Every label, description, context and change is translated in both
// locales.
func TestKeysTranslated(t *testing.T) {
	for _, l := range []string{"fr", "en"} {
		withLocale(t, l)
		check := func(key string) {
			t.Helper()
			assert.NotEqual(t, key, i18n.T(key), "%s: %s", l, key)
		}
		for _, a := range append(All(), Navigate, Ends) {
			check(a.LabelKey())
			check("tui.keys.describe." + a.ID)
			if a.nameKey != "" {
				check(a.nameKey)
			}
		}
		for _, c := range Contexts() {
			check("tui.keys.context." + string(c))
		}
		for _, c := range Changes() {
			check(c.LabelKey())
		}
	}
}

// Within a context, a key has one action; Everywhere keys are never taken
// by another context.
func TestKeysUnique(t *testing.T) {
	everywhere := map[Binding]string{}
	for _, a := range ByContext(Everywhere) {
		for _, b := range a.Bindings {
			everywhere[b] = a.ID
		}
	}
	for _, c := range Contexts() {
		seen := map[Binding]string{}
		for _, a := range ByContext(c) {
			for _, b := range a.Bindings {
				if prev, ok := seen[b]; ok && c != Reader {
					t.Errorf("%s: %q bound to %s and %s", c, b.Rune, prev, a.ID)
				}
				seen[b] = a.ID
				if c != Everywhere && c != Reader {
					if id, ok := everywhere[b]; ok {
						t.Errorf("%s: %s takes the key of %s", c, a.ID, id)
					}
				}
			}
		}
	}
}

func TestMatches(t *testing.T) {
	ev := func(r rune, m tcell.ModMask) *tcell.EventKey { return tcell.NewEventKey(tcell.KeyRune, r, m) }
	assert.True(t, Delete.Matches(ev('d', tcell.ModNone)))
	assert.False(t, Delete.Matches(ev('D', tcell.ModShift)))
	assert.False(t, Delete.Matches(ev('d', tcell.ModCtrl)))
	assert.True(t, Resume.Matches(ev('R', tcell.ModShift)))
	assert.True(t, Down.Matches(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)))
	assert.True(t, Down.Matches(ev('j', tcell.ModNone)))
	assert.True(t, Open.Matches(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)))
	assert.True(t, Help.Matches(tcell.NewEventKey(tcell.KeyF1, 0, tcell.ModNone)))
	assert.True(t, Navigate.Matches(ev('k', tcell.ModNone)))
	assert.False(t, Open.Matches(nil))
	assert.Equal(t, 'R', Resume.Rune())
	assert.Equal(t, 'x', Archive.Rune())
	assert.Zero(t, Open.Rune())
}

func TestHints(t *testing.T) {
	withLocale(t, "fr")
	assert.Equal(t, "d supprimer", Delete.Hint())
	assert.Equal(t, "R récupérer", Resume.HintAs("récupérer"))
	assert.Equal(t, "Entrée ouvrir · Échap revenir", HintsOf(Open, Back))
	assert.Equal(t, "y valider · a attacher", Hints(Accept.Hint(), "", " a attacher "))
	assert.Equal(t, "↑↓/jk naviguer", Navigate.Hint())
	withLocale(t, "en")
	assert.Equal(t, "Enter open · Esc back", HintsOf(Open, Back))
	assert.Equal(t, "Delete (always confirmed)", Delete.Description())
}

func TestHelpRows(t *testing.T) {
	withLocale(t, "fr")
	rows := HelpRows(Everywhere)
	assert.Equal(t, HelpRow{Keys: "↑↓/jk", Label: "Monter / descendre"}, rows[0])
	assert.Equal(t, "g/G", rows[1].Keys)
	assert.Len(t, HelpRows(List), 6)
	assert.Equal(t, []string{"y", "n", "e", "s", "A"}, keysOf(ByContext(Decision)))
	assert.Equal(t, []string{"n", "e", "d", "x", "R", "c"}, keysOf(ByContext(List)))
}

func keysOf(actions []Action) []string {
	out := make([]string, len(actions))
	for i, a := range actions {
		out[i] = a.Keys()
	}
	return out
}
