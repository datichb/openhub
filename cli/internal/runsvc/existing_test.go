package runsvc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/worktree"
)

func (f *rtFixture) addSession(t *testing.T, s domain.Session) {
	t.Helper()
	if s.ProjectID == "" {
		s.ProjectID = "p1"
	}
	if s.Status == "" {
		s.Status = domain.SessionStatusRunning
	}
	if s.WorkflowRisk == "" {
		s.WorkflowRisk = "write"
	}
	require.NoError(t, f.svc.Sessions.Create(context.Background(), &s))
}

func ticketInput(ticket string) PlannedInput {
	return PlannedInput{Label: ticket, Branch: "feat/" + ticket, Tickets: []string{ticket}}
}

func TestParseOnExisting(t *testing.T) {
	for _, c := range OnExistingChoices {
		got, err := ParseOnExisting(string(c))
		require.NoError(t, err)
		assert.Equal(t, c, got)
	}
	got, err := ParseOnExisting("")
	require.NoError(t, err)
	assert.Equal(t, OnExistingUnset, got)
	_, err = ParseOnExisting("drop")
	assert.Error(t, err)
}

// A launch on a ticket finds the open sessions of the ticket whatever their
// workflow; the same workflow needs a choice before Start.
func TestPlanDetectsExistingSessionsByTicket(t *testing.T) {
	f, _ := runFixture(t)
	ctx := context.Background()
	wt := f.worktree(t, "proj-feat-bd-1")
	f.addSession(t, domain.Session{ID: "ses_same", LaunchPath: wt, State: domain.RunIdle, WorkflowID: "ticket", WorkflowVersion: 1,
		Tickets: []string{"bd-1"}, Runtime: "container", Mode: "auto", Location: "worktree", WorktreeBranch: "feat/bd-1", GroupKey: "g", BundleHash: "old"})
	f.addSession(t, domain.Session{ID: "ses_other", LaunchPath: f.project, State: domain.RunSleeping, WorkflowID: "review", WorkflowRisk: "read",
		Tickets: []string{"bd-1", "bd-9"}, StartedAt: time.Now().Add(-time.Hour)})
	f.addSession(t, domain.Session{ID: "ses_stopped", LaunchPath: f.project, State: domain.RunStopped, WorkflowID: "ticket", Tickets: []string{"bd-1"}})
	f.addSession(t, domain.Session{ID: "ses_unrelated", LaunchPath: f.project, State: domain.RunActive, WorkflowID: "ticket", WorkflowRisk: "read", Tickets: []string{"bd-2"}})

	plan, err := f.svc.Plan(ctx, f.run("write", LocationChoice{Kind: LocationBase}, ticketInput("bd-1")))
	require.NoError(t, err)
	ids := []string{}
	for _, e := range plan.Existing {
		ids = append(ids, e.ID)
	}
	assert.ElementsMatch(t, []string{"ses_same", "ses_other"}, ids)
	assert.True(t, plan.NeedsChoice, "same workflow: a choice is needed")
	assert.Equal(t, "ses_same", plan.Sessions[0].Existing, "the choice applies to the session of the same workflow")
	same, _ := existingByID(plan.Existing, "ses_same")
	assert.True(t, same.SameWorkflow)
	assert.Equal(t, []string{"bd-1"}, same.Shared)
	assert.True(t, same.Differs(DiffRuntime), "container vs local")
	assert.True(t, same.Differs(DiffVersion), "v1 vs v2")
	assert.True(t, same.Differs(DiffBundle))
	assert.True(t, same.Allows(OnExistingTransfer))
	assert.Contains(t, warningCodes(plan), WarnParallel, "ses_other is not the target")

	_, err = f.svc.Start(ctx, plan)
	var ce *ExistingChoiceError
	require.ErrorAs(t, err, &ce)
	assert.ErrorIs(t, err, ErrNeedsChoice)
	assert.Empty(t, f.ad.created, "nothing started")

	// Another workflow only: informational, the launch goes on beside it.
	other := f.run("write", LocationChoice{Kind: LocationBase}, ticketInput("bd-9"))
	plan, err = f.svc.Plan(ctx, other)
	require.NoError(t, err)
	require.Len(t, plan.Existing, 1)
	assert.False(t, plan.NeedsChoice)
	assert.Contains(t, warningCodes(plan), WarnExisting)
	_, err = f.svc.Start(ctx, plan)
	require.NoError(t, err)

	// A remote session can only be worked on in parallel.
	f.addSession(t, domain.Session{ID: "ses_remote", State: domain.RunActive, WorkflowID: "ticket", Runtime: "remote", Tickets: []string{"bd-5"}})
	req := f.run("write", LocationChoice{Kind: LocationBase}, ticketInput("bd-5"))
	req.OnExisting = OnExistingResume
	_, err = f.svc.Plan(ctx, req)
	assert.ErrorIs(t, err, ErrChoiceUnavailable)
}

