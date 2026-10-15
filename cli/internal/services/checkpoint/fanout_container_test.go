package checkpoint

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// fanout-container: the sessions of a fan-out whose parent works in a
// container are verified in a container, never on the machine, with every
// guard of CP-R3 unchanged.

// containerExec records the verifications run "in a container".
type containerExec struct {
	mu        sync.Mutex
	locations []string // sess.LaunchPath of each preparation
	dirs      []string
	envs      [][]string
	done      int
	exit      int
	err       error // preparation refused (engine stopped…)
}

func (c *containerExec) exec(_ context.Context, sess *domain.Session) (ExecFunc, func(), error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return nil, nil, c.err
	}
	c.locations = append(c.locations, sess.LaunchPath)
	run := func(_ context.Context, dir string, _, env []string, _ *TailBuffer) (int, error) {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.dirs = append(c.dirs, dir)
		c.envs = append(c.envs, env)
		return c.exit, nil
	}
	return run, func() { c.mu.Lock(); c.done++; c.mu.Unlock() }, nil
}

// newContainerFanout is the fan-out fixture with the parent and its child
// in a container.
func newContainerFanout(t *testing.T) (*fanoutFixture, *containerExec) {
	t.Helper()
	f := newFanoutFixture(t)
	for _, id := range []string{"ses_p", "ses_c"} {
		s, err := f.svc.Sessions.Get(f.ctx, id)
		require.NoError(t, err)
		s.Runtime = string(sessionspec.RuntimeContainer)
		require.NoError(t, f.svc.Sessions.Update(f.ctx, s))
	}
	ce := &containerExec{}
	f.svc.ContainerExec = ce.exec
	return f, ce
}

func TestContainerFanoutMergeVerifiedInContainer(t *testing.T) {
	f, ce := newContainerFanout(t)
	f.ready(t)
	res, err := reachValidated(t, f.svc, f.ctx, "ses_p", "cp-merge")
	require.NoError(t, err)
	assert.Contains(t, res.Message, "merged by oh")
	assert.Equal(t, 1, f.integrated)
	assert.Equal(t, []string{f.intg}, ce.locations, "the container mounts the integration worktree")
	assert.Equal(t, []string{f.intg}, slices.Compact(ce.dirs))
	assert.Equal(t, containerVerifyEnv, ce.envs[0], "nothing of the machine environment")
	assert.Equal(t, 1, ce.done, "the container of the verification is removed")
	assert.Empty(t, f.execDirs, "never on the machine")
}

func TestContainerFanoutChildVerifiedInContainer(t *testing.T) {
	f, ce := newContainerFanout(t)
	res, err := f.svc.Reached(f.ctx, "ses_c", Call{ID: "cp-2"})
	require.NoError(t, err, "cp-2 still delegated to cp-merge in a container")
	assert.Equal(t, []string{"commit", "close"}, res.Unlocked)
	assert.Equal(t, []string{f.childDir}, slices.Compact(ce.dirs))
	assert.Empty(t, f.execDirs)

	f2, ce2 := newContainerFanout(t)
	ce2.exit = 1
	_, err = f2.svc.Reached(f2.ctx, "ses_c", Call{ID: "cp-2"})
	var ve *VerifyError
	require.ErrorAs(t, err, &ve, "a failed verification in the container is never passed")
}

