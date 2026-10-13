package checkpoint

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// LD-J: when a checkpoint that unlocks the commit is passed, oh applies the
// wiki proposals accepted on the card before the call returns (they are
// committed with the code); the agent is told, the timeline records it.
func TestCommitPassedAppliesTheWiki(t *testing.T) {
	svc, ctx := newTestService(t)
	wf := sessionspec.WorkflowRuntime{ID: "quick", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-1", Behaviors: map[string]string{"semi-auto": "auto"}},
		{ID: "cp-commit", Behaviors: map[string]string{"semi-auto": "auto"}, Unlocks: []string{sessionspec.UnlockCommit}},
		{ID: "cp-sync", Behaviors: map[string]string{"semi-auto": "auto"}, Unlocks: []string{sessionspec.UnlockCommit}, Wiki: sessionspec.WikiSync},
	}}
	writeBundle(t, svc.BundlesDir, "h_commit", &wf)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_c", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_commit", Mode: "semi-auto", State: domain.RunActive, LaunchPath: "/work"}))
	svc.Head = func(context.Context, string) (string, bool, error) { return "abc", false, nil }
	var calls []string
	svc.CommitPassed = func(_ context.Context, sess *domain.Session, cp string, sync bool) (string, []string) {
		calls = append(calls, fmt.Sprintf("%s/%s/%s/%v", sess.ID, cp, sess.LaunchPath, sync))
		return "Wiki : wp-1 appliquée", []string{"wp-1"}
	}

	res, err := svc.Reached(ctx, "ses_c", Call{ID: "cp-1"})
	require.NoError(t, err)
	assert.Empty(t, res.Wiki)
	assert.Empty(t, calls, "cp-1 unlocks no commit")

	res, err = svc.Reached(ctx, "ses_c", Call{ID: "cp-commit"})
	require.NoError(t, err)
	assert.Equal(t, "Wiki : wp-1 appliquée", res.Wiki)
	assert.Equal(t, []string{"ses_c/cp-commit//work/false"}, calls)

	// LD-D: a checkpoint `wiki: sync` (doc-sync) says so.
	_, err = svc.Reached(ctx, "ses_c", Call{ID: "cp-sync"})
	require.NoError(t, err)
	assert.Equal(t, "ses_c/cp-sync//work/true", calls[1])

	st, err := svc.State(ctx, "ses_c")
	require.NoError(t, err)
	steps := Timeline(&wf, "semi-auto", i18n.Locale(), st)
	assert.Contains(t, TimelineText(steps, 0), "✎ wp-1")
}
