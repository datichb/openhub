package shell

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

type testView struct {
	id      string
	title   string
	mounted bool
}

var _ views.View = (*testView)(nil)

func (v *testView) ID() string    { return v.id }
func (v *testView) Title() string { return v.title }
func (v *testView) Mount(content *tview.Flex, _ *tview.Application) {
	v.mounted = true
	content.AddItem(tview.NewBox(), 0, 1, false)
}
func (v *testView) Unmount()                                    { v.mounted = false }
func (v *testView) StatusHints() string                         { return v.id + " hints" }
func (v *testView) HandleKey(_ *tcell.EventKey) *tcell.EventKey { return nil }

func TestNew(t *testing.T) {
	homeView := &testView{id: "home", title: "Home"}
	boardView := &testView{id: "board", title: "Board"}

	cfg := Config{
		ProjectName: "test-project",
		Commands: []Command{
			{ID: "home", Label: "Home", ViewID: "home"},
			{ID: "board", Label: "Board", ViewID: "board"},
		},
		Views:      []views.View{homeView, boardView},
		HomeViewID: "home",
	}

	s := New(cfg)
	assert.NotNil(t, s)
	assert.NotNil(t, s.app)
	assert.NotNil(t, s.pages)
	assert.NotNil(t, s.content)
	assert.NotNil(t, s.omnibar)
	assert.NotNil(t, s.router)
	assert.NotNil(t, s.registry)
}

func TestShell_NavigateHome(t *testing.T) {
	homeView := &testView{id: "home", title: "Home"}

	cfg := Config{
		ProjectName: "test",
		Commands:    []Command{{ID: "home", Label: "Home", ViewID: "home"}},
		Views:       []views.View{homeView},
		HomeViewID:  "home",
	}

	s := New(cfg)
	s.NavigateHome("home", views.ModeHub)

	assert.True(t, homeView.mounted)
	assert.Equal(t, "home", s.router.Current().ID())
}

// `oh board`, `oh team board`, `oh dashboard`: the initial view opens on top
// of the home view of the mode, and Esc goes back to that home view.
func TestShell_InitialViewOpensOnTopOfHome(t *testing.T) {
	home := &testView{id: "project.mode", title: "Project"}
	board := &testView{id: "board", title: "Board"}

	s := New(Config{Views: []views.View{home, board}, HomeViewID: "project.mode", InitialViewID: "board"})
	s.NavigateHome("project.mode", views.ModeProject)

	assert.Equal(t, "board", s.router.Current().ID())
	assert.True(t, board.mounted)
	assert.Equal(t, 2, s.router.StackDepth())
	assert.Equal(t, views.ModeProject, s.Mode())

	assert.True(t, s.router.Pop())
	assert.Equal(t, "project.mode", s.router.Current().ID())
	assert.True(t, home.mounted)
}

func TestShell_NoInitialViewStaysOnHome(t *testing.T) {
	home := &testView{id: "home", title: "Home"}
	s := New(Config{Views: []views.View{home}, HomeViewID: "home"})
	s.NavigateHome("home", views.ModeHub)
	assert.Equal(t, 1, s.router.StackDepth())

	// An initial view that is not registered leaves the home view in place.
	s = New(Config{Views: []views.View{home}, HomeViewID: "home", InitialViewID: "nope"})
	s.NavigateHome("home", views.ModeHub)
	assert.Equal(t, "home", s.router.Current().ID())
}

func TestOmnibar_SetHints(t *testing.T) {
	homeView := &testView{id: "home", title: "Home"}

	cfg := Config{
		ProjectName: "test",
		Commands:    []Command{{ID: "home", Label: "Home", ViewID: "home"}},
		Views:       []views.View{homeView},
		HomeViewID:  "home",
	}

	s := New(cfg)
	// Should not panic
	s.omnibar.SetHints("j/k nav · Enter select")
	assert.NotNil(t, s.omnibar.hints)
}

