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
	status   string
	noReview bool
	calls    []string
}

func (f *fakeVerdictStore) GetClaim(_ context.Context, project, ticketID string) (*teamstate.Claim, error) {
	return &teamstate.Claim{Project: project, TicketID: ticketID, Status: f.status}, nil
}

func (f *fakeVerdictStore) UpdateClaimStatus(_ context.Context, _, _, newStatus string) error {
	f.calls = append(f.calls, "status:"+newStatus)
	if f.noReview && newStatus == teamstate.ClaimStatusReview {
		return teamstate.ErrInvalidStatus
	}
	return nil
}

func (f *fakeVerdictStore) UpdateClaimStatusFromTracker(_ context.Context, _, _, newStatus string) error {
	f.calls = append(f.calls, "tracker:"+newStatus)
	return nil
}

func TestApplyVerdictStatus_FakeStore(t *testing.T) {
	tests := []struct {
		name  string
		fake  fakeVerdictStore
		to    string
		calls []string
	}{
		{"approval goes through review", fakeVerdictStore{status: teamstate.ClaimStatusInProgress}, teamstate.ClaimStatusDone,
			[]string{"status:review", "status:done"}},
		{"board without review column", fakeVerdictStore{status: teamstate.ClaimStatusInProgress, noReview: true}, teamstate.ClaimStatusDone,
			[]string{"status:review", "tracker:done"}},
		{"claim already at the target", fakeVerdictStore{status: teamstate.ClaimStatusDone}, teamstate.ClaimStatusDone, nil},
		{"rejection", fakeVerdictStore{status: teamstate.ClaimStatusReview}, teamstate.ClaimStatusInProgress,
			[]string{"status:in_progress"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, applyVerdictStatus(t.Context(), &tt.fake, "p", "t-1", tt.to))
			assert.Equal(t, tt.calls, tt.fake.calls)
		})
	}
}
