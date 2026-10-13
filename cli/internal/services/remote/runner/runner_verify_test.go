package runner

import (
	"context"
	"encoding/json"
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
	"github.com/datichb/openhub/cli/internal/remote"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// R3-remote: a remote session simulated end to end on the services of the
// job. The verification of a checkpoint runs in the clone of the job, its
// failure is never overridden there, an impossible verification is
// deferred to the machine, and the result travels in summary.json.

type verifyExec struct {
	mu    sync.Mutex
	exit  int
	delay time.Duration
	dirs  []string
}

func (e *verifyExec) run(ctx context.Context, dir string, _, _ []string, out *checkpoint.TailBuffer) (int, error) {
	e.mu.Lock()
	e.dirs = append(e.dirs, dir)
	exit, delay := e.exit, e.delay
	e.mu.Unlock()
	select {
	case <-time.After(delay):
	case <-ctx.Done():
		return -1, ctx.Err()
	}
	if exit != 0 {
		out.WriteString("--- FAIL: TestSub\n")
	}
	return exit, nil
}

type cpTool struct {
	mu      sync.Mutex
	replies []string
	steers  []string
}

func (f *cpTool) ReplyPermission(_ context.Context, _ domain.Decision, decision string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.replies = append(f.replies, decision)
	return nil
}
func (f *cpTool) Steer(_ context.Context, _ string, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.steers = append(f.steers, text)
	return nil
}
func (f *cpTool) Refresh(context.Context, string) error { return nil }

// jobCheckpoints is the checkpoint service of a job: cp-2 is verified by
// oh, unlocks the commit and waits for the user in semi-auto; the session
// works in clone.
func jobCheckpoints(t *testing.T, clone string) (*checkpoint.Service, *verifyExec) {
	t.Helper()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p-api','api','/work')`)
	require.NoError(t, err)
	cs := sqlite.NewCheckpointStore(st)
	svc := &checkpoint.Service{Sessions: sqlite.NewSessionStore(st), States: cs, SessionOutputs: cs, BundlesDir: t.TempDir(),
		SessionsDir: t.TempDir(), ApprovalWait: 20 * time.Millisecond, VerifyWait: 2 * time.Second}
	wf := &sessionspec.WorkflowRuntime{ID: "ticket", DefaultMode: "semi-auto", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-2", Label: map[string]string{"": "Commit"}, Behaviors: map[string]string{"semi-auto": "pause"}, Mandatory: true,
			Unlocks: []string{sessionspec.UnlockCommit}, Verify: &sessionspec.VerifyDef{Cmd: []string{"make check"}, Expect: "pass", TimeoutSec: 60}},
	}}
	require.NoError(t, os.MkdirAll(filepath.Join(svc.BundlesDir, "h_job"), 0o755))
	data, err := json.Marshal(sessionspec.BundleSpec{Hash: "h_job", EntryAgent: "orchestrator-dev", Workflow: wf})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(svc.BundlesDir, "h_job", "bundle.json"), data, 0o644))
	require.NoError(t, svc.Sessions.Create(context.Background(), &domain.Session{ID: "ses_1", ProjectID: "p-api", Status: domain.SessionStatusRunning,
		GroupKey: "g1", BundleHash: "h_job", Mode: "semi-auto", State: domain.RunActive, LaunchPath: clone, EntryAgent: "orchestrator-dev",
		Runtime: string(sessionspec.RuntimeRemote)}))
	ex := &verifyExec{}
	svc.Exec = ex.run
	svc.Fingerprint = func(context.Context, string) string { return "fp1" }
	svc.Head = func(context.Context, string) (string, bool, error) { return "c1", false, nil }
	return svc, ex
}

// askCheckpoint raises cp-2 as the daemon of the job does.
func askCheckpoint(t *testing.T, svc *checkpoint.Service) domain.Decision {
	t.Helper()
	d, auto, _, err := svc.Asked(context.Background(), "ses_1", "ses_1", adapters.PendingDecision{ID: "per_cp2",
		Call: &adapters.ToolCall{Input: map[string]any{"id": "cp-2", "summary": "done"}}})
	require.NoError(t, err)
	require.False(t, auto)
	return *d
}

