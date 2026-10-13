package checkpoint

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// R3-remote: a verified checkpoint never passes without its verification,
// even in a mode where it would pass on its own, when the verification
// cannot run where the work is; the user alone overrides it (traced).
func TestVerifyUnavailableNeverPassesAlone(t *testing.T) {
	svc, ctx := newTestService(t)
	wf := lockWorkflow()
	wf.Checkpoints[2].Verify = &sessionspec.VerifyDef{Cmd: []string{"make check"}, Expect: "pass", TimeoutSec: 60}
	writeBundle(t, svc.BundlesDir, "h_auto", wf)
	gone := filepath.Join(t.TempDir(), "clone")
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_r", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_auto", Mode: "semi-auto", State: domain.RunActive, LaunchPath: gone, EntryAgent: "orchestrator-dev"}))
	ex := &fakeExec{exits: map[string]int{}}
	svc.Exec = ex.run
	svc.Fingerprint = func(context.Context, string) string { return "fp" }

	effect, err := svc.Evaluate(ctx, "ses_r", bundle.CheckpointAction(), map[string]any{"id": "cp-3"})
	require.NoError(t, err)
	assert.Equal(t, string(sessionspec.EffectAsk), effect, "cp-3 is automatic in semi-auto, but its verification cannot run")

	// Not asked (the tool let it run): refused, nothing verified.
	_, err = svc.Reached(ctx, "ses_r", Call{ID: "cp-3"})
	require.ErrorIs(t, err, ErrNotApproved)

	d, auto, _, err := svc.Asked(ctx, "ses_r", "ses_r", adapters.PendingDecision{ID: "per_cp3", Call: &adapters.ToolCall{Input: map[string]any{"id": "cp-3"}}})
	require.NoError(t, err)
	require.False(t, auto, "a pause, never a silent pass")
	r, ok := svc.VerifyResult(ctx, "ses_r", "cp-3")
	require.True(t, ok)
	assert.Equal(t, domain.VerifyUnavailable, r.Status)
	assert.True(t, svc.Card(ctx, "ses_r", "cp-3").Verify.NeedsOverride(), "the card offers the override")

	// A plain validation (oh or the tool) does not pass it.
	err = svc.Resolve(ctx, &fakeTool{}, *d, ChoiceApprove, "")
	require.ErrorIs(t, err, ErrOverrideRequired)
	_, err = reachValidated(t, svc, ctx, "ses_r", "cp-3")
	require.ErrorIs(t, err, ErrVerifyUnavailable)
	assert.Contains(t, err.Error(), "cp-3")
	st, _ := svc.State(ctx, "ses_r")
	assert.False(t, st.HasPassed("cp-3"))

	// The user overrides: passed, traced.
	require.NoError(t, svc.Resolve(ctx, &fakeTool{}, *d, ChoiceOverride, "verified by hand"))
	_, err = svc.Reached(ctx, "ses_r", Call{ID: "cp-3"})
	require.NoError(t, err)
	st, _ = svc.State(ctx, "ses_r")
	assert.True(t, st.HasPassed("cp-3"))
	var overridden, failed int
	for _, e := range st.Timeline {
		switch {
		case e.Kind == domain.CheckpointOverridden && e.ID == "cp-3":
			overridden++
			assert.Equal(t, "verified by hand", e.Message)
			assert.Equal(t, string(domain.ResolvedByOh), e.By)
		case e.Kind == domain.CheckpointVerifyFail && e.ID == "cp-3":
			failed++
		}
	}
	assert.Equal(t, 1, overridden)
	assert.Equal(t, 1, failed, "the impossible verification is traced once")
	assert.Equal(t, 0, ex.count(), "nothing ran")
}

// Non-regression: a checkpoint automatic in the mode, whose location is
// there, still passes alone once verified.
func TestVerifyAutoLocalStillPasses(t *testing.T) {
	svc, ctx := newTestService(t)
	wf := lockWorkflow()
	wf.Checkpoints[2].Verify = &sessionspec.VerifyDef{Cmd: []string{"make check"}, Expect: "pass", TimeoutSec: 60}
	writeBundle(t, svc.BundlesDir, "h_auto", wf)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_l", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_auto", Mode: "semi-auto", State: domain.RunActive, LaunchPath: t.TempDir(), EntryAgent: "orchestrator-dev"}))
	ex := &fakeExec{exits: map[string]int{}}
	svc.Exec = ex.run
	svc.Fingerprint = func(context.Context, string) string { return "fp" }
	effect, err := svc.Evaluate(ctx, "ses_l", bundle.CheckpointAction(), map[string]any{"id": "cp-3"})
	require.NoError(t, err)
	assert.Equal(t, string(sessionspec.EffectAllow), effect)
	_, err = svc.Reached(ctx, "ses_l", Call{ID: "cp-3"})
	require.NoError(t, err)
	assert.Equal(t, 1, ex.count())
}

