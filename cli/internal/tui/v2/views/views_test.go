package views

import (
	"context"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/testutil/golden"

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
func (m *mockShell) Confirm(string, string, bool, func())              {}
func (m *mockShell) NavigateTo(string)                                 {}
func (m *mockShell) SetProjectMode(*ActiveProject)                     {}
func (m *mockShell) SetActiveProject(*ActiveProject)                   {}
func (m *mockShell) ActiveProject() *ActiveProject                     { return m.project }
func (m *mockShell) SetMode(Mode)                                      {}
func (m *mockShell) Mode() Mode {
	switch {
	case m.team != nil:
		return ModeTeam
	case m.project != nil:
		return ModeProject
	}
	return ModeHub
}
func (m *mockShell) SetActiveTeam(*ActiveTeam) {}
func (m *mockShell) ActiveTeam() *ActiveTeam   { return m.team }
func (m *mockShell) PushView(View)             {}
func (m *mockShell) PopView() bool             { return true }
func (m *mockShell) SetOmnibarVisible(bool)    {}

// every view mounts in a content flex, is identified, and lets go of the
// application on Unmount.
func TestViews_MountUnmount(t *testing.T) {
	projectShell := &mockShell{project: &ActiveProject{ID: "p1", Name: "my-app", Path: "/tmp/my-app"}}
	teamShell := &mockShell{team: &ActiveTeam{ID: "t1", Name: "alpha"}}
	tests := []struct {
		name     string
		view     View
		shell    ShellAccess // set on the views that read the active project or team
		id       string
		title    string // "" = any non-empty title
		released func(View) bool
	}{
		{
			name: "board",
			view: NewBoardView(BoardViewConfig{Tickets: []BoardTicket{
				{ID: "1", Title: "Fix bug", Status: "planned", Priority: "high"},
				{ID: "2", Title: "Add feature", Status: "in_progress", Priority: "medium"},
				{ID: "3", Title: "Release", Status: "done", Priority: "low"},
			}}),
			id: "board", title: i18n.T("tui.board.title"),
			released: func(v View) bool { return v.(*BoardView).app == nil },
		},
		{
			name: "team board",
			view: NewTeamBoardView(TeamBoardViewConfig{Tickets: []TeamTicket{
				{ID: "1", Title: "Review PR", Status: "in_progress", Assignee: "alice"},
			}}),
			id: "team.board",
			released: func(v View) bool {
				b := v.(*TeamBoardView)
				return b.app == nil && b.content == nil && b.emptyTV == nil
			},
		},
		{
			name: "home", view: NewHomeView(HomeViewConfig{}), id: "home",
			released: func(v View) bool { return v.(*HomeView).app == nil },
		},
		{
			name: "projects",
			view: NewProjectsView(ProjectsViewConfig{Projects: []ProjectItem{{ID: "p1", Name: "my-app", Path: "/tmp/my-app"}}}),
			id:   "projects.list", title: i18n.T("tui.projects.title"),
			released: func(v View) bool { return v.(*ProjectsView).app == nil },
		},
		{
			name: "status", view: NewStatusView(nil), id: "status",
			released: func(v View) bool { return v.(*StatusView).app == nil },
		},
		{
			name: "metrics", view: NewMetricsView(MetricsViewConfig{}), id: "metrics",
			released: func(v View) bool { return v.(*MetricsView).app == nil },
		},
		{
			name: "doctor", view: NewDoctorView(nil), id: "doctor",
			released: func(v View) bool { return v.(*DoctorView).app == nil },
		},
		{
			name: "mcp", view: NewMCPView(nil, MCPViewConfig{}), id: "mcp",
			released: func(v View) bool { return v.(*MCPView).app == nil },
		},
		{
			name: "help", view: NewHelpView(), id: "help",
			released: func(v View) bool { return v.(*HelpView).app == nil },
		},
		{
			name: "team status (team disabled)",
			view: NewTeamStatusView(func() TeamResolution { return TeamResolution{} }), id: "team.status",
			released: func(v View) bool { return v.(*TeamStatusView).app == nil },
		},
		{
			name: "worktrees", view: NewWorktreeView(nil, WorktreeViewConfig{}), id: "worktrees", title: i18n.T("tui.worktree.title"),
			released: func(v View) bool { return v.(*WorktreeView).app == nil },
		},
		{
			name: "project mode", view: NewProjectModeView(ProjectModeConfig{}), shell: projectShell, id: "project.mode",
			released: func(v View) bool { return v.(*ProjectModeView).app == nil },
		},
		{
			name: "team mode", view: NewTeamModeView(TeamModeConfig{}), shell: teamShell, id: "team.mode",
			released: func(v View) bool { return v.(*TeamModeView).app == nil },
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.shell != nil {
				tc.view.(interface{ SetShell(ShellAccess) }).SetShell(tc.shell)
			}
			content := tview.NewFlex().SetDirection(tview.FlexRow)

			tc.view.Mount(content, tview.NewApplication())
			assert.Greater(t, content.GetItemCount(), 0, "content mounted")
			assert.Equal(t, tc.id, tc.view.ID(), "view ID")
			if tc.title != "" {
				assert.Equal(t, tc.title, tc.view.Title(), "title")
			} else {
				assert.NotEmpty(t, tc.view.Title(), "title")
			}

			tc.view.Unmount()
			assert.True(t, tc.released(tc.view), "Unmount releases the application")
		})
	}

	t.Run("project mode and team mode without an active project or team show their empty state", func(t *testing.T) {
		for name, v := range map[string]View{
			"project mode": NewProjectModeView(ProjectModeConfig{}),
			"team mode":    NewTeamModeView(TeamModeConfig{}),
		} {
			content := tview.NewFlex().SetDirection(tview.FlexRow)
			v.Mount(content, tview.NewApplication())
			assert.Greater(t, content.GetItemCount(), 0, "%s: empty state mounted", name)
		}
	})
}

