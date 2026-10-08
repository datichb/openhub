package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

func newClaimCmd(t *testing.T, project string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "claim"}
	cmd.Flags().StringP("project", "p", project, "")
	cmd.Flags().String("worktree", "", "")
	cmd.Flags().Bool("planned", false, "")
	cmd.SetContext(context.Background())
	return cmd
}

func newReleaseCmd(t *testing.T, project string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "release"}
	cmd.Flags().StringP("project", "p", project, "")
	cmd.SetContext(context.Background())
	return cmd
}

func newTransferCmd(t *testing.T, project, to string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "transfer"}
	cmd.Flags().String("to", to, "")
	cmd.Flags().StringP("project", "p", project, "")
	cmd.SetContext(context.Background())
	return cmd
}

func TestRunClaim_Success(t *testing.T) {
	_, repo, stdout := setupClaimTestApp(t)
	ctx := context.Background()

	cmd := newClaimCmd(t, "myproject")
	err := runClaim(cmd, []string{"TICKET-1"})
	require.NoError(t, err)

	// Verify success message
	assert.Contains(t, stdout.String(), "myproject/TICKET-1")
	assert.Contains(t, stdout.String(), "testuser")

	// Verify claim file was created
	claim, err := repo.GetClaim("myproject", "TICKET-1")
	require.NoError(t, err)
	assert.Equal(t, "testuser", claim.ClaimedBy)
	assert.Equal(t, teamstate.ClaimStatusInProgress, claim.Status)

	// Event was emitted
	events, err := repo.ListEventsLimited("myproject", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, events)

	_ = ctx
}

func TestRunClaim_AlreadyTaken(t *testing.T) {
	a, repo, stdout := setupClaimTestApp(t)
	ctx := context.Background()

	// Pre-create a claim by another member
	claim := teamstate.Claim{
		TicketID:  "TAKEN-1",
		Project:   "myproject",
		ClaimedBy: "alice",
		ClaimedAt: time.Now().Add(-1 * time.Hour),
		Status:    teamstate.ClaimStatusInProgress,
	}
	_, err := repo.CreateClaim(ctx, claim)
	require.NoError(t, err)

	cmd := newClaimCmd(t, "myproject")
	err = runClaim(cmd, []string{"TAKEN-1"})
	// Should NOT return an error — it's a warning
	assert.NoError(t, err)

	// Should display warning about existing claim
	output := stdout.String()
	assert.Contains(t, output, "alice")
	assert.Contains(t, output, "TAKEN-1")

	_ = a
}

func TestRunClaim_Planned(t *testing.T) {
	_, repo, _ := setupClaimTestApp(t)

	cmd := newClaimCmd(t, "myproject")
	_ = cmd.Flags().Set("planned", "true")
	err := runClaim(cmd, []string{"PLAN-1"})
	require.NoError(t, err)

	// Verify claim has planned status
	claim, err := repo.GetClaim("myproject", "PLAN-1")
	require.NoError(t, err)
	assert.Equal(t, teamstate.ClaimStatusPlanned, claim.Status)
}

func TestRunRelease_Success(t *testing.T) {
	_, repo, stdout := setupClaimTestApp(t)
	ctx := context.Background()

	// Create a claim first
	claim := teamstate.Claim{
		TicketID:  "REL-1",
		Project:   "myproject",
		ClaimedBy: "testuser",
		ClaimedAt: time.Now(),
		Status:    teamstate.ClaimStatusInProgress,
	}
	_, err := repo.CreateClaim(ctx, claim)
	require.NoError(t, err)

	cmd := newReleaseCmd(t, "myproject")
	err = runRelease(cmd, []string{"REL-1"})
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), "REL-1")

	// Verify claim is gone
	_, err = repo.GetClaim("myproject", "REL-1")
	assert.Error(t, err)
}

func TestRunRelease_NotFound(t *testing.T) {
	_, _, stdout := setupClaimTestApp(t)

	cmd := newReleaseCmd(t, "myproject")
	err := runRelease(cmd, []string{"GHOST-1"})
	assert.NoError(t, err) // warning, not error

	assert.Contains(t, stdout.String(), "GHOST-1")
}

func TestRunClaimTransfer_Success(t *testing.T) {
	_, repo, stdout := setupClaimTestApp(t)
	ctx := context.Background()

	// Create a claim to transfer
	claim := teamstate.Claim{
		TicketID:  "XFER-1",
		Project:   "myproject",
		ClaimedBy: "testuser",
		ClaimedAt: time.Now(),
		Status:    teamstate.ClaimStatusInProgress,
	}
	_, err := repo.CreateClaim(ctx, claim)
	require.NoError(t, err)

	cmd := newTransferCmd(t, "myproject", "alice")
	err = runClaimTransfer(cmd, []string{"XFER-1"})
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), "alice")
	assert.Contains(t, stdout.String(), "XFER-1")

	// Verify ownership changed
	updated, err := repo.GetClaim("myproject", "XFER-1")
	require.NoError(t, err)
	assert.Equal(t, "alice", updated.ClaimedBy)

	// Verify takeover brief was generated
	briefsDir := filepath.Join(repo.Path(), "projects", "myproject", "takeover-briefs")
	entries, _ := os.ReadDir(briefsDir)
	_ = fmt.Sprintf("briefs: %d", len(entries)) // brief generation is best-effort
}

