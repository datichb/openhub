package views

import (
	"strings"
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/i18n"
)

type noticeShell struct {
	mockShell
	title, text string
	actions     []ModalAction
}

func (s *noticeShell) ShowScrollableModal(title, text string, actions []ModalAction) {
	s.title, s.text, s.actions = title, text, actions
}

func TestKeysNotice(t *testing.T) {
	prev := i18n.Locale()
	i18n.SetLocale("fr")
	t.Cleanup(func() { i18n.SetLocale(prev) })

	text := KeysNoticeText()
	for _, want := range []string{
		"Liste      n créer · e éditer · d supprimer · x archiver · R reprendre · c comparer",
		"Décision   y valider · n refuser · e modifier · s laisser · A tout accepter",
		"Partout    g/G début/fin · / filtrer · r rafraîchir · ? aide",
		"Sessions                   D→d supprimer · c→R reprendre · g→R récupérer · B valider en lot",
		"Historique                 r→R restaurer",
		"Propositions wiki          a→y accepter · x→n refuser",
		"Workflows                  D→c comparer · x→d supprimer un brouillon",
	} {
		assert.Contains(t, text, want)
	}
	assertGolden(t, "keys_notice", text)

	sh := &noticeShell{}
	closed := false
	ShowKeysNotice(sh, func() { closed = true })
	assert.Equal(t, "Nouvelles touches", sh.title)
	if assert.Len(t, sh.actions, 1) {
		assert.Equal(t, "Compris", sh.actions[0].Label)
		sh.actions[0].Callback()
	}
	assert.True(t, closed)
	ShowKeysNotice(nil, nil) // no shell: nothing
}

// The help view shows the convention and offers the notice again.
func TestHelpViewShowsTheConvention(t *testing.T) {
	prev := i18n.Locale()
	i18n.SetLocale("fr")
	t.Cleanup(func() { i18n.SetLocale(prev) })

	v := NewHelpView()
	sh := &noticeShell{}
	v.SetShell(sh)
	content := tview.NewFlex().SetDirection(tview.FlexRow)
	v.Mount(content, tview.NewApplication())
	screen := screenText(t, content, 120, 80)
	for _, want := range []string{"Convention des touches", "Supprimer (toujours confirmé)", "Refuser (avec une consigne, confirmé)", "Comparer (diff)", "Échap"} {
		assert.Contains(t, screen, want)
	}
	assert.Equal(t, "Ctrl+P Nouvelles touches · Échap revenir", v.StatusHints())
	cmds := v.ContextCommands()
	if assert.Len(t, cmds, 1) {
		cmds[0].Action()
	}
	assert.True(t, strings.HasPrefix(sh.text, "Les touches sont maintenant"))
}
