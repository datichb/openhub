package fanout

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

// fixFixture is a fan-out ready for its merge checkpoint, its driver
// waiting for a fix round.
func fixFixture(t *testing.T) (*fixture, context.CancelFunc, chan error) {
	t.Helper()
	f := newFixture(t, "ev-1")
	f.launcher.work["ev-1"] = [2]string{"a.go", "package calc\n"}
	f.wf.Fanout.Fix, f.wf.Fanout.FixInput = "quick", "request"
	f.wf.MaxFixRounds = 2
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	done := make(chan error, 1)
	go func() { done <- f.driver.Run(ctx, "ses_parent") }()
	t.Cleanup(func() { cancel(); <-done })
	f.waitStatus(t, domain.FanoutReady)
	return f, cancel, done
}

func (f *fixture) waitStatus(t *testing.T, want domain.FanoutStatus) *domain.SessionFanout {
	t.Helper()
	var fo *domain.SessionFanout
	require.Eventually(t, func() bool {
		p, err := f.store.Get(context.Background(), "ses_parent")
		if err != nil || p.Fanout == nil {
			return false
		}
		fo = p.Fanout
		return fo.Status == want && (want != domain.FanoutReady || len(f.launcher.toldCopy()) > len(fo.Fixes))
	}, 30*time.Second, 20*time.Millisecond)
	return fo
}

func (l *fakeLauncher) toldCopy() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.told...)
}

func (f *fixture) requestFix(t *testing.T, instruction string) error {
	t.Helper()
	p, err := f.store.Get(context.Background(), "ses_parent")
	require.NoError(t, err)
	_, _, err = RequestFix(context.Background(), f.store, p, f.wf, instruction, "go test: FAIL")
	return err
}

// The merge checkpoint refused: oh runs a fixer on the integration branch,
// takes its commit, the fan-out is ready again and the parent told; the
// merge checkpoint is then verified on the new head.
func TestFixRound(t *testing.T) {
	f, _, _ := fixFixture(t)
	before, _ := f.store.Get(context.Background(), "ses_parent")
	base := before.Fanout.Integration.Head
	f.launcher.fix = [2]string{"fix.go", "package calc // fixed\n"}
	require.NoError(t, f.requestFix(t, "corrige le test"))
	fo := f.waitStatus(t, domain.FanoutReady)
	require.Len(t, fo.Fixes, 1)
	x := fo.Fixes[0]
	assert.Equal(t, domain.ChildMerged, x.State, x.Error)
	assert.Equal(t, base, x.Base)
	assert.NotEqual(t, base, fo.Integration.Head, "oh took the fix")
	assert.Equal(t, x.Head, fo.Integration.Head)
	assert.FileExists(t, filepath.Join(fo.Integration.Path, "fix.go"))
	fixer := f.launcher.started[len(f.launcher.started)-1]
	assert.Equal(t, domain.RoleFixer, fixer.Record.Role)
	assert.Equal(t, "quick", fixer.Workflow)
	assert.Equal(t, fo.Integration.Path, fixer.Dir)
	assert.Contains(t, fixer.Inputs["request"], "corrige le test")
	assert.Contains(t, fixer.Inputs["request"], "go test: FAIL", "what failed is given")
	told := f.launcher.toldCopy()
	assert.Contains(t, told[len(told)-1], "cp-merge")
	kinds := eventKinds(fo)
	assert.Equal(t, 2, kinds[domain.FanoutEventFix])
	assert.Equal(t, 1, kinds[domain.FanoutEventFixed])

	// The merge checkpoint takes the new head: Integrate merges it.
	p, _ := f.store.Get(context.Background(), "ses_parent")
	_, err := (&Integrator{Sessions: f.store, Close: func(context.Context, string, string, string) error { return nil }}).Integrate(context.Background(), p, "cp-merge")
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(f.project, "fix.go"))
}

// A fixer that merges, or commits nothing, is not kept: the integration
// branch goes back where it was.
func TestFixRoundRefused(t *testing.T) {
	for _, tc := range []struct {
		name  string
		merge bool
		want  string
	}{{"merge commit", true, "merge"}, {"no commit", false, "commit"}} {
		t.Run(tc.name, func(t *testing.T) {
			f, _, _ := fixFixture(t)
			before, _ := f.store.Get(context.Background(), "ses_parent")
			base := before.Fanout.Integration.Head
			f.launcher.fixMerge = tc.merge
			f.driver.IdleTimeout = 300 * time.Millisecond
			require.NoError(t, f.requestFix(t, "corrige"))
			fo := f.waitStatus(t, domain.FanoutReady)
			require.Len(t, fo.Fixes, 1)
			assert.Equal(t, domain.ChildFailed, fo.Fixes[0].State)
			assert.Contains(t, fo.Fixes[0].Error, tc.want)
			assert.Equal(t, base, fo.Integration.Head)
			assert.Equal(t, base, run(t, fo.Integration.Path, "git", "rev-parse", "HEAD"), "back where it was")
			told := f.launcher.toldCopy()
			assert.Contains(t, told[len(told)-1], "cp-merge")
		})
	}
}

// Fix rounds are asked only while the fan-out waits for its merge
// checkpoint, at most limits.max_fix_rounds times, and only when declared.
func TestRequestFixGuards(t *testing.T) {
	f, _, _ := fixFixture(t)
	f.launcher.fix = [2]string{"fix.go", "package calc\n"}
	require.NoError(t, f.requestFix(t, "un"))
	assert.ErrorIs(t, f.requestFix(t, "pendant"), ErrFixPending, "one round at a time")
	f.waitStatus(t, domain.FanoutReady)
	f.launcher.fix = [2]string{"fix2.go", "package calc\n"}
	require.NoError(t, f.requestFix(t, "deux"))
	f.waitStatus(t, domain.FanoutReady)
	assert.ErrorIs(t, f.requestFix(t, "trois"), ErrFixRounds)

	g := newFixture(t, "ev-1")
	p, _ := g.store.Get(context.Background(), "ses_parent")
	_, _, err := RequestFix(context.Background(), g.store, p, g.wf, "x", "")
	assert.True(t, errors.Is(err, ErrNoFix), "no fix round declared")
	assert.Empty(t, p.Fanout.Fixes)
}
