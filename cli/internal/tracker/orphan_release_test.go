package tracker

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// A claim that its owner releases on purpose (oh team release, end of a
// session, board, retention of the done claims) is gone from the repository
// on purpose: the next synchronization, which saw it the time before, must not
// take it for a claim lost to a git conflict and recreate it. A claim that
// disappears any other way is still recovered (ADR-032).
func TestSyncDoesNotRecoverAClaimReleasedOnPurpose(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	ctx := context.Background()
	for _, c := range []struct {
		name        string
		remove      func(t *testing.T, repo *teamstate.Repo, clone string)
		wantRecover bool
	}{
		{"oh team release", func(t *testing.T, repo *teamstate.Repo, _ string) {
			require.NoError(t, repo.ReleaseClaim(ctx, "p", "1"))
		}, false},
		{"queued release of a session", func(t *testing.T, repo *teamstate.Repo, _ string) {
			require.NoError(t, repo.EnqueueClaimOp(teamstate.ClaimOp{Kind: teamstate.ClaimOpRelease, Project: "p", TicketID: "1", ClaimedBy: "alice"}))
			applied, _, err := repo.FlushClaimQueue(ctx)
			require.NoError(t, err)
			require.Equal(t, 1, applied)
		}, false},
		{"retention of the done claims", func(t *testing.T, repo *teamstate.Repo, _ string) {
			require.NoError(t, repo.UpdateClaimStatusFromTracker(ctx, "p", "1", teamstate.ClaimStatusDone))
			released, err := repo.CleanupDoneClaims(ctx, -1)
			require.NoError(t, err)
			require.Len(t, released, 1)
		}, false},
		{"claim file removed by hand", func(t *testing.T, repo *teamstate.Repo, clone string) {
			gittest.Run(t, clone, "rm", "-q", "projects/p/claims/1.toml")
			gittest.Run(t, clone, "commit", "-qm", "oops")
			gittest.Run(t, clone, "push", "-q")
		}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			bare, clone := gittest.BareClone(t)
			repo := teamstate.NewRepo(bare, clone)
			require.NoError(t, repo.AddMember(ctx, teamstate.Member{ID: "alice", DisplayName: "Alice"}))
			_, err := repo.CreateClaim(ctx, teamstate.Claim{TicketID: "1", Project: "p", ClaimedBy: "alice", ExternalIID: 1})
			require.NoError(t, err)

			tr := &stubTracker{issues: map[int]IssueState{1: {IID: 1, State: "open", StatusName: "opened", Title: "first"}}}
			stateDir := t.TempDir()
			cfg := teamstate.TrackerConfig{Projects: map[string]string{"p": "grp/p"}} //nolint:staticcheck // the engine still reads Projects
			_, err = NewEngine(tr, repo, cfg, stateDir).Run(ctx)
			require.NoError(t, err)
			state, err := LoadSyncState(stateDir)
			require.NoError(t, err)
			require.Len(t, state.GetKnownTickets("p"), 1, "the first run saw the claim")

			c.remove(t, repo, clone)
			_, err = repo.GetClaim("p", "1")
			require.ErrorIs(t, err, teamstate.ErrClaimNotFound)

			res, err := NewEngine(tr, repo, cfg, stateDir).Run(ctx)
			require.NoError(t, err)
			_, err = repo.GetClaim("p", "1")
			if c.wantRecover {
				require.NoError(t, err, "a claim lost by accident is recovered")
				assert.Equal(t, 1, res.Projects[0].ClaimsCreated)
				return
			}
			require.ErrorIs(t, err, teamstate.ErrClaimNotFound, "a released claim stays released")
			assert.Zero(t, res.Projects[0].ClaimsCreated)
			_, statErr := os.Stat(filepath.Join(clone, "projects", "p", "claims", "1.toml"))
			assert.True(t, os.IsNotExist(statErr))

			state, err = LoadSyncState(stateDir)
			require.NoError(t, err)
			assert.Empty(t, state.GetKnownTickets("p"), "the ticket leaves the known tickets")
		})
	}
}
