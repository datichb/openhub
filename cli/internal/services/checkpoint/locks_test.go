package checkpoint

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

func lockWorkflow() *sessionspec.WorkflowRuntime {
	return &sessionspec.WorkflowRuntime{ID: "ticket", DefaultMode: "manuel",
		Checkpoints: []sessionspec.CheckpointDef{
			{ID: "cp-1", Label: map[string]string{"": "Démarrer le ticket"}, Behaviors: map[string]string{"manuel": "pause", "semi-auto": "auto"}},
			{ID: "cp-2", Label: map[string]string{"": "Commit ou correction"}, Behaviors: map[string]string{"manuel": "pause", "semi-auto": "pause"}, Mandatory: true,
				Unlocks: []string{sessionspec.UnlockCommit, sessionspec.UnlockClose}},
			{ID: "cp-3", Label: map[string]string{"": "Ticket suivant"}, Behaviors: map[string]string{"manuel": "pause", "semi-auto": "auto"}},
		},
		Outputs: []sessionspec.OutputDef{{ID: "branch", Type: "branch"}, {ID: "tickets", Type: "beads-ids"}},
	}
}

func newLockService(t *testing.T) (*Service, context.Context, *fakeHead) {
	t.Helper()
	svc, ctx := newTestService(t)
	writeBundle(t, svc.BundlesDir, "h_lock", lockWorkflow())
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_l", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_lock", Mode: "manuel", State: domain.RunActive, LaunchPath: "/work"}))
	h := &fakeHead{head: "c1"}
	svc.Head = h.get
	return svc, ctx, h
}

type fakeHead struct {
	head  string
	dirty bool
}

func (f *fakeHead) get(_ context.Context, dir string) (string, bool, error) {
	if dir != "/work" {
		return "", false, errors.New("unexpected dir " + dir)
	}
	return f.head, f.dirty, nil
}

func askCP(t *testing.T, svc *Service, ctx context.Context, id string) bool {
	t.Helper()
	_, _, relocked, err := svc.Asked(ctx, "ses_l", "ses_l", adapters.PendingDecision{ID: "per_" + id, Call: &adapters.ToolCall{Input: map[string]any{"id": id}}})
	require.NoError(t, err)
	return relocked
}

// A17: closing waits for cp-2 and for a commit since; a closed ticket and a
// new round of cp-2 lock again.
func TestGuardCloseAndWindow(t *testing.T) {
	svc, ctx, h := newLockService(t)
	var lock *LockError
	err := svc.GuardClose(ctx, "ses_l", "")
	require.ErrorAs(t, err, &lock)
	assert.Contains(t, lock.Msg, "cp-2")
	rules, err := svc.Rules(ctx, "ses_l")
	require.NoError(t, err)
	assert.Contains(t, rules, sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: "*git*commit*", Effect: sessionspec.EffectDeny})

	res, err := svc.Reached(ctx, "ses_l", Call{ID: "cp-2"})
	require.NoError(t, err)
	assert.Equal(t, []string{"commit", "close"}, res.Unlocked)
	st, _ := svc.State(ctx, "ses_l")
	assert.Equal(t, "c1", st.Unlocks["close"].Head)
	rules, _ = svc.Rules(ctx, "ses_l")
	for _, r := range rules {
		assert.NotEqual(t, sessionspec.ActionShell, r.Action)
	}
	status, err := svc.Status(ctx, "ses_l")
	require.NoError(t, err)
	assert.Empty(t, status.LockedOps)

	h.dirty = true
	require.ErrorAs(t, svc.GuardClose(ctx, "ses_l", "/work"), &lock, "work not committed")
	assert.Contains(t, lock.Msg, "commit")
	h.head = "c2"
	assert.NoError(t, svc.GuardClose(ctx, "ses_l", "/work"), "committed since cp-2")
	h.head, h.dirty = "c1", false
	assert.NoError(t, svc.GuardClose(ctx, "ses_l", "/work"), "nothing to commit")

	changed, err := svc.BeadsDone(ctx, "ses_l", BeadsClose, []string{"pt-1"})
	require.NoError(t, err)
	assert.True(t, changed, "a closed ticket locks again")
	require.ErrorAs(t, svc.GuardClose(ctx, "ses_l", "/work"), &lock)

	_, err = svc.Reached(ctx, "ses_l", Call{ID: "cp-2"})
	require.NoError(t, err)
	assert.True(t, askCP(t, svc, ctx, "cp-2"), "cp-2 asked again: a new round")
	require.ErrorAs(t, svc.GuardClose(ctx, "ses_l", "/work"), &lock)
	assert.False(t, askCP(t, svc, ctx, "cp-2"))

	assert.NoError(t, svc.GuardClose(ctx, "ses_plain", "/work"), "no workflow: nothing locked")
}

func at(min int) time.Time { return time.Date(2026, 10, 8, 10, min, 0, 0, time.UTC) }

