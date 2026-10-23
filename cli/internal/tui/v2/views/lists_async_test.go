package views

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func listTexts(loop *tview.Application, items func() []ActionItem) (txt string) {
	onLoop(loop, func() {
		var parts []string
		for _, it := range items() {
			parts = append(parts, strings.Join(it.Cells, " "))
		}
		txt = strings.Join(parts, "\n")
	})
	return
}

// The brick catalogue is read in load(): « loading » then the list, a
// failure says so, and the answer of a read made before an unmount is not
// applied to the next mount (run with -race).
func TestBricksViewLoadStatesAndRemount(t *testing.T) {
	frLocale(t)
	var calls atomic.Int32
	first, release := make(chan struct{}), make(chan struct{})
	v := NewBricksView(BricksViewConfig{Load: func(context.Context) ([]BrickRow, error) {
		switch calls.Add(1) {
		case 1:
			close(first)
			<-release
			return []BrickRow{{Kind: "agent", ID: "ancien"}}, nil
		case 2:
			return nil, errors.New("catalogue illisible")
		}
		return []BrickRow{{Kind: "agent", ID: "recent", Origin: "hub"}}, nil
	}})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)
	empty := func() (txt string) {
		onLoop(loop, func() {
			if v.list != nil {
				txt = v.list.cfg.Empty
			}
		})
		return
	}
	cells := func() string { return listTexts(loop, func() []ActionItem { return v.list.Items() }) }

	onLoop(loop, func() { v.Mount(content, loop) })
	<-first
	assert.Contains(t, empty(), "Chargement")
	onLoop(loop, v.reload) // a newer read: a failure
	require.Eventually(t, func() bool { return strings.Contains(empty(), "Impossible de charger : catalogue illisible") }, 2*time.Second, 10*time.Millisecond)
	close(release)
	time.Sleep(50 * time.Millisecond)
	assert.NotContains(t, cells(), "ancien", "the older answer is dropped")

	onLoop(loop, v.Unmount)
	onLoop(loop, func() { v.Mount(content, loop) })
	require.Eventually(t, func() bool { return strings.Contains(cells(), "recent") }, 2*time.Second, 10*time.Millisecond)
}

// The notifications are read off the loop from the file: loading, then the
// list; a failure is shown; the answer of an unmounted view is dropped.
func TestNotificationsViewLoadStatesAndRemount(t *testing.T) {
	frLocale(t)
	var calls atomic.Int32
	first, release := make(chan struct{}), make(chan struct{})
	v := NewNotificationsView(NotificationsViewConfig{FilePath: "n.jsonl", ReadLastN: func(string, int) ([]NotificationEntry, error) {
		switch calls.Add(1) {
		case 1:
			close(first)
			<-release
			return []NotificationEntry{{TimeLabel: "10:00", Message: "ancienne"}}, nil
		case 2:
			return nil, errors.New("fichier illisible")
		}
		return []NotificationEntry{{TimeLabel: "10:05", Message: "recente"}}, nil
	}})
	content := tview.NewFlex()
	loop := runApp(t, content)
	text := func() (txt string) {
		onLoop(loop, func() {
			if v.list == nil {
				return
			}
			var parts []string
			for _, it := range v.list.GetItems() {
				parts = append(parts, it.MainText)
			}
			txt = strings.Join(parts, "\n")
		})
		return
	}
	onLoop(loop, func() { v.Mount(content, loop) })
	<-first
	assert.Contains(t, text(), "Chargement")
	onLoop(loop, v.Unmount)
	onLoop(loop, func() { v.Mount(content, loop) }) // the second read: a failure
	require.Eventually(t, func() bool { return strings.Contains(text(), "Impossible de charger : fichier illisible") }, 2*time.Second, 10*time.Millisecond)
	close(release)
	time.Sleep(50 * time.Millisecond)
	assert.NotContains(t, text(), "ancienne", "the answer of the unmounted view is dropped")
	onLoop(loop, v.Unmount)
	onLoop(loop, func() { v.Mount(content, loop) })
	require.Eventually(t, func() bool { return strings.Contains(text(), "recente") }, 2*time.Second, 10*time.Millisecond)
}

// The queue of wiki proposals says « loading » (not « no proposal ») until
// it is read, shows a failure with the common words, and an unmounted view
// does not receive its answer.
func TestProjectWikiQueueLoadStatesAndRemount(t *testing.T) {
	frLocale(t)
	var calls atomic.Int32
	first, release := make(chan struct{}), make(chan struct{})
	now := time.Unix(10_000, 0)
	v := NewProjectWikiView(ProjectWikiViewConfig{
		Project: func() (string, string) { return "atlas", "/p" },
		Now:     func() time.Time { return now },
		Metrics: func(context.Context, string) (ProjectWikiMetrics, error) { return ProjectWikiMetrics{Pages: 3}, nil },
		Proposals: func(context.Context, string) ([]WikiProposalView, error) {
			switch calls.Add(1) {
			case 1:
				close(first)
				<-release
				return []WikiProposalView{{ID: "wp-old", Page: "a.md", Status: "pending", Created: now}}, nil
			case 2:
				return nil, errors.New("file illisible")
			}
			return []WikiProposalView{{ID: "wp-new", Page: "b.md", Status: "pending", Created: now}}, nil
		},
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)
	empty := func() (txt string) {
		onLoop(loop, func() {
			if v.wiki != nil {
				txt = v.wiki.list.cfg.Empty
			}
		})
		return
	}
	cells := func() string {
		return listTexts(loop, func() []ActionItem {
			if v.wiki == nil {
				return nil
			}
			return v.wiki.list.Items()
		})
	}
	onLoop(loop, func() { v.Mount(content, loop) })
	<-first
	assert.Contains(t, empty(), "Chargement")
	onLoop(loop, v.Unmount)
	onLoop(loop, func() { v.Mount(content, loop) })
	require.Eventually(t, func() bool { return strings.Contains(empty(), "Impossible de charger : file illisible") }, 2*time.Second, 10*time.Millisecond)
	close(release)
	time.Sleep(50 * time.Millisecond)
	assert.NotContains(t, cells(), "wp-old", "the answer of the unmounted view is dropped")
	onLoop(loop, func() { v.HandleKey(key('r')) })
	require.Eventually(t, func() bool { return strings.Contains(cells(), "wp-new") }, 2*time.Second, 10*time.Millisecond)
}
