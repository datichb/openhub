package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// N5 (v5 critical, finitions): a checkpoint refused because the git
// configuration of the session changed raises an alert (✗) for the user
// that names the file, once per state of the repository.
func TestGitChangedAlert(t *testing.T) {
	gitutil.SetTrustDir(t.TempDir())
	t.Cleanup(func() { gitutil.SetTrustDir("") })
	p := shortPaths(t)
	st, err := sqlite.Open(filepath.Join(p.Dir, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	sessions, decisions := sqlite.NewSessionStore(st), sqlite.NewDecisionStore(st)

	repo := t.TempDir()
	out, err := exec.Command("git", "init", "-q", repo).CombinedOutput()
	require.NoError(t, err, string(out))
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
	c := NewClient(p)
	require.Eventually(t, func() bool { _, err := c.Health(ctx); return err == nil }, 5*time.Second, 20*time.Millisecond)
	open := func() []domain.Decision {
		l, err := decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: "ses_a"})
		require.NoError(t, err)
		return l
	}
	refused := func() {
		t.Helper()
		_, err := c.WorkflowCheckpoint(ctx, "ses_a", checkpoint.Call{ID: "cp-1", Summary: "s"})
		var api *APIError
		require.True(t, errors.As(err, &api), "%v", err)
		assert.Equal(t, 409, api.Status)
	}

	out, err = exec.Command("git", "-C", repo, "config", "core.fsmonitor", "touch /tmp/oh-pwned").CombinedOutput()
	require.NoError(t, err, string(out))
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

	_, err = decisions.Resolve(ctx, l[0].ID, domain.ResolvedByOh, nil, time.Now())
	require.NoError(t, err)
	refused()
	assert.Empty(t, open(), "dismissed: not raised again for the same change")

	require.NoError(t, os.WriteFile(filepath.Join(repo, ".git", "hooks", "post-checkout"), []byte("#!/bin/sh\n"), 0o755))
	refused()
	l = open()
	require.Len(t, l, 1, "a further change is raised")
	assert.Contains(t, l[0].Payload.Message, "post-checkout")
}
