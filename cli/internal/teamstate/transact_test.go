package teamstate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// counterBuild bumps counter.txt from what is on disk, like a publication
// computes its next version.
func counterBuild(calls *int, who ...string) func(context.Context, *Tx) (TxResult, error) {
	return func(_ context.Context, tx *Tx) (TxResult, error) {
		*calls++
		n := 0
		if data, err := tx.ReadFile("counter.txt"); err == nil {
			n, _ = strconv.Atoi(strings.TrimSpace(string(data)))
		}
		n++
		msg := "counter " + strconv.Itoa(n)
		if len(who) > 0 {
			msg += " by " + who[0]
		}
		return TxResult{Message: msg}, tx.WriteFile("counter.txt", []byte(strconv.Itoa(n)))
	}
}

func TestTransact_RebuildsAfterRejectedPush(t *testing.T) {
	repo, bare := setupGitTestRepo(t)
	gitIdentity(t)
	ctx := context.Background()
	other := NewRepo(bare, filepath.Join(t.TempDir(), "other"))
	require.NoError(t, other.Clone(ctx))

	// Between our commit and our push, another member publishes.
	var otherCalls int
	repo.beforePush = func() {
		repo.beforePush = nil
		// A different author, as in real life (identical commits would merge).
		require.NoError(t, other.Transact(ctx, counterBuild(&otherCalls, "bob")))
	}
	var calls int
	require.NoError(t, repo.Transact(ctx, counterBuild(&calls)))
	assert.Equal(t, 2, calls, "the build ran again on top of the other push")
	data, err := os.ReadFile(filepath.Join(repo.Path(), "counter.txt"))
	require.NoError(t, err)
	assert.Equal(t, "2", string(data), "computed from the other member's value")
	assert.Equal(t, "counter 2", gitOut(t, repo.Path(), "log", "-1", "--format=%s"))
	assert.Equal(t, "counter 1 by bob", gitOut(t, repo.Path(), "log", "-1", "--skip=1", "--format=%s"))
}

func TestTransact_BuildErrorRestoresFiles(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	gitIdentity(t)
	ctx := context.Background()
	err := repo.Transact(ctx, func(_ context.Context, tx *Tx) (TxResult, error) {
		require.NoError(t, tx.WriteFile("README.md", []byte("changed")))
		require.NoError(t, tx.WriteFile("new/file.txt", []byte("x")))
		return TxResult{}, errors.New("invalid")
	})
	require.EqualError(t, err, "invalid")
	data, _ := os.ReadFile(filepath.Join(repo.Path(), "README.md"))
	assert.Equal(t, "init", string(data))
	assert.NoFileExists(t, filepath.Join(repo.Path(), "new", "file.txt"))
	assert.Equal(t, "", gitOut(t, repo.Path(), "status", "--porcelain"))
}

func TestTransact_Offline(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	gitIdentity(t)
	ctx := context.Background()
	gitCmd(t, repo.Path(), "remote", "set-url", "origin", "http://127.0.0.1:1/x.git")
	var calls int
	err := repo.Transact(ctx, counterBuild(&calls))
	assert.ErrorIs(t, err, ErrOffline)
	assert.Zero(t, calls, "nothing built while offline")
	assert.Equal(t, "init", gitOut(t, repo.Path(), "log", "-1", "--format=%s"))
}

func TestQueue(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	require.NoError(t, repo.Enqueue(QueuedOp{Kind: "publish", Scope: "team", ID: "a", Member: "alice"}))
	require.NoError(t, repo.Enqueue(QueuedOp{Kind: "publish", Scope: "project:web", ID: "a", Member: "alice"}))
	require.NoError(t, repo.Enqueue(QueuedOp{Kind: "restore", Scope: "team", ID: "a", Member: "alice", Version: 2}))
	ops, err := repo.Queue()
	require.NoError(t, err)
	require.Len(t, ops, 2, "same workflow replaced")
	assert.Equal(t, "restore", ops[1].Kind)
	assert.Empty(t, gitOut(t, repo.Path(), "status", "--porcelain"), "the queue is not versioned")
	require.NoError(t, repo.Dequeue("team", "a"))
	require.NoError(t, repo.Dequeue("project:web", "a"))
	ops, _ = repo.Queue()
	assert.Empty(t, ops)

	s, err := ParseScope("project:web")
	require.NoError(t, err)
	assert.Equal(t, ProjectScope("web"), s)
	_, err = ParseScope("project:../x")
	assert.Error(t, err)
}
