package runsvc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/filelock"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/worktree"
)

type fakeGit struct {
	repo    bool
	dirty   map[string]bool
	created []string
	stashed []string
}

func (g *fakeGit) Stash(p, _ string) (string, error) {
	g.stashed = append(g.stashed, p)
	g.dirty[p] = false
	return "abc1234", nil
}

func (g *fakeGit) IsRepo(string) bool    { return g.repo }
func (g *fakeGit) IsDirty(p string) bool { return g.dirty[p] }
func (g *fakeGit) CreateWorktree(project, branch string) (string, error) {
	p := worktree.SiblingPath(project, branch)
	g.created = append(g.created, branch)
	return p, os.MkdirAll(p, 0o755)
}

func runFixture(t *testing.T) (*rtFixture, *fakeGit) {
	f := newRTFixture(t)
	g := &fakeGit{repo: true, dirty: map[string]bool{}}
	f.svc.Git = g
	f.svc.LaunchLocksDir = filepath.Join(f.root, "locks")
	return f, g
}

func (f *rtFixture) run(risk string, loc LocationChoice, sessions ...PlannedInput) RunRequest {
	base := f.request("")
	base.Runtime, base.Dockerfile, base.BuildArgs = "", "", nil
	return RunRequest{Base: base, Workflow: WorkflowRef{ID: "ticket", Layer: "hub", Version: 2, Risk: risk},
		ProjectPath: f.project, Location: loc, Sessions: sessions}
}

func TestPlanBase(t *testing.T) {
	f, g := runFixture(t)
	ctx := context.Background()
	plan, err := f.svc.Plan(ctx, f.run("write", LocationChoice{Kind: LocationBase}))
	require.NoError(t, err)
	require.Len(t, plan.Sessions, 1)
	assert.Equal(t, Location{Path: f.project, Kind: LocationBase}, plan.Sessions[0].Location)
	assert.Empty(t, plan.Warnings)

	g.dirty[f.project] = true
	plan, err = f.svc.Plan(ctx, f.run("read", LocationChoice{Kind: LocationBase}))
	require.NoError(t, err)
	assert.Empty(t, plan.Warnings, "a reader does not care about uncommitted changes")
}

// A18: a writer never starts in a directory with uncommitted changes
// without an explicit choice: a worktree by default, the changes untouched.
func TestPlanDirtyBaseUsesAWorktreeByDefault(t *testing.T) {
	f, g := runFixture(t)
	ctx := context.Background()
	g.dirty[f.project] = true
	in := PlannedInput{Label: "bd-1", Branch: "feat/bd-1"}

	plan, err := f.svc.Plan(ctx, f.run("write", LocationChoice{Kind: LocationBase}, in))
	require.NoError(t, err)
	loc := plan.Sessions[0].Location
	wt := worktree.SiblingPath(f.project, "feat/bd-1")
	assert.Equal(t, Location{Path: wt, Kind: LocationWorktree, Branch: "feat/bd-1", Create: true, Auto: true}, loc)
	assert.Equal(t, []Warning{{Code: WarnDirtyWorktree, Args: []any{f.project, wt}}}, plan.Warnings)

	req := f.run("write", LocationChoice{Kind: LocationBase}, in)
	req.Dirty = DirtyAllow
	plan, err = f.svc.Plan(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, Location{Path: f.project, Kind: LocationBase}, plan.Sessions[0].Location)
	assert.Equal(t, []Warning{{Code: WarnDirty, Args: []any{f.project}}}, plan.Warnings)

	req.Dirty = DirtyStash
	plan, err = f.svc.Plan(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, Location{Path: f.project, Kind: LocationBase, Stash: true}, plan.Sessions[0].Location)
	assert.Equal(t, []Warning{{Code: WarnDirtyStash, Args: []any{f.project}}}, plan.Warnings)

	_, err = f.svc.Plan(ctx, f.run("write", LocationChoice{Kind: LocationBase}))
	assert.ErrorIs(t, err, ErrNoBranch, "the worktree needs a branch")

	g.repo = false
	plan, err = f.svc.Plan(ctx, f.run("write", LocationChoice{Kind: LocationBase}, in))
	require.NoError(t, err)
	assert.Equal(t, LocationBase, plan.Sessions[0].Location.Kind, "no git: no worktree")
}

