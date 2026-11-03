package shell

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// y and n on a question or an alert do nothing: they say how to answer.
func TestSessionsYesOnAQuestionSaysHowToAnswer(t *testing.T) {
	p := sessionsSim(t, "d:d-q1", newDecidingBackend(), Config{})
	p.press(tcell.KeyRune, 'y')
	awaitText(t, p, i18n.T("tui.sessions.answer_with_enter"))
}
