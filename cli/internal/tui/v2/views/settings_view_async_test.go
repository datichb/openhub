package views

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
)

// The tool client is asked off the event loop: the mount returns with the
// loading placeholder while it is blocked, and the list comes after.
func TestSettingsViewMountDoesNotBlockOnTheToolVersion(t *testing.T) {
	frLocale(t)
	started, release := make(chan struct{}), make(chan struct{})
	cfg := &config.Config{}
	v := NewSettingsView(SettingsViewConfig{
		GetConfig: func() *config.Config { return cfg },
		ToolVersion: func() string {
			close(started)
			<-release
			return "2.0.20"
		},
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	app := runApp(t, content)

	onLoop(app, func() { v.Mount(content, app) })
	<-started
	onLoop(app, func() {
		assert.Nil(t, v.list, "the list waits for the tool client")
		assert.Equal(t, 1, content.GetItemCount(), "the loading placeholder is shown")
	})

	close(release)
	require.Eventually(t, func() bool {
		var ok bool
		onLoop(app, func() { ok = v.list != nil && len(v.fields) > 0 })
		return ok
	}, 2*time.Second, 10*time.Millisecond)
	onLoop(app, func() {
		f := settingsField(t, v, "exec_tool_version")
		assert.Contains(t, f.Source(), "2.0.20", "the version read at mount is shown")
	})
}

// The tool client read by a previous mount does not reach the new one, nor
// does its list reach the old content.
func TestSettingsViewStaleLoadAfterRemountIsIgnored(t *testing.T) {
	frLocale(t)
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	cfg := &config.Config{}
	v := NewSettingsView(SettingsViewConfig{
		GetConfig: func() *config.Config { return cfg },
		ToolVersion: func() string {
			if calls.Add(1) == 1 {
				close(started)
				<-release
				return "1.0.0"
			}
			return "2.0.20"
		},
	})
	v.SetShell(&mockShell{})
	app := runApp(t, tview.NewFlex())

	first := tview.NewFlex()
	onLoop(app, func() { v.Mount(first, app) })
	<-started
	onLoop(app, v.Unmount)
	second := tview.NewFlex()
	onLoop(app, func() { v.Mount(second, app) })
	require.Eventually(t, func() bool {
		var ok bool
		onLoop(app, func() { ok = v.list != nil })
		return ok
	}, 2*time.Second, 10*time.Millisecond)

	close(release)
	time.Sleep(50 * time.Millisecond)
	onLoop(app, func() {
		assert.Equal(t, "2.0.20", v.tool.client)
		assert.Equal(t, 1, first.GetItemCount(), "the old content keeps its placeholder only")
	})
}
