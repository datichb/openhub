package cmd

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// forceCmd is a release/transfer command with the flags the tests set.
func forceCmd(t *testing.T, name string, flags map[string]string, force bool) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: name}
	cmd.Flags().StringP("project", "p", claimsProject, "")
	cmd.Flags().String("to", "", "")
	cmd.Flags().String("from", "", "")
	cmd.Flags().Bool("release", false, "")
	cmd.Flags().Bool("force", false, "")
	for k, v := range flags {
		require.NoError(t, cmd.Flags().Set(k, v))
	}
	if force {
		require.NoError(t, cmd.Flags().Set("force", "true"))
	}
	cmd.SetContext(context.Background())
	return cmd
}

func eventsOn(t *testing.T, repo *teamstate.Repo, project string) []teamstate.Event {
	t.Helper()
	events, err := repo.ListEvents(context.Background(), project, time.Time{})
	require.NoError(t, err)
	return events
}

// The claim of another member is released only after a confirmation that names
// the owner; without a terminal that is --force. The event is signed by the
// member who released it, with the owner it had.
func TestReleaseOfAnotherMembersClaimNeedsForce(t *testing.T) {
	i18ntest.Set(t, "en")
	_, repo, stdout := setupClaimTestApp(t)
	addTeamMembers(t, repo, "alice")
	claimsCreate(t, repo, "ALI-1", "alice", "")
	terminalIs(t, false)

	err := runRelease(forceCmd(t, "release", nil, false), []string{"ALI-1"})
	require.Error(t, err)
	assert.Equal(t, ExitUsage, ExitCode(err))
	assert.Contains(t, err.Error(), "--force")
	assert.Equal(t, "alice", claimsOnDisk(t, repo, "ALI-1").ClaimedBy, "nothing was released")

	require.NoError(t, runRelease(forceCmd(t, "release", nil, true), []string{"ALI-1"}))
	_, err = repo.GetClaim(context.Background(), claimsProject, "ALI-1")
	require.ErrorIs(t, err, teamstate.ErrClaimNotFound)
	events := eventsOn(t, repo, claimsProject)
	require.Len(t, events, 1)
	assert.Equal(t, teamstate.EventClaimReleased, events[0].Type)
	assert.Equal(t, "testuser", events[0].Actor)
	assert.Equal(t, "alice", events[0].Data["owner"])
	assert.Contains(t, stdout.String(), "ALI-1")
}

func TestReleaseOfOwnClaimAsksNothing(t *testing.T) {
	i18ntest.Set(t, "en")
	_, repo, _ := setupClaimTestApp(t)
	claimsCreate(t, repo, "MINE-1", "testuser", "")
	terminalIs(t, false)

	require.NoError(t, runRelease(forceCmd(t, "release", nil, false), []string{"MINE-1"}))
	_, err := repo.GetClaim(context.Background(), claimsProject, "MINE-1")
	assert.ErrorIs(t, err, teamstate.ErrClaimNotFound)
	events := eventsOn(t, repo, claimsProject)
	require.Len(t, events, 1)
	assert.Equal(t, "testuser", events[0].Actor)
}

