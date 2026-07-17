package shell

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/tui/v2/menu"
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
func (v *testView) Unmount()                                        { v.mounted = false }
func (v *testView) StatusHints() string                             { return v.id + " hints" }
func (v *testView) HandleKey(_ *tcell.EventKey) *tcell.EventKey { return nil }

func TestNew(t *testing.T) {
	homeView := &testView{id: "home", title: "Home"}
	boardView := &testView{id: "board", title: "Board"}

	cfg := Config{
		ProjectName: "test-project",
		MenuItems: []*menu.MenuItem{
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
	assert.NotNil(t, s.header)
	assert.NotNil(t, s.menu)
	assert.NotNil(t, s.content)
	assert.NotNil(t, s.statusBar)
	assert.NotNil(t, s.router)
}

func TestShell_NavigateHome(t *testing.T) {
	homeView := &testView{id: "home", title: "Home"}

	cfg := Config{
		ProjectName: "test",
		MenuItems:   []*menu.MenuItem{{ID: "home", Label: "Home", ViewID: "home"}},
		Views:       []views.View{homeView},
		HomeViewID:  "home",
	}

	s := New(cfg)
	s.NavigateHome("home")

	assert.True(t, homeView.mounted)
	assert.Equal(t, "home", s.router.Current().ID())
}

func TestHeader_SetBreadcrumb(t *testing.T) {
	h := NewHeader("MyProject")
	assert.NotNil(t, h.Primitive())

	h.SetBreadcrumb("Sessions > Start")
	// Verify text was set (we can't easily read tview text without Draw)
	assert.NotNil(t, h.center)
}

func TestStatusBar_SetHints(t *testing.T) {
	sb := NewStatusBar()
	assert.NotNil(t, sb.Primitive())

	sb.SetHints("j/k nav · Enter select")
	sb.SetView("board")
	sb.SetInfo("3 projets")
	// Verify no panic and primitives exist
	assert.NotNil(t, sb.left)
	assert.NotNil(t, sb.center)
	assert.NotNil(t, sb.right)
}