// A38: end of the workflow.
func TestFinished(t *testing.T) {
	wf := lockWorkflow()
	ev := func(kind, id string, min int) domain.CheckpointEvent {
		return domain.CheckpointEvent{Kind: kind, ID: id, At: at(min)}
	}
	passed := func(ids ...string) map[string]time.Time {
		m := map[string]time.Time{}
		for _, id := range ids {
			m[id] = at(0)
		}
		return m
	}
	for _, tc := range []struct {
		name string
		st   domain.CheckpointState
		want bool
	}{
		{"nothing", domain.CheckpointState{}, false},
		{"last checkpoint", domain.CheckpointState{Passed: passed("cp-1", "cp-2", "cp-3"),
			Timeline: []domain.CheckpointEvent{ev("passed", "cp-1", 1), ev("passed", "cp-2", 2), ev("passed", "cp-3", 3)}}, true},
		{"next ticket started", domain.CheckpointState{Passed: passed("cp-1", "cp-2", "cp-3"),
			Timeline: []domain.CheckpointEvent{ev("passed", "cp-2", 2), ev("passed", "cp-3", 3), ev("passed", "cp-1", 4)}}, false},
		{"mandatory missing", domain.CheckpointState{Passed: passed("cp-1", "cp-3"),
			Timeline: []domain.CheckpointEvent{ev("passed", "cp-3", 3)}}, false},
		{"tickets closed", domain.CheckpointState{Passed: passed("cp-1", "cp-2"), Claimed: []string{"pt-1"}, Closed: []string{"pt-1"},
			Timeline: []domain.CheckpointEvent{ev("passed", "cp-2", 2), ev(timelineClosed, "pt-1", 3)}}, true},
		{"ticket left", domain.CheckpointState{Passed: passed("cp-1", "cp-2"), Claimed: []string{"pt-1", "pt-2"}, Closed: []string{"pt-1"},
			Timeline: []domain.CheckpointEvent{ev("passed", "cp-2", 2), ev(timelineClosed, "pt-1", 3)}}, false},
		{"cp-1 after the close", domain.CheckpointState{Passed: passed("cp-1", "cp-2"), Closed: []string{"pt-1"},
			Timeline: []domain.CheckpointEvent{ev("passed", "cp-2", 2), ev(timelineClosed, "pt-1", 3), ev("passed", "cp-1", 4)}}, false},
		{"waiting", domain.CheckpointState{Passed: passed("cp-1", "cp-2", "cp-3"), Waiting: "cp-3",
			Timeline: []domain.CheckpointEvent{ev("passed", "cp-3", 3)}}, false},
	} {
		assert.Equal(t, tc.want, finished(wf, "manuel", tc.st), tc.name)
	}
}

func TestFinishDeclaresOutputs(t *testing.T) {
	svc, ctx, _ := newLockService(t)
	branch := func() string { return "feat/pt-1" }
	done, err := svc.Finish(ctx, "ses_l", branch)
	require.NoError(t, err)
	assert.False(t, done)

	_, err = svc.Reached(ctx, "ses_l", Call{ID: "cp-2"})
	require.NoError(t, err)
	_, err = svc.BeadsDone(ctx, "ses_l", BeadsClaim, []string{"pt-1"})
	require.NoError(t, err)
	_, err = svc.BeadsDone(ctx, "ses_l", BeadsClose, []string{"pt-1"})
	require.NoError(t, err)
	done, err = svc.Finish(ctx, "ses_l", branch)
	require.NoError(t, err)
	assert.True(t, done)
	s, err := svc.Sessions.Get(ctx, "ses_l")
	require.NoError(t, err)
	assert.Equal(t, "feat/pt-1", s.Outputs["branch"])
	assert.Equal(t, []any{"pt-1"}, s.Outputs["tickets"])
	done, _ = svc.Finish(ctx, "ses_l", branch)
	assert.False(t, done, "once")
	svc.Reopen(ctx, "ses_l")
	st, _ := svc.State(ctx, "ses_l")
	assert.Nil(t, st.Finished)
}

// A36: a question naming a checkpoint (id or label) imitates it.
func TestImitation(t *testing.T) {
	svc, ctx, _ := newLockService(t)
	q := func(title, field string) adapters.PendingDecision {
		return adapters.PendingDecision{Kind: adapters.DecisionQuestion, Title: title, Fields: []adapters.FormField{{Key: "q0", Title: field}}}
	}
	for _, tc := range []struct {
		p    adapters.PendingDecision
		want string
	}{
		{q("[OrchestratorDev — CP-2]", "Commit ou corriger ?"), "cp-2"},
		{q("Ticket pt-1", "cp3 : passer au suivant ?"), "cp-3"},
		{q("Décision", "Commit ou correction pour pt-1 ?"), "cp-2"},
		{q("Clarification", "Quel framework de test utiliser ?"), ""},
		{q("Ticket pt-12", "cp-21 ?"), ""},
	} {
		c, ok := svc.Imitation(ctx, "ses_l", tc.p)
		assert.Equal(t, tc.want != "", ok, tc.p.Fields[0].Title)
		assert.Equal(t, tc.want, c.ID, tc.p.Fields[0].Title)
	}
	_, ok := svc.Imitation(ctx, "ses_l", adapters.PendingDecision{Kind: adapters.DecisionPermission, Title: "cp-2"})
	assert.False(t, ok, "permissions are not questions")
	_, ok = svc.Imitation(ctx, "ses_plain", q("CP-2", "Commit ?"))
	assert.False(t, ok, "no workflow")
	st, _ := svc.State(ctx, "ses_l")
	n := 0
	for _, e := range st.Timeline {
		if e.Kind == domain.CheckpointImitated {
			n++
		}
	}
	assert.Equal(t, 3, n)
}

func TestGitHead(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	git("init", "-q")
	head, dirty, err := GitHead(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, "", head, "no commit yet")
	assert.False(t, dirty)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".beads"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".beads", "issues.jsonl"), []byte("{}"), 0o644))
	_, dirty, _ = GitHead(context.Background(), dir)
	assert.False(t, dirty, "Beads files do not count")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("x"), 0o644))
	_, dirty, _ = GitHead(context.Background(), dir)
	assert.True(t, dirty)
	git("add", "a.go")
	git("commit", "-qm", "a")
	head, dirty, _ = GitHead(context.Background(), dir)
	assert.Len(t, head, 40)
	assert.False(t, dirty)
	_, _, err = GitHead(context.Background(), t.TempDir())
	assert.Error(t, err, "not a repository")
}