func warningCodes(p *RunPlan) []string {
	var out []string
	for _, w := range p.Warnings {
		out = append(out, w.Code)
	}
	return out
}

// Gaps of the location planning: an automatic worktree used by another
// writer, an existing worktree reused, the base chosen while the ticket is
// worked on in its worktree.
func TestPlanLocationGaps(t *testing.T) {
	f, _ := runFixture(t)
	ctx := context.Background()
	f.addSession(t, domain.Session{ID: "ses_base", LaunchPath: f.project, State: domain.RunActive, WorkflowID: "x"})
	wt := worktree.SiblingPath(f.project, "feat/x")
	require.NoError(t, os.MkdirAll(wt, 0o755))
	f.addSession(t, domain.Session{ID: "ses_wt", LaunchPath: wt, State: domain.RunIdle, WorkflowID: "x"})

	plan, err := f.svc.Plan(ctx, f.run("write", LocationChoice{Kind: LocationBase}, PlannedInput{Branch: "feat/x"}))
	require.NoError(t, err)
	assert.Equal(t, wt, plan.Sessions[0].Location.Path)
	assert.Contains(t, plan.Warnings, Warning{Code: WarnWorktreeBusy, Args: []any{"ses_wt", wt}})
	assert.Contains(t, plan.Warnings, Warning{Code: WarnAutoWorktreeExisting, Args: []any{"ses_base", wt}}, "the worktree exists: not a new one")

	f2, _ := runFixture(t)
	wt2 := f2.worktree(t, "proj-feat-bd-1")
	f2.addSession(t, domain.Session{ID: "ses_t", LaunchPath: wt2, State: domain.RunIdle, WorkflowID: "review", Tickets: []string{"bd-1"}})
	plan, err = f2.svc.Plan(ctx, f2.run("read", LocationChoice{Kind: LocationBase}, ticketInput("bd-1")))
	require.NoError(t, err)
	assert.Equal(t, LocationBase, plan.Sessions[0].Location.Kind)
	assert.Contains(t, plan.Warnings, Warning{Code: WarnTicketWorktree, Args: []any{"ses_t", wt2}})
}

func TestPendingWarningCounts(t *testing.T) {
	open := []domain.Decision{{SessionID: "a"}, {SessionID: "a"}, {SessionID: "b"}, {SessionID: "elsewhere"}}
	existing := []ExistingSession{{ID: "a"}}
	w := pendingWarning(open, []domain.Session{{ID: "a"}, {ID: "b"}}, existing)
	assert.Equal(t, Warning{Code: WarnPending, Args: []any{2, 3, 4}}, w)
	assert.Equal(t, 2, existing[0].OpenDecisions)
}

// sessionWorktree creates the worktree of a branch with a commit and an
// uncommitted change, and an open session working there.
func sessionWorktree(t *testing.T, f *rtFixture, id, branch string) string {
	t.Helper()
	wt, err := worktree.ResolveOrCreate(f.project, branch)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(wt) })
	gittest.Commit(t, wt, "store.py", "work\n", "feat: the store")
	require.NoError(t, os.WriteFile(filepath.Join(wt, "wip.txt"), []byte("wip\n"), 0o644))
	f.addSession(t, domain.Session{ID: id, LaunchPath: wt, State: domain.RunIdle, WorkflowID: "ticket", Tickets: []string{"bd-1"},
		Location: "worktree", WorktreeAuto: true, WorktreeBranch: branch, Title: ptr("p1 · ticket · bd-1")})
	return wt
}

func ptr(s string) *string { return &s }

