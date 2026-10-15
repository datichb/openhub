package views

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/storage/keychain"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// Tests of the views of the lot tui-cleanup that had none (construction,
// rendering, main keys, confirmations).

// viewShell records what a view asks the shell.
type viewShell struct {
	mockShell
	mu         sync.Mutex
	toasts     []string
	confirms   []string
	dangers    []bool
	onYes      func()
	form       *InlineFormConfig
	modalTitle string
	modalBody  string
	modal      []ModalAction
	inputTitle string
	input      func(string)
	selectOpts []SelectOption
	selectFn   func(string)
}

func (s *viewShell) ShowToastMsg(msg string, _ bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toasts = append(s.toasts, msg)
}

func (s *viewShell) Confirm(title, _ string, danger bool, onYes func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.confirms = append(s.confirms, title)
	s.dangers = append(s.dangers, danger)
	s.onYes = onYes
}

func (s *viewShell) ShowInlineForm(c InlineFormConfig) { s.form = &c }

func (s *viewShell) ShowScrollableModal(title, body string, a []ModalAction) {
	s.modalTitle, s.modalBody, s.modal = title, body, a
}

func (s *viewShell) ShowInputModal(title, _ string, fn func(string)) {
	s.inputTitle, s.input = title, fn
}

func (s *viewShell) ShowSelectModal(_ string, o []SelectOption, _ string, fn func(string)) {
	s.selectOpts, s.selectFn = o, fn
}

// confirmed runs the last confirmation (on the loop when app is set).
func (s *viewShell) confirmed(t *testing.T, app *tview.Application) {
	t.Helper()
	s.mu.Lock()
	yes := s.onYes
	s.onYes = nil
	s.mu.Unlock()
	require.NotNil(t, yes, "no confirmation asked")
	if app == nil {
		yes()
		return
	}
	onLoop(app, yes)
}

func (s *viewShell) confirmCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.confirms)
}

func (s *viewShell) lastToast() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.toasts) == 0 {
		return ""
	}
	return s.toasts[len(s.toasts)-1]
}

func runeKey(r rune) *tcell.EventKey { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone) }

// soloState creates a solo team-state and its resolution.
func soloState(t *testing.T) (*teamstate.Repo, TeamResolution) {
	t.Helper()
	dir := t.TempDir() + "/state"
	repo, err := teamstate.InitSolo(context.Background(), dir, teamstate.Member{ID: "alice"})
	require.NoError(t, err)
	return repo, TeamResolution{Enabled: true, Solo: true, TeamID: "solo", StatePath: dir, MemberID: "alice"}
}

// waitFor waits until cond (run on the loop) holds.
func waitFor(t *testing.T, app *tview.Application, cond func() bool) {
	t.Helper()
	require.Eventually(t, func() bool {
		var ok bool
		onLoop(app, func() { ok = cond() })
		return ok
	}, 3*time.Second, 10*time.Millisecond)
}

// drawn is the text of content drawn on the loop.
func drawn(t *testing.T, app *tview.Application, content tview.Primitive, w, h int) string {
	t.Helper()
	var s string
	onLoop(app, func() { s = screenText(t, content, w, h) })
	return s
}

// selectSection moves a sectioned list to the first item whose main text
// holds s.
func selectSection(t *testing.T, l *widgets.SectionedList, s string) {
	t.Helper()
	for i, it := range l.GetItems() {
		if strings.Contains(it.MainText, s) {
			l.SelectIndex(i)
			return
		}
	}
	t.Fatalf("no item %q", s)
}

// ── Models (hub) ───────────────────────────────────────────────────────────

func hubConfig(t *testing.T, fn func(*config.Config)) {
	t.Helper()
	t.Setenv("OH_HOME", t.TempDir())
	config.Reset()
	t.Cleanup(config.Reset)
	require.NoError(t, config.Update(func(c *config.Config) error { fn(c); return nil }))
}

