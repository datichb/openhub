package checkpoint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// R5: an output declared with `record:` is filled by oh from what the
// session did, never by an agent.
func TestRecordOutputs(t *testing.T) {
	svc, ctx := newTestService(t)
	wf := TestWorkflow
	wf.Outputs = []sessionspec.OutputDef{
		{ID: "tickets", Type: "beads-ids", Record: RecordCreated},
		{ID: "done", Type: "beads-ids", Record: RecordClosed},
		{ID: "spec", Type: "path", Record: RecordSpec},
		{ID: "report", Type: "report", Record: RecordReport},
		{ID: "branch", Type: "branch"},
	}
	writeBundle(t, svc.BundlesDir, "h_rec", &wf)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_r", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", BundleHash: "h_rec", Mode: "semi-auto", State: domain.RunActive}))

	changed, err := svc.Record(ctx, "ses_r", RecordCreated, "ev-1", " ev-2 ")
	require.NoError(t, err)
	assert.True(t, changed)
	changed, err = svc.Record(ctx, "ses_r", RecordCreated, "ev-2", "ev-3")
	require.NoError(t, err)
	assert.True(t, changed)
	changed, _ = svc.Record(ctx, "ses_r", RecordCreated, "ev-3")
	assert.False(t, changed, "already recorded")
	_, err = svc.Record(ctx, "ses_r", RecordClosed, "ev-1")
	require.NoError(t, err)
	_, err = svc.Record(ctx, "ses_r", RecordSpec, "docs/wiki/specs/a.md")
	require.NoError(t, err)
	_, err = svc.Record(ctx, "ses_r", RecordReport, "rp-1")
	require.NoError(t, err)
	_, err = svc.Record(ctx, "ses_r", RecordReport, "rp-2")
	require.NoError(t, err)

	sess, err := svc.Sessions.Get(ctx, "ses_r")
	require.NoError(t, err)
	assert.Equal(t, []any{"ev-1", "ev-2", "ev-3"}, sess.Outputs["tickets"])
	assert.Equal(t, []any{"ev-1"}, sess.Outputs["done"])
	assert.Equal(t, "docs/wiki/specs/a.md", sess.Outputs["spec"])
	assert.Equal(t, "rp-2", sess.Outputs["report"], "the last report")

	// The agents cannot declare a recorded output; the others still.
	assert.ErrorContains(t, svc.Declare(ctx, "ses_r", Output{ID: "tickets", Type: "beads-ids", Value: "ev-9"}), "recorded by oh")
	require.NoError(t, svc.Declare(ctx, "ses_r", Output{Type: "branch", Value: "feat/x"}))

	// Nothing to record: no output of that kind, no workflow, no value.
	changed, err = svc.Record(ctx, "ses_a", RecordCreated, "ev-1")
	require.NoError(t, err)
	assert.False(t, changed)
	changed, err = svc.Record(ctx, "ses_plain", RecordCreated, "ev-1")
	require.NoError(t, err)
	assert.False(t, changed)
	changed, _ = svc.Record(ctx, "ses_r", RecordCreated, " ")
	assert.False(t, changed)
}

// R5: workflow_status lists as expected only the outputs the agents declare.
func TestStatusLeavesRecordedOutputsOut(t *testing.T) {
	svc, ctx := newTestService(t)
	wf := TestWorkflow
	wf.Outputs = []sessionspec.OutputDef{{ID: "tickets", Type: "beads-ids", Record: RecordCreated}, {ID: "branch", Type: "branch"}}
	writeBundle(t, svc.BundlesDir, "h_rec", &wf)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_r", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", BundleHash: "h_rec", Mode: "semi-auto", State: domain.RunActive}))
	st, err := svc.Status(ctx, "ses_r")
	require.NoError(t, err)
	assert.Equal(t, []sessionspec.OutputDef{{ID: "branch", Type: "branch"}}, st.ExpectedOutputs)
}
