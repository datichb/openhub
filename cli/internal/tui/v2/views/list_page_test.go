package views

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// The detail of a config field shows its label, value, description, key,
// allowed values, lock and keys.
func TestConfigFieldDetail(t *testing.T) {
	prev := i18n.Locale()
	i18n.SetLocale("en")
	t.Cleanup(func() { i18n.SetLocale(prev) })
	val := "tab"
	f := configField{
		Key: "session_iterm_style", Kind: CfgFieldSelect, Label: "iTerm style",
		Description: "How sessions open in iTerm2",
		Options:     []SelectOption{{Label: "Tab", Value: "tab"}, {Label: "Split", Value: "split"}},
		Get:         func() string { return val },
		Source:      func() string { return "[team]" },
	}
	text := stripTags(configFieldDetail(f, "u", "undo"))
	for _, want := range []string{"iTerm style", "How sessions open in iTerm2", "session_iterm_style",
		i18n.T("tui.detail.options"), "Tab", "Split", "[team]", "Enter", i18n.T("tui.hints.edit"), "undo"} {
		assert.Contains(t, text, want)
	}
	assert.NotContains(t, text, i18n.T("tui.detail.locked"))

	f.Locked = func() bool { return true }
	text = stripTags(configFieldDetail(f))
	assert.Contains(t, text, i18n.T("tui.detail.locked"))
	assert.NotContains(t, text, "Enter", "a locked field offers no edit key")

	b := configField{Key: "auto_cleanup", Kind: CfgFieldBool, Label: "Cleanup", Get: func() string { return "true" }}
	assert.Contains(t, stripTags(configFieldDetail(b)), "Space")
}

// stripTags returns the text of s as displayed (colour tags removed).
func stripTags(s string) string {
	tv := tview.NewTextView().SetDynamicColors(true)
	tv.SetText(s)
	return tv.GetText(true)
}

// The page frames only the list, in the colour of the active mode.
func TestListPageFramesTheList(t *testing.T) {
	page := newListPage("Settings", widgets.NewSectionedList())
	assert.Equal(t, theme.ActiveMode.Primary, page.Left.GetBorderColor())
	assert.Equal(t, theme.FgMuted, page.Detail.GetBorderColor())
	assert.Equal(t, " "+i18n.T("tui.catalog.detail")+" ", page.Detail.GetTitle())
	content := tview.NewFlex()
	page.Mount(content)
	require.Equal(t, 1, content.GetItemCount())
	assert.Equal(t, page.Root, content.GetItem(0))
}

// The Settings view fills the detail pane with the field under the cursor.
func TestSettingsViewDetailFollowsSelection(t *testing.T) {
	cfg := &config.Config{}
	v := NewSettingsView(SettingsViewConfig{GetConfig: func() *config.Config { return cfg }})
	v.live = cfg
	v.list = widgets.NewSectionedList()
	v.page = newListPage(v.Title(), v.list)
	v.list.SetItemChangedFunc(func(_ int, it widgets.SectionItem) {
		showFieldItemDetail(v.page, v.fields, it, settingsExtraKeys()...)
	})
	v.buildFields()
	v.renderFields()

	_, it, ok := v.list.CurrentItem()
	require.True(t, ok)
	f, ok := fieldOfItem(v.fields, it)
	require.True(t, ok)
	assert.Contains(t, v.page.Detail.GetText(true), f.Label)

	for i, item := range v.list.GetItems() {
		if g, ok := fieldOfItem(v.fields, item); ok && g.Key == "language" {
			v.list.SelectIndex(i)
		}
	}
	text := v.page.Detail.GetText(true)
	assert.Contains(t, text, i18n.T("tui.config.field.language.desc"))
	assert.Contains(t, text, "Français")
}

// The doctor view lists the checks; the detail shows the selected one.
func TestDoctorViewDetail(t *testing.T) {
	v := NewDoctorView(nil)
	v.summary, v.list = tview.NewTextView(), widgets.NewSectionedList()
	v.page = newListPage(v.Title(), v.list)
	v.list.SetItemChangedFunc(func(_ int, it widgets.SectionItem) { v.showDetail(it) })
	v.checks = []DoctorCheck{{Name: "git", OK: true, Detail: "git 2.45"}, {Name: "bd", OK: false, Detail: "not installed"}}
	v.render()
	assert.Contains(t, v.page.Detail.GetText(true), "git 2.45")
	v.list.SelectIndex(1)
	text := v.page.Detail.GetText(true)
	assert.Contains(t, text, "not installed")
	assert.Contains(t, text, i18n.T("tui.detail.failed"))
}

// The notifications view shows the full message of the selected entry.
func TestNotificationsViewDetail(t *testing.T) {
	v := NewNotificationsView(NotificationsViewConfig{})
	v.list = widgets.NewSectionedList()
	v.page = newListPage(v.Title(), v.list)
	v.entries, v.loaded = []NotificationEntry{{TimeLabel: "12/10 10:00:00", Level: 1, Message: "push failed: remote rejected"}}, true
	v.render()
	text := v.page.Detail.GetText(true)
	assert.Contains(t, text, "push failed: remote rejected")
	assert.Contains(t, text, "12/10 10:00:00")
}