func TestModelsViewKeys(t *testing.T) {
	frLocale(t)
	hubConfig(t, func(c *config.Config) {
		c.Models.Default = "sonnet"
		c.Models.Families = map[string]string{"planning": "opus"}
	})
	v := NewModelsView(nil)
	sh := &viewShell{}
	v.SetShell(sh)
	assert.Equal(t, "Modèles", v.Title())
	assert.Contains(t, v.StatusHints(), "n créer")
	assert.NotContains(t, v.StatusHints(), "a ajouter")

	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return v.table != nil })
	text := drawn(t, app, content, 100, 12)
	assert.Contains(t, text, "planning")
	assert.Contains(t, text, "opus")

	// a no longer adds; n opens the form.
	onLoop(app, func() { v.HandleKey(runeKey('a')) })
	assert.Nil(t, sh.form)
	onLoop(app, func() { v.HandleKey(runeKey('n')) })
	require.NotNil(t, sh.form)

	// d asks first (danger); the override goes once confirmed.
	onLoop(app, func() { v.table.Select(2, 0); v.HandleKey(runeKey('d')) })
	require.Equal(t, 1, sh.confirmCount())
	assert.True(t, sh.dangers[0])
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "opus", cfg.Models.Families["planning"], "nothing deleted before the confirmation")
	sh.confirmed(t, app)
	config.Reset()
	cfg, err = config.Load()
	require.NoError(t, err)
	assert.NotContains(t, cfg.Models.Families, "planning")
}

// ── Project models ─────────────────────────────────────────────────────────

func TestProjectModelsViewKeys(t *testing.T) {
	frLocale(t)
	p := &domain.Project{ID: "p1", Name: "web", Model: "sonnet",
		ModelOverrides: &domain.ProjectModelOverrides{Families: map[string]string{"developer": "haiku"}}}
	saved := make(chan string, 4)
	v := NewProjectModelsView(ProjectModelsViewConfig{
		GetProject: func() *domain.Project { return p },
		SaveProject: func(_ context.Context, p *domain.Project) error {
			saved <- p.ModelOverrides.Families["developer"]
			return nil
		},
	})
	sh := &viewShell{}
	v.SetShell(sh)
	assert.Contains(t, v.StatusHints(), "n créer")
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return v.list != nil })
	text := drawn(t, app, content, 100, 16)
	assert.Contains(t, text, "Modèles du projet · web")
	assert.Contains(t, text, "haiku")

	onLoop(app, func() { v.HandleKey(runeKey('n')) })
	require.NotEmpty(t, sh.selectOpts, "n asks the kind of override")

	onLoop(app, func() { selectSection(t, v.list, "developer"); v.HandleKey(runeKey('d')) })
	require.Equal(t, 1, sh.confirmCount())
	assert.Equal(t, "haiku", p.ModelOverrides.Families["developer"], "nothing deleted before the confirmation")
	sh.confirmed(t, app)
	assert.NotContains(t, p.ModelOverrides.Families, "developer")
	select {
	case got := <-saved:
		assert.Empty(t, got)
	case <-time.After(3 * time.Second):
		t.Fatal("not saved")
	}
}

// ── Team models ────────────────────────────────────────────────────────────

func TestTeamModelsViewKeys(t *testing.T) {
	frLocale(t)
	repo, tc := soloState(t)
	cfg, err := repo.LoadConfig()
	require.NoError(t, err)
	cfg.Models.Families = map[string]string{"quality": "sonnet"}
	require.NoError(t, repo.SaveConfig(context.Background(), cfg))

	var saved *teamstate.TeamConfig
	v := NewTeamModelsView(TeamModelsViewConfig{
		ResolveTeam: func() TeamResolution { return tc },
		SaveTeamConfig: func(_ context.Context, c *teamstate.TeamConfig) error {
			saved = c
			return nil
		},
	})
	sh := &viewShell{}
	v.SetShell(sh)
	assert.Contains(t, v.StatusHints(), "d supprimer")
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return v.list != nil })
	assert.Contains(t, drawn(t, app, content, 100, 16), "quality")

	onLoop(app, func() { selectSection(t, v.list, "quality"); v.HandleKey(runeKey('d')) })
	require.Equal(t, 1, sh.confirmCount())
	sh.confirmed(t, app)
	onLoop(app, func() { v.HandleKey(runeKey('w')) })
	require.NotNil(t, saved)
	assert.NotContains(t, saved.Models.Families, "quality")
}