// The verification of cp-merge outside the container is impossible: oh
// does not fall back on the machine, the checkpoint never passes (not even
// overridden), and it is tried again once the container can run.
func TestContainerFanoutMergeNeverOnTheMachine(t *testing.T) {
	t.Run("no container in this process", func(t *testing.T) {
		f, _ := newContainerFanout(t)
		f.svc.ContainerExec = nil
		f.ready(t)
		_, err := reachValidated(t, f.svc, f.ctx, "ses_p", "cp-merge")
		var ve *VerifyError
		require.ErrorAs(t, err, &ve)
		assert.True(t, ve.Unavailable)
		require.ErrorIs(t, err, ErrVerifyUnavailable)
		assert.Empty(t, f.execDirs, "never on the machine instead")
		assert.Equal(t, 0, f.integrated)
	})
	t.Run("engine stopped, then back", func(t *testing.T) {
		f, ce := newContainerFanout(t)
		ce.err = errors.New("colima is not running")
		f.ready(t)
		_, err := reachValidated(t, f.svc, f.ctx, "ses_p", "cp-merge")
		require.ErrorIs(t, err, ErrVerifyUnavailable)
		r, ok := f.svc.VerifyResult(f.ctx, "ses_p", "cp-merge")
		require.True(t, ok)
		assert.Contains(t, r.Reason, "colima is not running")
		assert.Empty(t, f.execDirs)
		assert.Equal(t, 0, f.integrated)

		tool := &fakeTool{}
		d := domain.Decision{ID: "d1", Kind: domain.DecisionCheckpoint, SessionID: "ses_p", Payload: domain.DecisionPayload{Data: map[string]any{DataCheckpoint: "cp-merge"}}}
		require.Error(t, f.svc.Resolve(f.ctx, tool, d, ChoiceOverride, "passe"), "cp-merge is never overridden")

		ce.mu.Lock()
		ce.err = nil
		ce.mu.Unlock()
		_, err = reachValidated(t, f.svc, f.ctx, "ses_p", "cp-merge")
		require.NoError(t, err, "an impossible verification is not reused: tried again")
		assert.Equal(t, []string{f.intg}, slices.Compact(ce.dirs))
		assert.Equal(t, 1, f.integrated)
	})
	t.Run("local parent: on the machine as before", func(t *testing.T) {
		f := newFanoutFixture(t)
		ce := &containerExec{}
		f.svc.ContainerExec = ce.exec
		f.ready(t)
		_, err := reachValidated(t, f.svc, f.ctx, "ses_p", "cp-merge")
		require.NoError(t, err)
		assert.Contains(t, f.execDirs, f.intg)
		assert.Empty(t, ce.locations)
	})
}

// A session of a fan-out in a container sees the repository at other paths
// (/work/<name>) and the git common directory at its machine path: none of
// them gets around the rules (push, merge, close, branches of the fan-out).
func TestContainerFanoutChildBypass(t *testing.T) {
	f, _ := newContainerFanout(t)
	_, err := f.svc.Reached(f.ctx, "ses_c", Call{ID: "cp-2"})
	require.NoError(t, err, "even with cp-2 passed")
	for _, id := range []string{"ses_c", "ses_p"} {
		rules, err := f.svc.Rules(f.ctx, id)
		require.NoError(t, err)
		for _, cmd := range []string{
			"git -C /work/proj-oh-fanout-p-ev-1 push origin HEAD",
			"cd /work/proj-oh-fanout-p-ev-1 && git push",
			"git push /Users/me/proj/.git HEAD:main",
			"git --git-dir=/Users/me/proj/.git push origin oh/fanout/p/ev-1",
			"cd /work/proj && git merge oh/fanout/p/ev-1",
			"git -C /work/proj-oh-fanout-p-integration merge --no-ff oh/fanout/p/ev-1",
			"git -C /work/proj pull",
			"git --git-dir=/Users/me/proj/.git update-ref refs/heads/oh/fanout/p/integration HEAD",
			"echo 0123 > /Users/me/proj/.git/refs/heads/oh/fanout/p/integration",
			"git -C /work/proj branch -f oh/fanout/p/integration HEAD",
			"git -C /Users/me/proj worktree remove /Users/me/proj-oh-fanout-p-ev-1",
			"bd close ev-1",
			"bd -C /work/proj close ev-1",
		} {
			assert.Equal(t, sessionspec.EffectDeny, bundle.ShellEffect(rules, cmd), "%s: %s", id, cmd)
		}
	}
	var le *LockError
	require.ErrorAs(t, f.svc.GuardClose(f.ctx, "ses_c", f.childDir), &le, "the Beads gateway refuses the closing in a container too")
	st, err := f.svc.Status(f.ctx, "ses_c")
	require.NoError(t, err)
	assert.Subset(t, st.LockedOps, []string{"push", "merge", "close"})
}