// jobDecide answers through the checkpoint service, as the SessionService
// of the job does.
func jobDecide(svc *checkpoint.Service, fd *fakeDecisions, tool *cpTool) func(context.Context, sessionsvc.Reply) error {
	return func(ctx context.Context, r sessionsvc.Reply) error {
		var d *domain.Decision
		open, _ := fd.ListOpen(ctx, domain.DecisionFilter{})
		for i := range open {
			if open[i].ID == r.DecisionID {
				d = &open[i]
			}
		}
		if d == nil {
			return &sessionsvc.ResolvedError{By: domain.ResolvedByTool}
		}
		if err := svc.Resolve(ctx, tool, *d, r.Decision, r.Message); err != nil {
			return err
		}
		return fd.decide(ctx, r)
	}
}

func autoManifest() *remote.Manifest {
	return &remote.Manifest{Checkpoints: []remote.ManifestCheckpoint{{ID: "cp-2", Policy: "auto", Mandatory: true}}}
}

func TestResponderVerifyFailedInTheJob(t *testing.T) {
	ctx := context.Background()
	clone := t.TempDir()
	svc, ex := jobCheckpoints(t, clone)
	ex.exit = 1
	d := askCheckpoint(t, svc)
	svc.WaitVerify(ctx)
	assert.Equal(t, []string{clone}, ex.dirs, "run in the clone of the job")

	fd, tool := &fakeDecisions{}, &cpTool{}
	fd.add(d)
	r := NewResponder(fd, jobDecide(svc, fd, tool), autoManifest())
	assert.Nil(t, r.Step(ctx, "ses_1"), "the agent fixes the work: the session goes on")
	require.Len(t, fd.replies, 1)
	assert.Equal(t, "fix", fd.replies[0].Decision, "never overridden on the runner")
	assert.Equal(t, VerifyFixMessage, fd.replies[0].Message)
	assert.Equal(t, []string{"reject"}, tool.replies)
	require.NotEmpty(t, tool.steers)
	assert.Equal(t, "rejected", r.Answers()[0].Answer)
	st, _ := svc.State(ctx, "ses_1")
	assert.False(t, st.HasPassed("cp-2"))
	assert.NotContains(t, st.Unlocks, sessionspec.UnlockCommit)
}

func TestResponderVerifyUnavailableInTheJob(t *testing.T) {
	ctx := context.Background()
	clone := filepath.Join(t.TempDir(), "gone")
	svc, ex := jobCheckpoints(t, clone)
	d := askCheckpoint(t, svc)
	fd, tool := &fakeDecisions{}, &cpTool{}
	fd.add(d)
	r := NewResponder(fd, jobDecide(svc, fd, tool), autoManifest())
	stop := r.Step(ctx, "ses_1")
	require.NotNil(t, stop, "never passed without verification, even remote: auto")
	assert.Equal(t, remote.OutcomeDeferred, stop.Outcome)
	assert.Contains(t, stop.Reason, "cannot run in the job")
	assert.Empty(t, tool.replies, "not approved")
	assert.Empty(t, ex.dirs)
	st, _ := svc.State(ctx, "ses_1")
	assert.False(t, st.HasPassed("cp-2"))
}

func TestRunVerifiesInTheJob(t *testing.T) {
	clone := t.TempDir()
	svc, ex := jobCheckpoints(t, clone)
	ex.delay = 300 * time.Millisecond
	d := askCheckpoint(t, svc)

	fd, tool := &fakeDecisions{}, &cpTool{}
	sess := &fakeSession{fd: fd, raise: []domain.Decision{d}}
	in, out := runInput(t, sess, &fakeRepo{result: PushResult{Commit: "c9"}}, manifest())
	in.Responder = NewResponder(fd, jobDecide(svc, fd, tool), autoManifest())
	in.Responder.Poll = 10 * time.Millisecond
	in.WaitVerify = svc.WaitVerify
	in.Verifications = func(ctx context.Context, sid string) ([]remote.SummaryVerification, error) {
		list, err := svc.Verifications(ctx, sid)
		return checkpoint.SummaryVerifications(list), err
	}
	var atQuiesce string
	in.Quiesce = func(ctx context.Context) error {
		r, _ := svc.VerifyResult(ctx, "ses_1", "cp-2")
		atQuiesce = r.Status
		return nil
	}

	_, err := Run(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, domain.VerifyPassed, atQuiesce, "the verification ends before the agent processes are stopped")
	s := readSummary(t, out)
	require.Len(t, s.Verifications, 1)
	v := s.Verifications[0]
	assert.Equal(t, "cp-2", v.Checkpoint)
	assert.Equal(t, domain.VerifyPassed, v.Status)
	require.Len(t, v.Commands, 1)
	assert.Equal(t, "make check", v.Commands[0].Cmd)
	assert.True(t, strings.HasPrefix(ex.dirs[0], clone))
}
