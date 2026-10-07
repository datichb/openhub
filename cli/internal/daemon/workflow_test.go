package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

func TestWorkflowAPI(t *testing.T) {
	p := shortPaths(t)
	st, err := sqlite.Open(filepath.Join(p.Dir, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	sessions := sqlite.NewSessionStore(st)

	bundles := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(bundles, "h1"), 0o755))
	data, _ := json.Marshal(sessionspec.BundleSpec{Hash: "h1", Workflow: &sessionspec.WorkflowRuntime{ID: "ticket", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-1", Behaviors: map[string]string{"manuel": "pause"}},
	}, Outputs: []sessionspec.OutputDef{{ID: "branch", Type: "branch"}}}})
	require.NoError(t, os.WriteFile(filepath.Join(bundles, "h1", "bundle.json"), data, 0o644))
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", BundleHash: "h1", Mode: "manuel", State: domain.RunActive}))

	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	cp := &checkpoint.Service{Sessions: sessions, SessionOutputs: sqlite.NewCheckpointStore(st), BundlesDir: bundles, SessionsDir: t.TempDir()}
	go func() {
		done <- Run(dctx, Options{Paths: p, Version: "t", Servers: sqlite.NewServerStore(st), Sessions: sessions, Checkpoints: cp, Tick: time.Hour, IdleAfter: time.Hour})
	}()
	t.Cleanup(func() { cancel(); <-done })
	c := NewClient(p)
	require.Eventually(t, func() bool { _, err := c.Health(ctx); return err == nil }, 5*time.Second, 20*time.Millisecond)

	stat, err := c.WorkflowStatus(ctx, "ses_a")
	require.NoError(t, err)
	assert.Equal(t, "cp-1", stat.Next)

	res, err := c.WorkflowCheckpoint(ctx, "ses_a", checkpoint.Call{ID: "cp-1", Summary: "s"})
	require.NoError(t, err)
	assert.Equal(t, "pause", res.Behavior)

	_, err = c.WorkflowCheckpoint(ctx, "ses_a", checkpoint.Call{ID: "cp-9"})
	var api *APIError
	require.True(t, errors.As(err, &api))
	assert.Equal(t, 422, api.Status)

	require.NoError(t, c.WorkflowOutput(ctx, "ses_a", checkpoint.Output{Type: "branch", Value: "feat/x"}))
	assert.Error(t, c.WorkflowOutput(ctx, "ses_a", checkpoint.Output{Type: "path", Value: "x"}))

	_, err = c.WorkflowStatus(ctx, "ses_unknown")
	require.True(t, errors.As(err, &api))
	assert.Equal(t, 404, api.Status)

	// Plugin hooks on the proxy listener, authenticated by the group token.
	g, err := c.IssueGrant(ctx, GrantRequest{Owner: "g1", Provider: "amazon-bedrock", Region: "eu-west-1",
		Source: domain.CredentialSource{Kind: domain.CredentialBearer}, Secret: "real"})
	require.NoError(t, err)
	other, err := c.IssueGrant(ctx, GrantRequest{Owner: "g2", Provider: "amazon-bedrock", Region: "eu-west-1",
		Source: domain.CredentialSource{Kind: domain.CredentialBearer}, Secret: "real"})
	require.NoError(t, err)
	h, _ := c.Health(ctx)
	hook := func(token string, body string) (int, string) {
		req, _ := http.NewRequest(http.MethodPost, h.ProxyURL+"/oh/v1/hooks/permission", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(data)
	}
	cpReq := `{"sessionID":"ses_a","action":"mcp:workflow/checkpoint","input":{"id":"cp-1"}}`
	code, out := hook(g.Token, cpReq)
	assert.Equal(t, 200, code)
	assert.JSONEq(t, `{}`, out, "cp-1 pauses in manuel: decision kept")
	code, _ = hook(other.Token, cpReq)
	assert.Equal(t, 403, code, "another group cannot ask about this session")
	code, _ = hook("nope", cpReq)
	assert.Equal(t, 401, code)
	code, out = hook(g.Token, `{"sessionID":"ses_a","action":"mcp:workflow/checkpoint","input":{"id":"cp-9"}}`)
	assert.Equal(t, 200, code)
	assert.JSONEq(t, `{"effect":"allow"}`, out, "unknown checkpoint let through (the call lists the checkpoints)")
}

func TestRootSessionOfSubagent(t *testing.T) {
	d := &Daemon{watchers: map[string]*watcher{"g1": {children: map[string]string{"ses_child": "ses_root"}}}}
	assert.Equal(t, "ses_root", d.rootSession("ses_child"))
	assert.Equal(t, "ses_root", d.rootSession("ses_root"))
	assert.Equal(t, "ses_other", d.rootSession("ses_other"))
}