// ── Patterns ───────────────────────────────────────────────────────────────

func TestPatternsViewKeys(t *testing.T) {
	frLocale(t)
	repo, tc := soloState(t)
	ctx := context.Background()
	require.NoError(t, repo.CreatePattern(ctx, teamstate.Pattern{Name: "api-crud", Description: "Ressource CRUD", Complexity: "medium", Tags: []string{"api"}, CreatedAt: "2026-10-01"}, "# api-crud\n"))
	v := NewPatternsView(func() TeamResolution { return tc })
	sh := &viewShell{}
	v.SetShell(sh)
	hints := v.StatusHints()
	assert.Contains(t, hints, "y valider")
	assert.Contains(t, hints, "n créer")
	assert.NotContains(t, hints, "v valider")

	content := tview.NewFlex()
	v.Mount(content, nil) // without app, the pull is skipped: the list is built at once
	text := screenText(t, content, 110, 18)
	assert.Contains(t, text, "api-crud")
	assert.Contains(t, text, "En attente de validation (1)")
	assertGolden(t, "patterns_view", text)

	v.HandleKey(runeKey('n'))
	require.NotNil(t, sh.form)

	selectSection(t, v.slist, "api-crud")
	v.HandleKey(runeKey('y'))
	ps, err := repo.ListPatterns(nil, 0)
	require.NoError(t, err)
	require.Len(t, ps, 1)
	assert.True(t, ps[0].Validated, "y validates the pattern")

	selectSection(t, v.slist, "api-crud")
	v.HandleKey(runeKey('d'))
	require.Equal(t, 1, sh.confirmCount())
	ps, _ = repo.ListPatterns(nil, 0)
	assert.Len(t, ps, 1, "nothing deleted before the confirmation")
	sh.confirmed(t, nil)
	ps, _ = repo.ListPatterns(nil, 0)
	assert.Empty(t, ps)
}

// ── Policies ───────────────────────────────────────────────────────────────

func TestPoliciesViewKeys(t *testing.T) {
	frLocale(t)
	repo, tc := soloState(t)
	ctx := context.Background()
	require.NoError(t, repo.SavePolicies(ctx, map[string]teamstate.Policy{
		"branch-naming": {Type: teamstate.PolicyTypeRegex, Target: "branch_name", Rule: "^feat/", Enforcement: teamstate.EnforcementRefuse, Message: "feat/*"},
	}))
	v := NewPoliciesView(func() TeamResolution { return tc }, nil)
	sh := &viewShell{}
	v.SetShell(sh)
	hints := v.StatusHints()
	assert.Contains(t, hints, "n créer")
	assert.Contains(t, hints, "c vérifier")
	assert.NotContains(t, hints, "a ajouter")

	content := tview.NewFlex()
	v.Mount(content, nil)
	text := screenText(t, content, 110, 18)
	assert.Contains(t, text, "branch-naming")
	assertGolden(t, "policies_view", text)

	v.HandleKey(runeKey('n'))
	require.NotNil(t, sh.form)
	assert.Equal(t, "Regex (branche, commit)", sh.form.Fields[1].Options[0].Label, "translated types")

	v.HandleKey(runeKey('t'))
	require.NotEmpty(t, sh.selectOpts, "t offers the templates")

	selectSection(t, v.slist, "branch-naming")
	v.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	assert.Equal(t, "Policy : branch-naming", sh.modalTitle)

	v.HandleKey(runeKey('d'))
	require.Equal(t, 1, sh.confirmCount())
	ps, err := repo.LoadPolicies("")
	require.NoError(t, err)
	assert.Len(t, ps, 1, "nothing deleted before the confirmation")
	sh.confirmed(t, nil)
	ps, _ = repo.LoadPolicies("")
	assert.Empty(t, ps)
}

// ── Activity ───────────────────────────────────────────────────────────────

