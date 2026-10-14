package checkpoint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// E9 (R8): cp-release unlocks commit, tag and mr; the tag waits for the
// release commit, and the tag and mr windows close at the next mandatory
// checkpoint (CP-R8) or when cp-release is asked again.
func TestReleaseWindows(t *testing.T) {
	svc, ctx := newTestService(t)
	wf := &sessionspec.WorkflowRuntime{ID: "release", DefaultMode: "manuel",
		Checkpoints: []sessionspec.CheckpointDef{
			{ID: "cp-release", Label: map[string]string{"": "Release"}, Behaviors: map[string]string{"manuel": "pause"}, Mandatory: true,
				Unlocks: []string{sessionspec.UnlockCommit, sessionspec.UnlockTag, sessionspec.UnlockMR}},
			{ID: "cp-after", Label: map[string]string{"": "Après"}, Behaviors: map[string]string{"manuel": "pause"}, Mandatory: true},
		}}
	writeBundle(t, svc.BundlesDir, "h_release", wf)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_r", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_release", Mode: "manuel", State: domain.RunActive, LaunchPath: "/work"}))
	h := &fakeHead{head: "c1"}
	svc.Head = h.get

	tag := sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: "git tag*", Effect: sessionspec.EffectDeny}
	mr := sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: "glab mr create*", Effect: sessionspec.EffectDeny}
	commit := sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: "git commit*", Effect: sessionspec.EffectDeny}
	rules := func() []sessionspec.PermissionRule {
		r, err := svc.Rules(ctx, "ses_r")
		require.NoError(t, err)
		return r
	}
	assert.Subset(t, rules(), []sessionspec.PermissionRule{commit, tag, mr}, "before cp-release")

	_, err := reachValidated(t, svc, ctx, "ses_r", "cp-release")
	require.NoError(t, err)
	assert.NotContains(t, rules(), commit)
	assert.NotContains(t, rules(), mr)
	assert.Contains(t, rules(), tag, "the tag waits for the release commit")

	h.head = "c2"
	_, _, err = svc.OnCall(ctx, "ses_r", adapters.ToolCall{ID: "s1", Action: sessionspec.ActionShell, Status: adapters.CallCalled, Input: map[string]any{"command": "git commit -m 'chore(release): v1.3.0'"}})
	require.NoError(t, err)
	changed, _, err := svc.OnCall(ctx, "ses_r", adapters.ToolCall{ID: "s1", Action: sessionspec.ActionShell, Status: adapters.CallOK})
	require.NoError(t, err)
	assert.True(t, changed, "committed: the rules change")
	assert.NotContains(t, rules(), tag, "committed: the tag opens")
	assert.Contains(t, rules(), commit)

	// Asked again on the release commit (nothing new to commit): the tag
	// does not wait for a commit that will never come.
	_, err = reachValidated(t, svc, ctx, "ses_r", "cp-release")
	require.NoError(t, err)
	assert.NotContains(t, rules(), tag, "passed again on the release commit: the tag stays open")
	assert.NotContains(t, rules(), commit)

	_, err = reachValidated(t, svc, ctx, "ses_r", "cp-after")
	require.NoError(t, err)
	assert.Subset(t, rules(), []sessionspec.PermissionRule{tag, mr}, "the next mandatory checkpoint closes them")
	st, _ := svc.State(ctx, "ses_r")
	assert.NotContains(t, st.Unlocks, sessionspec.UnlockTag)
	assert.NotContains(t, st.Unlocks, sessionspec.UnlockMR)
}
