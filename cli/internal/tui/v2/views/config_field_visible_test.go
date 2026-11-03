package views

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

func listText(l *widgets.SectionedList) string {
	var b strings.Builder
	for _, it := range l.GetItems() {
		b.WriteString(it.MainText + "\n")
	}
	return b.String()
}

// The field Visible of a configField is applied by the lists: a field that is
// not visible is not drawn, the other ones keep their reference.
func TestConfigFieldVisibleIsAppliedByTheList(t *testing.T) {
	frLocale(t)
	shown := false
	fields := []configField{
		{Key: "a", Kind: CfgFieldString, Label: "Alpha", Get: func() string { return "1" }},
		{Key: "b", Kind: CfgFieldString, Label: "Bravo", Get: func() string { return "2" }, Visible: func() bool { return shown }},
		{Key: "c", Kind: CfgFieldString, Label: "Charlie", Get: func() string { return "3" }},
	}

	items := configItems(fields, 28)
	require.Len(t, items, 2)
	assert.Contains(t, items[0].MainText, "Alpha")
	assert.Contains(t, items[1].MainText, "Charlie")
	assert.Equal(t, 2, items[1].Reference, "the reference is the index in the fields, hidden ones included")

	shown = true
	assert.Len(t, configItems(fields, 28), 3)
}

// Team detail: the server of the team token means nothing for Jira (it is a
// GitLab rule), the line is not drawn; it is for GitLab.
func TestTeamDetailHidesTheTokenServerForJira(t *testing.T) {
	for _, tt := range []struct {
		trackerType string
		shown       bool
	}{{"gitlab", true}, {"jira", false}} {
		e := newTokenEnv(t, tt.trackerType)
		e.v.list = widgets.NewSectionedList()
		e.v.renderFields()
		text := listText(e.v.list)
		assert.Contains(t, text, "Jeton du tracker", tt.trackerType)
		assert.Equal(t, tt.shown, strings.Contains(text, "Serveur du jeton (équipe)"), tt.trackerType)
	}

	// the type changed in the view changes what is drawn
	e := newTokenEnv(t, "gitlab")
	e.v.list = widgets.NewSectionedList()
	e.v.teamCfg.Tracker.Type = "jira"
	e.v.renderFields()
	assert.NotContains(t, listText(e.v.list), "Serveur du jeton (équipe)")
}

// settingsTrackerView mounts the settings with a tracker of the given type
// and returns the text of its list, read on the event loop.
func settingsTrackerView(t *testing.T, trackerType string) (*SettingsView, func() string) {
	t.Helper()
	frLocale(t)
	shared := seedHome(t)
	v := NewSettingsView(SettingsViewConfig{
		GetConfig:    func() *config.Config { return shared },
		UpdateConfig: hubHost{shared}.update,
		TrackerType:  func(context.Context, config.TeamConfig) string { return trackerType },
	})
	v.SetShell(&askShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return v.list != nil && len(v.fields) > 0 })
	return v, func() string {
		var text string
		onLoop(app, func() { text = listText(v.list) })
		return text
	}
}

// Settings: « Token Tracker » has the server of the key next to it, written
// to tracker_token_url of hub.toml.
func TestSettingsTrackerTokenServer(t *testing.T) {
	v, text := settingsTrackerView(t, "gitlab")
	f := settingsField(t, v, "tracker_token_url")
	assert.Equal(t, ScopeHub, f.Scope)
	require.Error(t, f.Validator.Validate("not a server"))
	require.NoError(t, f.Validator.Validate("https://gitlab.forge.example"))
	require.NoError(t, f.Validator.Validate(""))
	assert.Contains(t, text(), "Serveur du jeton")

	v.pushUndo()
	f.Set("https://gitlab.forge.example")
	v.scheduleAutoSave()
	v.doSave() // the write runs off the event loop
	require.Eventually(t, func() bool {
		got, err := config.Read()
		return err == nil && got.Tracker.TrackerTokenURL == "https://gitlab.forge.example"
	}, 5*time.Second, 10*time.Millisecond)
}

// The server of the key is a GitLab rule: nothing is drawn for it when the
// tracker is Jira, and an unknown tracker keeps the line.
func TestSettingsHidesTheTokenServerForJira(t *testing.T) {
	_, text := settingsTrackerView(t, "jira")
	assert.NotContains(t, text(), "Serveur du jeton")
	assert.Contains(t, text(), "Token Tracker", "the key stays")

	_, text = settingsTrackerView(t, "")
	assert.Contains(t, text(), "Serveur du jeton")
}