func TestParseDirtyPolicy(t *testing.T) {
	for in, want := range map[string]DirtyPolicy{"": DirtyWorktree, "worktree": DirtyWorktree, "allow": DirtyAllow, "stash": DirtyStash} {
		got, err := ParseDirtyPolicy(in)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}
	_, err := ParseDirtyPolicy("drop")
	assert.Error(t, err)
}

func TestPlanAutoWorktreeWhenAnotherWriterIsActive(t *testing.T) {
	f, _ := runFixture(t)
	ctx := context.Background()
	require.NoError(t, f.svc.Sessions.Create(ctx, &domain.Session{ID: "ses_other", ProjectID: "p1", LaunchPath: f.project,
		State: domain.RunActive, WorkflowRisk: "write", Status: domain.SessionStatusRunning}))

	plan, err := f.svc.Plan(ctx, f.run("write", LocationChoice{Kind: LocationBase}, PlannedInput{Branch: "feat/x"}))
	require.NoError(t, err)
	loc := plan.Sessions[0].Location
	assert.True(t, loc.Auto)
	assert.True(t, loc.Create)
	assert.Equal(t, LocationWorktree, loc.Kind)
	assert.Equal(t, worktree.SiblingPath(f.project, "feat/x"), loc.Path)
	assert.Equal(t, WarnAutoWorktree, plan.Warnings[0].Code)

	plan, err = f.svc.Plan(ctx, f.run("read", LocationChoice{Kind: LocationBase}))
	require.NoError(t, err)
	assert.Equal(t, LocationBase, plan.Sessions[0].Location.Kind, "readers share the base")

	_, err = f.svc.Plan(ctx, f.run("write", LocationChoice{Kind: LocationBase}))
	assert.ErrorIs(t, err, ErrNoBranch, "O10 needs a branch")
}

func TestPlanReadingOrEndedSessionsDoNotHoldTheBase(t *testing.T) {
	f, _ := runFixture(t)
	ctx := context.Background()
	require.NoError(t, f.svc.Sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", LaunchPath: f.project,
		State: domain.RunStopped, WorkflowRisk: "write", Status: domain.SessionStatusRunning}))
	require.NoError(t, f.svc.Sessions.Create(ctx, &domain.Session{ID: "ses_b", ProjectID: "p1", LaunchPath: f.project,
		State: domain.RunActive, WorkflowRisk: "read", Status: domain.SessionStatusRunning}))
	plan, err := f.svc.Plan(ctx, f.run("write", LocationChoice{Kind: LocationBase}))
	require.NoError(t, err)
	assert.Equal(t, LocationBase, plan.Sessions[0].Location.Kind)
}

// A18: a sleeping writer resumes in its directory, on its branch: a new
// writer does not take it.
func TestPlanSleepingWriterHoldsTheBase(t *testing.T) {
	f, _ := runFixture(t)
	ctx := context.Background()
	require.NoError(t, f.svc.Sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", LaunchPath: f.project,
		State: domain.RunSleeping, WorkflowRisk: "write", Status: domain.SessionStatusRunning}))
	plan, err := f.svc.Plan(ctx, f.run("write", LocationChoice{Kind: LocationBase}, PlannedInput{Branch: "feat/x"}))
	require.NoError(t, err)
	assert.True(t, plan.Sessions[0].Location.Auto)
	assert.Equal(t, []Warning{{Code: WarnAutoWorktree, Args: []any{"ses_a", worktree.SiblingPath(f.project, "feat/x")}}}, plan.Warnings)
}

func TestPlanMultiTicketsOneWorktreeEach(t *testing.T) {
	f, _ := runFixture(t)
	plan, err := f.svc.Plan(context.Background(), f.run("write", LocationChoice{Kind: LocationBase},
		PlannedInput{Label: "bd-1", Branch: "feat/bd-1"}, PlannedInput{Label: "bd-2", Branch: "feat/bd-2"}))
	require.NoError(t, err)
	require.Len(t, plan.Sessions, 2)
	assert.Equal(t, 2, plan.Worktrees())
	assert.NotEqual(t, plan.Sessions[0].Location.Path, plan.Sessions[1].Location.Path)

	plan, err = f.svc.Plan(context.Background(), f.run("read", LocationChoice{Kind: LocationBase},
		PlannedInput{Label: "bd-1"}, PlannedInput{Label: "bd-2"}))
	require.NoError(t, err)
	assert.Equal(t, 0, plan.Worktrees(), "readers stay on the base")
}

