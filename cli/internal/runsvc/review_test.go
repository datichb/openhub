package runsvc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
	"github.com/datichb/openhub/cli/internal/worktree"
)

func reviewFixture(t *testing.T) (*rtFixture, *fakeGit) {
	f, g := runFixture(t)
	f.svc.WorktreesDir = filepath.Join(f.root, "worktrees")
	g.targets = map[string]worktree.ReadTarget{
		"feat/x": {Ref: "refs/heads/feat/x"},
		"main":   {Ref: "refs/heads/main", Current: true},
		"feat/y": {Ref: "refs/remotes/origin/feat/y"},
	}
	return f, g
}

func TestPlanReviewOfAnotherBranchGetsATemporaryWorktree(t *testing.T) {
	f, _ := reviewFixture(t)
	ctx := context.Background()
	for _, choice := range []LocationChoice{{Kind: LocationBase}, {Kind: LocationNew}} {
		plan, err := f.svc.Plan(ctx, f.run("read", choice, PlannedInput{Branch: "oh/review-1", ReadBranch: "feat/x"}))
		require.NoError(t, err)
		loc := plan.Sessions[0].Location
		assert.True(t, loc.Temp && loc.Create, "%+v", loc)
		assert.Equal(t, LocationWorktree, loc.Kind)
		assert.Equal(t, "feat/x", loc.Branch)
		assert.Equal(t, "refs/heads/feat/x", loc.Ref)
		assert.True(t, worktree.IsTemp(f.svc.WorktreesDir, loc.Path), loc.Path)
		assert.Equal(t, []Warning{{Code: WarnReviewWorktree, Args: []any{"feat/x", loc.Path}}}, plan.Warnings)
		assert.Empty(t, loc.WorkBranch(f.project), "a detached checkout is the work of no branch")
	}

	plan, err := f.svc.Plan(ctx, f.run("read", LocationChoice{Kind: LocationBase}, PlannedInput{ReadBranch: "feat/y"}))
	require.NoError(t, err)
	assert.Equal(t, "refs/remotes/origin/feat/y", plan.Sessions[0].Location.Ref, "a branch only the remote has")
}

func TestPlanReviewOfTheCurrentBranchIsUnchanged(t *testing.T) {
	f, g := reviewFixture(t)
	ctx := context.Background()
	plan, err := f.svc.Plan(ctx, f.run("read", LocationChoice{Kind: LocationBase}, PlannedInput{ReadBranch: "main"}))
	require.NoError(t, err)
	assert.Equal(t, Location{Path: f.project, Kind: LocationBase}, plan.Sessions[0].Location)
	assert.Empty(t, plan.Warnings)

	plan, err = f.svc.Plan(ctx, f.run("read", LocationChoice{Kind: LocationBase}, PlannedInput{}))
	require.NoError(t, err)
	assert.Equal(t, LocationBase, plan.Sessions[0].Location.Kind, "no branch given: recent changes of the current branch")

	g.repo = false
	plan, err = f.svc.Plan(ctx, f.run("read", LocationChoice{Kind: LocationBase}, PlannedInput{ReadBranch: "feat/x"}))
	require.NoError(t, err)
	assert.Equal(t, LocationBase, plan.Sessions[0].Location.Kind, "no git: nothing to check out")
}

func TestPlanReviewOfAWriterIsNotATemporaryWorktree(t *testing.T) {
	f, _ := reviewFixture(t)
	plan, err := f.svc.Plan(context.Background(), f.run("write", LocationChoice{Kind: LocationBase}, PlannedInput{Branch: "feat/x", ReadBranch: "feat/x"}))
	require.NoError(t, err)
	assert.False(t, plan.Sessions[0].Location.Temp)
}