func TestActivityViewRendersEvents(t *testing.T) {
	frLocale(t)
	repo, tc := soloState(t)
	require.NoError(t, repo.AppendEvent(context.Background(), teamstate.Event{
		Timestamp: time.Now().Add(-time.Hour), Actor: "alice", Type: teamstate.EventClaimTaken, Project: "web", Ticket: "oh-12",
	}))
	v := NewActivityView(func() TeamResolution { return tc })
	v.SetShell(&viewShell{})
	assert.Contains(t, v.StatusHints(), "r rafraîchir")
	content := tview.NewFlex()
	v.Mount(content, nil)
	assert.Contains(t, screenText(t, content, 100, 12), "oh-12")

	// t (today) and 0 (all) change the filter, r refreshes.
	for _, r := range "t0r" {
		assert.Nil(t, v.HandleKey(runeKey(r)))
	}
	assert.Equal(t, "all", v.filter)
	assert.Contains(t, screenText(t, content, 100, 12), "oh-12")

	// Without a team.
	v = NewActivityView(func() TeamResolution { return TeamResolution{} })
	content = tview.NewFlex()
	v.Mount(content, nil)
	assert.NotContains(t, screenText(t, content, 100, 6), "tui.")
}

// ── Takeover briefs ────────────────────────────────────────────────────────

func TestTakeoverViewShowsBriefs(t *testing.T) {
	frLocale(t)
	repo, tc := soloState(t)
	ctx := context.Background()
	b, err := repo.GenerateRawBrief(ctx, "web", "oh-7", "alice", "bob", "absence")
	require.NoError(t, err)
	require.NoError(t, repo.SaveBrief(ctx, b))

	v := NewTakeoverView(func() TeamResolution { return tc })
	sh := &viewShell{}
	v.SetShell(sh)
	enriched := make(chan string, 1)
	v.SetOnEnrich(func(_, id string) error { enriched <- id; return nil })
	content := tview.NewFlex()
	v.Mount(content, nil)
	text := screenText(t, content, 110, 14)
	assert.Contains(t, text, "oh-7")

	v.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	assert.Equal(t, "Brief : oh-7", sh.modalTitle)
	assert.NotEmpty(t, sh.modalBody)
	v.HandleKey(runeKey('e'))
	select {
	case id := <-enriched:
		assert.Equal(t, "oh-7", id, "e enriches the brief")
	case <-time.After(2 * time.Second):
		t.Fatal("not enriched")
	}
}

// ── Secrets ────────────────────────────────────────────────────────────────

func TestSecretsViewKeys(t *testing.T) {
	frLocale(t)
	v := NewSecretsView(SecretsViewConfig{
		GetStore: func() *keychain.Store { return nil },
		GetExpectedKeys: func() []ExpectedSecret {
			return []ExpectedSecret{{Key: "gitlab-token", Source: "mcp.gitlab", Scope: "global"}}
		},
		GetActiveProjectID:   func() string { return "" },
		GetActiveProjectName: func() string { return "" },
	})
	sh := &viewShell{}
	v.SetShell(sh)
	assert.Equal(t, "Secrets et tokens", v.Title())
	assert.Contains(t, v.StatusHints(), "n créer")
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return v.list != nil })
	text := drawn(t, app, content, 100, 12)
	assert.Contains(t, text, "gitlab-token")
	assert.Contains(t, text, "absent")

	onLoop(app, func() { selectSection(t, v.list, "gitlab-token"); v.HandleKey(runeKey('d')) })
	assert.Zero(t, sh.confirmCount(), "nothing to delete: no confirmation")
	assert.NotEmpty(t, sh.lastToast())

	onLoop(app, func() { v.HandleKey(runeKey('n')) })
	require.NotNil(t, sh.input, "n asks the key name")
}

// ── Project MCP ────────────────────────────────────────────────────────────

