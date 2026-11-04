package teamstate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

// A Repo made without a folder (state_path empty) is refused: its folder "" is
// the working directory, whose own repository (the project's) was pulled,
// locked and pushed to as if it were the team-state.
func TestNewRepoWithoutAFolderRefusesEveryOperation(t *testing.T) {
	ctx := context.Background()
	bare, project := gittest.BareClone(t)
	t.Chdir(project)

	headBefore := gittest.Run(t, project, "rev-parse", "HEAD")
	remoteBefore := gittest.Run(t, project, "remote", "get-url", "origin")
	bareBefore := gittest.Run(t, bare, "rev-parse", "main")

	for _, remote := range []string{"x", bare, ""} {
		r := NewRepo(remote, "")
		require.NotNil(t, r)
		assert.Equal(t, "", r.Path(), "no folder")
		assert.False(t, r.IsCloned(), "the project in the working directory is not a clone of the team-state")

		assert.ErrorIs(t, r.EnsureReady(ctx), ErrNoStatePath)
		assert.ErrorIs(t, r.Clone(ctx), ErrNoStatePath)
		assert.ErrorIs(t, r.Pull(ctx), ErrNoStatePath)
		assert.Error(t, r.CommitAndPush(ctx, "team: stray", "README.md"))
		assert.Error(t, r.SaveConfig(ctx, &TeamConfig{}), "Ctrl+S of the team detail must not commit in the project")
		assert.Error(t, r.AddMember(ctx, Member{ID: "alice", DisplayName: "Alice"}))
		_, err := r.ListMembers(ctx)
		assert.Error(t, err)
		assert.NoError(t, r.CheckRemote(), "nothing to compare")
	}

	_, err := os.Stat(filepath.Join(project, ".git", "oh.lock"))
	assert.True(t, os.IsNotExist(err), "no lock file in the project")
	assert.Equal(t, headBefore, gittest.Run(t, project, "rev-parse", "HEAD"), "no commit in the project")
	assert.Equal(t, remoteBefore, gittest.Run(t, project, "remote", "get-url", "origin"), "the origin of the project is untouched")
	assert.Equal(t, bareBefore, gittest.Run(t, bare, "rev-parse", "main"), "nothing was pushed")
	assert.Empty(t, gittest.Run(t, project, "status", "--porcelain", "--untracked-files=all"), "no file written in the project")
	entries, err := os.ReadDir(project)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.ElementsMatch(t, []string{".git", "README.md"}, names, "the working directory holds what it held")
}

// A blank folder is no folder either.
func TestNewRepoWithABlankFolderIsRefused(t *testing.T) {
	r := NewRepo("x", "  ")
	assert.ErrorIs(t, r.EnsureReady(context.Background()), ErrNoStatePath)
}
