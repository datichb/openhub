package views

import (
	"strings"

	"github.com/rivo/tview"
)

// formNote is a read-only line under a field of a form (help, type error,
// value computed by oh): a text view without label, skipped by Tab, whose
// height follows its wrapped text, plus an empty line before the next
// field (the form has no item padding).
type formNote struct {
	*tview.TextView
	form *tview.Form
}

func newFormNote(f *tview.Form, text string) *formNote {
	tv := tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetWordWrap(true).SetScrollable(false)
	tv.SetText(text)
	return &formNote{TextView: tv, form: f}
}

// GetFieldHeight is the number of wrapped lines of the note, plus the gap.
func (n *formNote) GetFieldHeight() int {
	text := strings.TrimRight(n.GetText(false), "\n")
	if strings.TrimSpace(text) == "" {
		return 1
	}
	w := n.textWidth()
	lines := 0
	for _, l := range strings.Split(text, "\n") {
		lines += max(1, len(tview.WordWrap(l, w)))
	}
	return lines + 1
}

// textWidth is the width left to the note by the labels of the form.
func (n *formNote) textWidth() int {
	_, _, w, _ := n.form.GetInnerRect()
	label := 0
	for i := 0; i < n.form.GetFormItemCount(); i++ {
		label = max(label, tview.TaggedStringWidth(n.form.GetFormItem(i).GetLabel()))
	}
	if w -= label + 1; w < 20 {
		return 60
	}
	return w
}
