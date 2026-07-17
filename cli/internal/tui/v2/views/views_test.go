package views

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
)

func TestBoardView_ImplementsView(t *testing.T) {
	var _ View = (*BoardView)(nil)
}

func TestBoardView_MountUnmount(t *testing.T) {
	v := NewBoardView(BoardViewConfig{
		Tickets: []BoardTicket{
			{ID: "1", Title: "Fix bug", Status: "todo", Priority: "high"},
			{ID: "2", Title: "Add feature", Status: "in_progress", Priority: "medium"},
			{ID: "3", Title: "Release", Status: "done", Priority: "low"},
		},
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.Greater(t, content.GetItemCount(), 0)
	assert.Equal(t, "board", v.ID())
	assert.Equal(t, "Board", v.Title())
	assert.NotEmpty(t, v.StatusHints())

	v.Unmount()
	assert.Nil(t, v.app)
}

func TestTeamBoardView_ImplementsView(t *testing.T) {
	var _ View = (*TeamBoardView)(nil)
}

func TestTeamBoardView_MountUnmount(t *testing.T) {
	v := NewTeamBoardView(TeamBoardViewConfig{
		Tickets: []TeamTicket{
			{ID: "1", Title: "Review PR", Status: "in_progress", Assignee: "alice"},
		},
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.Greater(t, content.GetItemCount(), 0)
	assert.Equal(t, "team.board", v.ID())

	v.Unmount()
	assert.Nil(t, v.app)
}

func TestParallelView_ImplementsView(t *testing.T) {
	var _ View = (*ParallelView)(nil)
}

func TestParallelView_MountUnmount(t *testing.T) {
	v := NewParallelView(ParallelViewConfig{
		Sessions: []ParallelSession{
			{ID: "s1", Name: "session-1", Status: "running", Branch: "feat/x", Agent: "developer"},
		},
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.Greater(t, content.GetItemCount(), 0)
	assert.Equal(t, "parallel", v.ID())

	v.Unmount()
	assert.Nil(t, v.app)
}

func TestHomeView_ImplementsView(t *testing.T) {
	var _ View = (*HomeView)(nil)
}

func TestHomeView_MountUnmount(t *testing.T) {
	v := NewHomeView()

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.Greater(t, content.GetItemCount(), 0)
	assert.Equal(t, "home", v.ID())

	v.Unmount()
	assert.Nil(t, v.app)
}

// ── Phase 3 views ──

func TestProjectsView_ImplementsView(t *testing.T) {
	var _ View = (*ProjectsView)(nil)
}

func TestProjectsView_MountUnmount(t *testing.T) {
	v := NewProjectsView(ProjectsViewConfig{
		Projects: []ProjectItem{
			{ID: "p1", Name: "my-app", Path: "/tmp/my-app"},
		},
	})
	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.Greater(t, content.GetItemCount(), 0)
	assert.Equal(t, "projects.list", v.ID())
	assert.Equal(t, "Projets", v.Title())

	v.Unmount()
}

func TestStatusView_ImplementsView(t *testing.T) {
	var _ View = (*StatusView)(nil)

	v := NewStatusView(nil)
	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.Greater(t, content.GetItemCount(), 0)
	assert.Equal(t, "status", v.ID())
	v.Unmount()
}

func TestMetricsView_ImplementsView(t *testing.T) {
	var _ View = (*MetricsView)(nil)

	v := NewMetricsView()
	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.Greater(t, content.GetItemCount(), 0)
	assert.Equal(t, "metrics", v.ID())
	v.Unmount()
}

func TestDoctorView_ImplementsView(t *testing.T) {
	var _ View = (*DoctorView)(nil)

	v := NewDoctorView(nil)
	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.Greater(t, content.GetItemCount(), 0)
	assert.Equal(t, "doctor", v.ID())
	v.Unmount()
}

func TestConfigView_ImplementsView(t *testing.T) {
	var _ View = (*ConfigView)(nil)

	v := NewConfigView()
	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.Greater(t, content.GetItemCount(), 0)
	assert.Equal(t, "config", v.ID())
	v.Unmount()
}

func TestMCPView_ImplementsView(t *testing.T) {
	var _ View = (*MCPView)(nil)

	v := NewMCPView(nil)
	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.Greater(t, content.GetItemCount(), 0)
	assert.Equal(t, "mcp", v.ID())
	v.Unmount()
}

func TestHelpView_ImplementsView(t *testing.T) {
	var _ View = (*HelpView)(nil)

	v := NewHelpView()
	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.Greater(t, content.GetItemCount(), 0)
	assert.Equal(t, "help", v.ID())
	v.Unmount()
}

func TestTeamStatusView_ImplementsView(t *testing.T) {
	var _ View = (*TeamStatusView)(nil)

	v := NewTeamStatusView(nil)
	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.Greater(t, content.GetItemCount(), 0)
	assert.Equal(t, "team.status", v.ID())
	assert.Equal(t, "Team Status", v.Title())
	v.Unmount()
}

func TestWorktreeView_ImplementsView(t *testing.T) {
	var _ View = (*WorktreeView)(nil)

	v := NewWorktreeView(nil)
	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.Greater(t, content.GetItemCount(), 0)
	assert.Equal(t, "worktrees", v.ID())
	assert.Equal(t, "Worktrees", v.Title())
	v.Unmount()
}
