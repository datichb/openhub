package views

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/config"
)

// countingShell records the toasts of a view.
type countingShell struct {
	mockShell
	toasts atomic.Int32
}

func (s *countingShell) ShowToastMsg(string, bool) { s.toasts.Add(1) }

// A view that finishes loading while the palette (or a window) holds the
// focus leaves it there; with the focus in its own content it takes it.
func TestLoadEndKeepsTheFocusOfThePaletteOrAWindow(t *testing.T) {
	for _, tc := range []struct {
		name       string
		steal      bool
		wantOnList bool
	}{
		{"focus in the content", false, true},
		{"focus in the palette", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frLocale(t)
			started, release := make(chan struct{}), make(chan struct{})
			live := &config.Config{}
			v := NewSettingsView(SettingsViewConfig{
				GetConfig: func() *config.Config { return live },
				TrackerType: func(context.Context, config.TeamConfig) string {
					close(started)
					<-release
					return ""
				},
			})
			v.SetShell(&mockShell{})
			content := tview.NewFlex()
			loop := runApp(t, content)
			palette := tview.NewInputField()
			onLoop(loop, func() { v.Mount(content, loop); loop.SetFocus(content) })
			<-started
			if tc.steal {
				onLoop(loop, func() { loop.SetFocus(palette) })
			}
			close(release)
			eventually(t, loop, func() bool { return v.list != nil })
			onLoop(loop, func() {
				if tc.wantOnList {
					assert.Equal(t, tview.Primitive(v.list), loop.GetFocus(), "the list takes the focus")
				} else {
					assert.Equal(t, tview.Primitive(palette), loop.GetFocus(), "the palette keeps the focus")
				}
			})
		})
	}
}

func soloDetail(t *testing.T, shell ShellAccess, saves *atomic.Int32) *TeamDetailView {
	t.Helper()
	_, tc := soloState(t)
	v := NewTeamDetailView(TeamDetailViewConfig{
		GetMCPConfig:          func() config.MCPConfig { return config.MCPConfig{} },
		GetTrackerLocalConfig: func() config.TrackerLocalConfig { return config.TrackerLocalConfig{} },
		ResolveTeam:           func() TeamResolution { return tc },
		SaveTeamConfig: func() TeamConfigWriter {
			saves.Add(1)
			return nil
		},
	})
	v.SetShell(shell)
	return v
}

// Edits left unpublished at the exit of the team detail are said once; the next
// visit starts clean (no flag, no undo) and leaves without a toast.
func TestTeamDetailMountForgetsThePreviousVisit(t *testing.T) {
	frLocale(t)
	sh := &countingShell{}
	var saves atomic.Int32
	v := soloDetail(t, sh, &saves)
	content := tview.NewFlex()
	loop := runApp(t, content)

	onLoop(loop, func() { v.Mount(content, loop) })
	eventually(t, loop, func() bool { return len(v.fields) > 0 })
	onLoop(loop, func() {
		v.dirtyTeam, v.dirtyLocal = true, true
		v.pushUndo()
		v.Unmount()
	})
	assert.EqualValues(t, 1, sh.toasts.Load(), "the edits left are said")

	onLoop(loop, func() { v.Mount(content, loop) })
	eventually(t, loop, func() bool { return len(v.fields) > 0 })
	onLoop(loop, func() {
		assert.False(t, v.dirtyTeam || v.dirtyLocal, "no flag from the previous visit")
		assert.Zero(t, v.undoStack.Len(), "no undo from the previous visit")
		v.Unmount()
	})
	assert.EqualValues(t, 1, sh.toasts.Load(), "a visit without edit leaves without a toast")
}

// With no team to resolve, the detail says so, offers the way back and
// saves nothing.
func TestTeamDetailWithoutTeamIsEmptyAndSavesNothing(t *testing.T) {
	frLocale(t)
	var saves atomic.Int32
	v := NewTeamDetailView(TeamDetailViewConfig{
		GetMCPConfig:          func() config.MCPConfig { return config.MCPConfig{} },
		GetTrackerLocalConfig: func() config.TrackerLocalConfig { return config.TrackerLocalConfig{} },
		ResolveTeam:           func() TeamResolution { return TeamResolution{} },
		SaveTeamConfig: func() TeamConfigWriter {
			saves.Add(1)
			return nil
		},
	})
	sh := &countingShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	loop := runApp(t, content)
	onLoop(loop, func() { v.Mount(content, loop) })

	out := drawn(t, loop, content, 120, 30)
	assert.Contains(t, out, "Aucune équipe à configurer")
	assert.Contains(t, out, "Échap")
	onLoop(loop, func() {
		assert.Contains(t, v.StatusHints(), "Échap")
		v.dirtyTeam = true
		ev := tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModCtrl)
		assert.Equal(t, ev, v.HandleKey(ev), "Ctrl+S is not taken")
		v.save()
	})
	assert.Zero(t, saves.Load(), "nothing is saved")
	onLoop(loop, func() { v.Unmount() })
}
