package checkpoint

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// R3-remote-git: a git read that fails keeps the protection closed.

// errDubious is git refusing a repository of another owner.
var errDubious = errors.New("fatal: detected dubious ownership in repository")

// The commit window does not stay open when the state of the work cannot be
// read: it closes (asking the checkpoint again opens it).
func TestCommitWindowClosesWhenGitFails(t *testing.T) {
	svc, ctx, _, _ := newVerifyService(t, nil)
	_, err := reachValidated(t, svc, ctx, "ses_v", "cp-2")
	require.NoError(t, err)
	shell := func(id string) bool {
		_, _, err := svc.OnCall(ctx, "ses_v", adapters.ToolCall{ID: id, Action: sessionspec.ActionShell, Status: adapters.CallCalled, Input: map[string]any{"command": "git commit -m x"}})
		require.NoError(t, err)
		changed, _, err := svc.OnCall(ctx, "ses_v", adapters.ToolCall{ID: id, Action: sessionspec.ActionShell, Status: adapters.CallOK})
		require.NoError(t, err)
		return changed
	}
	assert.False(t, shell("s1"), "nothing committed yet: open")
	svc.Head = func(context.Context, string) (string, bool, error) { return "", false, errDubious }
	assert.True(t, shell("s2"), "git refuses: the window closes")
	st, _ := svc.State(ctx, "ses_v")
	assert.NotContains(t, st.Unlocks, sessionspec.UnlockCommit)
	last := st.Timeline[len(st.Timeline)-1]
	assert.Equal(t, domain.CheckpointWindow, last.Kind)
	assert.Contains(t, last.Message, "dubious ownership", "the reason is traced")
	rules, _ := svc.Rules(ctx, "ses_v")
	assert.Contains(t, rules, sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: "git commit*", Effect: sessionspec.EffectDeny})
}

// The closing guard refuses when git cannot tell what was committed.
func TestGuardCloseRefusesWhenGitFails(t *testing.T) {
	svc, ctx, _, _ := newVerifyService(t, nil)
	_, err := reachValidated(t, svc, ctx, "ses_v", "cp-2")
	require.NoError(t, err)
	svc.Work = func(context.Context, string, string, time.Time) (Work, error) { return Work{}, errDubious }
	var lock *LockError
	require.ErrorAs(t, svc.GuardClose(ctx, "ses_v", ""), &lock)
	assert.Contains(t, lock.Msg, "dubious ownership")
}

// gitRefused is a location git refuses (a repository is there).
func gitRefused(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: "+filepath.Join(dir, "gone")+"\n"), 0o644))
	return dir
}

// A verification is never tied to a state of the work git cannot read: it
// is unavailable (pause, only the traced override passes it), no command
// runs and no earlier result is reused.
func TestVerifyUnavailableWhenGitRefuses(t *testing.T) {
	svc, ctx, ex, _ := newVerifyService(t, &sessionspec.VerifyDef{Cmd: []string{"make check"}, Expect: "pass", TimeoutSec: 60})
	sess, err := svc.Sessions.Get(ctx, "ses_v")
	require.NoError(t, err)
	sess.LaunchPath = gitRefused(t)
	require.NoError(t, svc.Sessions.Update(ctx, sess))
	svc.Fingerprint = nil

	fp := gitFingerprint(ctx, sess.LaunchPath, nil)
	assert.True(t, unreadable(fp), fp)
	assert.NotEqual(t, fp, gitFingerprint(ctx, sess.LaunchPath, nil), "never the same state twice")
	assert.False(t, unreadable(gitFingerprint(ctx, t.TempDir(), nil)), "not a repository: as before")

	d := askV(t, svc, ctx, "cp-2", nil)
	r := waitVerify(t, svc, ctx, "cp-2")
	assert.Equal(t, domain.VerifyUnavailable, r.Status)
	assert.Equal(t, "git cannot read the state of the work (repository refused): the verification cannot be tied to it", r.Reason)
	require.ErrorIs(t, svc.Resolve(ctx, &fakeTool{}, *d, ChoiceApprove, ""), ErrOverrideRequired, "a plain validation is refused")
	assert.Equal(t, 0, ex.count(), "nothing ran on an unknown state of the work")
}

// A paths condition holds when the changed paths cannot be read: the pause
// is added rather than dropped.
func TestWhenPathsUnknownPauses(t *testing.T) {
	svc, ctx := newTestService(t)
	wf := &sessionspec.WorkflowRuntime{ID: "feature", DefaultMode: "semi-auto", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-spec", Behaviors: map[string]string{"semi-auto": "conditional"}, When: &sessionspec.WhenDef{Paths: []string{"docs/specs/**"}}},
	}}
	writeBundle(t, svc.BundlesDir, "h_when", wf)
	repo := gitRefused(t)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_w", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_when", Mode: "semi-auto", State: domain.RunActive, LaunchPath: repo}))
	svc.Changed = func(context.Context, string, string) ([]string, error) { return nil, errDubious }
	c, _ := wf.Checkpoint("cp-spec")
	sess, err := svc.Sessions.Get(ctx, "ses_w")
	require.NoError(t, err)
	assert.Equal(t, sessionspec.CheckpointPause, svc.behavior(ctx, sess, c), "git refuses: the pause stays")

	sess.LaunchPath = t.TempDir()
	assert.Equal(t, sessionspec.CheckpointAuto, svc.behavior(ctx, sess, c), "no repository: nothing changed to read")
}
