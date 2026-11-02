package views

import (
	"time"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// SaveDelay is the one delay after which the configuration pages of the hub
// and of a project save what was edited (AutoSaver): the same in all of them,
// so that a change is as safe in one as in another. The team pages do not
// save on their own, they publish to the team-state on Ctrl+S (keys.Save).
const SaveDelay = 500 * time.Millisecond

// saveStatus tells whether the edits of a page are saved.
type saveStatus int

const (
	// saveStatusNone: nothing was edited on this page yet, nothing to say.
	saveStatusNone saveStatus = iota
	// saveStatusSaved: nothing waits, what is shown is what is stored.
	saveStatusSaved
	// saveStatusPending: an edit was made and the page is about to save it
	// (hub and project pages, within SaveDelay).
	saveStatusPending
	// saveStatusUnpublished: an edit is not published to the team-state yet;
	// Ctrl+S publishes it (team pages).
	saveStatusUnpublished
	// saveStatusFailed: the last save did not work (the toast says why).
	saveStatusFailed
)

// text is the indicator shown next to the title of the page.
func (s saveStatus) text() string {
	switch s {
	case saveStatusPending:
		return theme.Paint(theme.TextMutedHex, theme.SymModified+" "+i18n.T("tui.save.pending"))
	case saveStatusUnpublished:
		return theme.Paint(theme.WarningHex, theme.SymModified+" "+i18n.Tf("tui.save.unpublished", keys.Save.Keys()))
	case saveStatusFailed:
		return theme.Paint(theme.ErrorHex, theme.SymFailure+" "+i18n.T("tui.save.failed"))
	}
	return theme.Paint(theme.SuccessHex, theme.SymSuccess+" "+i18n.T("tui.save.saved"))
}

// plain is the indicator without colours (tests, hints).
func (s saveStatus) plain() string {
	switch s {
	case saveStatusPending:
		return i18n.T("tui.save.pending")
	case saveStatusUnpublished:
		return i18n.Tf("tui.save.unpublished", keys.Save.Keys())
	case saveStatusFailed:
		return i18n.T("tui.save.failed")
	}
	return i18n.T("tui.save.saved")
}

// saveBadge shows the saveStatus of a page in the title of its frame
// (« Réglages  ✔ enregistré »). It belongs to the event loop.
type saveBadge struct {
	page   *widgets.ListDetailPage
	title  string
	status saveStatus
}

// attach binds the badge to a page; title is the title without indicator.
func (b *saveBadge) attach(page *widgets.ListDetailPage, title string) {
	b.page, b.title = page, title
	b.show()
}

// detach forgets the page (Unmount).
func (b *saveBadge) detach() { b.page = nil }

// retitle changes the title of the page, keeping the indicator.
func (b *saveBadge) retitle(title string) {
	b.title = title
	b.show()
}

// set records the status and shows it.
func (b *saveBadge) set(s saveStatus) {
	b.status = s
	b.show()
}

// settle shows where a team page stands: unpublished while dirty, saved once
// it was published; a page that was never edited says nothing.
func (b *saveBadge) settle(dirty bool) {
	switch {
	case dirty:
		b.set(saveStatusUnpublished)
	case b.status != saveStatusNone:
		b.set(saveStatusSaved)
	}
}

func (b *saveBadge) show() {
	if b.page == nil {
		return
	}
	if b.status == saveStatusNone {
		b.page.SetTitle(b.title)
		return
	}
	b.page.SetTitle(b.title + "  " + b.status.text())
}
