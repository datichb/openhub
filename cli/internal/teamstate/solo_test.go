package teamstate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/workflow"
)

func gitIdentity(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@test.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@test.com")
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return strings.TrimSpace(string(out))
}

func TestInitSolo(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	gitIdentity(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "teams", "solo")
	repo, err := InitSolo(ctx, path, Member{ID: "alice", DisplayName: "Alice", Role: "dev"})
	require.NoError(t, err)

	assert.True(t, repo.IsCloned())
	assert.True(t, repo.IsLocalOnly(ctx))
	assert.Equal(t, "main", gitOut(t, path, "rev-parse", "--abbrev-ref", "HEAD"))
	assert.Equal(t, "", gitOut(t, path, "status", "--porcelain"), "everything is committed")

	members, err := repo.ListMembers()
	require.NoError(t, err)
	require.Len(t, members, 1)
	assert.Equal(t, "alice", members[0].ID)
	assert.Equal(t, SoloMemberRole, members[0].Role)
	cfg, err := repo.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, GovernancePublishAnyMember, cfg.Governance.Publish)
	require.NoError(t, repo.CheckPublish("alice"))
	_, err = os.Stat(filepath.Join(path, "workflows", "published", ".gitkeep"))
	require.NoError(t, err)

	// Commits stay local: pull, push and CommitAndPush work without remote.
	require.NoError(t, repo.Pull(ctx))
	require.NoError(t, repo.WithWriteLock(ctx, func(ctx context.Context) error {
		rel, err := PublishedRel(TeamScope(), "quick")
		require.NoError(t, err)
		writeRepoFile(t, repo, rel, "apiVersion: oh/v1\nkind: Workflow\nid: quick\nrisk: read\n")
		_, files, err := repo.SealWorkflowLocal(TeamScope(), "quick", LockMeta{Version: 1, PublishedBy: "alice", PublishedAt: time.Now()})
		require.NoError(t, err)
		return repo.commitAndPush(ctx, "workflow: publish quick v1", files...)
	}))
	assert.Equal(t, "workflow: publish quick v1", gitOut(t, path, "log", "-1", "--format=%s"))
	cat := workflow.NewMemCatalog()
	require.Empty(t, repo.LoadWorkflowLayers(cat, ""))
	assert.Len(t, cat.Refs(), 1)

	// A second solo space in the same folder is refused.
	_, err = InitSolo(ctx, path, Member{ID: "alice"})
	assert.ErrorIs(t, err, ErrSoloExists)
	_, err = InitSolo(ctx, filepath.Join(t.TempDir(), "x"), Member{ID: "../alice"})
	assert.ErrorIs(t, err, ErrUnsafeName)
}

// Acceptance criterion 4: promoting a solo space keeps everything.
func TestPromote(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	gitIdentity(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "solo")
	repo, err := InitSolo(ctx, path, Member{ID: "alice"})
	require.NoError(t, err)
	head := gitOut(t, path, "rev-parse", "HEAD")

	// A non-empty remote is refused and the space stays solo.
	busy := t.TempDir()
	gitCmd(t, busy, "init", "--bare")
	tmp := filepath.Join(t.TempDir(), "w")
	gitCmd(t, t.TempDir(), "clone", busy, tmp)
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "x"), []byte("x"), 0o644))
	gitCmd(t, tmp, "add", ".")
	gitCmd(t, tmp, "commit", "-m", "x")
	gitCmd(t, tmp, "push", "origin", "HEAD:main")
	err = repo.Promote(ctx, busy)
	require.Error(t, err)
	assert.True(t, repo.IsLocalOnly(ctx))

	bare := t.TempDir()
	gitCmd(t, bare, "init", "--bare")
	require.NoError(t, repo.Promote(ctx, bare))
	assert.False(t, repo.IsLocalOnly(ctx))
	assert.Equal(t, bare, repo.Remote())
	assert.ErrorIs(t, repo.Promote(ctx, bare), ErrHasRemote)

	// Another member clones the promoted team-state: same history.
	other := filepath.Join(t.TempDir(), "other")
	gitCmd(t, t.TempDir(), "clone", bare, other)
	assert.Equal(t, head, gitOut(t, other, "rev-parse", "HEAD"))
	oRepo := NewRepo(bare, other)
	require.NoError(t, oRepo.CheckPublish("alice"))

	// Pushes now reach the remote.
	require.NoError(t, repo.WithWriteLock(ctx, func(ctx context.Context) error {
		writeRepoFile(t, repo, "wiki/note.md", "hello")
		return repo.commitAndPush(ctx, "wiki: note", "wiki/note.md")
	}))
	require.NoError(t, oRepo.Pull(ctx))
	_, err = os.Stat(filepath.Join(other, "wiki", "note.md"))
	assert.NoError(t, err)
}
