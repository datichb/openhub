package shell

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

func (s *simShell) screen() string {
	var b strings.Builder
	s.loop(func() {})
	cells, w, _ := s.sim.GetContents()
	for i, c := range cells {
		if len(c.Runes) > 0 {
			b.WriteRune(c.Runes[0])
		} else {
			b.WriteByte(' ')
		}
		if (i+1)%w == 0 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// The help (?) shows the key convention and offers the notice of the new
// keys again.
func TestHelpShowsTheKeyConvention(t *testing.T) {
	prev := i18n.Locale()
	i18n.SetLocale("fr")
	t.Cleanup(func() { i18n.SetLocale(prev) })

	s := newKeyShell(t, &keyView{id: "home", focusList: true})
	s.key(tcell.KeyRune, '?', tcell.ModNone)
	screen := s.screen()
	assert.Contains(t, screen, "Convention des touches")
	assert.Contains(t, screen, "Nouvelles touches")

	// The button of the help opens the notice.
	s.key(tcell.KeyTab, 0, tcell.ModNone)
	s.key(tcell.KeyEnter, 0, tcell.ModNone)
	assert.Contains(t, s.screen(), "Les touches sont maintenant les mêmes")
}

// At startup, the notice waits when a window is open; else it is shown and
// recorded.
func TestOfferKeysNotice(t *testing.T) {
	s := newKeyShell(t, &keyView{id: "home", focusList: true})
	seen := 0
	var shown bool
	s.loop(func() { s.Confirm("busy", "", false, func() {}) })
	s.loop(func() { shown = s.OfferKeysNotice(func() { seen++ }) })
	assert.False(t, shown)
	assert.Zero(t, seen)
	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	s.loop(func() { shown = s.OfferKeysNotice(func() { seen++ }) })
	assert.True(t, shown)
	assert.Equal(t, 1, seen)
}

// helpKeysView shows only its essential keys in the hints bar.
type helpKeysView struct{ keyView }

func (v *helpKeysView) StatusHints() string { return "Entrée ouvrir · ? aide" }
func (v *helpKeysView) HelpKeys() string    { return "Entrée ouvrir · Z zoomer · ? aide" }

// U-07: the help (?) lists all the keys of a view that has a short hints bar.
func TestHelpListsAllTheKeysOfAView(t *testing.T) {
	prev := i18n.Locale()
	i18n.SetLocale("fr")
	t.Cleanup(func() { i18n.SetLocale(prev) })

	home := &helpKeysView{keyView{id: "home", focusList: true}}
	s := runSimShell(t, 120, 40, Config{ProjectName: "test", Views: []views.View{home}, HomeViewID: home.id})
	s.loop(func() { s.NavigateHome(home.id, views.ModeHub) })
	s.key(tcell.KeyRune, '?', tcell.ModNone)
	for range 4 {
		s.key(tcell.KeyPgDn, 0, tcell.ModNone) // the keys of the view are below the convention
	}
	screen := s.screen()
	assert.Contains(t, screen, "Z zoomer")
	assert.NotContains(t, screen, "Entrée ouvrir · ? aide")
}
