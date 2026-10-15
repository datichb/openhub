package runsvc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// fanout-container: the sessions of a fan-out whose parent works in a
// container. The children, the merger and the fixer are launched by the
// driver with the runtime of the parent: each joins the container group of
// its own bundle (never the parent's: another workflow, another closed
// world), with its worktree mounted, and gets the rules of its role from
// its first call, whatever the runtime.

func fanoutBundle(f *rtFixture, hash string) *bundle.Bundle {
	b := *f.b
	b.Spec.Hash = hash
	b.Spec.Workflow = &sessionspec.WorkflowRuntime{ID: "ticket", DefaultMode: "semi-auto", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-2", Mandatory: true, Unlocks: []string{sessionspec.UnlockCommit, sessionspec.UnlockClose},
			Behaviors: map[string]string{"manuel": "pause", "semi-auto": "pause", "auto": "pause"}},
	}}
	return &b
}

func TestFanoutSessionsInContainer(t *testing.T) {
	f := newRTFixture(t)
	ctx := context.Background()
	parent := f.request(f.project)
	parent.Bundle = fanoutBundle(f, "parenthash000001")
	parent.Fanout = &domain.SessionFanout{Role: domain.RoleParent, Experimental: true, Status: domain.FanoutWaiting}
	rp, err := f.svc.StartSession(ctx, parent)
	require.NoError(t, err)

	child := fanoutBundle(f, "childhash0000001")
	var groups []string
	for _, wt := range []string{"proj-oh-fanout-p-ev-1", "proj-oh-fanout-p-ev-2"} {
		req := f.request(f.worktree(t, wt))
		req.Bundle = child
		req.Fanout = &domain.SessionFanout{Role: domain.RoleChild, Parent: rp.SessionID, Ticket: wt[len(wt)-4:], Branch: "oh/fanout/p/" + wt[len(wt)-4:]}
		res, err := f.svc.StartSession(ctx, req)
		require.NoError(t, err)
		groups = append(groups, res.GroupKey)
		sess, err := f.svc.Sessions.Get(ctx, res.SessionID)
		require.NoError(t, err)
		assert.Equal(t, "container", sess.Runtime)
		assert.Equal(t, req.Location, sess.LaunchPath, "oh keeps the machine path (its git, its merges)")
	}
	assert.NotEqual(t, rp.GroupKey, groups[0], "a child never joins the container of the parent")
	assert.NotEqual(t, groups[0], groups[1], "two children at once: two containers (the first is busy)")

	merger := f.request(f.worktree(t, "proj-oh-fanout-p-integration"))
	merger.Bundle = fanoutBundle(f, "mergerhash000001")
	merger.Fanout = &domain.SessionFanout{Role: domain.RoleMerger, Parent: rp.SessionID, Ticket: "ev-1", Branch: "oh/fanout/p/ev-1"}
	_, err = f.svc.StartSession(ctx, merger)
	require.NoError(t, err)

	require.Len(t, f.ad.created, 4)
	assert.Equal(t, []string{"/work/proj", "/work/proj-oh-fanout-p-ev-1", "/work/proj-oh-fanout-p-ev-2", "/work/proj-oh-fanout-p-integration"},
		[]string{f.ad.created[0].Location, f.ad.created[1].Location, f.ad.created[2].Location, f.ad.created[3].Location},
		"each worktree mounted in the container of its session")
	for i, ss := range f.ad.created {
		assert.Equal(t, sessionspec.RuntimeContainer, ss.Runtime)
		for _, cmd := range []string{"git push origin HEAD", "git -C /work/proj merge oh/fanout/p/ev-1", "bd close ev-1",
			"git update-ref refs/heads/oh/fanout/p/integration HEAD"} {
			assert.Equal(t, sessionspec.EffectDeny, bundle.ShellEffect(ss.SessionRules, cmd), "session %d, from its creation: %s", i, cmd)
		}
	}
	assert.Equal(t, sessionspec.EffectDeny, bundle.ShellEffect(f.ad.created[0].SessionRules, "git commit -m x"), "the parent never commits")
	for i, sg := range f.ad.started {
		assert.NotEmpty(t, sg.Bundle.PendingGuard, "server %d: closed until oh confirms the rules of its sessions", i)
		assert.Equal(t, sessionspec.RuntimeContainer, sg.Key.Runtime)
	}
}
