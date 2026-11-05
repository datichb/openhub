package teamstate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

// A repository moved on the server is the same team-state; another one is not.
func TestVerifyRemoteAndSetRemote(t *testing.T) {
	skipWithoutSh(t)
	emptyTmp(t)
	ctx := t.Context()
	bare, clone := gittest.BareClone(t)
	repo := NewRepo(bare, clone)

	moved := filepath.Join(t.TempDir(), "moved.git")
	gittest.Run(t, t.TempDir(), "clone", "-q", "--bare", bare, moved)
	require.NoError(t, repo.VerifyRemote(ctx, moved), "same history")

	other, otherClone := gittest.BareClone(t)
	gittest.Run(t, otherClone, "checkout", "-q", "--orphan", "unrelated")
	gittest.Commit(t, otherClone, "other.txt", "other\n", "another team-state")
	gittest.Run(t, otherClone, "push", "-q", "-f", "origin", "unrelated:main")
	assert.ErrorIs(t, repo.VerifyRemote(ctx, other), ErrUnrelatedRemote)
	assert.ErrorIs(t, repo.VerifyRemote(ctx, gittest.EmptyBare(t)), ErrEmptyRemote)
	err := repo.VerifyRemote(ctx, filepath.Join(t.TempDir(), "absent.git"))
	require.Error(t, err)
	assert.Nil(t, AccessError(err), "a missing repository is not an access problem")

	// a colleague pushed meanwhile: the new repository is ahead, still the same
	gittest.Commit(t, clone, "a.txt", "a\n", "ahead")
	gittest.Run(t, clone, "push", "-q")
	require.NoError(t, repo.VerifyRemote(ctx, bare))

	require.NoError(t, repo.SetRemote(ctx, moved))
	assert.Equal(t, moved, repo.Remote())
	assert.Equal(t, moved, gittest.Run(t, clone, "remote", "get-url", "origin"))
	_, err = os.Stat(filepath.Join(clone, "a.txt"))
	require.NoError(t, err, "the clone is untouched")
	assert.ErrorIs(t, NewRepo(moved, filepath.Join(t.TempDir(), "x")).SetRemote(ctx, bare), ErrNotCloned)
}

func TestVerifyRemoteRefusedAccessIsTyped(t *testing.T) {
	skipWithoutSh(t)
	emptyTmp(t)
	bare, clone := gittest.BareClone(t)
	srv := gittest.ServeHTTP(t, bare, "team-state", "bob", "good")
	restore := SetCredentialSource(nil)
	defer restore()
	err := NewRepo(bare, clone).VerifyRemote(t.Context(), srv.URL)
	require.Error(t, err)
	assert.NotNil(t, AccessError(err), "%v", err)

	restore2 := SetCredentialSource(credsFor(srv.URL, "bob", "good", nil))
	defer restore2()
	require.NoError(t, NewRepo(bare, clone).VerifyRemote(t.Context(), srv.URL))
}
