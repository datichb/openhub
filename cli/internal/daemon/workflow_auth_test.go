package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

// N1 (v5 critical): the workflow routes of the socket are reserved to the
// workflow MCP server of the session (group token); the routes only oh
// calls need the capability.
func TestWorkflowRoutesNeedTheGroupToken(t *testing.T) {
	p := shortPaths(t)
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	sessions := sqlite.NewSessionStore(st)
	bundles := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(bundles, "h1"), 0o755))
	data, _ := json.Marshal(sessionspec.BundleSpec{Hash: "h1", Workflow: &sessionspec.WorkflowRuntime{ID: "ticket", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-1", Behaviors: map[string]string{"manuel": "auto"}},
	}}})
	require.NoError(t, os.WriteFile(filepath.Join(bundles, "h1", "bundle.json"), data, 0o644))
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", BundleHash: "h1", Mode: "manuel", State: domain.RunActive}))
	cs := sqlite.NewCheckpointStore(st)
	cp := &checkpoint.Service{Sessions: sessions, States: cs, SessionOutputs: cs, BundlesDir: bundles, SessionsDir: t.TempDir()}
	runDaemon(t, Options{Paths: p, Version: "t", Servers: sqlite.NewServerStore(st), Sessions: sessions, Checkpoints: cp,
		Capability: "cap", Tick: time.Hour, IdleAfter: time.Hour})

	oh := NewClient(p).WithCapability("cap")
	issue := func(owner string) string {
		g, err := oh.IssueGrant(ctx, GrantRequest{Owner: owner, Provider: "amazon-bedrock", Region: "eu-west-1",
			Source: domain.CredentialSource{Kind: domain.CredentialBearer}, Secret: "real"})
		require.NoError(t, err)
		return g.Token
	}
	status := func(err error) int {
		var api *APIError
		if errors.As(err, &api) {
			return api.Status
		}
		return 0
	}

	anon := NewClient(p)
	_, err := anon.WorkflowStatus(ctx, "ses_a")
	assert.Equal(t, http.StatusUnauthorized, status(err), "no token")
	_, err = anon.WorkflowCheckpoint(ctx, "ses_a", checkpoint.Call{ID: "cp-1"})
	assert.Equal(t, http.StatusUnauthorized, status(err))
	assert.Equal(t, http.StatusUnauthorized, status(anon.WorkflowOutput(ctx, "ses_a", checkpoint.Output{Type: "branch", Value: "x"})))
	_, err = anon.WithGroupToken("ohs_forged").WorkflowStatus(ctx, "ses_a")
	assert.Equal(t, http.StatusUnauthorized, status(err), "unknown token")

	other := NewClient(p).WithGroupToken(issue("g2"))
	_, err = other.WorkflowCheckpoint(ctx, "ses_a", checkpoint.Call{ID: "cp-1"})
	assert.Equal(t, http.StatusForbidden, status(err), "token of another group")

	mine := NewClient(p).WithGroupToken(issue("g1"))
	res, err := mine.WorkflowCheckpoint(ctx, "ses_a", checkpoint.Call{ID: "cp-1"})
	require.NoError(t, err)
	assert.Equal(t, "cp-1", res.ID)

	// Tokens of the workflow servers the daemon runs itself (container).
	d := &Daemon{}
	tok := d.workflowToken("g1")
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	g, ok := d.workflowOwner(req)
	assert.True(t, ok)
	assert.Equal(t, "g1", g)

	// Routes only oh calls.
	assert.Equal(t, http.StatusForbidden, status(anon.WorkflowRefresh(ctx, "ses_a")))
	require.NoError(t, oh.WorkflowRefresh(ctx, "ses_a"))
	assert.Equal(t, http.StatusForbidden, status(anon.SetPolicy(ctx, "g1", PolicyBackground)))
	_, err = anon.AttachedClients(ctx, "ses_a")
	assert.Equal(t, http.StatusForbidden, status(err))
}

func TestHeartbeatTTLIsBounded(t *testing.T) {
	p := shortPaths(t)
	d := &Daemon{opts: Options{Paths: p}, clients: map[string]client{}}
	body, _ := json.Marshal(HeartbeatRequest{ClientID: "x", Kind: ClientPresence, TTL: 86400})
	rec := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	d.handleHeartbeat(rec, req)
	require.Less(t, rec.Code, 300)
	assert.WithinDuration(t, time.Now().Add(maxClientTTL), d.clients["x"].expires, 5*time.Second)
}
