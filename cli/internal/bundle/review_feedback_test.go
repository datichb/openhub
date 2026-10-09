package bundle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// N8 (DC12): review-feedback fixes without committing, has the fixes
// reviewed, commits after cp-2 only, and leaves the push and the MR replies
// to the user.
func TestReviewFeedbackWorkflow(t *testing.T) {
	hub, cat, env := shippedWorkflows(t)
	r, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerHub, ID: "review-feedback"}, nil, env)
	require.False(t, diags.HasErrors(), "%v", diags)
	b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"developer", "reviewer"}, b.Spec.SubagentGraph["orchestrator-dev"])
	wf := b.Spec.Workflow
	assert.Equal(t, []string{sessionspec.UnlockCommit}, LockedOps(wf, "semi-auto", nil), "commit locked until cp-2, no push")
	assert.Subset(t, LockedAgents(wf, "semi-auto", nil), []string{"developer", "reviewer"})

	data, err := os.ReadFile(filepath.Join(hub, "skills", "orchestrator", "orchestrator-dev-feedback-mode.md"))
	require.NoError(t, err)
	text := string(data)
	for _, banned := range []string{"Faire un commit", "gitlab_reply_to_mr_discussion", "team_review_verdict", "team_notify", "Ticket : <ID>", "npm ", "lancer le pre-review"} {
		assert.NotContains(t, text, banned)
	}
	for _, want := range []string{"workflow_checkpoint({ id: \"cp-fix\"", "[MODE:feedback]", "workflow_checkpoint({ id: \"cp-2\"", "Ne pas commiter", "git push"} {
		assert.Contains(t, text, want)
	}
}
