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

// v5.1: oh team claim on one's own claim renews it whether it expired or not,
// without any confirmation: the claim taken by hand has no session to renew it.
func TestRunClaimRenewsTheOwnClaimWhichIsNotExpired(t *testing.T) {
	i18ntest.Set(t, "en")
	_, repo, stdout := setupClaimTestApp(t)
	soon := teamstate.InstantOf(time.Now().Add(time.Hour))
	for _, c := range []teamstate.Claim{
		{TicketID: "HAND-1", Project: claimsProject, ClaimedBy: "testuser", Status: teamstate.ClaimStatusInProgress, LeaseUntil: soon},
		{TicketID: "SES-1", Project: claimsProject, ClaimedBy: "testuser", Status: teamstate.ClaimStatusInProgress, LeaseUntil: soon, Session: "ses_mine"},
		{TicketID: "PLAN-1", Project: claimsProject, ClaimedBy: "testuser", Status: teamstate.ClaimStatusPlanned},
	} {
		_, err := repo.CreateClaim(context.Background(), c)
		require.NoError(t, err)
	}
	terminalIs(t, false)

	require.NoError(t, runClaim(newClaimYesCmd(t, claimsProject, false), []string{"HAND-1"}))
	c := claimsOnDisk(t, repo, "HAND-1")
	assert.True(t, c.LeaseUntil.Time().After(time.Now().Add(71*time.Hour)), "the lease of a claim taken by hand: 72 h by default")
	assert.Contains(t, stdout.String(), "renewed")
	assert.NotContains(t, stdout.String(), "had expired")

	stdout.Reset()
	require.NoError(t, runClaim(newClaimYesCmd(t, claimsProject, false), []string{"SES-1"}))
	c = claimsOnDisk(t, repo, "SES-1")
	assert.Equal(t, "ses_mine", c.Session)
	assert.True(t, c.LeaseUntil.Time().After(time.Now().Add(23*time.Hour)) && c.LeaseUntil.Time().Before(time.Now().Add(25*time.Hour)))

	stdout.Reset()
	require.NoError(t, runClaim(newClaimYesCmd(t, claimsProject, false), []string{"PLAN-1"}))
	assert.Empty(t, claimsOnDisk(t, repo, "PLAN-1").LeaseUntil, "a planned claim has no lease")
	assert.Contains(t, stdout.String(), "no lease")
}

// A claim taken by hand is held for lease_hours_manual (72 h by default).
func TestRunClaimByHandHoldsTheLongLease(t *testing.T) {
	_, repo, _ := setupClaimTestApp(t)
	terminalIs(t, false)

	require.NoError(t, runClaim(newClaimYesCmd(t, claimsProject, false), []string{"NEW-1"}))
	c := claimsOnDisk(t, repo, "NEW-1")
	assert.Empty(t, c.Session)
	assert.True(t, c.LeaseUntil.Time().After(time.Now().Add(71*time.Hour)))
	assert.True(t, c.LeaseUntil.Time().Before(time.Now().Add(73*time.Hour)))
}