func TestPlanLocationChoices(t *testing.T) {
	f, g := runFixture(t)
	ctx := context.Background()
	plan, err := f.svc.Plan(ctx, f.run("write", LocationChoice{Kind: LocationNew}, PlannedInput{Branch: "feat/y"}))
	require.NoError(t, err)
	assert.Equal(t, LocationWorktree, plan.Sessions[0].Location.Kind)
	assert.False(t, plan.Sessions[0].Location.Auto)

	wt := f.worktree(t, "proj-existing")
	plan, err = f.svc.Plan(ctx, f.run("write", LocationChoice{Kind: LocationWorktree, Path: wt}))
	require.NoError(t, err)
	assert.Equal(t, Location{Path: wt, Kind: LocationWorktree}, plan.Sessions[0].Location)

	_, err = f.svc.Plan(ctx, f.run("write", LocationChoice{Kind: LocationWorktree, Path: filepath.Join(f.root, "missing")}))
	assert.ErrorIs(t, err, os.ErrNotExist)

	g.repo = false
	_, err = f.svc.Plan(ctx, f.run("write", LocationChoice{Kind: LocationNew}, PlannedInput{Branch: "feat/z"}))
	assert.ErrorIs(t, err, ErrNotGitRepo)
}

func TestParseLocation(t *testing.T) {
	assert.Equal(t, LocationChoice{Kind: LocationBase}, ParseLocation(""))
	assert.Equal(t, LocationChoice{Kind: LocationBase}, ParseLocation("base"))
	assert.Equal(t, LocationChoice{Kind: LocationNew}, ParseLocation("new"))
	assert.Equal(t, LocationChoice{Kind: LocationWorktree, Path: "/tmp/x"}, ParseLocation("/tmp/x"))
}

func TestStartRunsTheSessionsInOneGroup(t *testing.T) {
	f, g := runFixture(t)
	ctx := context.Background()
	plan, err := f.svc.Plan(ctx, f.run("write", LocationChoice{Kind: LocationBase},
		PlannedInput{Label: "bd-1", Branch: "feat/bd-1", Title: "p1 · ticket bd-1"},
		PlannedInput{Label: "bd-2", Branch: "feat/bd-2", Title: "p1 · ticket bd-2"}))
	require.NoError(t, err)
	plan.Sessions[0].Prompt, plan.Sessions[1].Prompt = "do bd-1", "do bd-2"
	plan.Request.ParentSessionID = "ses_parent"

	res, err := f.svc.Start(ctx, plan)
	require.NoError(t, err)
	require.Len(t, res, 2)
	assert.Equal(t, res[0].GroupKey, res[1].GroupKey, "one server group (I5)")
	assert.Len(t, f.ad.started, 1, "one server")
	assert.Equal(t, []string{"feat/bd-1", "feat/bd-2"}, g.created)
	require.Len(t, f.ad.created, 2)
	assert.Equal(t, "do bd-1", f.ad.created[0].Prompt)
	assert.Equal(t, worktree.SiblingPath(f.project, "feat/bd-2"), f.ad.created[1].Location)

	sess, err := f.svc.Sessions.Get(ctx, res[1].SessionID)
	require.NoError(t, err)
	assert.Equal(t, "ticket", sess.WorkflowID)
	assert.Equal(t, "hub", sess.WorkflowLayer)
	assert.Equal(t, 2, sess.WorkflowVersion)
	assert.Equal(t, "write", sess.WorkflowRisk)
	assert.Equal(t, "worktree", sess.Location)
	assert.Equal(t, "ses_parent", sess.ParentSessionID)
	assert.Equal(t, "p1 · ticket bd-2", *sess.Title)
}

func TestStartRefusesADoubleLaunch(t *testing.T) {
	f, _ := runFixture(t)
	ctx := context.Background()
	plan, err := f.svc.Plan(ctx, f.run("write", LocationChoice{Kind: LocationBase}))
	require.NoError(t, err)
	unlock, err := filelock.TryLock(f.svc.launchLockPath("p1", f.project))
	require.NoError(t, err)
	_, err = f.svc.Start(ctx, plan)
	assert.ErrorIs(t, err, ErrLaunchInProgress)
	assert.Empty(t, f.ad.created)
	unlock()
	_, err = f.svc.Start(ctx, plan)
	require.NoError(t, err)
}

