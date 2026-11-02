package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/daemon/api"
	dclient "github.com/datichb/openhub/cli/internal/daemon/client"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway"
	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

// N5 (v5 critical, finitions): a checkpoint refused because the git
// configuration of the session changed raises an alert (✗) for the user
// that names the file, once per state of the repository.
func TestGitChangedAlert(t *testing.T) {
	gitutil.SetTrustDir(t.TempDir())
	t.Cleanup(func() { gitutil.SetTrustDir("") })
	p := shortPaths(t)
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	sessions, decisions := sqlite.NewSessionStore(st), sqlite.NewDecisionStore(st)

	repo := t.TempDir()
	gittest.Run(t, repo, "init", "-q")
	bundles := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(bundles, "h1"), 0o755))
	data, _ := json.Marshal(sessionspec.BundleSpec{Hash: "h1", Workflow: &sessionspec.WorkflowRuntime{ID: "ticket", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-1", Behaviors: map[string]string{"auto": sessionspec.CheckpointAuto}, Unlocks: []string{"commit"}},
	}}})
	require.NoError(t, os.WriteFile(filepath.Join(bundles, "h1", "bundle.json"), data, 0o644))
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h1", Mode: "auto", State: domain.RunActive, LaunchPath: repo}))
	require.NoError(t, gitutil.Trust(ctx, repo))

	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	cs := sqlite.NewCheckpointStore(st)
	cp := &checkpoint.Service{Sessions: sessions, States: cs, SessionOutputs: cs, BundlesDir: bundles, SessionsDir: t.TempDir(), ApprovalWait: 10 * time.Millisecond}
	go func() {
		done <- Run(dctx, Options{Paths: p, Version: "t", Servers: sqlite.NewServerStore(st), Sessions: sessions, Decisions: decisions, Checkpoints: cp, Tick: time.Hour, IdleAfter: time.Hour})
	}()
	t.Cleanup(func() { cancel(); <-done })
	c := dclient.NewClient(p)
	require.Eventually(t, func() bool { _, err := c.Health(ctx); return err == nil }, 5*time.Second, 20*time.Millisecond)
	open := func() []domain.Decision {
		l, err := decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: "ses_a"})
		require.NoError(t, err)
		return l
	}
	refused := func() {
		t.Helper()
		_, err := c.WorkflowCheckpoint(ctx, "ses_a", checkpoint.Call{ID: "cp-1", Summary: "s"})
		var api *api.APIError
		require.True(t, errors.As(err, &api), "%v", err)
		assert.Equal(t, 409, api.Status)
	}

	gittest.Run(t, repo, "config", "core.fsmonitor", "touch /tmp/oh-pwned")
	refused()
	l := open()
	require.Len(t, l, 1)
	assert.Equal(t, domain.DecisionError, l[0].Kind)
	assert.True(t, isOhAlert(l[0]), "dismissed by the user only")
	assert.Contains(t, l[0].Payload.Message, filepath.Join(".git", "config"), "the file that changed")
	assert.Contains(t, l[0].Payload.Message, "cp-1")
	assert.NotEmpty(t, l[0].Payload.Title)

	refused()
	assert.Len(t, open(), 1, "the same change is not raised twice")

	_, err := decisions.Resolve(ctx, l[0].ID, domain.ResolvedByOh, nil, time.Now())
	require.NoError(t, err)
	refused()
	assert.Empty(t, open(), "dismissed: not raised again for the same change")

	require.NoError(t, os.WriteFile(filepath.Join(repo, ".git", "hooks", "post-checkout"), []byte("#!/bin/sh\n"), 0o755))
	refused()
	l = open()
	require.Len(t, l, 1, "a further change is raised")
	assert.Contains(t, l[0].Payload.Message, "post-checkout")
}

// The closing guard of a ticket raises the same alert as a checkpoint when the
// git configuration of the session changed: the agent is refused and the user
// is told, once.
func TestGuardCloseRaisesGitChangedAlert(t *testing.T) {
	gitutil.SetTrustDir(t.TempDir())
	t.Cleanup(func() { gitutil.SetTrustDir("") })
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	sessions, decisions := sqlite.NewSessionStore(st), sqlite.NewDecisionStore(st)

	repo := t.TempDir()
	gittest.Run(t, repo, "init", "-q")
	gittest.Commit(t, repo, "a.txt", "a", "init")
	bundles := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(bundles, "h1"), 0o755))
	data, _ := json.Marshal(sessionspec.BundleSpec{Hash: "h1", Workflow: &sessionspec.WorkflowRuntime{ID: "ticket", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-1", Behaviors: map[string]string{"auto": sessionspec.CheckpointAuto}, Unlocks: []string{sessionspec.UnlockClose}},
	}}})
	require.NoError(t, os.WriteFile(filepath.Join(bundles, "h1", "bundle.json"), data, 0o644))
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h1", Mode: "auto", State: domain.RunActive, LaunchPath: repo}))
	require.NoError(t, gitutil.Trust(ctx, repo))
	cs := sqlite.NewCheckpointStore(st)
	cp := &checkpoint.Service{Sessions: sessions, States: cs, SessionOutputs: cs, BundlesDir: bundles, SessionsDir: t.TempDir(), ApprovalWait: 10 * time.Millisecond}
	_, err := cp.Reached(ctx, "ses_a", checkpoint.Call{ID: "cp-1", Summary: "s"})
	require.NoError(t, err)

	d := &Daemon{opts: Options{Sessions: sessions, Decisions: decisions, Checkpoints: cp}}
	grant := gateway.Grant{SessionID: "ses_a", Location: repo}
	closing := gateway.BeadsOp{Kind: gateway.OpClose, IDs: []string{"pt-1"}}
	open := func() []domain.Decision {
		l, err := decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: "ses_a"})
		require.NoError(t, err)
		return l
	}

	gittest.Run(t, repo, "config", "core.fsmonitor", "touch /tmp/oh-pwned")
	err = d.guardTickets(ctx, grant, closing)
	require.Error(t, err, "closing is refused")
	assert.ErrorIs(t, err, gitutil.ErrConfigChanged)
	l := open()
	require.Len(t, l, 1, "the user is told")
	assert.True(t, isOhAlert(l[0]))
	assert.Contains(t, l[0].Payload.Message, filepath.Join(".git", "config"), "the file that changed")
	assert.Contains(t, l[0].Payload.Message, "cp-1", "the checkpoint that unlocks closing")

	require.Error(t, d.guardTickets(ctx, grant, closing))
	assert.Len(t, open(), 1, "the same change is not raised twice")
}