// keep: the existing session stops, the new one continues in its worktree
// with a handoff brief (commits, files).
func TestStartKeepContinuesTheWork(t *testing.T) {
	f := newRTFixture(t)
	f.svc.LaunchLocksDir = filepath.Join(f.root, "locks")
	f.svc.SessionsDir = filepath.Join(f.root, "sessions")
	ctx := context.Background()
	gittest.Run(t, f.project, "init", "-q", "-b", "main")
	gittest.Commit(t, f.project, "README", "v1\n", "init")
	wt := sessionWorktree(t, f, "ses_old", "feat/bd-1")

	req := f.run("write", LocationChoice{Kind: LocationBase}, ticketInput("bd-1"))
	plan, err := f.svc.Plan(ctx, req)
	require.NoError(t, err)
	require.Len(t, plan.Existing, 1)
	e := plan.Existing[0]
	assert.Equal(t, WorkInfo{Known: true, CommitsAhead: 1, CommitsUnpushed: 1, ChangedFiles: 1}, e.Work)

	req.OnExisting = OnExistingKeep
	plan, err = f.svc.Plan(ctx, req)
	require.NoError(t, err)
	assert.False(t, plan.NeedsChoice)
	loc := plan.Sessions[0].Location
	assert.Equal(t, filepath.Clean(wt), loc.Path, "the worktree of the work is reused")
	assert.False(t, loc.Create)
	assert.Equal(t, OnExistingKeep, plan.Sessions[0].Action)
	assert.NotContains(t, warningCodes(plan), WarnDirty, "its changes are the work being kept")

	res, err := f.svc.Start(ctx, plan)
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "ses_old", res[0].Replaced)
	old, err := f.svc.Sessions.Get(ctx, "ses_old")
	require.NoError(t, err)
	assert.Equal(t, domain.RunStopped, old.State)
	sess, err := f.svc.Sessions.Get(ctx, res[0].SessionID)
	require.NoError(t, err)
	assert.Equal(t, "ses_old", sess.ParentSessionID)
	assert.Equal(t, filepath.Clean(wt), filepath.Clean(sess.LaunchPath))
	assert.True(t, sess.WorktreeAuto)
	assert.Equal(t, "feat/bd-1", sess.WorktreeBranch)

	brief, err := os.ReadFile(filepath.Join(f.svc.SessionsDir, res[0].SessionID, "handoff.md"))
	require.NoError(t, err)
	for _, want := range []string{"ses_old", "feat: the store", "store.py", "wip.txt", "bd-1"} {
		assert.Contains(t, string(brief), want)
	}
}

// resume: no new session, the existing one is returned.
func TestStartResumeReturnsTheExistingSession(t *testing.T) {
	f, _ := runFixture(t)
	ctx := context.Background()
	r, err := f.svc.StartSession(ctx, StartRequest{ProjectID: "p1", Location: f.project, Bundle: f.b, Provider: "bedrock",
		ProviderCfg: providerCfg(), Attach: sessionspec.AttachNone, WorkflowID: "ticket", Tickets: []string{"bd-1"}, WorkflowRisk: "write"})
	require.NoError(t, err)
	req := f.run("write", LocationChoice{Kind: LocationBase}, ticketInput("bd-1"))
	req.OnExisting = OnExistingResume
	plan, err := f.svc.Plan(ctx, req)
	require.NoError(t, err)
	res, err := f.svc.Start(ctx, plan)
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, r.SessionID, res[0].SessionID)
	assert.Equal(t, OnExistingResume, res[0].Existing)
	assert.Len(t, f.ad.created, 1, "no new session")
}

