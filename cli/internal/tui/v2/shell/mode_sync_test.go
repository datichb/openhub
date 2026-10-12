package shell

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// themeMountRecorder records the theme active when the view is mounted.
type themeMountRecorder struct {
	id   string
	seen string
}

func (v *themeMountRecorder) ID() string    { return v.id }
func (v *themeMountRecorder) Title() string { return v.id }
func (v *themeMountRecorder) Mount(content *tview.Flex, _ *tview.Application) {
	v.seen = theme.ActiveMode.PrimaryHex
	content.AddItem(tview.NewBox(), 0, 1, false)
}
func (v *themeMountRecorder) Unmount()                                    {}
func (v *themeMountRecorder) StatusHints() string                         { return "" }
func (v *themeMountRecorder) HandleKey(_ *tcell.EventKey) *tcell.EventKey { return nil }

func newModeShell(vs ...views.View) *Shell {
	return New(Config{Views: vs, HomeViewID: "home"})
}

func TestBackToHubRestoresHubMode(t *testing.T) {
	s := newModeShell(&testView{id: "home"}, &testView{id: "team.mode"}, &testView{id: "team.board"})
	s.NavigateHome("home", views.ModeHub)

	s.SetActiveTeam(&views.ActiveTeam{ID: "t1", Name: "T1"})
	s.SetMode(views.ModeTeam)
	s.NavigateTo("team.board")
	assert.Equal(t, views.ModeTeam, s.Mode())
	assert.Equal(t, theme.ThemeTeam.PrimaryHex, theme.ActiveMode.PrimaryHex)

	s.NavigateTo("home") // « home » command: pops back to the hub landing
	assert.Equal(t, views.ModeHub, s.Mode())
	assert.Nil(t, s.ActiveTeam())
	assert.Equal(t, theme.ThemeHub.PrimaryHex, theme.ActiveMode.PrimaryHex)
}

func TestEscBackFromProjectToTeamKeepsTeam(t *testing.T) {
	s := newModeShell(&testView{id: "home"}, &testView{id: "team.mode"},
		&testView{id: "projects.list"}, &testView{id: "project.mode"})
	s.NavigateHome("home", views.ModeHub)
	s.SetActiveTeam(&views.ActiveTeam{ID: "t1", Name: "T1"})
	s.SetMode(views.ModeTeam)
	s.NavigateTo("projects.list")
	s.SetProjectMode(&views.ActiveProject{ID: "p1", Name: "P1"})
	assert.Equal(t, views.ModeProject, s.Mode())

	s.router.Pop() // Esc
	assert.Equal(t, views.ModeTeam, s.Mode())
	assert.Nil(t, s.ActiveProject())
	assert.Equal(t, "t1", s.ActiveTeam().ID)

	s.router.Pop()
	s.router.Pop()
	assert.Equal(t, views.ModeHub, s.Mode())
	assert.Nil(t, s.ActiveTeam())
}

func TestThemeSetBeforeMount(t *testing.T) {
	home := &testView{id: "home"}
	pm := &themeMountRecorder{id: "project.mode"}
	s := newModeShell(home, pm)
	s.NavigateHome("home", views.ModeHub)
	s.SetProjectMode(&views.ActiveProject{ID: "p1", Name: "P1"})
	assert.Equal(t, theme.ThemeProject.PrimaryHex, pm.seen)
}
