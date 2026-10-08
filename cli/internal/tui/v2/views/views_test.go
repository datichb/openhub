package views

import (
	"context"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/i18n"

	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ── Mock shell for tests ────────────────────────────────────────────────────

type mockShell struct {
	project *ActiveProject
	team    *ActiveTeam
}

func (m *mockShell) Context() context.Context                                     { return context.Background() }
func (m *mockShell) ShowInputModal(string, string, func(string))                  {}
func (m *mockShell) ShowPasswordModal(string, func(string))                       {}
func (m *mockShell) ShowSelectModal(string, []SelectOption, string, func(string)) {}
func (m *mockShell) ShowMultiSelectModal(string, []SelectOption, []string, func([]string)) {
}
func (m *mockShell) ShowScrollableModal(string, string, []ModalAction) {}
func (m *mockShell) ShowToastMsg(string, bool)                         {}
func (m *mockShell) ShowInlineForm(InlineFormConfig)                   {}
func (m *mockShell) NavigateTo(string)                                 {}
func (m *mockShell) SetProjectMode(*ActiveProject)                     {}
func (m *mockShell) SetActiveProject(*ActiveProject)                   {}
func (m *mockShell) ActiveProject() *ActiveProject                     { return m.project }
func (m *mockShell) SetMode(Mode)                                      {}
func (m *mockShell) Mode() Mode                                        { return ModeHub }
func (m *mockShell) SetActiveTeam(*ActiveTeam)                         {}
func (m *mockShell) ActiveTeam() *ActiveTeam                           { return m.team }
func (m *mockShell) PushView(View)                                     {}
func (m *mockShell) PopView() bool                                     { return true }
func (m *mockShell) SetOmnibarVisible(bool)                            {}

func TestBoardView_ImplementsView(t *testing.T) {
	var _ View = (*BoardView)(nil)
}

func TestBoardView_MountUnmount(t *testing.T) {
	v := NewBoardView(BoardViewConfig{
		Tickets: []BoardTicket{
			{ID: "1", Title: "Fix bug", Status: "planned", Priority: "high"},
			{ID: "2", Title: "Add feature", Status: "in_progress", Priority: "medium"},
			{ID: "3", Title: "Release", Status: "done", Priority: "low"},
		},
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.Greater(t, content.GetItemCount(), 0)
	assert.Equal(t, "board", v.ID())
	assert.Equal(t, i18n.T("tui.board.title"), v.Title())
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
	assert.Nil(t, v.content)
	assert.Nil(t, v.emptyTV)
}

func TestTeamBoardView_MountEmpty_TransitionToPopulated(t *testing.T) {
	tickets := []TeamTicket{
		{ID: "1", Title: "Review PR", Status: "in_progress", Assignee: "alice"},
	}
	v := NewTeamBoardView(TeamBoardViewConfig{
		Tickets: nil, // start empty
		RefreshFunc: func() []TeamTicket {
			return tickets
		},
		IsConfigured: func() bool { return true },
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	// When SyncFunc is nil, syncFuncAsync calls onDone synchronously during Mount,
	// which triggers refreshOnEventLoop → populateColumns with real tickets.
	// This means the empty → populated transition happens within Mount itself.
	v.Mount(content, app)

	// The board should already have transitioned to the populated state.
	assert.Nil(t, v.emptyTV, "emptyTV should be cleared — sync completed during Mount")
	assert.Equal(t, 1, content.GetItemCount(), "content should have the boardLayout")
	assert.Greater(t, len(v.allTickets), 0, "tickets should be populated")

	v.Unmount()
}

func TestTeamBoardView_MountEmpty_NoSyncFunc_StaysEmpty(t *testing.T) {
	// When there is NO RefreshFunc, the board stays in empty state.
	v := NewTeamBoardView(TeamBoardViewConfig{
		Tickets:      nil,
		IsConfigured: func() bool { return true },
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.NotNil(t, v.emptyTV, "emptyTV should be set when mounted with no tickets and no refresh")
	assert.Equal(t, 1, content.GetItemCount(), "content should have the emptyTV")

	// Now simulate an external call to populateColumns (as if a manual refresh happened)
	tickets := []TeamTicket{
		{ID: "1", Title: "Review PR", Status: "in_progress", Assignee: "alice"},
	}
	v.populateColumns(tickets, DefaultColumns())

	assert.Nil(t, v.emptyTV, "emptyTV should be cleared after tickets arrive")
	assert.Equal(t, 1, content.GetItemCount(), "content should now have the boardLayout")

	v.Unmount()
}

func TestBoardView_MountEmpty_TransitionToPopulated(t *testing.T) {
	tickets := []BoardTicket{
		{ID: "1", Title: "Fix bug", Status: "planned", Priority: "high"},
	}
	v := NewBoardView(BoardViewConfig{
		Tickets: nil, // start empty
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	// Should show the empty-state placeholder
	assert.NotNil(t, v.emptyTV, "emptyTV should be set when mounted with no tickets")
	assert.Equal(t, 1, content.GetItemCount(), "content should have the emptyTV")

	// Simulate a refresh delivering tickets
	v.populateColumns(tickets, DefaultColumns())

	assert.Nil(t, v.emptyTV, "emptyTV should be cleared after tickets arrive")
	assert.Equal(t, 1, content.GetItemCount(), "content should now have the columnFlex")

	v.Unmount()
}

func TestHomeView_ImplementsView(t *testing.T) {
	var _ View = (*HomeView)(nil)
}

func TestHomeView_MountUnmount(t *testing.T) {
	v := NewHomeView(HomeViewConfig{})

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
	assert.Equal(t, i18n.T("tui.projects.title"), v.Title())

	v.Unmount()
}

func TestProjectsView_RefreshFunc_CalledOnMount(t *testing.T) {
	called := make(chan struct{}, 1)
	freshProjects := []ProjectItem{
		{ID: "p2", Name: "refreshed-app", Path: "/tmp/refreshed"},
	}
	v := NewProjectsView(ProjectsViewConfig{
		Projects: nil, // start empty
		RefreshFunc: func() []ProjectItem {
			called <- struct{}{}
			return freshProjects
		},
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	// Mount launches a goroutine that calls RefreshFunc before QueueUpdateDraw.
	v.Mount(content, app)

	// Wait for the goroutine to call RefreshFunc (with timeout to avoid hanging).
	select {
	case <-called:
		// ok — RefreshFunc was invoked
	case <-time.After(2 * time.Second):
		t.Fatal("RefreshFunc was not called within timeout")
	}

	// cfg.Projects is now assigned inside QueueUpdateDraw, which requires the
	// tview event loop to run. Since we don't start the tview app in this test,
	// we verify only that RefreshFunc was called (the channel confirms it).
	// The actual assignment is tested indirectly by the integration tests.

	v.Unmount()
}

func TestProjectsView_NoRefreshFunc_UsesStaticProjects(t *testing.T) {
	staticProjects := []ProjectItem{
		{ID: "p1", Name: "static-app", Path: "/tmp/static"},
	}
	v := NewProjectsView(ProjectsViewConfig{
		Projects: staticProjects,
		// RefreshFunc intentionally nil
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)

	// Without RefreshFunc, the original slice should be preserved.
	assert.Equal(t, staticProjects, v.cfg.Projects, "cfg.Projects should remain unchanged without RefreshFunc")

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

	v := NewMetricsView(MetricsViewConfig{})
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

// A40: the view shows exactly the checks of `oh doctor` (warnings counted as
// passed), not a subset of its own.
func TestDoctorView_ShowsDoctorRegistry(t *testing.T) {
	v := NewDoctorView(nil)
	v.run = func() []DoctorCheck {
		return []DoctorCheck{
			{Name: "oh version", OK: true, Warn: true, Detail: "5.0.1 available"},
			{Name: "Provider credentials", OK: false, Detail: "missing"},
		}
	}
	v.tv = tview.NewTextView()
	v.checks = v.collectChecks()
	v.render()
	text := v.tv.GetText(true)
	assert.Contains(t, text, "oh version")
	assert.Contains(t, text, "5.0.1 available")
	assert.Contains(t, text, "Provider credentials")
	assert.Contains(t, text, i18n.Tf("tui.doctor.summary", 1, 2))
}

func TestMCPView_ImplementsView(t *testing.T) {
	var _ View = (*MCPView)(nil)

	v := NewMCPView(nil, MCPViewConfig{})
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

	// Provide a no-op resolve func — team disabled
	v := NewTeamStatusView(func() TeamResolution { return TeamResolution{} })
	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.Greater(t, content.GetItemCount(), 0)
	assert.Equal(t, "team.status", v.ID())
	assert.NotEmpty(t, v.Title()) // i18n-dependent, don't assert exact string
	v.Unmount()
}

func TestWorktreeView_ImplementsView(t *testing.T) {
	var _ View = (*WorktreeView)(nil)

	v := NewWorktreeView(nil, WorktreeViewConfig{})
	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.Greater(t, content.GetItemCount(), 0)
	assert.Equal(t, "worktrees", v.ID())
	assert.Equal(t, i18n.T("tui.worktree.title"), v.Title())
	v.Unmount()
}

// ─────────────────────────────────────────────────────────────────────────────
// ProjectModeView tests
// ─────────────────────────────────────────────────────────────────────────────

func TestProjectModeView_MountUnmount(t *testing.T) {
	v := NewProjectModeView(ProjectModeConfig{})
	v.SetShell(&mockShell{
		project: &ActiveProject{ID: "p1", Name: "my-app", Path: "/tmp/my-app"},
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.Greater(t, content.GetItemCount(), 0)
	assert.Equal(t, "project.mode", v.ID())

	v.Unmount()
	assert.Nil(t, v.app)
}

func TestProjectModeView_MountNoProject(t *testing.T) {
	v := NewProjectModeView(ProjectModeConfig{})
	// No shell or project set

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	// Should show empty state (text view with "Aucun projet actif")
	assert.Greater(t, content.GetItemCount(), 0)
}

// ─────────────────────────────────────────────────────────────────────────────
// TeamModeView tests
// ─────────────────────────────────────────────────────────────────────────────

func TestTeamModeView_MountUnmount(t *testing.T) {
	v := NewTeamModeView(TeamModeConfig{})
	v.SetShell(&mockShell{
		team: &ActiveTeam{ID: "t1", Name: "alpha"},
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.Greater(t, content.GetItemCount(), 0)
	assert.Equal(t, "team.mode", v.ID())

	v.Unmount()
	assert.Nil(t, v.app)
}

func TestTeamModeView_MountNoTeam(t *testing.T) {
	v := NewTeamModeView(TeamModeConfig{})
	// No shell or team set

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	// Should show empty state (text view with "Aucune équipe active")
	assert.Greater(t, content.GetItemCount(), 0)
}

// ─────────────────────────────────────────────────────────────────────────────
// HomeView splitItems tests
// ─────────────────────────────────────────────────────────────────────────────

func TestHomeView_SplitItems_DualColumn(t *testing.T) {
	v := NewHomeView(HomeViewConfig{
		ListTeams: func() []TeamEntry {
			return []TeamEntry{
				{ID: "t1", Name: "alpha", MemberCount: 3, ActiveCount: 5},
			}
		},
		ListProjects: func() []ProjectEntry {
			return []ProjectEntry{
				{ID: "p1", Name: "my-app", Path: "/tmp/my-app"},
			}
		},
	})

	v.items = v.buildItems()
	left, right := v.splitItems()

	// Collect section headers from left column
	leftHeaders := collectHeaders(left)
	// Collect section headers from right column
	rightHeaders := collectHeaders(right)

	// Left should contain: Sessions, Teams, System
	assert.Contains(t, leftHeaders, "Teams")
	assert.Contains(t, leftHeaders, "System")

	// Right should contain: Projects (replaces old Quick Actions)
	assert.Contains(t, rightHeaders, "Projects")
}

// ─────────────────────────────────────────────────────────────────────────────
// ProjectModeView splitItems tests
// ─────────────────────────────────────────────────────────────────────────────

func TestProjectModeView_SplitItems(t *testing.T) {
	v := NewProjectModeView(ProjectModeConfig{
		Start: StartSectionConfig{Entries: func(StartScope) StartEntries { return StartEntries{Loaded: true} }},
	})
	v.project = &ActiveProject{ID: "p1", Name: "my-app", Path: "/tmp/my-app"}

	v.items = v.buildItems()
	left, right := v.splitItems()

	leftHeaders := collectHeaders(left)
	rightHeaders := collectHeaders(right)

	// Left: Démarrer + Project
	assert.Contains(t, leftHeaders, i18n.T("tui.start.section"))
	assert.Contains(t, leftHeaders, "Project")

	// Right: Configuration (no deploy section since v5)
	assert.Contains(t, rightHeaders, "Configuration")
	assert.NotContains(t, rightHeaders, "Deploy")
}

// ─────────────────────────────────────────────────────────────────────────────
// TeamModeView splitItems tests
// ─────────────────────────────────────────────────────────────────────────────

func TestTeamModeView_SplitItems(t *testing.T) {
	v := NewTeamModeView(TeamModeConfig{
		Start: StartSectionConfig{Entries: func(StartScope) StartEntries { return StartEntries{Loaded: true} }},
	})
	v.team = &ActiveTeam{ID: "t1", Name: "alpha"}

	v.items = v.buildItems()
	left, right := v.splitItems()

	leftHeaders := collectHeaders(left)
	rightHeaders := collectHeaders(right)

	// Left: Démarrer + Board
	assert.Contains(t, leftHeaders, i18n.T("tui.start.section"))
	assert.Contains(t, leftHeaders, i18n.T("tui.tm.section.board"))

	// Right: Configuration + Navigation
	assert.Contains(t, rightHeaders, i18n.T("tui.tm.section.configuration"))
	assert.Contains(t, rightHeaders, i18n.T("tui.tm.section.navigation"))
}

// ─────────────────────────────────────────────────────────────────────────────
// SectionedList GotoFirst/GotoLast tests
// ─────────────────────────────────────────────────────────────────────────────

func TestSectionedList_GotoFirstLast(t *testing.T) {
	sl := widgets.NewSectionedList()
	sl.SetItems([]widgets.SectionItem{
		{MainText: "Section A", IsHeader: true},
		{MainText: "item1", Reference: "ref1"},
		{MainText: "item2", Reference: "ref2"},
		{MainText: "Section B", IsHeader: true},
		{MainText: "item3", Reference: "ref3"},
		{MainText: "item4", Reference: "ref4"},
	})

	inputHandler := sl.GetInputCapture()
	assert.NotNil(t, inputHandler, "SectionedList should have an input capture handler")

	// Simulate 'g' key → gotoFirst()
	gEvent := tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone)
	result := inputHandler(gEvent)
	assert.Nil(t, result, "'g' key should be consumed by the handler")

	idx, item, ok := sl.CurrentItem()
	assert.True(t, ok, "should have a selectable item after gotoFirst")
	assert.Equal(t, 1, idx, "gotoFirst should land on first selectable item (index 1)")
	assert.Equal(t, "item1", item.MainText)

	// Move cursor to the middle first
	sl.SelectIndex(3) // This will skip header B and land on item3 (index 4)

	// Simulate 'G' key → gotoLast()
	bigGEvent := tcell.NewEventKey(tcell.KeyRune, 'G', tcell.ModNone)
	result = inputHandler(bigGEvent)
	assert.Nil(t, result, "'G' key should be consumed by the handler")

	idx, item, ok = sl.CurrentItem()
	assert.True(t, ok, "should have a selectable item after gotoLast")
	assert.Equal(t, 5, idx, "gotoLast should land on last selectable item (index 5)")
	assert.Equal(t, "item4", item.MainText)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// collectHeaders extracts the MainText of all header items from a SectionItem slice.
func collectHeaders(items []widgets.SectionItem) []string {
	var headers []string
	for _, it := range items {
		if it.IsHeader {
			headers = append(headers, it.MainText)
		}
	}
	return headers
}
