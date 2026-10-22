package views

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Two quick reloads whose answers come back in the wrong order: the list
// shows the answer of the last request, not the slowest one.
func TestHistoryViewStaleReloadIsIgnored(t *testing.T) {
	var calls atomic.Int32
	release, firstStarted := make(chan struct{}), make(chan struct{})
	v := NewHistoryView(HistoryViewConfig{
		Ref: "team:hotfix",
		Load: func(context.Context) ([]HistoryVersion, error) {
			if calls.Add(1) == 1 {
				close(firstStarted)
				<-release
				return []HistoryVersion{{Version: 1, By: "old"}}, nil
			}
			return []HistoryVersion{{Version: 2, By: "new", Current: true}}, nil
		},
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	<-firstStarted
	onLoop(app, func() { v.HandleKey(key('r')) }) // second request

	require.Eventually(t, func() bool {
		var n int
		onLoop(app, func() { n = len(v.versions) })
		return n == 1
	}, 2*time.Second, 10*time.Millisecond)
	close(release) // the first answer arrives last
	time.Sleep(50 * time.Millisecond)
	onLoop(app, func() {
		require.Len(t, v.versions, 1)
		assert.Equal(t, 2, v.versions[0].Version, "the answer of the last request stays")
	})
}

// A load that answers after Unmount does not touch the view.
func TestHistoryViewLoadAfterUnmountIsIgnored(t *testing.T) {
	release, started, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	v := NewHistoryView(HistoryViewConfig{
		Ref: "team:hotfix",
		Load: func(context.Context) ([]HistoryVersion, error) {
			close(started)
			<-release
			defer close(done)
			return []HistoryVersion{{Version: 7}}, nil
		},
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	<-started
	onLoop(app, v.Unmount)
	close(release)
	<-done
	time.Sleep(50 * time.Millisecond)
	onLoop(app, func() { assert.Empty(t, v.versions) })
}
