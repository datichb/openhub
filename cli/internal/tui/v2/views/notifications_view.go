package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/textfmt"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// NotificationEntry is a display-agnostic representation of a past notification.
// It decouples the view from the shell package (avoiding an import cycle).
type NotificationEntry struct {
	// TimeLabel is a pre-formatted time string (e.g. "24/07 14:32:05").
	TimeLabel string
	// Level is the severity: 0=Success, 1=Error, 2=Warning, 3=Info
	// (mirrors shell.ToastLevel ordering).
	Level int
	// Message is the full, untruncated notification text.
	Message string
}

// NotificationsProvider is the minimal interface for in-memory entries.
type NotificationsProvider interface {
	Entries() []NotificationEntry
}

// NotificationsViewConfig holds the configuration for the NotificationsView.
// Separating these from the constructor keeps the views package free of imports
// from the shell package (which imports views — cycle avoidance).
type NotificationsViewConfig struct {
	// FilePath is the path to the persistent JSONL notifications file.
	// When empty, the view only shows the current session's notifications.
	FilePath string
	// ReadLastN reads the last n entries from the persistent file.
	// Signature matches shell.ReadLastN so it can be passed directly.
	ReadLastN func(path string, n int) ([]NotificationEntry, error)
}

// NotificationsView displays the full notification history.
// On every Mount it asynchronously loads the last 50 entries from the
// persistent JSONL file (cross-session history) and displays them.
type NotificationsView struct {
	cfg     NotificationsViewConfig
	list    *widgets.SectionedList
	page    *widgets.ListDetailPage
	app     *tview.Application
	entries []NotificationEntry
	loaded  bool
	loadErr error
	loads   loadSlot // the reads of the file: only the latest is applied
	ctx     context.Context
	cancel  context.CancelFunc
}

var _ View = (*NotificationsView)(nil)

// NewNotificationsView creates the notifications view.
// cfg.ReadLastN may be nil if only in-memory display is desired.
func NewNotificationsView(cfg NotificationsViewConfig) *NotificationsView {
	return &NotificationsView{cfg: cfg}
}

// ID returns the view identifier.
func (v *NotificationsView) ID() string { return "notifications" }

// Title returns the display title.
func (v *NotificationsView) Title() string { return i18n.T("tui.notifications.title") }

// StatusHints returns keybinding hints.
func (v *NotificationsView) StatusHints() string {
	return keys.Hints(keys.Navigate.HintAs(i18n.T("tui.hints.nav")), "Ctrl+P "+i18n.T("tui.hints.commands"))
}

// Mount builds and populates the notifications display.
// Content is loaded asynchronously from the persistent file to avoid blocking
// the TUI event loop.
func (v *NotificationsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.loaded, v.loadErr = false, nil
	v.ctx, v.cancel = context.WithCancel(context.Background())
	v.list = widgets.NewSectionedList()
	v.list.SetApp(app)
	v.list.SetItemChangedFunc(func(_ int, it widgets.SectionItem) { v.showDetail(it) })
	v.page = newListPage(v.Title(), v.list)
	v.page.Mount(content)
	v.render()
	if app != nil {
		app.SetFocus(v.list)
	}

	// Load entries from the persistent file off the event loop.
	load(app, v.ctx, &v.loads, v.loadEntries, func(entries []NotificationEntry, err error) {
		if v.list == nil {
			return // view was unmounted before the read finished
		}
		v.entries, v.loaded, v.loadErr = entries, true, err
		v.render()
	})
}

// loadEntries fetches entries from the persistent file (most recent 50).
func (v *NotificationsView) loadEntries(context.Context) ([]NotificationEntry, error) {
	if v.cfg.ReadLastN == nil || v.cfg.FilePath == "" {
		return nil, nil
	}
	return v.cfg.ReadLastN(v.cfg.FilePath, 50)
}

// Unmount cleans up resources.
func (v *NotificationsView) Unmount() {
	v.loads.Invalidate()
	if v.cancel != nil {
		v.cancel()
	}
	v.list, v.page = nil, nil
	v.app = nil
}

// HandleKey processes view-specific key events.
func (v *NotificationsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	return event
}

// ── Rendering ─────────────────────────────────────────────────────────────────

// render fills the list with the entries (already most-recent-first).
func (v *NotificationsView) render() {
	if v.list == nil {
		return
	}
	items := make([]widgets.SectionItem, 0, len(v.entries))
	for i, e := range v.entries {
		icon, color := notificationStyle(e.Level)
		msg := strings.ReplaceAll(e.Message, "\n", " ")
		items = append(items, widgets.SectionItem{
			MainText:  fmt.Sprintf("%s  %s  %s", muted(e.TimeLabel), theme.Paint(color, icon), textfmt.Clip(msg, 120)),
			Reference: i,
		})
	}
	if len(items) == 0 {
		state := stateText(stateLoading, i18n.T("tui.notifications.loading"))
		switch {
		case v.loadErr != nil:
			state = stateText(stateError, v.loadErr.Error())
		case v.loaded:
			state = stateText(stateEmpty, i18n.T("tui.notifications.empty"))
		}
		items = append(items, widgets.SectionItem{MainText: state, IsHeader: true})
	}
	v.list.SetItems(items)
	_, it, _ := v.list.CurrentItem()
	v.showDetail(it)
}

// showDetail fills the detail pane with the full notification of it.
func (v *NotificationsView) showDetail(it widgets.SectionItem) {
	if v.page == nil {
		return
	}
	i, ok := it.Reference.(int)
	if !ok || i < 0 || i >= len(v.entries) {
		v.page.SetDetail(emptyDetail())
		return
	}
	e := v.entries[i]
	icon, color := notificationStyle(e.Level)
	var d detailText
	d.title(theme.Paint(color, icon) + " " + notificationLevelLabel(e.Level))
	d.field(i18n.T("tui.detail.date"), e.TimeLabel)
	d.gap().para(e.Message)
	v.page.SetDetail(d.String())
}

// notificationLevelLabel is the translated label of a level integer.
func notificationLevelLabel(level int) string {
	switch level {
	case 0:
		return i18n.T("tui.detail.ok")
	case 1:
		return i18n.T("tui.detail.failed")
	case 2:
		return i18n.T("tui.detail.warn")
	default:
		return i18n.T("tui.detail.info")
	}
}

// notificationStyle returns the icon and the color (hex) for a level integer.
// Level mirrors shell.ToastLevel: 0=Success, 1=Error, 2=Warning, 3=Info.
func notificationStyle(level int) (icon, hex string) {
	switch level {
	case 0:
		return theme.IconSuccess, theme.SuccessHex
	case 1:
		return theme.IconError, theme.ErrorHex
	case 2:
		return theme.IconWarning, theme.WarningHex
	default:
		return theme.IconArrow, theme.ActiveMode.PrimaryHex
	}
}
