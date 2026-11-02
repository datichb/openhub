package keys_test

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	_ "github.com/datichb/openhub/cli/internal/tui/v2/views" // declares the actions of the views
)

// TestOwnActions: the actions of the views (keys.Own) are translated, have
// one ID each and keep the convention (no action of the convention on a
// letter of a view: « a ajouter », « f filtrer »…).
func TestOwnActions(t *testing.T) {
	prev := i18n.Locale()
	t.Cleanup(func() { i18n.SetLocale(prev) })
	owned := keys.Owned()
	assert.NotEmpty(t, owned)
	ids := map[string]bool{}
	for _, a := range owned {
		assert.False(t, ids[a.ID], "two actions %s", a.ID)
		ids[a.ID] = true
		for _, l := range []string{"en", "fr"} {
			i18n.SetLocale(l)
			assert.NotEqual(t, a.LabelKey(), a.Label(), "%s: %s", l, a.ID)
		}
		assert.Empty(t, keys.CheckHintLine(a.Hint()), a.ID)
	}
}

// w has one meaning: open in the browser (the sessions). Saving is Ctrl+S
// (keys.Save), the other uses moved: the week of the activity, the write mode
// of an MCP service, the worktree of a modified directory (keys.Changes
// tells the user).
func TestWHasOneMeaning(t *testing.T) {
	var ids []string
	for _, a := range keys.Owned() {
		if a.Rune() == 'w' {
			ids = append(ids, a.ID)
		}
	}
	assert.Equal(t, []string{"sessions.browser"}, ids)

	assert.True(t, keys.Save.Matches(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone)))
	assert.False(t, keys.Save.Matches(tcell.NewEventKey(tcell.KeyRune, 'w', tcell.ModNone)))
	assert.Equal(t, "Ctrl+S", keys.Save.Keys())

	moved := map[string]string{}
	for _, c := range keys.Changes() {
		if c.From == "w" {
			moved[c.ViewKey()] = c.To
		}
	}
	assert.Equal(t, map[string]string{
		"tui.keys.change.view.editor":   "Ctrl+S",
		"tui.keys.change.view.team":     "Ctrl+S",
		"tui.keys.change.view.activity": "s",
		"tui.keys.change.view.mcp":      "m",
		"tui.keys.change.view.launch":   "t",
	}, moved, "every former use of w is declared in keys.Changes")
}

// Ctrl+T changed meaning in 5.1 (the cycle Hub → Project → Team → Hub, it
// only went between the hub and the team): the notice of the new keys says so.
func TestCtrlTCycleIsDeclared(t *testing.T) {
	var found bool
	for _, c := range keys.Changes() {
		if c.To == "Ctrl+T" {
			found = true
			assert.Empty(t, c.From, "a key the notice presents as new")
			assert.Equal(t, "tui.keys.change.view.modes", c.ViewKey())
		}
	}
	assert.True(t, found)
}
