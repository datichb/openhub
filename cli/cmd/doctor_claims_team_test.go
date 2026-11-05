package cmd

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

func TestSyncFinding(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	for name, tc := range map[string]struct {
		last     time.Time
		auto     bool
		interval time.Duration
		want     string
	}{
		"never":                     {time.Time{}, true, 0, "never"},
		"auto, recent":              {now.Add(-10 * time.Minute), true, 5 * time.Minute, ""},
		"auto, three intervals":     {now.Add(-16 * time.Minute), true, 5 * time.Minute, "old"},
		"auto, default interval":    {now.Add(-20 * time.Minute), true, 0, "old"},
		"manual, a week":            {now.Add(-7 * 24 * time.Hour), false, 0, ""},
		"manual, more than 2 weeks": {now.Add(-15 * 24 * time.Hour), false, 0, "old"},
	} {
		t.Run(name, func(t *testing.T) {
			got, _ := syncFinding(tc.last, tc.auto, tc.interval, now)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestClaimsOutsideTheBoard(t *testing.T) {
	claims := []teamstate.Claim{
		{TicketID: "1", Status: teamstate.ClaimStatusInProgress},
		{TicketID: "2", Status: "weird"},
		{TicketID: "3", Status: teamstate.ClaimStatusPlanned},
	}
	got := claimsOutsideBoard(claims, teamstate.BoardConfig{})
	require.Len(t, got, 1)
	assert.Equal(t, "2", got[0].TicketID)
}

func TestDuplicateClaims(t *testing.T) {
	trackerOf := func(project string) string {
		switch project {
		case "api", "api-2":
			return "gitlab:acme/api"
		case "web":
			return "gitlab:acme/web"
		}
		return ""
	}
	claims := []teamstate.Claim{
		{Project: "api", TicketID: "SRU-42", ExternalIID: 42},
		{Project: "api", TicketID: "42", ExternalIID: 42},
		{Project: "api-2", TicketID: "7", ExternalIID: 7},
		{Project: "api", TicketID: "7", ExternalIID: 7},
		{Project: "web", TicketID: "7", ExternalIID: 7},
		{Project: "other", TicketID: "9", ExternalIID: 9},
		{Project: "elsewhere", TicketID: "9", ExternalIID: 9},
		{Project: "api", TicketID: "no-iid"},
		{Project: "api", TicketID: "no-iid-2"},
	}
	got := duplicateClaims(claims, trackerOf)
	require.Len(t, got, 2)
	assert.Equal(t, claimDuplicate{IID: 7, Claims: []string{"api-2/7", "api/7"}}, got[0],
		"the same issue in two projects of one tracker project; web is another tracker project; unknown trackers do not mix")
	assert.Equal(t, claimDuplicate{IID: 42, Claims: []string{"api/42", "api/SRU-42"}}, got[1])
}

func TestGoneMemberClaims(t *testing.T) {
	claims := []teamstate.Claim{
		{TicketID: "1", ClaimedBy: "ghost", Status: teamstate.ClaimStatusInProgress},
		{TicketID: "2", ClaimedBy: "ghost", Status: teamstate.ClaimStatusReview},
		{TicketID: "3", ClaimedBy: "ghost", Status: teamstate.ClaimStatusDone},
		{TicketID: "4", ClaimedBy: "ana", Status: teamstate.ClaimStatusInProgress},
		{TicketID: "5", ClaimedBy: "", Status: teamstate.ClaimStatusPlanned},
	}
	assert.Equal(t, map[string]int{"ghost": 2}, goneMemberClaims(claims, map[string]bool{"ana": true}, nil),
		"a finished claim is left to the cleanup, a pool claim has no owner")
}

// The doctor looks at the whole team-state: a claim outside the board, two
// claims for one issue, a claim of a member without a card and the card of the
// member removed are each reported with the command that settles it.
func TestTeamLifecycleDoctorReportsWhatIsWrong(t *testing.T) {
	i18ntest.Set(t, "en")
	configtest.Home(t)
	a, repo, _ := setupClaimTestApp(t)
	ctx := context.Background()
	addTeamMembers(t, repo, "ana")
	for _, c := range []teamstate.Claim{
		{TicketID: "1", Project: "api", ClaimedBy: "ana", Status: "weird"},
		{TicketID: "SRU-42", Project: "api", ClaimedBy: "ana", Status: teamstate.ClaimStatusInProgress, ExternalIID: 42},
		{TicketID: "42", Project: "api", ClaimedBy: "ana", Status: teamstate.ClaimStatusInProgress, ExternalIID: 42},
		{TicketID: "G-1", Project: "api", ClaimedBy: "ghost", Status: teamstate.ClaimStatusInProgress},
	} {
		_, err := repo.CreateClaim(ctx, c)
		require.NoError(t, err)
	}
	require.NoError(t, repo.RemoveMember(ctx, "testuser")) // the member of hub.toml has no card any more

	byID := map[string]string{}
	for _, r := range teamLifecycleResults(ctx, a, time.Now()) {
		assert.True(t, r.OK && r.Warn, "%s is a warning, never a failure", r.ID)
		byID[r.ID] = r.Detail
	}
	assert.Contains(t, byID[doctorIDClaimsBoardStatus], "api/1 (weird)")
	assert.Contains(t, byID[doctorIDClaimsDuplicates], "#42 : api/42 = api/SRU-42")
	assert.Contains(t, byID[doctorIDClaimsGone], "ghost (1)")
	assert.Contains(t, byID[doctorIDClaimsGone], "oh team claim transfer --from ghost --release --team test-team")
	assert.Contains(t, byID[doctorIDMemberRemoved], "your card (testuser) is no longer in members.toml")
	assert.Contains(t, byID[doctorIDMemberRemoved], "oh teams leave test-team")
	assert.NotContains(t, byID, doctorIDClaimsSync, "no tracker: no sync to report")
	assert.NotContains(t, byID, doctorIDClaimsDaemon)

	// a healthy team says nothing
	_, err := repo.ReleaseClaimAs(ctx, "api", "1", teamstate.ReleaseOpts{})
	require.NoError(t, err)
	var ids []string
	for _, r := range teamLifecycleResults(ctx, a, time.Now()) {
		ids = append(ids, r.ID)
	}
	assert.NotContains(t, strings.Join(ids, " "), doctorIDClaimsBoardStatus)
}

// Claims of an open session need the heartbeat of the daemon: without it the
// doctor says they will run out.
func TestTeamLifecycleDoctorWarnsOfAStoppedDaemon(t *testing.T) {
	i18ntest.Set(t, "en")
	configtest.Home(t)
	a, repo := setupBeat(t, openSession("ses_1", "T-1"))
	require.NoError(t, config.Save(a.Config)) // the daemon reads hub.toml
	claimsCreate(t, repo, "T-1", "testuser", "ses_1")
	claimsCreate(t, repo, "T-2", "testuser", "") // no session: nobody renews it anyway

	var detail string
	for _, r := range teamLifecycleResults(context.Background(), a, time.Now()) {
		if r.ID == doctorIDClaimsDaemon {
			detail = r.Detail
		}
	}
	assert.Contains(t, detail, "The daemon is stopped and 1 claim(s) of 1 open session(s)")
}
