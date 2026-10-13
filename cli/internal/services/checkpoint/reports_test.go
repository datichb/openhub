package checkpoint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
)

// L4: each stored report is a timeline step; the reports the agent cites
// in workflow_checkpoint ride on the decision.
func TestReportsInTheWorkflowState(t *testing.T) {
	svc, ctx := newTestService(t)
	require.NoError(t, svc.Reported(ctx, "ses_a", "rp-1", "developer"))
	require.NoError(t, svc.Reported(ctx, "ses_a", "rp-2", ""))
	st, err := svc.State(ctx, "ses_a")
	require.NoError(t, err)
	require.Len(t, st.Timeline, 2)
	assert.Equal(t, domain.CheckpointEvent{At: st.Timeline[0].At, Kind: domain.CheckpointReport, ID: "rp-1", By: "developer"}, st.Timeline[0])
	text := TimelineText(Timeline(&TestWorkflow, "semi-auto", "fr", st), 0)
	assert.Contains(t, text, "▤ rp-1 (developer) → ▤ rp-2 → ○ cp-1")

	d, auto, _, err := svc.Asked(ctx, "ses_a", "ses_a", adapters.PendingDecision{ID: "per_2", Call: &adapters.ToolCall{Input: map[string]any{
		"id": "cp-2", "summary": "prêt", "reports": []any{"rp-1", " rp-2 ", 3}}}})
	require.NoError(t, err)
	require.False(t, auto)
	assert.Equal(t, []string{"rp-1", "rp-2"}, d.Payload.Data[domain.DataReports])

	d, _, _, err = svc.Asked(ctx, "ses_a", "ses_a", adapters.PendingDecision{ID: "per_3", Call: &adapters.ToolCall{Input: map[string]any{"id": "cp-2"}}})
	require.NoError(t, err)
	_, cited := d.Payload.Data[domain.DataReports]
	assert.False(t, cited)

	assert.Equal(t, []string{"rp-1", "rp-3"}, citedReports("rp-1, rp-3"))
	assert.Nil(t, citedReports(nil))
}
