package teamstate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/filelock"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

func countEventLines(t *testing.T, repo *Repo, project string) int {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(repo.Path(), "projects", project, "events", "*.jsonl"))
	n := 0
	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(t, err)
		n += strings.Count(string(data), "\n")
	}
	return n
}

// N15: two Repo values on the same clone, 50 concurrent writers: 50 lines,
// 50 commits (the in-process lock is per path, not per Repo).
func TestConcurrentWritersSameClone(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("git")
	}
	dir := filepath.Join(t.TempDir(), "solo")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	gittest.Run(t, dir, "init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("init"), 0o644))
	gittest.Run(t, dir, "add", ".")
	gittest.Run(t, dir, "commit", "-qm", "init")
	repo := NewRepo("", dir) // no remote: commits only (the push path is covered across processes)
	ctx := context.Background()
	twin := NewRepo("", dir)
	base, _ := strconv.Atoi(gittest.Run(t, repo.Path(), "rev-list", "--count", "HEAD"))

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := repo
			if i%2 == 1 {
				r = twin
			}
			errs <- r.AppendEvent(WithLockWait(ctx, 2*time.Minute), Event{Type: "test", Actor: "a", Project: "p", Data: map[string]any{"i": i}})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	assert.Equal(t, n, countEventLines(t, repo, "p"))
	commits, _ := strconv.Atoi(gittest.Run(t, repo.Path(), "rev-list", "--count", "HEAD"))
	assert.Equal(t, base+n, commits)
	assert.Empty(t, gittest.Run(t, repo.Path(), "status", "--porcelain"))
}