func TestBoardViews_EmptyThenPopulated(t *testing.T) {
	t.Run("board", func(t *testing.T) {
		v := NewBoardView(BoardViewConfig{})
		content := tview.NewFlex().SetDirection(tview.FlexRow)
		v.Mount(content, tview.NewApplication())
		defer v.Unmount()
		assert.NotNil(t, v.emptyTV, "empty-state placeholder with no tickets")
		assert.Equal(t, 1, content.GetItemCount(), "content holds the placeholder")

		v.populateColumns([]BoardTicket{{ID: "1", Title: "Fix bug", Status: "planned", Priority: "high"}}, DefaultColumns())
		assert.Nil(t, v.emptyTV, "placeholder cleared once tickets arrive")
		assert.Equal(t, 1, content.GetItemCount(), "content holds the columns")
	})

	t.Run("team board without refresh stays empty until populated", func(t *testing.T) {
		v := NewTeamBoardView(TeamBoardViewConfig{IsConfigured: func() bool { return true }})
		content := tview.NewFlex().SetDirection(tview.FlexRow)
		v.Mount(content, tview.NewApplication())
		defer v.Unmount()
		assert.NotNil(t, v.emptyTV, "empty-state placeholder with no tickets and no refresh")
		assert.Equal(t, 1, content.GetItemCount(), "content holds the placeholder")

		v.populateColumns([]TeamTicket{{ID: "1", Title: "Review PR", Status: "in_progress", Assignee: "alice"}}, DefaultColumns())
		assert.Nil(t, v.emptyTV, "placeholder cleared once tickets arrive")
		assert.Equal(t, 1, content.GetItemCount(), "content holds the board layout")
	})

	t.Run("team board with a refresh is populated during the mount", func(t *testing.T) {
		// Without SyncFunc, syncFuncAsync calls onDone synchronously during
		// Mount, which runs refreshOnEventLoop → populateColumns.
		v := NewTeamBoardView(TeamBoardViewConfig{
			RefreshFunc: func() []TeamTicket {
				return []TeamTicket{{ID: "1", Title: "Review PR", Status: "in_progress", Assignee: "alice"}}
			},
			IsConfigured: func() bool { return true },
		})
		content := tview.NewFlex().SetDirection(tview.FlexRow)
		v.Mount(content, tview.NewApplication())
		defer v.Unmount()
		assert.Nil(t, v.emptyTV, "placeholder cleared: the sync completed during the mount")
		assert.Equal(t, 1, content.GetItemCount(), "content holds the board layout")
		assert.NotEmpty(t, v.allTickets, "tickets populated")
	})
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
	v.summary, v.list = tview.NewTextView(), widgets.NewSectionedList()
	v.checks = v.collectChecks()
	v.render()
	text := v.summary.GetText(true)
	for _, it := range v.list.GetItems() {
		text += "\n" + it.MainText + " " + it.SecondaryText
	}
	assert.Contains(t, text, "oh version")
	assert.Contains(t, text, "5.0.1 available")
	assert.Contains(t, text, "Provider credentials")
	assert.Contains(t, text, i18n.Tf("tui.doctor.summary", 1, 2))
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

func TestHomeView_SingleProjectsSection_AddLast(t *testing.T) {
	for _, tc := range []struct {
		name     string
		projects []ProjectEntry
	}{
		{"with projects", []ProjectEntry{{ID: "p1", Name: "my-app", Path: "/tmp/my-app"}, {ID: "p2", Name: "other", Path: "/tmp/other"}}},
		{"no project", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := NewHomeView(HomeViewConfig{ListProjects: func() []ProjectEntry { return tc.projects }})
			for _, items := range [][]homeItem{v.buildItems(), v.buildStaticItems()} {
				v.items = items
				_, right := v.splitItems()
				assert.Equal(t, []string{"Projects"}, collectHeaders(right))
				require.NotEmpty(t, right)
				assert.Contains(t, right[len(right)-1].MainText, i18n.T("tui.home.project_add"))
			}
			v.items = v.buildItems()
			_, right := v.splitItems()
			assert.Len(t, right, 1+len(tc.projects)+1)
		})
	}
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

// The two empty states of the team board: the colours come from the theme,
// the translations hold only text.
func TestTeamBoardEmptyStates(t *testing.T) {
	frLocale(t)
	tests := []struct {
		name       string
		configured bool
	}{
		{"no_team", false},
		{"no_tickets", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := NewTeamBoardView(TeamBoardViewConfig{IsConfigured: func() bool { return tc.configured }})
			v.Mount(tview.NewFlex().SetDirection(tview.FlexRow), tview.NewApplication())
			defer v.Unmount()
			require.NotNil(t, v.emptyTV, "empty state shown")
			assert.NotContains(t, v.emptyTV.GetText(false), "[yellow]", "no colour name in the text")
			golden.Assert(t, "team_board_empty_"+tc.name, screenText(t, v.emptyTV, 70, 8))
		})
	}
}
