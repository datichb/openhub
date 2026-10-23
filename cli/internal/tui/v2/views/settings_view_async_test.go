package views

import (
	"errors"
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

// mountedSettings mounts the view on a running app and waits for its list.
func mountedSettings(t *testing.T, cfg SettingsViewConfig) (v *SettingsView, loop *tview.Application, sh *trustShell, live *config.Config) {
	t.Helper()
	live = &config.Config{Name: "live"}
	cfg.GetConfig = func() *config.Config { return live }
	v = NewSettingsView(cfg)
	sh = &trustShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	loop = runApp(t, content)
	onLoop(loop, func() { v.Mount(content, loop) })
	require.Eventually(t, func() bool {
		var ok bool
		onLoop(loop, func() { ok = v.list != nil && len(v.fields) > 0 })
		return ok
	}, 2*time.Second, 10*time.Millisecond)
	return v, loop, sh, live
}

func settingsToasts(loop *tview.Application, sh *trustShell) (out []string) {
	onLoop(loop, func() { out = append(out, sh.toasts...) })
	return
}

// r reads hub.toml off the event loop: the loop stays free while the disk is
// slow, the fields cannot be edited meanwhile, and the live config is replaced
// on the loop once the file is read.
func TestSettingsViewRefreshReadsOffTheLoop(t *testing.T) {
	frLocale(t)
	var reads atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	v, loop, sh, live := mountedSettings(t, SettingsViewConfig{
		ReadConfig: func() (*config.Config, error) {
			reads.Add(1)
			close(started)
			<-release
			return &config.Config{Name: "from disk"}, nil
		},
	})

	onLoop(loop, func() { v.HandleKey(key('r')) }) // returns although the disk waits
	<-started
	onLoop(loop, func() {
		assert.True(t, v.reading)
		assert.Equal(t, "live", live.Name, "nothing is replaced before the read ends")
		v.HandleKey(key('r')) // a second request while reading is ignored
		v.HandleKey(key('u'))
		v.HandleKey(key(' ')) // and so are the edits
	})
	assert.Empty(t, settingsToasts(loop, sh))

	close(release)
	require.Eventually(t, func() bool { return len(settingsToasts(loop, sh)) == 1 }, 2*time.Second, 10*time.Millisecond)
	onLoop(loop, func() {
		assert.False(t, v.reading)
		assert.Equal(t, "from disk", live.Name, "the live config is replaced on the loop")
	})
	assert.Equal(t, int32(1), reads.Load())
}

// A read that fails keeps the config and says so.
func TestSettingsViewRefreshFailureKeepsTheConfig(t *testing.T) {
	frLocale(t)
	v, loop, sh, live := mountedSettings(t, SettingsViewConfig{
		ReadConfig: func() (*config.Config, error) { return nil, errors.New("hub.toml unreadable") },
	})
	onLoop(loop, func() { v.HandleKey(key('r')) })
	require.Eventually(t, func() bool { return len(settingsToasts(loop, sh)) == 1 }, 2*time.Second, 10*time.Millisecond)
	ts := settingsToasts(loop, sh)
	assert.Contains(t, ts[0], "hub.toml unreadable")
	onLoop(loop, func() {
		assert.False(t, v.reading, "the view can be edited again")
		assert.Equal(t, "live", live.Name)
	})
}

// A read started by a previous mount does not reach the new one.
func TestSettingsViewRefreshAfterRemountIsIgnored(t *testing.T) {
	frLocale(t)
	started, release := make(chan struct{}), make(chan struct{})
	v, loop, sh, live := mountedSettings(t, SettingsViewConfig{
		ReadConfig: func() (*config.Config, error) {
			close(started)
			<-release
			return &config.Config{Name: "from disk"}, nil
		},
	})
	onLoop(loop, func() { v.HandleKey(key('r')) })
	<-started
	onLoop(loop, v.Unmount)
	second := tview.NewFlex()
	onLoop(loop, func() { v.Mount(second, loop) })
	require.Eventually(t, func() bool {
		var ok bool
		onLoop(loop, func() { ok = v.list != nil })
		return ok
	}, 2*time.Second, 10*time.Millisecond)

	close(release)
	time.Sleep(50 * time.Millisecond)
	assert.Empty(t, settingsToasts(loop, sh), "no « reloaded » from a read of the previous mount")
	onLoop(loop, func() {
		assert.False(t, v.reading)
		assert.Equal(t, "live", live.Name)
	})
}

// Without ReadConfig (a host that only knows ReloadConfig) the refresh still
// works, on the loop.
func TestSettingsViewRefreshFallsBackOnReloadConfig(t *testing.T) {
	frLocale(t)
	var reloaded atomic.Int32
	v, loop, sh, _ := mountedSettings(t, SettingsViewConfig{
		ReloadConfig: func() *config.Config { reloaded.Add(1); return &config.Config{Name: "reloaded"} },
	})
	onLoop(loop, func() { v.HandleKey(key('r')) })
	assert.Equal(t, int32(1), reloaded.Load())
	assert.Len(t, settingsToasts(loop, sh), 1)
}
