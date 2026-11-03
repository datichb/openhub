package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// The commit of the structure of the team-state ("team: init structure")
// that fails is told to the user, in his language, with what stays true (the
// folders exist, the team works) and what happens next. It does not stop the
// join, and a team-state whose structure is already there says nothing.
func TestEnsureTeamStructureTellsAFailedCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	defer i18n.SetLocale("en")
	ctx := context.Background()
	bare, clone := gittest.BareClone(t)
	repo := teamstate.NewRepo(bare, clone)

	moved := bare + ".away"
	require.NoError(t, os.Rename(bare, moved))
	i18n.SetLocale("en")
	warn, err := ensureTeamStructure(ctx, repo)
	require.NoError(t, err, "a commit that fails does not stop the join")
	assert.Contains(t, warn, "team: init structure")
	assert.Contains(t, warn, "oh team init", "what to do")
	assert.FileExists(t, filepath.Join(clone, "catalog", "agents", ".gitkeep"))

	i18n.SetLocale("fr")
	warn, err = ensureTeamStructure(ctx, repo)
	require.NoError(t, err)
	assert.Contains(t, warn, "commit", "in French")
	assert.True(t, strings.Contains(warn, "dossiers") || strings.Contains(warn, "structure"), warn)

	require.NoError(t, os.Rename(moved, bare))
	warn, err = ensureTeamStructure(ctx, repo)
	require.NoError(t, err)
	assert.Empty(t, warn, "committed this time: nothing to tell")
	warn, err = ensureTeamStructure(ctx, repo)
	require.NoError(t, err)
	assert.Empty(t, warn, "already there: nothing to tell")
}

// A failure that is not the commit (the folders cannot be created) stays an error.
func TestEnsureTeamStructureKeepsHardErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(file, nil, 0o644))
	warn, err := ensureTeamStructure(context.Background(), teamstate.NewRepo("", filepath.Join(file, "state")))
	require.Error(t, err)
	assert.Empty(t, warn)
}
