package fanout

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

func branchExists(t *testing.T, repo, branch string) bool {
	t.Helper()
	_, err := BranchHead(context.Background(), repo, branch)
	return err == nil
}

func eventKinds(f *domain.SessionFanout) map[string]int {
	out := map[string]int{}
	for _, e := range f.Events {
		out[e.Kind]++
	}
	return out
}

// After the merge checkpoint everything is on the branch of the parent:
// oh removes every worktree and branch of the fan-out.
func TestCleanAfterMerge(t *testing.T) {
	f := newFixture(t, "ev-1", "ev-2")
	f.launcher.work["ev-1"] = [2]string{"a.go", "package calc\n"}
	f.launcher.work["ev-2"] = [2]string{"b.go", "package calc\n"}
	fo := f.runDriver(t)
	require.Equal(t, domain.FanoutReady, fo.Status)
	p, _ := f.store.Get(context.Background(), "ses_parent")
	_, err := (&Integrator{Sessions: f.store, Close: func(context.Context, string, string, string) error { return nil }}).Integrate(context.Background(), p, "cp-merge")
	require.NoError(t, err)

	p, _ = f.store.Get(context.Background(), "ses_parent")
	assert.NoDirExists(t, p.Fanout.Integration.Path)
	assert.False(t, branchExists(t, f.project, p.Fanout.Integration.Branch))
	for _, c := range p.Fanout.Children {
		assert.NoDirExists(t, c.Path)
		assert.False(t, branchExists(t, f.project, c.Branch), c.Branch)
	}
	assert.True(t, p.Fanout.Cleaned)
	assert.Equal(t, 1, eventKinds(p.Fanout)[domain.FanoutEventCleaned])
	assert.FileExists(t, filepath.Join(f.project, "a.go"), "the work is on the branch of the parent")
}

// An abandoned fan-out keeps what is merged nowhere else: the integration
// branch (with the children merged into it, whose branches go) and the
// child left unmerged. --discard deletes them, traced with their commit.
func TestCleanAbandoned(t *testing.T) {
	f := newFixture(t, "ev-1", "ev-2")
	f.launcher.hold = 150 * time.Millisecond
	f.launcher.work["ev-1"] = [2]string{"calc.go", "package calc\n\nfunc Sub() {}\n"}
	f.launcher.work["ev-2"] = [2]string{"calc.go", "package calc\n\nfunc Mul() {}\n"}
	fo := f.runDriver(t) // the merger gives up: one child unmerged
	unmerged := conflicted(t, fo)
	require.Equal(t, domain.ChildConflict, unmerged.State)
	var merged domain.FanoutChild
	for _, c := range fo.Children {
		if c.Ticket != unmerged.Ticket {
			merged = c
		}
	}
	ctx := context.Background()
	p, _ := f.store.Get(ctx, "ses_parent")
	require.False(t, Cleanable(p), "a fan-out waiting for its merge checkpoint is not cleaned")
	p.State = domain.RunStopped
	require.NoError(t, f.store.Update(ctx, p))
	p, _ = f.store.Get(ctx, "ses_parent")
	require.True(t, Cleanable(p))

	// A dry run changes nothing.
	plan, err := Clean(ctx, f.store, p, CleanOptions{DryRun: true})
	require.NoError(t, err)
	assert.Len(t, plan.Worktrees, 3)
	assert.DirExists(t, unmerged.Path)

	res, err := Clean(ctx, f.store, p, CleanOptions{})
	require.NoError(t, err)
	assert.Len(t, res.Worktrees, 3, "every worktree clean goes, the branches keep the commits")
	assert.Equal(t, []string{merged.Branch}, res.Branches, "the merged child: its commits are on the integration branch")
	assert.True(t, branchExists(t, f.project, unmerged.Branch))
	assert.True(t, branchExists(t, f.project, fo.Integration.Branch))
	assert.Len(t, res.Kept, 2)
	p, _ = f.store.Get(ctx, "ses_parent")
	assert.False(t, p.Fanout.Cleaned)
	assert.Equal(t, 2, eventKinds(p.Fanout)[domain.FanoutEventKept])

	// Again: nothing new, nothing traced twice.
	_, err = Clean(ctx, f.store, p, CleanOptions{})
	require.NoError(t, err)
	p, _ = f.store.Get(ctx, "ses_parent")
	assert.Equal(t, 2, eventKinds(p.Fanout)[domain.FanoutEventKept])

	// Explicit: the unmerged branches go, their commit traced.
	head, _ := BranchHead(ctx, f.project, unmerged.Branch)
	res, err = Clean(ctx, f.store, p, CleanOptions{Discard: true})
	require.NoError(t, err)
	require.Len(t, res.Discarded, 2)
	assert.False(t, branchExists(t, f.project, unmerged.Branch))
	p, _ = f.store.Get(ctx, "ses_parent")
	assert.True(t, p.Fanout.Cleaned)
	var traced bool
	for _, e := range p.Fanout.Events {
		traced = traced || (e.Kind == domain.FanoutEventDiscarded && strings.Contains(e.Message, unmerged.Branch+" "+head))
	}
	assert.True(t, traced, "the commit of a discarded branch is in the events")
	run(t, f.project, "git", "branch", "restored", head) // still recoverable
}

