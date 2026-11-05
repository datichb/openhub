package team

import (
	"context"
	"testing"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeVerdictStore fakes the three methods applyVerdictStatus calls; the
// others come from the nil interfaces it embeds (and panic if called).
type fakeVerdictStore struct {
	verdictStore
	status string
	calls  []string
}

func (f *fakeVerdictStore) GetClaim(_ context.Context, project, ticketID string) (*teamstate.Claim, error) {
	return &teamstate.Claim{Project: project, TicketID: ticketID, Status: f.status}, nil
}

func (f *fakeVerdictStore) UpdateClaimStatus(_ context.Context, _, _, newStatus string) error {
	f.calls = append(f.calls, "status:"+newStatus)
	return nil
}

func (f *fakeVerdictStore) UpdateClaimStatusFromTracker(_ context.Context, _, _, newStatus string) error {
	f.calls = append(f.calls, "tracker:"+newStatus)
	return nil
}

func TestApplyVerdictStatus_FakeStore(t *testing.T) {
	noReview := teamstate.BoardConfig{Columns: []teamstate.BoardColumnConfig{
		{ID: "backlog", Name: "Backlog", Role: teamstate.ColumnRoleInitial},
		{ID: "dev", Name: "Dev", Role: teamstate.ColumnRoleActive},
		{ID: "shipped", Name: "Shipped", Role: teamstate.ColumnRoleTerminal},
	}}
	withReview := teamstate.BoardConfig{Columns: []teamstate.BoardColumnConfig{
		{ID: "todo", Name: "To do", Role: teamstate.ColumnRoleInitial},
		{ID: "in_progress", Name: "Doing", Role: teamstate.ColumnRoleActive},
		{ID: "review", Name: "Review", Role: teamstate.ColumnRoleReview},
		{ID: "done", Name: "Done", Role: teamstate.ColumnRoleTerminal},
	}}
	tests := []struct {
		name  string
		fake  fakeVerdictStore
		board teamstate.BoardConfig
		to    string
		calls []string
	}{
		{"approval goes through review", fakeVerdictStore{status: teamstate.ClaimStatusInProgress}, teamstate.BoardConfig{}, teamstate.ClaimStatusDone,
			[]string{"status:review", "status:done"}},
		{"board with a review column of its own", fakeVerdictStore{status: teamstate.ClaimStatusInProgress}, withReview, teamstate.ClaimStatusDone,
			[]string{"status:review", "status:done"}},
		{"board without review column: custom statuses go straight to the terminal column", fakeVerdictStore{status: "dev"}, noReview, "shipped",
			[]string{"status:shipped"}},
		{"claim already at the target", fakeVerdictStore{status: teamstate.ClaimStatusDone}, teamstate.BoardConfig{}, teamstate.ClaimStatusDone, nil},
		{"rejection", fakeVerdictStore{status: teamstate.ClaimStatusReview}, teamstate.BoardConfig{}, teamstate.ClaimStatusInProgress,
			[]string{"status:in_progress"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, applyVerdictStatus(t.Context(), &tt.fake, tt.board, "p", "t-1", tt.to))
			assert.Equal(t, tt.calls, tt.fake.calls)
		})
	}
}

// The verdict leads to the statuses of the board of the team.
func TestVerdictTargetFollowsTheBoard(t *testing.T) {
	assert.Equal(t, "done", verdictTarget(teamstate.BoardConfig{}, true))
	assert.Equal(t, "in_progress", verdictTarget(teamstate.BoardConfig{}, false))
	board := teamstate.BoardConfig{Columns: []teamstate.BoardColumnConfig{
		{ID: "backlog", Name: "Backlog", Role: teamstate.ColumnRoleInitial},
		{ID: "dev", Name: "Dev", Role: teamstate.ColumnRoleActive},
		{ID: "shipped", Name: "Shipped", Role: teamstate.ColumnRoleTerminal},
	}}
	assert.Equal(t, "shipped", verdictTarget(board, true))
	assert.Equal(t, "dev", verdictTarget(board, false))
}
