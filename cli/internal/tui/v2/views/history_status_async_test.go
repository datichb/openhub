package views

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Before the first answer the history says it is loading (not that it is
// empty), a failure is shown in the list, and the diff of a version is read
// off the event loop: its failure is shown in the modal.
func TestHistoryViewStatesAndDiffFailure(t *testing.T) {
	frLocale(t)
	started, release := make(chan struct{}), make(chan struct{})
	v := NewHistoryView(HistoryViewConfig{
		Ref: "team:hotfix",
		Load: func(context.Context) ([]HistoryVersion, error) {
			close(started)
			<-release
			return []HistoryVersion{{Version: 2, By: "ben", Current: true}}, nil
		},
		Diff: func(context.Context, int) (string, error) { return "", errors.New("git indisponible") },
	})
	sh := &formShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	items := func() (txt string) {
		onLoop(app, func() {
			var parts []string
			for _, it := range v.list.GetItems() {
				parts = append(parts, it.MainText)
			}
			txt = strings.Join(parts, "\n")
		})
		return
	}
	onLoop(app, func() { v.Mount(content, app) })
	<-started
	assert.Contains(t, items(), "Chargement")
	assert.NotContains(t, items(), "Aucune version")
	close(release)
	require.Eventually(t, func() bool { return strings.Contains(items(), "v2") }, 2*time.Second, 10*time.Millisecond)

	onLoop(app, func() { v.showDiff(2) })
	require.Eventually(t, func() bool {
		var body string
		onLoop(app, func() { body = sh.body })
		return strings.Contains(body, "Impossible de charger : git indisponible")
	}, 2*time.Second, 10*time.Millisecond)
}

// The status of the hub is read in load(): the loading text first, then the
// status; a status asked before an unmount is not applied to the next mount.
func TestStatusViewLoadAndRemount(t *testing.T) {
	frLocale(t)
	v := NewStatusView(nil)
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	text := func() (txt string) {
		onLoop(app, func() {
			if v.tv != nil {
				txt = v.tv.GetText(true)
			}
		})
		return
	}
	onLoop(app, func() { v.Mount(content, app) })
	require.Eventually(t, func() bool { return strings.Contains(text(), "OpenHub") }, 2*time.Second, 10*time.Millisecond)
	onLoop(app, v.Unmount)
	onLoop(app, func() { v.Mount(tview.NewFlex(), app) })
	require.Eventually(t, func() bool { return strings.Contains(text(), "OpenHub") }, 2*time.Second, 10*time.Millisecond)
	onLoop(app, func() { assert.Nil(t, v.HandleKey(key('r'))); assert.Contains(t, v.tv.GetText(true), "Chargement") })
	require.Eventually(t, func() bool { return strings.Contains(text(), "OpenHub") }, 2*time.Second, 10*time.Millisecond)
}