// The child process of TestConcurrentWritersProcesses.
func TestHelperTeamStateWriter(t *testing.T) {
	path := os.Getenv("OH_TEST_TEAMSTATE_CLONE")
	if path == "" {
		t.Skip("helper process")
	}
	repo := NewRepo(os.Getenv("OH_TEST_TEAMSTATE_REMOTE"), path)
	ctx := WithLockWait(context.Background(), 3*time.Minute)
	for i := range 5 {
		if err := repo.AppendEvent(ctx, Event{Type: "test", Actor: "proc", Project: "p", Data: map[string]any{"pid": os.Getpid(), "i": i}}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
	}
}

// N15: the same race across processes (CLI, TUI, daemon on one clone).
func TestConcurrentWritersProcesses(t *testing.T) {
	repo, bare := setupGitTestRepo(t)
	base, _ := strconv.Atoi(gittest.Run(t, repo.Path(), "rev-list", "--count", "HEAD"))
	const procs = 4
	cmds := make([]*exec.Cmd, procs)
	outs := make([]*strings.Builder, procs)
	for i := range procs {
		c := exec.Command(os.Args[0], "-test.run=^TestHelperTeamStateWriter$", "-test.count=1")
		c.Env = append(os.Environ(), "OH_TEST_TEAMSTATE_CLONE="+repo.Path(), "OH_TEST_TEAMSTATE_REMOTE="+bare)
		outs[i] = &strings.Builder{}
		c.Stdout, c.Stderr = outs[i], outs[i]
		require.NoError(t, c.Start())
		cmds[i] = c
	}
	for i, c := range cmds {
		require.NoError(t, c.Wait(), "process %d: %s", i, outs[i].String())
	}
	assert.Equal(t, procs*5, countEventLines(t, repo, "p"))
	commits, _ := strconv.Atoi(gittest.Run(t, repo.Path(), "rev-list", "--count", "HEAD"))
	assert.Equal(t, base+procs*5, commits)
	assert.Empty(t, gittest.Run(t, repo.Path(), "status", "--porcelain"))
	assert.Equal(t, gittest.Run(t, repo.Path(), "rev-parse", "HEAD"), gittest.Run(t, bare, "rev-parse", "HEAD"), "everything pushed")
}

// N15: a busy clone gives ErrBusy naming the holder; WithTryLock does not
// wait (TUI background synchronizations skip a turn).
func TestBusyClone(t *testing.T) {
	repo, bare := setupGitTestRepo(t)
	ctx := context.Background()
	release, err := repo.lockGit(ctx)
	require.NoError(t, err)

	other := NewRepo(bare, repo.Path())
	start := time.Now()
	err = other.Pull(WithTryLock(ctx))
	require.ErrorIs(t, err, ErrBusy)
	assert.Less(t, time.Since(start), time.Second, "no wait")
	assert.Contains(t, err.Error(), "pid "+strconv.Itoa(os.Getpid()))

	done := make(chan error, 1)
	go func() { done <- other.Pull(WithLockWait(ctx, 10*time.Second)) }()
	time.Sleep(100 * time.Millisecond)
	release()
	require.NoError(t, <-done, "waits for the holder")
	assert.NoFileExists(t, filepath.Join(repo.Path(), ".git", "oh.lock.info"))

	// Held by another process (a separate file lock): the wait is bounded.
	lockFile := filepath.Join(repo.Path(), ".git", "oh.lock")
	unlock, err := filelock.Lock(lockFile)
	require.NoError(t, err)
	writeHolder(lockFile)
	start = time.Now()
	err = other.Pull(WithLockWait(ctx, 300*time.Millisecond))
	require.ErrorIs(t, err, ErrBusy)
	assert.Less(t, time.Since(start), 5*time.Second)
	assert.Contains(t, err.Error(), "held by pid")
	unlock()
	require.NoError(t, other.Pull(ctx))
}

// N15: a stale .git/index.lock (killed git) is removed under the lock; a
// recent one (a git running in the clone) makes the clone busy.
func TestIndexLock(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	idx := filepath.Join(repo.Path(), ".git", "index.lock")

	require.NoError(t, os.WriteFile(idx, nil, 0o644))
	err := repo.Pull(WithLockWait(ctx, 300*time.Millisecond))
	require.ErrorIs(t, err, ErrBusy)
	assert.Contains(t, err.Error(), "index.lock")
	assert.FileExists(t, idx, "a recent lock is not removed")

	old := time.Now().Add(-10 * time.Minute)
	require.NoError(t, os.Chtimes(idx, old, old))
	require.NoError(t, repo.Pull(ctx))
	assert.NoFileExists(t, idx)
}

// N15: a rejected push drops our commit and puts back only the files of the
// transaction: changes of the clone that are not ours are kept.
func TestTransactKeepsOtherChanges(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := context.Background()
	other := NewRepo(bare, filepath.Join(t.TempDir(), "other"))
	require.NoError(t, other.Clone(ctx))

	// Not part of the transaction: an untracked file and a tracked change.
	require.NoError(t, os.WriteFile(filepath.Join(repo.Path(), "draft.txt"), []byte("mine"), 0o644))
	var otherCalls int
	repo.beforePush = func() {
		repo.beforePush = nil
		require.NoError(t, os.WriteFile(filepath.Join(repo.Path(), "README.md"), []byte("edited meanwhile"), 0o644))
		require.NoError(t, other.Transact(ctx, counterBuild(&otherCalls, "bob")))
	}
	var calls int
	err := repo.Transact(ctx, counterBuild(&calls))
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
	got, _ := os.ReadFile(filepath.Join(repo.Path(), "draft.txt"))
	assert.Equal(t, "mine", string(got))
	got, _ = os.ReadFile(filepath.Join(repo.Path(), "README.md"))
	assert.Equal(t, "edited meanwhile", string(got), "a hard reset would have lost it")
}

// N15: the offline queue is written under the lock.
func TestQueueUnderLock(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	release, err := repo.lockGit(context.Background())
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- repo.Enqueue(QueuedOp{Kind: "publish", Scope: "team", ID: "x"}) }()
	select {
	case err := <-done:
		t.Fatalf("enqueued while locked: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	release()
	require.NoError(t, <-done)
	ops, err := repo.Queue()
	require.NoError(t, err)
	require.Len(t, ops, 1)
	assert.True(t, errors.Is(&BusyError{}, ErrBusy))
}