// Guards: a worktree with uncommitted changes, or where a session of the
// fan-out still runs, stays (and its branch); a branch oh did not create
// is never touched.
func TestCleanKeeps(t *testing.T) {
	f := newFixture(t, "ev-1", "ev-2")
	f.launcher.work["ev-1"] = [2]string{"a.go", "package calc\n"}
	f.launcher.work["ev-2"] = [2]string{"b.go", "package calc\n"}
	fo := f.runDriver(t)
	ctx := context.Background()
	p, _ := f.store.Get(ctx, "ses_parent")
	p.State = domain.RunStopped
	require.NoError(t, f.store.Update(ctx, p))
	p, _ = f.store.Get(ctx, "ses_parent")

	dirty, live := fo.Children[0], fo.Children[1]
	require.NoError(t, os.WriteFile(filepath.Join(dirty.Path, "wip.go"), []byte("package calc\n"), 0o644))
	run(t, f.project, "git", "branch", "feature/mine")
	p.Fanout.Children = append(p.Fanout.Children, domain.FanoutChild{Ticket: "ev-x", Branch: "feature/mine", Path: f.project, State: domain.ChildFailed})
	res, err := Clean(ctx, f.store, p, CleanOptions{Discard: true, Live: func(_ context.Context, id string) bool { return id == live.Session }})
	require.NoError(t, err)
	assert.DirExists(t, dirty.Path)
	assert.FileExists(t, filepath.Join(dirty.Path, "wip.go"))
	assert.DirExists(t, live.Path)
	assert.True(t, branchExists(t, f.project, dirty.Branch))
	assert.True(t, branchExists(t, f.project, live.Branch))
	assert.True(t, branchExists(t, f.project, "feature/mine"), "never a branch outside oh/fanout/")
	assert.Contains(t, res.Kept, dirty.Branch+": "+KeptDirty)
	assert.Contains(t, res.Kept, live.Branch+": "+KeptRunning)
	assert.DirExists(t, f.project)
}

// The parent stopped for good while its children run: oh stops them, the
// fan-out is stopped and cleaned up.
func TestDriverParentAbandoned(t *testing.T) {
	f := newFixture(t, "ev-1", "ev-2")
	f.wf.Fanout.Parallel = 1
	f.launcher.hold = time.Hour // the children never finish by themselves
	go func() {
		for {
			p, err := f.store.Get(context.Background(), "ses_parent")
			if err == nil && p.Fanout.Integration != nil {
				if c, ok := p.Fanout.Child("ev-1"); ok && c.Session != "" {
					p.State = domain.RunStopped
					_ = f.store.Update(context.Background(), p)
					return
				}
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
	fo := f.runDriver(t)
	assert.Equal(t, domain.FanoutStopped, fo.Status)
	for _, c := range fo.Children {
		assert.Equal(t, domain.ChildFailed, c.State, c.Ticket)
	}
	s, err := f.store.Get(context.Background(), fo.Children[0].Session)
	require.NoError(t, err)
	assert.Equal(t, domain.RunStopped, s.State, "the running child is stopped")
	require.Len(t, f.launcher.started, 1, "no child launched after")
	assert.NoDirExists(t, fo.Children[0].Path, "its worktree, without commit, goes")
	assert.False(t, branchExists(t, f.project, fo.Children[0].Branch))
	assert.Empty(t, f.launcher.told, "nobody to tell")
}