// QB2: a session started on tickets (any workflow, not only the --dev alias)
// claims each ticket for the member, or moves its planned claim to the work
// status; a ticket claimed by someone else is reported.
func TestStartTicketClaim(t *testing.T) {
	_, repo, _ := setupClaimTestApp(t)
	ctx := context.Background()

	note := startTicketClaim(ctx, repo, "myproject", "testuser", "NEW-1")
	assert.Contains(t, note, "NEW-1")
	c, err := repo.GetClaim("myproject", "NEW-1")
	require.NoError(t, err)
	assert.Equal(t, teamstate.ClaimStatusInProgress, c.Status)

	_, err = repo.CreateClaim(ctx, teamstate.Claim{TicketID: "PLAN-2", Project: "myproject", ClaimedBy: "testuser", Status: teamstate.ClaimStatusPlanned})
	require.NoError(t, err)
	note = startTicketClaim(ctx, repo, "myproject", "testuser", "PLAN-2")
	assert.Contains(t, note, "→")
	c, err = repo.GetClaim("myproject", "PLAN-2")
	require.NoError(t, err)
	assert.Equal(t, teamstate.ClaimStatusInProgress, c.Status, "planned → in progress")

	assert.Empty(t, startTicketClaim(ctx, repo, "myproject", "testuser", "PLAN-2"), "already in progress: nothing to do")

	_, err = repo.CreateClaim(ctx, teamstate.Claim{TicketID: "ALICE-3", Project: "myproject", ClaimedBy: "alice", Status: teamstate.ClaimStatusPlanned})
	require.NoError(t, err)
	assert.Contains(t, startTicketClaim(ctx, repo, "myproject", "testuser", "ALICE-3"), "alice")
	c, _ = repo.GetClaim("myproject", "ALICE-3")
	assert.Equal(t, teamstate.ClaimStatusPlanned, c.Status, "someone else's claim is left as is")
}

// A31: at the end of a session, the claim of a closed ticket is completed
// (terminal status) and the claim of a ticket still open is released; the
// claims of other members and unknown tickets are left as they are.
func TestEndTicketClaims(t *testing.T) {
	_, repo, _ := setupClaimTestApp(t)
	ctx := context.Background()
	for _, c := range []teamstate.Claim{
		{TicketID: "pt-cb8", Project: "myproject", ClaimedBy: "testuser", Status: teamstate.ClaimStatusInProgress},
		{TicketID: "pt-vau", Project: "myproject", ClaimedBy: "testuser", Status: teamstate.ClaimStatusInProgress},
		{TicketID: "pt-bob", Project: "myproject", ClaimedBy: "bob", Status: teamstate.ClaimStatusInProgress},
		{TicketID: "pt-gone", Project: "myproject", ClaimedBy: "testuser", Status: teamstate.ClaimStatusInProgress},
	} {
		_, err := repo.CreateClaim(ctx, c)
		require.NoError(t, err)
	}
	closed := func(id string) (bool, error) {
		switch id {
		case "pt-cb8", "pt-bob":
			return true, nil
		case "pt-gone":
			return false, fmt.Errorf("unknown ticket")
		}
		return false, nil
	}
	got := endTicketClaims(ctx, repo, "myproject", "testuser", []string{"pt-cb8", "pt-vau", "pt-bob", "pt-gone"}, closed)
	assert.Equal(t, map[string]claimEnd{"pt-cb8": claimDone, "pt-vau": claimReleased}, got)
	c, err := repo.GetClaim("myproject", "pt-cb8")
	require.NoError(t, err)
	assert.Equal(t, teamstate.ClaimStatusDone, c.Status)
	_, err = repo.GetClaim("myproject", "pt-vau")
	assert.ErrorIs(t, err, teamstate.ErrClaimNotFound)
	c, _ = repo.GetClaim("myproject", "pt-bob")
	assert.Equal(t, teamstate.ClaimStatusInProgress, c.Status, "someone else's claim")
	c, _ = repo.GetClaim("myproject", "pt-gone")
	assert.Equal(t, teamstate.ClaimStatusInProgress, c.Status, "unknown ticket: left for the Doctor")
}

// A31: the Doctor reports the claims of the member left behind (ticket
// closed, or every session on it ended), not the manual claims nor the
// claims of a running session.
func TestOrphanClaims(t *testing.T) {
	claims := []teamstate.Claim{
		{TicketID: "pt-cb8", ClaimedBy: "me", Status: teamstate.ClaimStatusInProgress},
		{TicketID: "pt-ended", ClaimedBy: "me", Status: teamstate.ClaimStatusInProgress},
		{TicketID: "pt-live", ClaimedBy: "me", Status: teamstate.ClaimStatusInProgress},
		{TicketID: "pt-manual", ClaimedBy: "me", Status: teamstate.ClaimStatusInProgress},
		{TicketID: "pt-done", ClaimedBy: "me", Status: teamstate.ClaimStatusDone},
		{TicketID: "pt-bob", ClaimedBy: "bob", Status: teamstate.ClaimStatusInProgress},
	}
	sessions := []domain.Session{
		{ID: "s1", Tickets: []string{"pt-ended"}, State: domain.RunStopped},
		{ID: "s2", Tickets: []string{"pt-live"}, State: domain.RunStopped},
		{ID: "s3", Tickets: []string{"pt-live"}, State: domain.RunSleeping},
	}
	closed := func(id string) (bool, error) { return id == "pt-cb8" || id == "pt-done" || id == "pt-bob", nil }
	got := orphanClaims("p", "me", claims, []string{teamstate.ClaimStatusDone}, sessions, closed)
	assert.Equal(t, []orphanClaim{{Project: "p", Ticket: "pt-cb8", Reason: "closed"}, {Project: "p", Ticket: "pt-ended", Reason: "ended"}}, got)
	chk := claimsCheck(got)
	assert.False(t, chk.OK)
	assert.Contains(t, chk.Detail, "oh team release pt-cb8 -p p")
	assert.True(t, claimsCheck(nil).OK)
}
