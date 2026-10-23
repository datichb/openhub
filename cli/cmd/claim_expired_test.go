package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

func newClaimYesCmd(t *testing.T, project string, yes bool) *cobra.Command {
	t.Helper()
	cmd := newClaimCmd(t, project)
	addYesFlag(cmd)
	if yes {
		require.NoError(t, cmd.Flags().Set("yes", "true"))
	}
	return cmd
}

// expiredClaimOf creates a claim of alice whose lease ended an hour ago.
func expiredClaimOf(t *testing.T, repo *teamstate.Repo, ticket, member string) {
	t.Helper()
	_, err := repo.CreateClaim(context.Background(), teamstate.Claim{
		TicketID: ticket, Project: claimsProject, ClaimedBy: member, Status: teamstate.ClaimStatusInProgress,
		Session: "ses_alice", LeaseUntil: teamstate.InstantOf(time.Now().Add(-time.Hour)),
	})
	require.NoError(t, err)
}

// v5.1 (B3b): a claim of another member whose lease is past can be taken over
// by oh team claim, as a stale one could, after the common confirmation.
func TestRunClaimTakesOverAnExpiredClaim(t *testing.T) {
	i18ntest.Set(t, "en")
	_, repo, stdout := setupClaimTestApp(t)
	expiredClaimOf(t, repo, "EXP-1", "alice")
	terminalIs(t, false)

	require.NoError(t, runClaim(newClaimYesCmd(t, claimsProject, true), []string{"EXP-1"}))

	c := claimsOnDisk(t, repo, "EXP-1")
	assert.Equal(t, "testuser", c.ClaimedBy)
	assert.Empty(t, c.Session, "the session of the previous owner is gone")
	assert.True(t, c.LeaseUntil.Time().After(time.Now().Add(23*time.Hour)), "a new lease")
	assert.Contains(t, stdout.String(), "lease")
	assert.Contains(t, stdout.String(), "alice")
	assert.Contains(t, stdout.String(), "EXP-1")
}

// Without --yes and without a terminal nobody can confirm: refused (code 2),
// the claim is left as it is.
func TestRunClaimExpiredNeedsConfirmation(t *testing.T) {
	_, repo, _ := setupClaimTestApp(t)
	expiredClaimOf(t, repo, "EXP-1", "alice")
	terminalIs(t, false)

	err := runClaim(newClaimYesCmd(t, claimsProject, false), []string{"EXP-1"})
	require.Error(t, err)
	assert.Equal(t, "alice", claimsOnDisk(t, repo, "EXP-1").ClaimedBy)
}

// With a terminal, the answer decides: no leaves the claim to its owner.
func TestRunClaimExpiredDeclined(t *testing.T) {
	i18ntest.Set(t, "en")
	_, repo, stdout := setupClaimTestApp(t)
	expiredClaimOf(t, repo, "EXP-1", "alice")
	terminalIs(t, true)
	fa := stubConfirmForm(t, false)

	require.NoError(t, runClaim(newClaimYesCmd(t, claimsProject, false), []string{"EXP-1"}))
	require.Len(t, fa.questions, 1)
	assert.Contains(t, fa.questions[0], "EXP-1")
	assert.Equal(t, "alice", claimsOnDisk(t, repo, "EXP-1").ClaimedBy)
	assert.Contains(t, stdout.String(), "alice")
}

// A claim whose lease is still valid is not taken over, whatever --yes says.
func TestRunClaimWithAValidLeaseIsNotTakenOver(t *testing.T) {
	_, repo, _ := setupClaimTestApp(t)
	claimsCreate(t, repo, "LIVE-1", "alice", "ses_alice")
	terminalIs(t, false)

	require.NoError(t, runClaim(newClaimYesCmd(t, claimsProject, true), []string{"LIVE-1"}))
	assert.Equal(t, "alice", claimsOnDisk(t, repo, "LIVE-1").ClaimedBy)
}

// The member's own expired claim is renewed (lease and nothing else), no
// takeover brief for oneself.
func TestRunClaimRenewsTheOwnExpiredClaim(t *testing.T) {
	i18ntest.Set(t, "en")
	_, repo, stdout := setupClaimTestApp(t)
	expiredClaimOf(t, repo, "MINE-1", "testuser")
	terminalIs(t, false)

	require.NoError(t, runClaim(newClaimYesCmd(t, claimsProject, false), []string{"MINE-1"}))
	c := claimsOnDisk(t, repo, "MINE-1")
	assert.Equal(t, "testuser", c.ClaimedBy)
	assert.Equal(t, "ses_alice", c.Session, "the session stays")
	assert.True(t, c.LeaseUntil.Time().After(time.Now().Add(23*time.Hour)))
	assert.Contains(t, stdout.String(), "renewed")
}
