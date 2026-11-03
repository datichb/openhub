package shell

import (
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Confirm asks for a confirmation before an action (views.ShellAccess).
// The focus starts on the primary button « Oui », except for a
// dangerous action where it starts on « Annuler », so an Enter typed by
// reflex does not destroy anything; Tab reaches the text and the buttons.
// Esc or « Annuler » close the window without acting. The focus returns to
// the widget that had it. Asked from a window (a button of the decision
// card), it opens over it: « Oui » closes both, « Annuler » gives the
// window back.
func (s *Shell) Confirm(title, detail string, danger bool, onYes func()) {
	w := s.openPrompt()
	text := tview.NewTextView().SetText(detail)
	styleScrollableText(text)
	yes := i18n.T("tui.confirm.yes")
	actions := []views.ModalAction{
		{Label: yes, Danger: danger, KeepOpen: true, Callback: func() {
			s.answerWindow(w, onYes)
		}},
		{Label: i18n.T("tui.confirm.cancel")},
	}
	focus := 1 // « Oui »
	if danger {
		focus = 2 // « Annuler »
	}
	s.showModal(modalConfig{
		Title:        title,
		Content:      text,
		Actions:      actions,
		Size:         modalSizeHint{ContentLines: countLines(detail), HasButtons: true, ButtonCount: len(actions), ProportionalWidth: -4},
		Hints:        i18n.T("tui.confirm.hints"),
		LightDismiss: true,
		Win:          w,
		FocusTarget:  text,
		FocusAction:  focus,
	})
}
