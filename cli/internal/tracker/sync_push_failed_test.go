package tracker

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

// A synchronization whose push to the team-state fails is not a success: the
// error is returned and the sync state (timestamps) is not recorded.
func TestSyncReturnsTheErrorOfAFailedPush(t *testing.T) {
	f := newHubSyncFixture(t)
	f.gl.set(1, "opened", "bug")
	f.claim(t, teamstate.Claim{TicketID: "1", Status: teamstate.ClaimStatusInProgress})
	hook := filepath.Join(f.bare, "hooks", "pre-receive")
	require.NoError(t, os.MkdirAll(filepath.Dir(hook), 0o755))
	require.NoError(t, os.WriteFile(hook, []byte("#!/bin/sh\necho refused >&2\nexit 1\n"), 0o755)) //nolint:gosec // a hook must be executable

	tr := newGitLab(Config{Type: TypeGitLab, BaseURL: f.cfg.TrackerURL, Token: "tok"})
	res, err := NewEngine(tr, f.repo, f.cfg, f.stateDir).Run(context.Background())
	require.Error(t, err)
	assert.Nil(t, res)
	assert.NoFileExists(t, filepath.Join(f.stateDir, "sync-state.json"))
}
