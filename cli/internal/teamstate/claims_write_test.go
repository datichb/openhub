package teamstate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

func TestWriteClaimTakesATicketInOneCommit(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := t.Context()
	before := commitCount(t, bare)

	res, err := repo.WriteClaim(ctx, ClaimWrite{
		Project: "T-SRU", TicketID: "SRU-1", Member: "benjamin", Session: "ses_1",
		Create: &Claim{Status: ClaimStatusInProgress},
	})
	require.NoError(t, err)
	assert.True(t, res.Created)
	assert.Equal(t, before+1, commitCount(t, bare), "one commit for the claim and its event")
	c, err := repo.GetClaim(ctx, "T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.Equal(t, "benjamin", c.ClaimedBy)
	assert.Equal(t, "ses_1", c.Session)
	events, err := repo.ListEvents(ctx, "T-SRU", c.ClaimedAt.Add(-1))
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, EventClaimTaken, events[0].Type)
}

// A claim the member holds that is at the initial status moves to work and is
// tied to the session: the two steps are one commit and one push.
func TestWriteClaimMovesAndAdoptsInOneCommit(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := t.Context()
	mustCreate(t, repo, Claim{TicketID: "SRU-1", ClaimedBy: "benjamin", Status: ClaimStatusPlanned, Session: "ses_old"})
	before := commitCount(t, bare)

	res, err := repo.WriteClaim(ctx, ClaimWrite{
		Project: "T-SRU", TicketID: "SRU-1", Member: "benjamin", Session: "ses_new",
		Status: &ClaimStatusMove{To: ClaimStatusInProgress, From: []string{ClaimStatusPlanned}, BestEffort: true},
	})
	require.NoError(t, err)
	assert.False(t, res.Created)
	assert.True(t, res.Moved)
	assert.True(t, res.Adopted)
	assert.Equal(t, ClaimStatusPlanned, res.From)
	assert.Equal(t, before+1, commitCount(t, bare))
	assert.Equal(t, before+1, commitCount(t, repo.Path()), "pushed: the clone and the remote agree")
	c, err := repo.GetClaim(ctx, "T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.Equal(t, ClaimStatusInProgress, c.Status)
	assert.Equal(t, "ses_new", c.Session)

	// Nothing left to change: no commit.
	res, err = repo.WriteClaim(ctx, ClaimWrite{
		Project: "T-SRU", TicketID: "SRU-1", Member: "benjamin", Session: "ses_new",
		Status: &ClaimStatusMove{To: ClaimStatusInProgress, From: []string{ClaimStatusPlanned}},
	})
	require.NoError(t, err)
	assert.False(t, res.Moved || res.Adopted)
	assert.Equal(t, before+1, commitCount(t, bare))
}

func TestWriteClaimLeavesTheClaimOfAnotherMember(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := t.Context()
	mustCreate(t, repo, Claim{TicketID: "SRU-1", ClaimedBy: "alice", Status: ClaimStatusPlanned})
	before := commitCount(t, bare)

	res, err := repo.WriteClaim(ctx, ClaimWrite{
		Project: "T-SRU", TicketID: "SRU-1", Member: "benjamin", Session: "ses_1",
		Create: &Claim{}, Status: &ClaimStatusMove{To: ClaimStatusInProgress},
	})
	require.NoError(t, err)
	require.NotNil(t, res.Held)
	assert.Equal(t, "alice", res.Held.ClaimedBy)
	assert.False(t, res.Created || res.Moved || res.Adopted)
	assert.Equal(t, before, commitCount(t, bare))
}

func TestWriteClaimMissingAndRefusedMoves(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := t.Context()
	_, err := repo.WriteClaim(ctx, ClaimWrite{Project: "T-SRU", TicketID: "NOPE-1", Status: &ClaimStatusMove{To: ClaimStatusDone}})
	require.ErrorIs(t, err, ErrClaimNotFound)
	_, err = repo.WriteClaim(ctx, ClaimWrite{Project: "../x", TicketID: "NOPE-1"})
	require.Error(t, err)

	mustCreate(t, repo, Claim{TicketID: "SRU-1", ClaimedBy: "benjamin", Status: ClaimStatusBlocked})
	before := commitCount(t, bare)
	// blocked → done is not a transition: refused as a whole...
	_, err = repo.WriteClaim(ctx, ClaimWrite{Project: "T-SRU", TicketID: "SRU-1", Member: "benjamin", Session: "ses_1", Status: &ClaimStatusMove{To: ClaimStatusDone}})
	require.ErrorIs(t, err, ErrInvalidTransition)
	c, _ := repo.GetClaim(ctx, "T-SRU", "SRU-1")
	assert.Empty(t, c.Session, "nothing is written when a step fails")
	assert.Equal(t, before, commitCount(t, bare))

	// ...unless the move is best effort: the session is still tied.
	res, err := repo.WriteClaim(ctx, ClaimWrite{Project: "T-SRU", TicketID: "SRU-1", Member: "benjamin", Session: "ses_1", Status: &ClaimStatusMove{To: ClaimStatusDone, BestEffort: true}})
	require.NoError(t, err)
	require.ErrorIs(t, res.StatusErr, ErrInvalidTransition)
	assert.True(t, res.Adopted)
	assert.Equal(t, before+1, commitCount(t, bare))
}

// A verdict: the status (through the review column when the board has one),
// the label and the event are one commit.
func TestWriteClaimVerdictThroughTheReviewColumn(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := t.Context()
	mustCreate(t, repo, Claim{TicketID: "SRU-1", ClaimedBy: "benjamin", Status: ClaimStatusInProgress})
	board := BoardConfig{}
	repo.SetBoardStatuses(board.AllStatuses())
	before := commitCount(t, bare)

	res, err := repo.WriteClaim(ctx, ClaimWrite{
		Project: "T-SRU", TicketID: "SRU-1",
		Status:  &ClaimStatusMove{To: board.TerminalStatuses()[0], Board: &board},
		Labels:  []string{LabelAgentReviewed},
		Events:  []Event{{Actor: "benjamin", Type: EventReviewApproved, Project: "T-SRU", Ticket: "SRU-1"}},
		Message: "verdict: approved T-SRU/SRU-1 → done",
	})
	require.NoError(t, err)
	assert.True(t, res.Moved)
	assert.Equal(t, before+1, commitCount(t, bare))
	c, err := repo.GetClaim(ctx, "T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.Equal(t, ClaimStatusDone, c.Status)
	assert.Contains(t, c.Labels, LabelAgentReviewed)
	assert.Equal(t, "verdict: approved T-SRU/SRU-1 → done", gittest.Run(t, bare, "log", "-1", "--format=%s", "main"))
}

// The routing of a verdict on the board of the team: through the review
// column when the board has one, straight to the terminal column when it has
// none, and a claim already at the target is left as it is.
func TestWriteClaimMoveFollowsTheBoard(t *testing.T) {
	t.Parallel()
	noReview := BoardConfig{Columns: []BoardColumnConfig{
		{ID: "backlog", Name: "Backlog", Role: ColumnRoleInitial},
		{ID: "dev", Name: "Dev", Role: ColumnRoleActive},
		{ID: "shipped", Name: "Shipped", Role: ColumnRoleTerminal},
	}}
	withReview := BoardConfig{Columns: []BoardColumnConfig{
		{ID: "todo", Name: "To do", Role: ColumnRoleInitial},
		{ID: "in_progress", Name: "Doing", Role: ColumnRoleActive},
		{ID: "review", Name: "Review", Role: ColumnRoleReview},
		{ID: "done", Name: "Done", Role: ColumnRoleTerminal},
	}}
	tests := []struct {
		name      string
		board     BoardConfig
		from, to  string
		wantMoved bool
		wantLog   string // subject of the commit, "" when none
	}{
		{"approval goes through review", BoardConfig{}, ClaimStatusInProgress, ClaimStatusDone, true, "status: T-SRU/SRU-1 → review (+1)"},
		{"board with a review column of its own", withReview, ClaimStatusInProgress, ClaimStatusDone, true, "status: T-SRU/SRU-1 → review (+1)"},
		{"board without review column goes straight to the terminal column", noReview, "dev", "shipped", true, "status: T-SRU/SRU-1 → shipped"},
		{"claim already at the target", BoardConfig{}, ClaimStatusDone, ClaimStatusDone, false, ""},
		{"rejection", BoardConfig{}, ClaimStatusReview, ClaimStatusInProgress, true, "status: T-SRU/SRU-1 → in_progress"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, bare := setupGitTestRepo(t)
			mustCreate(t, repo, Claim{TicketID: "SRU-1", ClaimedBy: "benjamin", Status: tt.from})
			repo.SetBoardStatuses(tt.board.AllStatuses())
			before := commitCount(t, bare)
			board := tt.board

			res, err := repo.WriteClaim(t.Context(), ClaimWrite{Project: "T-SRU", TicketID: "SRU-1", Status: &ClaimStatusMove{To: tt.to, Board: &board}})
			require.NoError(t, err)
			assert.Equal(t, tt.wantMoved, res.Moved)
			c, err := repo.GetClaim(t.Context(), "T-SRU", "SRU-1")
			require.NoError(t, err)
			assert.Equal(t, tt.to, c.Status)
			if tt.wantLog == "" {
				assert.Equal(t, before, commitCount(t, bare))
				return
			}
			assert.Equal(t, before+1, commitCount(t, bare), "one commit whatever the number of hops")
			assert.Equal(t, tt.wantLog, gittest.Run(t, bare, "log", "-1", "--format=%s", "main"))
		})
	}
}
