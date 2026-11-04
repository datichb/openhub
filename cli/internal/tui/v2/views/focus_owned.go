package views

import "github.com/rivo/tview"

// ownsFocus reports whether the focus is the view's to give: nothing has it,
// or it is in the content of the view (the placeholder shown while it loads).
// Read it when a load ends, before the content is rebuilt: the palette or a
// window opened during the load holds the focus and must keep it.
func ownsFocus(app *tview.Application, content *tview.Flex) bool {
	if app == nil {
		return false
	}
	f := app.GetFocus()
	return f == nil || (content != nil && content.HasFocus())
}

// focusIf gives the focus to p when owned (see ownsFocus).
func focusIf(app *tview.Application, owned bool, p tview.Primitive) {
	if owned && app != nil && p != nil {
		app.SetFocus(p)
	}
}

// focusIfOwned gives the focus to p when it is the view's to give. Use
// ownsFocus and focusIf when the content is rebuilt in between.
func focusIfOwned(app *tview.Application, content *tview.Flex, p tview.Primitive) {
	focusIf(app, ownsFocus(app, content), p)
}
