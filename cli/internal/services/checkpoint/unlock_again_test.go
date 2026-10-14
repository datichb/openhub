package checkpoint

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// p5-fixes: a second commit checkpoint (`unlock_again`, incident). The
// window of the earlier one holds one commit: not opened with nothing to
// commit, closed after the first commit, and at the latest when the later
// commit checkpoint is asked; the later one behaves as any commit
// checkpoint.

func againWorkflow() *sessionspec.WorkflowRuntime {
	pause := map[string]string{"manuel": "pause", "semi-auto": "pause", "auto": "pause"}
	return &sessionspec.WorkflowRuntime{ID: "incident", DefaultMode: "semi-auto",
		Checkpoints: []sessionspec.CheckpointDef{
			{ID: "cp-mitigated", Behaviors: pause, Mandatory: true, Unlocks: []string{sessionspec.UnlockCommit}, CommitNext: "cp-postmortem"},
			{ID: "cp-postmortem", Behaviors: pause, Mandatory: true, Unlocks: []string{sessionspec.UnlockCommit}},
		},
	}
}

func newAgainService(t *testing.T) (*Service, context.Context, *fakeHead) {
	t.Helper()
	svc, ctx := newTestService(t)
	writeBundle(t, svc.BundlesDir, "h_again", againWorkflow())
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_l", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_again", Mode: "semi-auto", State: domain.RunActive, LaunchPath: "/work"}))
	h := &fakeHead{head: "c1"}
	svc.Head = h.get
	svc.Work = h.work
	return svc, ctx, h
}

func commitOpen(t *testing.T, svc *Service, ctx context.Context) bool {
	t.Helper()
	rules, err := svc.Rules(ctx, "ses_l")
	require.NoError(t, err)
	for _, r := range rules {
		if r.Action == sessionspec.ActionShell && r.Resource == "git commit*" && r.Effect == sessionspec.EffectDeny {
			return false
		}
	}
	return true
}

func shellDone(t *testing.T, svc *Service, ctx context.Context, id string) bool {
	t.Helper()
	_, _, err := svc.OnCall(ctx, "ses_l", adapters.ToolCall{ID: id, Action: sessionspec.ActionShell, Status: adapters.CallCalled, Input: map[string]any{"command": "git commit -m x"}})
	require.NoError(t, err)
	changed, _, err := svc.OnCall(ctx, "ses_l", adapters.ToolCall{ID: id, Action: sessionspec.ActionShell, Status: adapters.CallOK})
	require.NoError(t, err)
	return changed
}

func TestUnlockAgainNothingToCommit(t *testing.T) {
	svc, ctx, _ := newAgainService(t)
	askCP(t, svc, ctx, "cp-mitigated")
	res, err := reachValidated(t, svc, ctx, "ses_l", "cp-mitigated")
	require.NoError(t, err)
	assert.Empty(t, res.Unlocked, "a clean tree: oh does not open the commit")
	assert.Equal(t, "cp-postmortem", res.CommitNext)
	assert.False(t, commitOpen(t, svc, ctx))
	st, _ := svc.State(ctx, "ses_l")
	assert.Equal(t, windowNothingToCommit, st.Timeline[len(st.Timeline)-2].Message)

	askCP(t, svc, ctx, "cp-postmortem")
	res, err = reachValidated(t, svc, ctx, "ses_l", "cp-postmortem")
	require.NoError(t, err)
	assert.Equal(t, []string{sessionspec.UnlockCommit}, res.Unlocked)
	assert.Empty(t, res.CommitNext)
	assert.True(t, commitOpen(t, svc, ctx))
}

func TestUnlockAgainOneCommit(t *testing.T) {
	svc, ctx, h := newAgainService(t)
	h.dirty = true // the mitigation, in the work tree
	askCP(t, svc, ctx, "cp-mitigated")
	res, err := reachValidated(t, svc, ctx, "ses_l", "cp-mitigated")
	require.NoError(t, err)
	assert.Equal(t, []string{sessionspec.UnlockCommit}, res.Unlocked)
	assert.True(t, commitOpen(t, svc, ctx))

	assert.False(t, shellDone(t, svc, ctx, "s1"), "HEAD did not move")
	h.head = "c2" // committed; something else is still in the tree
	assert.True(t, shellDone(t, svc, ctx, "s2"), "one commit: the window closes even with changes left")
	assert.False(t, commitOpen(t, svc, ctx), "the next commit waits for cp-postmortem")

	askCP(t, svc, ctx, "cp-postmortem")
	_, err = reachValidated(t, svc, ctx, "ses_l", "cp-postmortem")
	require.NoError(t, err)
	assert.True(t, commitOpen(t, svc, ctx))
	h.head = "c3"
	assert.False(t, shellDone(t, svc, ctx, "s3"), "the later window: more to commit, it stays open")
	h.dirty = false
	assert.True(t, shellDone(t, svc, ctx, "s4"))
	assert.False(t, commitOpen(t, svc, ctx))
}

func TestUnlockAgainClosedWhenTheLaterIsAsked(t *testing.T) {
	svc, ctx, h := newAgainService(t)
	h.dirty = true
	askCP(t, svc, ctx, "cp-mitigated")
	_, err := reachValidated(t, svc, ctx, "ses_l", "cp-mitigated")
	require.NoError(t, err)
	require.True(t, commitOpen(t, svc, ctx))
	assert.True(t, askCP(t, svc, ctx, "cp-postmortem"), "the earlier window closes: the rules change")
	assert.False(t, commitOpen(t, svc, ctx), "nothing committed in the earlier window: it waits for cp-postmortem")
}
