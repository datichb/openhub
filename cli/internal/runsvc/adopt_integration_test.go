//go:build integration

package runsvc

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// A session exported from another server (remote job) is imported into the
// local group of its bundle and becomes a local, resumable session.
func TestAdoptSession(t *testing.T) {
	f := newFixture(t, mapSecrets{"openhub.provider.bedrock.token": "fake-key"})
	ctx := context.Background()
	dir := filepath.Join(f.svc.BundlesDir, f.bundle.Spec.Hash)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	data, _ := json.Marshal(f.bundle.Spec)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bundle.json"), data, 0o644))

	// The "job": a session on a server, exported, then under a new id.
	r, err := f.svc.StartSession(ctx, f.request(""))
	require.NoError(t, err)
	tr, err := f.adapter.ExportSession(ctx, handle(r.Server), r.SessionID)
	require.NoError(t, err)
	remoteID := sessionspec.NewSessionID()
	var m map[string]any
	require.NoError(t, json.Unmarshal(tr, &m))
	m["info"].(map[string]any)["id"] = remoteID
	tr, _ = json.Marshal(m)

	now := time.Now()
	require.NoError(t, f.svc.Sessions.Create(ctx, &domain.Session{ID: remoteID, ProjectID: "p1", StartedAt: now,
		Status: domain.SessionStatusRunning, Runtime: "remote", State: domain.RunCompleted, BundleHash: f.bundle.Spec.Hash,
		LaunchPath: f.project, Location: "remote"}))

	req := f.request("")
	req.Bundle, req.Location = nil, f.project
	require.NoError(t, f.svc.AdoptSession(ctx, remoteID, [][]byte{tr}, req))
	sess, err := f.svc.Sessions.Get(ctx, remoteID)
	require.NoError(t, err)
	assert.Equal(t, "local", sess.Runtime)
	assert.Equal(t, domain.RunIdle, sess.State)
	assert.NotEmpty(t, sess.GroupKey)
	srv, err := f.svc.Servers.Get(ctx, sess.GroupKey)
	require.NoError(t, err)
	got, err := opencodev2.NewClient(srv.URL, srv.Password).GetSession(ctx, remoteID)
	require.NoError(t, err)
	assert.Equal(t, f.project, got.Location.Directory)
	assert.Contains(t, sessionShellEnv(t, srv, remoteID, f.project), "id="+remoteID, "environment applied")
	_, _, err = f.svc.AttachCommand(ctx, remoteID)
	require.NoError(t, err, "attachable")

	// Again: the session already there is kept.
	require.NoError(t, f.svc.AdoptSession(ctx, remoteID, [][]byte{tr}, req))
}