func TestPlanReviewOfAMissingBranch(t *testing.T) {
	f, _ := reviewFixture(t)
	_, err := f.svc.Plan(context.Background(), f.run("read", LocationChoice{Kind: LocationBase}, PlannedInput{ReadBranch: "nope"}))
	var rb *ReadBranchError
	require.ErrorAs(t, err, &rb)
	assert.Equal(t, "nope", rb.Branch)
	assert.ErrorIs(t, err, worktree.ErrBranchNotFound)
}

// An explicit worktree is the user's choice, but it is said when it is not
// on the reviewed branch.
func TestPlanReviewInAWorktreeOfAnotherBranchWarns(t *testing.T) {
	f, _ := reviewFixture(t)
	wt := f.worktree(t, "proj-other")
	gittest.Run(t, wt, "init", "-q", "-b", "other")
	gittest.Commit(t, wt, "a.txt", "a\n", "a")
	plan, err := f.svc.Plan(context.Background(), f.run("read", LocationChoice{Kind: LocationWorktree, Path: wt}, PlannedInput{ReadBranch: "feat/x"}))
	require.NoError(t, err)
	assert.False(t, plan.Sessions[0].Location.Temp)
	assert.Equal(t, []Warning{{Code: WarnReadOtherBranch, Args: []any{"feat/x", wt, "other"}}}, plan.Warnings)
}

func TestStartReviewCreatesTheWorktreeAndStopRemovesIt(t *testing.T) {
	f, g := reviewFixture(t)
	ctx := context.Background()
	plan, err := f.svc.Plan(ctx, f.run("read", LocationChoice{Kind: LocationBase}, PlannedInput{ReadBranch: "feat/x"}))
	require.NoError(t, err)
	path := plan.Sessions[0].Location.Path

	res, err := f.svc.Start(ctx, plan)
	require.NoError(t, err)
	assert.Equal(t, [][2]string{{path, "refs/heads/feat/x"}}, g.detached)
	assert.Empty(t, g.created, "no branch is created")
	assert.Equal(t, path, f.ad.created[0].Location, "the session works there")
	sess, err := f.svc.Session(ctx, res[0].SessionID)
	require.NoError(t, err)
	assert.Equal(t, path, sess.LaunchPath)
	assert.False(t, sess.WorktreeAuto, "not a worktree for oh gc to clean with its branch")
	assert.Empty(t, sess.WorktreeBranch)
	assert.DirExists(t, path)

	require.NoError(t, f.svc.StopSession(ctx, sess.ID))
	assert.NoDirExists(t, path)
}

func TestStopKeepsTheTemporaryWorktreeAnotherOpenSessionUses(t *testing.T) {
	f, _ := reviewFixture(t)
	ctx := context.Background()
	path := worktree.NewTempPath(f.svc.WorktreesDir, "p1", "feat/x")
	require.NoError(t, os.MkdirAll(path, 0o755))
	f.addSession(t, domain.Session{ID: "ses_a", LaunchPath: path, State: domain.RunIdle, WorkflowRisk: "read"})
	f.addSession(t, domain.Session{ID: "ses_b", LaunchPath: path, State: domain.RunIdle, WorkflowRisk: "read"})
	require.NoError(t, f.svc.StopSession(ctx, "ses_a"))
	assert.DirExists(t, path)
	require.NoError(t, f.svc.StopSession(ctx, "ses_b"))
	assert.NoDirExists(t, path)
}

func TestStopLeavesOtherLocationsAlone(t *testing.T) {
	f, _ := reviewFixture(t)
	ctx := context.Background()
	wt := f.worktree(t, "proj-feat-x") // a worktree of the user's: not under the temporary folder
	f.addSession(t, domain.Session{ID: "ses_a", LaunchPath: wt, State: domain.RunIdle})
	f.addSession(t, domain.Session{ID: "ses_base", LaunchPath: f.project, State: domain.RunIdle})
	require.NoError(t, f.svc.StopSession(ctx, "ses_a"))
	require.NoError(t, f.svc.StopSession(ctx, "ses_base"))
	assert.DirExists(t, wt)
	assert.DirExists(t, f.project)
}

