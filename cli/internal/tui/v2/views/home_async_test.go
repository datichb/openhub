package views

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
)

// The landings read their data in load(): the mount returns at once with the
// static items, the answer of a read made before an unmount is not applied to
// the next mount, and the view can be mounted again (run with -race).

func homeLabels(v *HomeView) (labels []string) {
	for _, it := range v.items {
		labels = append(labels, it.Label)
	}
	return labels
}

func TestHomeViewLoadAndRemount(t *testing.T) {
	frLocale(t)
	var calls atomic.Int32
	first, release := make(chan struct{}), make(chan struct{})
	v := NewHomeView(HomeViewConfig{
		ListProjects: func() []ProjectEntry {
			if calls.Add(1) == 1 {
				close(first)
				<-release
				return []ProjectEntry{{ID: "old", Name: "ancien-projet"}}
			}
			return []ProjectEntry{{ID: "new", Name: "nouveau-projet"}}
		},
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)

	onLoop(loop, func() { v.Mount(content, loop) })
	<-first
	onLoop(loop, func() {
		assert.NotEmpty(t, v.items, "the static items are there before the projects")
		assert.NotContains(t, homeLabels(v), "ancien-projet")
	})

	onLoop(loop, v.Unmount)
	second := tview.NewFlex()
	onLoop(loop, func() { v.Mount(second, loop) })
	eventually(t, loop, func() bool { return contains(homeLabels(v), "nouveau-projet") })

	close(release)
	time.Sleep(50 * time.Millisecond)
	onLoop(loop, func() {
		assert.NotContains(t, homeLabels(v), "ancien-projet", "the answer of the previous mount is dropped")
		assert.Contains(t, homeLabels(v), "nouveau-projet")
	})
}

func TestHomeViewReadAfterUnmountIsIgnored(t *testing.T) {
	frLocale(t)
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	v := NewHomeView(HomeViewConfig{
		ListProjects: func() []ProjectEntry {
			close(started)
			<-release
			defer close(done)
			return []ProjectEntry{{ID: "p", Name: "tard-venu"}}
		},
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	loop := runApp(t, content)
	onLoop(loop, func() { v.Mount(content, loop) })
	<-started
	onLoop(loop, v.Unmount)
	close(release)
	<-done
	time.Sleep(50 * time.Millisecond)
	onLoop(loop, func() { assert.NotContains(t, homeLabels(v), "tard-venu") })
}

// The stats of the team landing are read off the loop: the header says
// « loading » until they arrive, the mode bar is redrawn with them, and an
// answer that comes after an unmount changes nothing.
func TestTeamModeViewStatsLoadAndRemount(t *testing.T) {
	frLocale(t)
	var calls atomic.Int32
	first, release := make(chan struct{}), make(chan struct{})
	v := NewTeamModeView(TeamModeConfig{
		TeamStats: func() TeamModeStats {
			if calls.Add(1) == 1 {
				close(first)
				<-release
				return TeamModeStats{MemberCount: 9, ActiveCount: 9}
			}
			return TeamModeStats{MemberCount: 3, ActiveCount: 2}
		},
	})
	sh := &modeBarShell{mockShell: mockShell{team: &ActiveTeam{ID: "t1", Name: "alpha"}}}
	v.SetShell(sh)
	content := tview.NewFlex()
	loop := runApp(t, content)

	onLoop(loop, func() { v.Mount(content, loop) })
	<-first
	onLoop(loop, func() {
		assert.Empty(t, v.ModeInfo().Right, "no stats before the read: the mode bar does not wait for them")
		assert.Nil(t, v.summary)
	})

	onLoop(loop, v.Unmount)
	onLoop(loop, func() { v.Mount(tview.NewFlex(), loop) })
	eventually(t, loop, func() bool { return v.summary != nil })
	onLoop(loop, func() {
		assert.Equal(t, 3, v.summary.MemberCount)
		assert.Contains(t, v.ModeInfo().Right, "3")
		assert.Equal(t, int32(1), sh.refreshed.Load(), "the mode bar is redrawn with the stats")
	})

	close(release)
	time.Sleep(50 * time.Millisecond)
	onLoop(loop, func() { assert.Equal(t, 3, v.summary.MemberCount, "the late answer is dropped") })
}

type modeBarShell struct {
	mockShell
	refreshed atomic.Int32
}

func (s *modeBarShell) RefreshModeBar() { s.refreshed.Add(1) }

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestHomeShortcutsFooter(t *testing.T) {
	frLocale(t)
	hub := homeShortcutsFooter()
	for _, k := range []string{"Ctrl+P", "?", "Ctrl+T", "Ctrl+Q"} {
		assert.Contains(t, hub, k)
	}
	assert.Contains(t, tview.NewTextView().SetDynamicColors(true).SetText(hub).GetText(true), "Ctrl+T vue suivante", "Hub → Projet → Équipe → Hub")
}
