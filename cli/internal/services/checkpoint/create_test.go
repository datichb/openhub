package checkpoint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// N9: creating tickets waits for the checkpoint that unlocks `create`; a
// closed ticket does not lock it again, the next mandatory checkpoint does
// (CP-R8).
func TestGuardCreate(t *testing.T) {
	svc, ctx := newTestService(t)
	wf := lockWorkflow()
	wf.Checkpoints = append([]sessionspec.CheckpointDef{{ID: "cp-tickets", Label: map[string]string{"": "Valider le découpage"},
		Behaviors: map[string]string{"manuel": "pause"}, Mandatory: true, Unlocks: []string{sessionspec.UnlockCreate}}}, wf.Checkpoints...)
	writeBundle(t, svc.BundlesDir, "h_create", wf)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_c", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_create", Mode: "manuel", State: domain.RunActive, LaunchPath: "/work"}))
	h := &fakeHead{head: "c1"}
	svc.Head = h.get

	var lock *LockError
	require.ErrorAs(t, svc.GuardCreate(ctx, "ses_c"), &lock)
	assert.Contains(t, lock.Msg, "cp-tickets")
	rules, err := svc.Rules(ctx, "ses_c")
	require.NoError(t, err)
	assert.Contains(t, rules, sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: "bd create*", Effect: sessionspec.EffectDeny})

	_, err = reachValidated(t, svc, ctx, "ses_c", "cp-tickets")
	require.NoError(t, err)
	assert.NoError(t, svc.GuardCreate(ctx, "ses_c"))

	_, err = svc.BeadsDone(ctx, "ses_c", BeadsClose, []string{"pt-0"})
	require.NoError(t, err)
	assert.NoError(t, svc.GuardCreate(ctx, "ses_c"), "a closed ticket does not lock creation again")

	// CP-R8: the next mandatory checkpoint closes the create window.
	_, err = reachValidated(t, svc, ctx, "ses_c", "cp-2")
	require.NoError(t, err)
	require.ErrorAs(t, svc.GuardCreate(ctx, "ses_c"), &lock, "the window closed at cp-2")
	_, err = svc.BeadsDone(ctx, "ses_c", BeadsClose, []string{"pt-1"})
	require.NoError(t, err)
	st, _ := svc.State(ctx, "ses_c")
	_, commitOpen := st.Unlocks[sessionspec.UnlockCommit]
	assert.False(t, commitOpen, "but it locks the commit again")

	assert.NoError(t, svc.GuardCreate(ctx, "ses_plain"), "no workflow: nothing locked")
}

// r9b: changing tickets waits for the checkpoint that unlocks `update`; so
// does a change that closes (duplicate) in a workflow where nothing unlocks
// closing; the next mandatory checkpoint closes the window (CP-R8).
func TestGuardUpdate(t *testing.T) {
	svc, ctx := newTestService(t)
	wf := &sessionspec.WorkflowRuntime{ID: "triage", DefaultMode: "manuel", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-triage", Label: map[string]string{"": "Changements proposés"}, Behaviors: map[string]string{"manuel": "pause"}, Mandatory: true,
			Unlocks: []string{sessionspec.UnlockCreate, sessionspec.UnlockUpdate}},
		{ID: "cp-end", Label: map[string]string{"": "Fin"}, Behaviors: map[string]string{"manuel": "pause"}, Mandatory: true},
	}}
	writeBundle(t, svc.BundlesDir, "h_update", wf)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_u", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_update", Mode: "manuel", State: domain.RunActive, LaunchPath: "/work"}))
	h := &fakeHead{head: "c1"}
	svc.Head = h.get

	var lock *LockError
	require.ErrorAs(t, svc.GuardUpdate(ctx, "ses_u", false), &lock)
	assert.Contains(t, lock.Msg, "cp-triage")
	require.ErrorAs(t, svc.GuardUpdate(ctx, "ses_u", true), &lock, "a duplicate waits too: nothing unlocks closing")

	_, err := reachValidated(t, svc, ctx, "ses_u", "cp-triage")
	require.NoError(t, err)
	assert.NoError(t, svc.GuardUpdate(ctx, "ses_u", false))
	_, err = svc.BeadsDone(ctx, "ses_u", BeadsClose, []string{"ev-2"})
	require.NoError(t, err)
	assert.NoError(t, svc.GuardUpdate(ctx, "ses_u", true), "a closed duplicate does not lock changes again")

	_, err = reachValidated(t, svc, ctx, "ses_u", "cp-end")
	require.NoError(t, err)
	require.ErrorAs(t, svc.GuardUpdate(ctx, "ses_u", false), &lock, "the window closed at the next mandatory checkpoint")
	assert.NoError(t, svc.GuardUpdate(ctx, "ses_plain", false), "no workflow: nothing locked")

	// A workflow that unlocks closing: closing follows GuardClose only.
	lw := lockWorkflow()
	lw.Checkpoints[0].Unlocks = []string{sessionspec.UnlockUpdate}
	writeBundle(t, svc.BundlesDir, "h_update2", lw)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_u2", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_update2", Mode: "manuel", State: domain.RunActive, LaunchPath: "/work"}))
	assert.NoError(t, svc.GuardUpdate(ctx, "ses_u2", true))
	require.ErrorAs(t, svc.GuardUpdate(ctx, "ses_u2", false), &lock)
}