// Deleting a review session reports its temporary worktree removed once, not
// as a failure of a second removal.
func TestDeleteReviewSession(t *testing.T) {
	repo := gittest.Repo(t)
	gittest.Run(t, repo, "switch", "-q", "-c", "feat/x")
	gittest.Commit(t, repo, "x.txt", "x\n", "x")
	gittest.Run(t, repo, "switch", "-q", "main")
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", repo)
	root := filepath.Join(t.TempDir(), "worktrees")
	path := worktree.NewTempPath(root, "p1", "feat/x")
	require.NoError(t, worktree.AddDetached(repo, path, "refs/heads/feat/x"))
	sessions := sqlite.NewSessionStore(st)
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_r", ProjectID: "p1", LaunchPath: path, State: domain.RunIdle, Status: domain.SessionStatusRunning}))
	svc := &Service{Sessions: sessions, Servers: sqlite.NewServerStore(st), WorktreesDir: root}

	rep, err := svc.DeleteSession(ctx, "ses_r", DeleteOptions{ProjectPath: repo})
	require.NoError(t, err)
	assert.Empty(t, rep.Errors)
	assert.Equal(t, path, rep.WorktreeRemoved)
	assert.Empty(t, rep.BranchDeleted, "the reviewed branch is never deleted")
	assert.NoDirExists(t, path)
	assert.True(t, worktree.BranchExists(repo, "feat/x"))
}

// With a real repository: the session's directory holds the files of the
// reviewed branch, the project keeps its own, and the stop cleans up.
func TestStartReviewWithARealRepository(t *testing.T) {
	f := newRTFixture(t)
	f.svc.LaunchLocksDir = filepath.Join(f.root, "locks")
	f.svc.WorktreesDir = filepath.Join(f.root, "worktrees")
	ctx := context.Background()
	gittest.Run(t, f.project, "init", "-q", "-b", "main")
	gittest.Commit(t, f.project, "store.py", "main\n", "init")
	gittest.Run(t, f.project, "switch", "-q", "-c", "feat/x")
	gittest.Commit(t, f.project, "store.py", "branch\n", "change")
	gittest.Run(t, f.project, "switch", "-q", "main")

	plan, err := f.svc.Plan(ctx, f.run("read", LocationChoice{Kind: LocationBase}, PlannedInput{ReadBranch: "feat/x"}))
	require.NoError(t, err)
	res, err := f.svc.Start(ctx, plan)
	require.NoError(t, err)
	dir := f.ad.created[0].Location
	assert.NotEqual(t, f.project, dir)
	got, err := os.ReadFile(filepath.Join(dir, "store.py"))
	require.NoError(t, err)
	assert.Equal(t, "branch\n", string(got), "read, grep and glob see the reviewed branch")
	got, err = os.ReadFile(filepath.Join(f.project, "store.py"))
	require.NoError(t, err)
	assert.Equal(t, "main\n", string(got), "the project is untouched")
	assert.Equal(t, "main", gittest.Run(t, f.project, "branch", "--show-current"))

	require.NoError(t, f.svc.StopSession(ctx, res[0].SessionID))
	assert.NoDirExists(t, dir)
	assert.NotContains(t, gittest.Run(t, f.project, "worktree", "list"), dir)
}

func TestStartReviewFailureRemovesTheWorktree(t *testing.T) {
	f, g := reviewFixture(t)
	ctx := context.Background()
	plan, err := f.svc.Plan(ctx, f.run("read", LocationChoice{Kind: LocationBase}, PlannedInput{ReadBranch: "feat/x"}))
	require.NoError(t, err)
	f.ad.attestErr = errors.New("attestation refused")
	_, err = f.svc.Start(ctx, plan)
	require.Error(t, err)
	require.Len(t, g.detached, 1)
	assert.NoDirExists(t, g.detached[0][0])
}