// The job (remote session simulated): the verification runs in the clone,
// its result is exported for summary.json; the machine imports it apart
// from its own verifications (card, timeline) and verifies again locally.
func TestVerifyRemoteRoundTrip(t *testing.T) {
	job, ctx, ex, _ := newVerifyService(t, &sessionspec.VerifyDef{Cmd: []string{"make check"}, Expect: "pass", TimeoutSec: 60})
	sess, err := job.Sessions.Get(ctx, "ses_v")
	require.NoError(t, err)
	sess.Runtime = string(sessionspec.RuntimeRemote)
	require.NoError(t, job.Sessions.Update(ctx, sess))
	var dirs []string
	ex.exits["make check"] = 2
	ex.out = "--- FAIL: TestSub\x1b]0;pwned\x07\n"
	job.Exec = func(ctx context.Context, dir string, args, env []string, out *TailBuffer) (int, error) {
		dirs = append(dirs, dir)
		return ex.run(ctx, dir, args, env, out)
	}
	askV(t, job, ctx, "cp-2", nil)
	job.WaitVerify(ctx)
	assert.Equal(t, []string{sess.LaunchPath}, dirs, "run where the work is (the clone of the job)")
	list, err := job.Verifications(ctx, "ses_v")
	require.NoError(t, err)
	sum := SummaryVerifications(list)
	require.Len(t, sum, 1)
	assert.Equal(t, "cp-2", sum[0].Checkpoint)
	assert.Equal(t, domain.VerifyFailed, sum[0].Status)
	require.Len(t, sum[0].Commands, 1)
	assert.Equal(t, 2, sum[0].Commands[0].Exit)

	// The machine: same session, its own store.
	machine, mctx, mex, _ := newVerifyService(t, &sessionspec.VerifyDef{Cmd: []string{"make check"}, Expect: "pass", TimeoutSec: 60})
	in := FromSummary(sum)
	in = append(in, VerifyOf{ID: "cp-unknown", Result: domain.CheckpointVerify{Status: domain.VerifyPassed}},
		VerifyOf{ID: "cp-1", Result: domain.CheckpointVerify{Status: "forged"}})
	n, err := machine.ImportRemoteVerify(mctx, "ses_v", in)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "unknown checkpoints and statuses are dropped")
	n, err = machine.ImportRemoteVerify(mctx, "ses_v", in)
	require.NoError(t, err)
	assert.Equal(t, 0, n, "fetched again: nothing new")

	card := machine.Card(mctx, "ses_v", "cp-2")
	assert.Nil(t, card.Verify, "never taken for a verification of the machine")
	require.NotNil(t, card.Remote)
	assert.NotContains(t, card.Remote.Excerpt, "\x1b", "control characters neutralized")
	assert.Contains(t, card.Remote.Excerpt, "--- FAIL: TestSub")
	st, _ := machine.State(mctx, "ses_v")
	var remote int
	for _, e := range st.Timeline {
		if e.By == RemoteAuthor && e.Kind == domain.CheckpointVerifyFail && e.ID == "cp-2" {
			remote++
		}
	}
	assert.Equal(t, 1, remote)

	// Asked again on the machine: verified again, here.
	d := askV(t, machine, mctx, "cp-2", nil)
	r := waitVerify(t, machine, mctx, "cp-2")
	assert.Equal(t, domain.VerifyPassed, r.Status)
	assert.Equal(t, 1, mex.count())
	require.NoError(t, machine.Resolve(mctx, &fakeTool{}, *d, ChoiceApprove, ""), "the remote failure does not block the local verification")
}

func TestWaitVerify(t *testing.T) {
	svc, ctx, ex, _ := newVerifyService(t, &sessionspec.VerifyDef{Cmd: []string{"make check"}, Expect: "pass", TimeoutSec: 60})
	ex.block = make(chan struct{})
	askV(t, svc, ctx, "cp-2", nil)
	short, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	svc.WaitVerify(short)
	r, _ := svc.VerifyResult(ctx, "ses_v", "cp-2")
	assert.Equal(t, domain.VerifyRunning, r.Status, "bounded by its context")
	close(ex.block)
	svc.WaitVerify(ctx)
	r, _ = svc.VerifyResult(ctx, "ses_v", "cp-2")
	assert.Equal(t, domain.VerifyPassed, r.Status, "the result is stored when it returns")
}