func TestOmnibar_ActivateDeactivate(t *testing.T) {
	homeView := &testView{id: "home", title: "Home"}

	cfg := Config{
		ProjectName: "test",
		Commands: []Command{
			{ID: "home", Label: "Home", ViewID: "home"},
			{ID: "board", Label: "Board", ViewID: "board"},
		},
		Views:      []views.View{homeView},
		HomeViewID: "home",
	}

	s := New(cfg)
	s.NavigateHome("home", views.ModeHub)

	assert.False(t, s.omnibar.IsActive())
	s.omnibar.Activate()
	assert.True(t, s.omnibar.IsActive())
	s.omnibar.Deactivate()
	assert.False(t, s.omnibar.IsActive())
}

func TestShell_SessionsBadge(t *testing.T) {
	homeView := &testView{id: "home", title: "Home"}
	s := New(Config{ProjectName: "test", Views: []views.View{homeView}, HomeViewID: "home"})
	s.NavigateHome("home", views.ModeHub)
	s.SetSessionsBadge("● 2  ◐ 1", true)
	assert.Contains(t, s.omnibar.modeBar.GetText(true), "● 2  ◐ 1")
	s.SetSessionsBadge("", false)
	assert.NotContains(t, s.omnibar.modeBar.GetText(true), "●")
}

// infoView is a view whose mode bar text changes after the mount (data read
// off the event loop): RefreshModeBar draws the bar again from the view.
type infoView struct {
	testView
	right string
}

func (v *infoView) ModeInfo() views.ModeBarInfo {
	return views.ModeBarInfo{Label: "alpha", Right: v.right}
}

func TestShell_RefreshModeBar(t *testing.T) {
	v := &infoView{testView: testView{id: "home", title: "Home"}}
	s := New(Config{ProjectName: "test", Views: []views.View{v}, HomeViewID: "home"})
	s.NavigateHome("home", views.ModeHub)
	assert.NotContains(t, s.omnibar.modeBar.GetText(true), "3 members")

	v.right = "3 members"
	s.RefreshModeBar()
	assert.Contains(t, s.omnibar.modeBar.GetText(true), "3 members")
}

func TestShell_RequestQuitGoesThroughBeforeQuit(t *testing.T) {
	homeView := &testView{id: "home", title: "Home"}
	calls := 0
	s := New(Config{ProjectName: "test", Views: []views.View{homeView}, HomeViewID: "home",
		BeforeQuit: func(func()) { calls++ }})
	s.RequestQuit()
	assert.Equal(t, 1, calls)
	assert.True(t, s.quitPending)
	s.CancelQuit() // Esc on the quit dialog
	assert.False(t, s.quitPending)
	s.RequestQuit()
	assert.Equal(t, 2, calls, "asked again after a cancel")
}

// A toast shown while a form is open must not take the focus from the form.
func TestToastKeepsFormFocus(t *testing.T) {
	homeView := &testView{id: "home", title: "Home"}
	s := New(Config{ProjectName: "test", Views: []views.View{homeView}, HomeViewID: "home"})
	s.NavigateHome("home", views.ModeHub)
	s.ShowInlineForm(views.InlineFormConfig{Title: "q", Fields: []views.FormField{{Key: "k", Label: "k", Type: views.FieldText}}})
	formFocus := s.app.GetFocus()
	assert.NotNil(t, formFocus)
	s.pages.AddPage("toast-x", tview.NewBox(), true, true)
	s.restoreFocusAfterToast(formFocus)
	assert.Equal(t, formFocus, s.app.GetFocus())
	s.closeWindows()
	widget := tview.NewBox()
	s.content.AddItem(widget, 0, 1, true)
	s.restoreFocusAfterToast(widget)
	assert.Same(t, widget, s.app.GetFocus(), "no modal: back to the widget of the view that had the focus")
	s.app.SetFocus(tview.NewBox())
	s.restoreFocusAfterToast(nil)
	assert.True(t, s.content.HasFocus(), "none had it: the view")
}
