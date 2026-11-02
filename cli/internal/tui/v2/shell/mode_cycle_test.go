package shell

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Ctrl+T cycles Hub → Project → Team → Hub, skipping what is not configured
// (BL-24: the project mode had no shortcut, Ctrl+T only went between the hub
// and the team).

type cycleWorld struct {
	teams    []views.SelectOption
	projects []views.SelectOption
	resolved atomic.Int32
}

func newCycleShell(t *testing.T, w *cycleWorld) *simShell {
	t.Helper()
	cfg := Config{
		ProjectName: "test", HomeViewID: "home",
		Views:            []views.View{&keyView{id: "home"}, &keyView{id: "project.mode"}, &keyView{id: "team.mode"}, &keyView{id: "projects.list"}},
		TeamsProvider:    func() []views.SelectOption { return w.teams },
		ProjectsProvider: func() []views.SelectOption { return w.projects },
		ProjectResolver: func(id string) *views.ActiveProject {
			w.resolved.Add(1)
			for _, p := range w.projects {
				if p.Value == id {
					return &views.ActiveProject{ID: id, Name: p.Label, Path: "/src/" + p.Label, Branch: "main"}
				}
			}
			return nil
		},
	}
	s := runSimShell(t, 120, 40, cfg)
	s.loop(func() { s.NavigateHome("home", views.ModeHub) })
	return s
}

func (s *simShell) mode() (m views.Mode) {
	s.loop(func() { m = s.Mode() })
	return m
}

func (s *simShell) awaitMode(want views.Mode) {
	s.t.Helper()
	require.Eventually(s.t, func() bool { return s.mode() == want }, 3*time.Second, 10*time.Millisecond, "mode %s", want)
}

func TestCtrlTCyclesHubProjectTeamHub(t *testing.T) {
	w := &cycleWorld{
		teams:    []views.SelectOption{{Label: "core", Value: "core"}},
		projects: []views.SelectOption{{Label: "openhub", Value: "p1"}},
	}
	s := newCycleShell(t, w)

	s.key(tcell.KeyCtrlT, 0, tcell.ModNone)
	s.awaitMode(views.ModeProject)
	var p *views.ActiveProject
	s.loop(func() { p = s.ActiveProject() })
	require.NotNil(t, p)
	assert.Equal(t, "openhub", p.Name)
	assert.Equal(t, "main", p.Branch, "completed by the resolver")

	s.key(tcell.KeyCtrlT, 0, tcell.ModNone)
	s.awaitMode(views.ModeTeam)
	var team *views.ActiveTeam
	s.loop(func() { team = s.ActiveTeam() })
	require.NotNil(t, team)
	assert.Equal(t, "core", team.Name)

	s.key(tcell.KeyCtrlT, 0, tcell.ModNone)
	s.awaitMode(views.ModeHub)
}

func TestCtrlTSkipsWhatIsNotConfigured(t *testing.T) {
	t.Run("no project: the hub goes to the team", func(t *testing.T) {
		s := newCycleShell(t, &cycleWorld{teams: []views.SelectOption{{Label: "core", Value: "core"}}})
		s.key(tcell.KeyCtrlT, 0, tcell.ModNone)
		s.awaitMode(views.ModeTeam)
	})
	t.Run("no team: the project goes back to the hub", func(t *testing.T) {
		s := newCycleShell(t, &cycleWorld{projects: []views.SelectOption{{Label: "openhub", Value: "p1"}}})
		s.key(tcell.KeyCtrlT, 0, tcell.ModNone)
		s.awaitMode(views.ModeProject)
		s.key(tcell.KeyCtrlT, 0, tcell.ModNone)
		s.awaitMode(views.ModeHub)
	})
	t.Run("nothing configured: a toast, the hub stays", func(t *testing.T) {
		s := newCycleShell(t, &cycleWorld{})
		s.key(tcell.KeyCtrlT, 0, tcell.ModNone)
		require.Eventually(t, func() bool { return s.toastCount() > 0 }, 3*time.Second, 10*time.Millisecond)
		assert.Equal(t, views.ModeHub, s.mode())
	})
}

// Several projects: the choice is asked once; the next time the hub comes
// back to the project it left.
func TestCtrlTAsksWhichProjectThenRemembersIt(t *testing.T) {
	w := &cycleWorld{
		teams:    []views.SelectOption{{Label: "core", Value: "core"}},
		projects: []views.SelectOption{{Label: "api", Value: "p1"}, {Label: "web", Value: "p2"}},
	}
	s := newCycleShell(t, w)

	s.key(tcell.KeyCtrlT, 0, tcell.ModNone)
	require.Eventually(t, func() bool {
		var overlay bool
		s.loop(func() { overlay = s.hasOverlay() })
		return overlay
	}, 3*time.Second, 10*time.Millisecond, "the choice of the project")
	assert.Equal(t, views.ModeHub, s.mode(), "nothing chosen yet")

	// Choose the second one the way the modal does (it closes, then calls back).
	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	s.loop(func() { s.enterProject("p2") })
	s.awaitMode(views.ModeProject)
	s.key(tcell.KeyCtrlT, 0, tcell.ModNone) // → team
	s.awaitMode(views.ModeTeam)
	s.key(tcell.KeyCtrlT, 0, tcell.ModNone) // → hub
	s.awaitMode(views.ModeHub)

	before := w.resolved.Load()
	s.key(tcell.KeyCtrlT, 0, tcell.ModNone) // → the project it left, without asking
	s.awaitMode(views.ModeProject)
	var p *views.ActiveProject
	s.loop(func() { p = s.ActiveProject() })
	require.NotNil(t, p)
	assert.Equal(t, "web", p.Name)
	assert.Equal(t, before+1, w.resolved.Load())
}
