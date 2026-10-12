package shell

import (
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Confirm asks for a confirmation before an action (views.ShellAccess,
// N19). The focus starts on the primary button « Oui », except for a
// dangerous action where it starts on « Annuler », so an Enter typed by
// reflex does not destroy anything; Tab reaches the text and the buttons.
// Esc or « Annuler » close the window without acting. The focus returns to
// the widget that had it.
func (s *Shell) Confirm(title, detail string, danger bool, onYes func()) {
	back := s.app.GetFocus()
	if back == nil {
		back = s.content
	}
	text := tview.NewTextView().SetText(detail)
	styleScrollableText(text)
	yes := i18n.T("tui.confirm.yes")
	actions := []views.ModalAction{
		{Label: yes, Callback: onYes, Danger: danger},
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
		PageName:     "inline-overlay",
		FocusReturn:  back,
		FocusTarget:  text,
		FocusAction:  focus,
	})
}