func TestProjectMCPViewToggleAndUndo(t *testing.T) {
	frLocale(t)
	p := &domain.Project{ID: "p1", Name: "web"}
	saves := make(chan struct{}, 8)
	v := NewProjectMCPView(ProjectMCPViewConfig{
		GetProject:  func() *domain.Project { return p },
		SaveProject: func(context.Context, *domain.Project) error { saves <- struct{}{}; return nil },
	})
	v.SetShell(&viewShell{})
	assert.Contains(t, v.StatusHints(), "u ")
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return v.list != nil })
	assert.Contains(t, drawn(t, app, content, 100, 20), "GitLab")

	var before, after string
	onLoop(app, func() {
		selectSection(t, v.list, i18n.T("tui.config.field.mcp_enabled.label"))
		before = mcpServiceEnabled(v.live, "gitlab")
		v.HandleKey(runeKey(' '))
		after = mcpServiceEnabled(v.live, "gitlab")
	})
	assert.NotEqual(t, before, after, "Space toggles")
	select {
	case <-saves:
	case <-time.After(3 * time.Second):
		t.Fatal("not auto-saved")
	}
	onLoop(app, func() { v.HandleKey(runeKey('u')); after = mcpServiceEnabled(v.live, "gitlab") })
	assert.Equal(t, before, after, "u undoes")
}

// ── Team MCP ───────────────────────────────────────────────────────────────

func TestTeamMCPViewRefreshAsksWhenDirty(t *testing.T) {
	frLocale(t)
	_, tc := soloState(t)
	var saved *teamstate.TeamConfig
	v := NewTeamMCPView(TeamMCPViewConfig{
		ResolveTeam:    func() TeamResolution { return tc },
		GetMCPConfig:   func() config.MCPConfig { return config.MCPConfig{} },
		SaveTeamConfig: func(_ context.Context, c *teamstate.TeamConfig) error { saved = c; return nil },
		GetHubConfig:   func() *config.Config { return &config.Config{} },
	})
	sh := &viewShell{}
	v.SetShell(sh)
	assert.Equal(t, "Services MCP", v.Title())
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return v.list != nil })

	// Clean: r reloads without asking.
	onLoop(app, func() { v.HandleKey(runeKey('r')) })
	assert.Zero(t, sh.confirmCount())

	onLoop(app, func() {
		selectSection(t, v.list, i18n.T("tui.config.field.mcp_enabled.label"))
		v.HandleKey(runeKey(' '))
	})
	waitFor(t, app, func() bool { return v.dirtyTeam || v.dirtyLocal })
	onLoop(app, func() { v.HandleKey(runeKey('r')) })
	assert.Equal(t, 1, sh.confirmCount(), "r asks before dropping the changes")
	onLoop(app, func() { v.HandleKey(runeKey('w')) })
	assert.NotNil(t, saved)
}

// ── History ────────────────────────────────────────────────────────────────

func TestHistoryViewRendersAndKeys(t *testing.T) {
	frLocale(t)
	at := time.Date(2026, 10, 14, 9, 30, 0, 0, time.Local)
	diffs := make(chan int, 2)
	v := NewHistoryView(HistoryViewConfig{
		Ref:     "team:hotfix",
		Diff:    func(_ context.Context, n int) (string, error) { diffs <- n; return "-a\n+b\n", nil },
		Restore: func(context.Context, int) (*PublishResult, error) { return &PublishResult{Version: 3}, nil },
	})
	sh := &viewShell{}
	v.SetShell(sh)
	assert.Contains(t, v.StatusHints(), "R restaurer")
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() {
		v.Mount(content, app)
		v.SetVersions([]HistoryVersion{
			{Version: 2, By: "ben", At: at, Message: "corrige le risque", Current: true},
			{Version: 1, By: "alice", At: at.Add(-24 * time.Hour), Message: "première version"},
		})
	})
	text := drawn(t, app, content, 100, 18)
	assertGolden(t, "history_view", text)

	onLoop(app, func() { v.list.SelectIndex(1); v.HandleKey(runeKey('c')) })
	select {
	case n := <-diffs:
		assert.Equal(t, 1, n, "c compares with the current version")
	case <-time.After(2 * time.Second):
		t.Fatal("no diff")
	}
	onLoop(app, func() { v.HandleKey(runeKey('R')) })
	assert.Equal(t, 1, sh.confirmCount())
}