// delete: the plan lists the losses, the deletion removes the worktree, the
// local branch, the record and the folder of the session.
func TestPlanDeleteAndDeleteSession(t *testing.T) {
	f := newRTFixture(t)
	f.svc.SessionsDir = filepath.Join(f.root, "sessions")
	ctx := context.Background()
	gittest.Run(t, f.project, "init", "-q", "-b", "main")
	gittest.Commit(t, f.project, "README", "v1\n", "init")
	wt := sessionWorktree(t, f, "ses_old", "feat/bd-1")
	require.NoError(t, os.MkdirAll(filepath.Join(f.svc.SessionsDir, "ses_old"), 0o700))

	p, err := f.svc.PlanDelete(ctx, "ses_old", DeleteOptions{ProjectPath: f.project})
	require.NoError(t, err)
	assert.False(t, p.InBase)
	assert.Equal(t, filepath.Clean(wt), p.Worktree)
	assert.Equal(t, "feat/bd-1", p.Branch)
	assert.False(t, p.RemoteBranch)
	require.Len(t, p.UnpushedCommits, 1)
	assert.Contains(t, p.UnpushedCommits[0], "feat: the store")
	assert.Equal(t, []string{"wip.txt"}, p.ChangedFiles)
	assert.True(t, p.Losses(false))

	// Another open session in the worktree: it stays.
	f.addSession(t, domain.Session{ID: "ses_peer", LaunchPath: wt, State: domain.RunIdle})
	p, err = f.svc.PlanDelete(ctx, "ses_old", DeleteOptions{ProjectPath: f.project})
	require.NoError(t, err)
	assert.Equal(t, []string{"ses_peer"}, p.SharedWith)
	assert.False(t, p.Losses(false))
	peer, _ := f.svc.Sessions.Get(ctx, "ses_peer")
	peer.State = domain.RunStopped
	require.NoError(t, f.svc.Sessions.Update(ctx, peer))

	rep, err := f.svc.DeleteSession(ctx, "ses_old", DeleteOptions{ProjectPath: f.project})
	require.NoError(t, err)
	assert.Empty(t, rep.Errors)
	assert.True(t, rep.Stopped)
	assert.Equal(t, filepath.Clean(wt), rep.WorktreeRemoved)
	assert.Equal(t, "feat/bd-1", rep.BranchDeleted)
	assert.NoDirExists(t, wt)
	assert.False(t, worktree.BranchExists(f.project, "feat/bd-1"))
	assert.NoDirExists(t, filepath.Join(f.svc.SessionsDir, "ses_old"))
	_, err = f.svc.Sessions.Get(ctx, "ses_old")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// A session of the project directory: its changes go only with
// DiscardBaseChanges.
func TestDeleteSessionInTheBase(t *testing.T) {
	f := newRTFixture(t)
	ctx := context.Background()
	gittest.Run(t, f.project, "init", "-q", "-b", "main")
	gittest.Commit(t, f.project, "README", "v1\n", "init")
	readme := filepath.Join(f.project, "README")
	require.NoError(t, os.WriteFile(readme, []byte("changed\n"), 0o644))
	for _, id := range []string{"ses_a", "ses_b"} {
		f.addSession(t, domain.Session{ID: id, LaunchPath: f.project, State: domain.RunIdle, Location: "base"})
	}
	p, err := f.svc.PlanDelete(ctx, "ses_a", DeleteOptions{})
	require.NoError(t, err)
	assert.True(t, p.InBase)
	assert.Empty(t, p.Worktree)
	assert.Equal(t, []string{"README"}, p.ChangedFiles)

	rep, err := f.svc.DeleteSession(ctx, "ses_a", DeleteOptions{})
	require.NoError(t, err)
	assert.True(t, rep.BaseChangesKept)
	data, _ := os.ReadFile(readme)
	assert.Equal(t, "changed\n", string(data))

	rep, err = f.svc.DeleteSession(ctx, "ses_b", DeleteOptions{DiscardBaseChanges: true})
	require.NoError(t, err)
	assert.True(t, rep.Discarded)
	data, _ = os.ReadFile(readme)
	assert.Equal(t, "v1\n", string(data))
}

// delete in a launch: the session and its worktree go, the new session
// starts in a fresh worktree of the branch (--location new; with the base,
// free again, the session starts there).
func TestStartDeleteThenLaunch(t *testing.T) {
	f := newRTFixture(t)
	f.svc.LaunchLocksDir = filepath.Join(f.root, "locks")
	ctx := context.Background()
	gittest.Run(t, f.project, "init", "-q", "-b", "main")
	gittest.Commit(t, f.project, "README", "v1\n", "init")
	wt := sessionWorktree(t, f, "ses_old", "feat/bd-1")
	req := f.run("write", LocationChoice{Kind: LocationNew}, ticketInput("bd-1"))
	req.OnExisting = OnExistingDelete
	plan, err := f.svc.Plan(ctx, req)
	require.NoError(t, err)
	assert.True(t, plan.Sessions[0].Location.Create, "the worktree is created again")
	res, err := f.svc.Start(ctx, plan)
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.NotNil(t, res[0].Deleted)
	assert.Equal(t, "feat/bd-1", res[0].Deleted.BranchDeleted)
	_, err = f.svc.Sessions.Get(ctx, "ses_old")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.NoFileExists(t, filepath.Join(wt, "store.py"), "the new worktree starts from main")
	assert.Equal(t, filepath.Clean(wt), filepath.Clean(f.ad.created[0].Location))
}

// porterAdapter moves transcripts between the fake servers.
type porterAdapter struct {
	*rtAdapter
	pmu       sync.Mutex
	imported  []string // locations
	importErr error
	switched  []string
}

func (a *porterAdapter) ExportSession(_ context.Context, _ adapters.ServerHandle, id string) ([]byte, error) {
	return []byte("transcript " + id), nil
}

func (a *porterAdapter) ImportSession(_ context.Context, _ adapters.ServerHandle, _ []byte, location string) (string, error) {
	a.pmu.Lock()
	defer a.pmu.Unlock()
	if a.importErr != nil {
		return "", a.importErr
	}
	a.imported = append(a.imported, location)
	return "", nil
}

func (a *porterAdapter) Control(_ context.Context, _ adapters.ServerHandle, _ string, op adapters.ControlOp) error {
	if op.Kind == adapters.ControlSwitchModel && op.Model != nil {
		a.pmu.Lock()
		a.switched = append(a.switched, op.Model.String())
		a.pmu.Unlock()
	}
	return nil
}

// transfer: a local session moves to the container runtime (location as
// the container sees it) and the new model; the local server stops.
func TestTransferSessionLocalToContainer(t *testing.T) {
	f, _ := runFixture(t)
	pa := &porterAdapter{rtAdapter: f.ad}
	f.svc.Adapter = pa
	ctx := context.Background()
	local := f.request(f.project)
	local.Runtime, local.Dockerfile, local.BuildArgs = "", "", nil
	local.WorkflowID, local.Tickets = "ticket", []string{"bd-1"}
	r, err := f.svc.StartSession(ctx, local)
	require.NoError(t, err)
	srcGroup := r.GroupKey

	// Import refused: the session stays where it was.
	pa.importErr = errors.New("boom")
	target := f.request("")
	target.Bundle.Spec.DefaultModel = &sessionspec.ModelRef{Provider: "amazon-bedrock", Model: "claude-x"}
	_, err = f.svc.TransferSession(ctx, r.SessionID, target)
	var te *TransferError
	require.ErrorAs(t, err, &te)
	sess, err := f.svc.Sessions.Get(ctx, r.SessionID)
	require.NoError(t, err)
	assert.Equal(t, srcGroup, sess.GroupKey)
	assert.Equal(t, "local", sess.Runtime)
	assert.Equal(t, 0, f.ad.stopped, "the source server still runs")

	pa.importErr = nil
	var refreshed []string
	f.svc.RefreshRules = func(_ context.Context, id string) error { refreshed = append(refreshed, id); return nil }
	res, err := f.svc.TransferSession(ctx, r.SessionID, target)
	require.NoError(t, err)
	assert.Equal(t, OnExistingTransfer, res.Existing)
	assert.Equal(t, []string{r.SessionID}, refreshed, "p6-debts: the imported session gets its session rules")
	assert.Equal(t, []string{"/work/proj"}, pa.imported, "imported where the container sees the location")
	sess, err = f.svc.Sessions.Get(ctx, r.SessionID)
	require.NoError(t, err)
	assert.Equal(t, "container", sess.Runtime)
	assert.NotEqual(t, srcGroup, sess.GroupKey)
	assert.Equal(t, domain.RunIdle, sess.State)
	assert.Equal(t, "amazon-bedrock/claude-x", sess.Model)
	assert.Equal(t, []string{"amazon-bedrock/claude-x"}, pa.switched)
	assert.Equal(t, 1, f.ad.stopped, "the source server stopped (no other session)")
	src, err := f.svc.Servers.Get(ctx, srcGroup)
	require.NoError(t, err)
	assert.Equal(t, domain.ServerStopped, src.Status)
	assert.True(t, strings.HasSuffix(sess.GroupKey, "-container") || strings.Contains(sess.GroupKey, "container"))
}