func TestRuntimeAvailability(t *testing.T) {
	f, _ := runFixture(t)
	ctx := context.Background()
	av, err := f.svc.RuntimeAvailability(ctx, "")
	require.NoError(t, err)
	assert.True(t, av.OK)
	_, err = f.svc.RuntimeAvailability(ctx, sessionspec.RuntimeContainer)
	require.NoError(t, err)
	_, err = f.svc.RuntimeAvailability(ctx, sessionspec.RuntimeRemote)
	assert.Error(t, err)
}

// QB2: every session started on tickets calls the claims hook (planned →
// in progress for any workflow, formerly the --dev alias only).
func TestStartCallsTheTicketsHook(t *testing.T) {
	f, _ := runFixture(t)
	ctx := context.Background()
	var got [][]string
	f.svc.OnTicketsStarted = func(_ context.Context, projectID string, tickets []string) []string {
		got = append(got, tickets)
		return []string{"claimed " + tickets[0]}
	}
	plan, err := f.svc.Plan(ctx, f.run("write", LocationChoice{Kind: LocationBase},
		PlannedInput{Label: "bd-1", Branch: "feat/bd-1", Tickets: []string{"bd-1"}},
		PlannedInput{Label: "bd-2", Branch: "feat/bd-2", Tickets: []string{"bd-2"}}))
	require.NoError(t, err)
	res, err := f.svc.Start(ctx, plan)
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"bd-1"}, {"bd-2"}}, got)
	assert.Equal(t, []string{"claimed bd-2"}, res[1].Notes)
}

// A18: --stash puts the changes aside before the session starts and returns
// the stash commit.
func TestStartStashesTheChanges(t *testing.T) {
	f, g := runFixture(t)
	ctx := context.Background()
	g.dirty[f.project] = true
	req := f.run("write", LocationChoice{Kind: LocationBase}, PlannedInput{Branch: "feat/x"})
	req.Dirty = DirtyStash
	plan, err := f.svc.Plan(ctx, req)
	require.NoError(t, err)
	res, err := f.svc.Start(ctx, plan)
	require.NoError(t, err)
	assert.Equal(t, []string{f.project}, g.stashed)
	assert.Equal(t, "abc1234", res[0].Stashed)
	assert.Equal(t, f.project, f.ad.created[0].Location)
}

// A18 with a real repository: `oh run ticket` on a base with an uncommitted
// change starts the session in a new worktree and leaves the change as it is.
func TestStartOnADirtyBaseKeepsTheUserChanges(t *testing.T) {
	f := newRTFixture(t)
	f.svc.LaunchLocksDir = filepath.Join(f.root, "locks")
	ctx := context.Background()
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", f.project}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	git("init", "-b", "main")
	store := filepath.Join(f.project, "store.py")
	require.NoError(t, os.WriteFile(store, []byte("v1\n"), 0o644))
	git("add", "store.py")
	git("commit", "-m", "init")
	require.NoError(t, os.WriteFile(store, []byte("v1 + user change\n"), 0o644))

	plan, err := f.svc.Plan(ctx, f.run("write", LocationChoice{Kind: LocationBase}, PlannedInput{Branch: "feat/pt-1"}))
	require.NoError(t, err)
	assert.Equal(t, WarnDirtyWorktree, plan.Warnings[0].Code)
	_, err = f.svc.Start(ctx, plan)
	require.NoError(t, err)
	wt := worktree.SiblingPath(f.project, "feat/pt-1")
	t.Cleanup(func() { _ = os.RemoveAll(wt) })
	require.Len(t, f.ad.created, 1)
	assert.Equal(t, wt, f.ad.created[0].Location)
	got, err := os.ReadFile(store)
	require.NoError(t, err)
	assert.Equal(t, "v1 + user change\n", string(got), "the user's change is untouched")
	got, err = os.ReadFile(filepath.Join(wt, "store.py"))
	require.NoError(t, err)
	assert.Equal(t, "v1\n", string(got), "the session starts from the last commit")
}
