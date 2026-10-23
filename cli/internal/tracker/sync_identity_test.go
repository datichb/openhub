package tracker

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// assignedTracker answers the issues of the project with their assignees.
type assignedTracker struct {
	stubTracker
	listed []IssueState
}

func (a *assignedTracker) ListIssuesByLabels(context.Context, string, ListByLabelsOpts) ([]IssueState, error) {
	return a.listed, nil
}

// syncOf runs one sync of a team whose tracker has an issue assigned to the
// tracker name "benjamin.datiche" (nobody in the team), with the member alice
// described by alice, and returns the warnings of the run.
func syncOf(t *testing.T, alice teamstate.Member, id *Identity) []SyncWarning {
	t.Helper()
	if testing.Short() {
		t.Skip("git")
	}
	i18ntest.Set(t, "en")
	ctx := context.Background()
	bare, clone := gittest.BareClone(t)
	repo := teamstate.NewRepo(bare, clone)
	alice.ID = "alice"
	require.NoError(t, repo.AddMember(ctx, alice))
	require.NoError(t, repo.AddMember(ctx, teamstate.Member{ID: "bob", GitLabUsername: "bob"}))
	_, err := repo.CreateClaim(ctx, teamstate.Claim{TicketID: "1", Project: "p", ClaimedBy: "alice", ExternalIID: 1})
	require.NoError(t, err)

	tr := &assignedTracker{
		stubTracker: stubTracker{issues: map[int]IssueState{
			1: {IID: 1, State: "open", StatusName: "opened", Title: "mine", Assignees: []string{"benjamin.datiche"}},
		}},
		listed: []IssueState{
			{IID: 2, State: "open", StatusName: "opened", Title: "pool", Assignees: []string{"bob"}},
			{IID: 3, State: "open", StatusName: "opened", Title: "mine too", Assignees: []string{"Benjamin.Datiche"}},
		},
	}
	e := NewEngine(tr, repo, teamstate.TrackerConfig{ //nolint:staticcheck // the engine still reads Projects
		Projects: map[string]string{"p": "grp/p"}, AutoPlanUnassigned: true, UnassignedLabels: []string{"todo"}, MaxUnassignedIssues: 10,
	}, t.TempDir())
	if id != nil {
		e.SetIdentity(*id)
	}
	res, err := e.Run(ctx)
	require.NoError(t, err)
	return res.Warnings
}

// Bug 12: the tracker is on another server, the member has no tracker_username,
// the issues assigned to "benjamin.datiche" belong to nobody and nothing said so.
func TestSyncSaysWhenIssuesAreAssignedToANameNobodyHas(t *testing.T) {
	warnings := syncOf(t, teamstate.Member{GitLabUsername: "benjamin.datiche1"}, &Identity{TeamID: "core", MemberID: "alice", CrossServer: true})
	require.Len(t, warnings, 1, "one warning for the run")
	msg := warnings[0].Message
	assert.Contains(t, msg, "benjamin.datiche", "the unknown name")
	assert.Contains(t, msg, "tracker_username")
	assert.Contains(t, msg, "oh team profile --team core --tracker-username", "and the command")
	assert.NotContains(t, msg, "bob", "bob is a member")
	assert.Equal(t, 1, strings.Count(msg, "benjamin.datiche"), "a name is listed once, whatever its case")
}

func TestSyncSaysNothingWhenThereIsNothingToSay(t *testing.T) {
	t.Run("the member has a tracker_username", func(t *testing.T) {
		assert.Empty(t, syncOf(t, teamstate.Member{GitLabUsername: "benjamin.datiche1", TrackerUsername: "benjamin.datiche2"},
			&Identity{TeamID: "core", MemberID: "alice", CrossServer: true}))
	})
	t.Run("same server: the GitLab name is the tracker name", func(t *testing.T) {
		assert.Empty(t, syncOf(t, teamstate.Member{GitLabUsername: "benjamin.datiche1"},
			&Identity{TeamID: "core", MemberID: "alice", CrossServer: false}))
	})
	t.Run("same server but no name at all", func(t *testing.T) {
		assert.Len(t, syncOf(t, teamstate.Member{}, &Identity{TeamID: "core", MemberID: "alice"}), 1)
	})
	t.Run("the engine does not know who runs it", func(t *testing.T) {
		assert.Empty(t, syncOf(t, teamstate.Member{GitLabUsername: "benjamin.datiche1"}, nil))
	})
}