func TestTransferOfAnotherMembersClaimIsConfirmedAndSignedByTheActor(t *testing.T) {
	i18ntest.Set(t, "en")
	_, repo, _ := setupClaimTestApp(t)
	addTeamMembers(t, repo, "alice", "bob")
	claimsCreate(t, repo, "ALI-1", "alice", "")
	terminalIs(t, false)

	err := runClaimTransfer(forceCmd(t, "transfer", map[string]string{"to": "bob"}, false), []string{"ALI-1"})
	require.Error(t, err)
	assert.Equal(t, ExitUsage, ExitCode(err))
	assert.Equal(t, "alice", claimsOnDisk(t, repo, "ALI-1").ClaimedBy)

	require.NoError(t, runClaimTransfer(forceCmd(t, "transfer", map[string]string{"to": "bob"}, true), []string{"ALI-1"}))
	assert.Equal(t, "bob", claimsOnDisk(t, repo, "ALI-1").ClaimedBy)
	events := eventsOn(t, repo, claimsProject)
	require.Len(t, events, 1)
	assert.Equal(t, "testuser", events[0].Actor, "the actor is the member who acts, not the previous owner")
	assert.Equal(t, "alice", events[0].Data["from"])
	assert.Equal(t, "bob", events[0].Data["to"])

	// a ticket that is done is not handed over
	_, cerr := repo.CreateClaim(context.Background(), teamstate.Claim{TicketID: "DONE-1", Project: claimsProject, ClaimedBy: "bob", Status: teamstate.ClaimStatusDone})
	require.NoError(t, cerr)
	err = runClaimTransfer(forceCmd(t, "transfer", map[string]string{"to": "alice"}, true), []string{"DONE-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "finished")
}

func TestTransferFlagsAreChecked(t *testing.T) {
	i18ntest.Set(t, "en")
	setupClaimTestApp(t)
	for name, tc := range map[string]struct {
		flags map[string]string
		args  []string
		want  string
	}{
		"nothing":             {nil, nil, "Give a ticket"},
		"from alone":          {map[string]string{"from": "alice"}, nil, "--to <member> or --release"},
		"to and release":      {map[string]string{"from": "alice", "to": "bob", "release": "true"}, nil, "not both"},
		"ticket with from":    {map[string]string{"from": "alice", "to": "bob"}, []string{"T-1"}, "do not give a ticket"},
		"ticket with release": {map[string]string{"release": "true"}, []string{"T-1"}, "oh team release T-1"},
		"ticket without to":   {nil, []string{"T-1"}, "--to"},
	} {
		t.Run(name, func(t *testing.T) {
			err := runClaimTransfer(forceCmd(t, "transfer", tc.flags, true), tc.args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			assert.Equal(t, ExitUsage, ExitCode(err))
		})
	}
}

// oh team claim transfer --from alice: all the open claims move in one commit,
// the finished ones stay, and each leaves an event signed by the actor.
func TestTransferAllClaimsOfAMember(t *testing.T) {
	i18ntest.Set(t, "en")
	_, repo, stdout := setupClaimTestApp(t)
	addTeamMembers(t, repo, "alice", "bob")
	claimsCreate(t, repo, "A-1", "alice", "")
	claimsCreate(t, repo, "A-2", "alice", "")
	_, err := repo.CreateClaim(context.Background(), teamstate.Claim{TicketID: "A-3", Project: claimsProject, ClaimedBy: "alice", Status: teamstate.ClaimStatusDone})
	require.NoError(t, err)
	terminalIs(t, false)

	err = runClaimTransfer(forceCmd(t, "transfer", map[string]string{"from": "alice", "to": "bob"}, false), nil)
	require.Error(t, err, "a batch asks for a confirmation too")
	assert.Equal(t, "alice", claimsOnDisk(t, repo, "A-1").ClaimedBy)

	require.NoError(t, runClaimTransfer(forceCmd(t, "transfer", map[string]string{"from": "alice", "to": "bob"}, true), nil))
	assert.Equal(t, "bob", claimsOnDisk(t, repo, "A-1").ClaimedBy)
	assert.Equal(t, "bob", claimsOnDisk(t, repo, "A-2").ClaimedBy)
	assert.Equal(t, "alice", claimsOnDisk(t, repo, "A-3").ClaimedBy, "a finished claim stays")
	assert.Contains(t, stdout.String(), "2 claim(s) of alice handed to bob")
	events := eventsOn(t, repo, claimsProject)
	require.Len(t, events, 2)
	for _, e := range events {
		assert.Equal(t, teamstate.EventClaimTransferred, e.Type)
		assert.Equal(t, "testuser", e.Actor)
	}

	stdout.Reset()
	require.NoError(t, runClaimTransfer(forceCmd(t, "transfer", map[string]string{"from": "alice", "release": "true"}, true), nil))
	assert.Contains(t, stdout.String(), "alice has no open claim")
}

func TestReleaseAllClaimsOfAMember(t *testing.T) {
	i18ntest.Set(t, "en")
	_, repo, _ := setupClaimTestApp(t)
	addTeamMembers(t, repo, "alice")
	claimsCreate(t, repo, "A-1", "alice", "")
	claimsCreate(t, repo, "A-2", "alice", "")
	terminalIs(t, false)

	require.NoError(t, runClaimTransfer(forceCmd(t, "transfer", map[string]string{"from": "alice", "release": "true"}, true), nil))
	left, err := repo.ListClaims(context.Background(), claimsProject)
	require.NoError(t, err)
	assert.Empty(t, left)
	assert.True(t, repo.ClaimRemovedOnPurpose(context.Background(), claimsProject, "A-1"))
}

// Taking over an expired claim leaves a claim.transferred event signed by the
// member who takes over, in the commit of the takeover brief.
func TestTakeOverOfAnExpiredClaimLeavesAnEvent(t *testing.T) {
	i18ntest.Set(t, "en")
	_, repo, _ := setupClaimTestApp(t)
	addTeamMembers(t, repo, "alice")
	expiredClaimOf(t, repo, "EXP-1", "alice")
	terminalIs(t, false)

	require.NoError(t, runClaim(newClaimYesCmd(t, claimsProject, true), []string{"EXP-1"}))
	events := eventsOn(t, repo, claimsProject)
	var transferred []teamstate.Event
	for _, e := range events {
		if e.Type == teamstate.EventClaimTransferred {
			transferred = append(transferred, e)
		}
	}
	require.Len(t, transferred, 1)
	assert.Equal(t, "testuser", transferred[0].Actor)
	assert.Equal(t, "expired", transferred[0].Data["reason"])
	assert.Equal(t, "alice", transferred[0].Data["from"])
}

// oh team claim leaves the claim.taken event in the commit of the claim.
func TestClaimEventIsInTheCommitOfTheClaim(t *testing.T) {
	i18ntest.Set(t, "en")
	_, repo, _ := setupClaimTestApp(t)
	before := gitCommitCount(t, repo.Path())

	require.NoError(t, runClaim(newClaimCmd(t, claimsProject), []string{"ONE-1"}))
	assert.Equal(t, before+1, gitCommitCount(t, repo.Path()), "the claim and its event are one commit")
	events := eventsOn(t, repo, claimsProject)
	require.Len(t, events, 1)
	assert.Equal(t, teamstate.EventClaimTaken, events[0].Type)
	assert.Equal(t, "testuser", events[0].Actor)
}

func gitCommitCount(t *testing.T, dir string) int {
	t.Helper()
	n, err := strconv.Atoi(strings.TrimSpace(gittest.Run(t, dir, "rev-list", "--count", "HEAD")))
	require.NoError(t, err)
	return n
}
