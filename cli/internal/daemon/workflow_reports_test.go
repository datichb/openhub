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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/services/report"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

type reportEnv struct {
	sessions domain.SessionStore
	reports  *report.Service
	cp       *checkpoint.Service
}

func newReportEnv(t *testing.T, dir string) reportEnv {
	t.Helper()
	st, err := sqlite.Open(filepath.Join(dir, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	sessions := sqlite.NewSessionStore(st)
	bundles := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(bundles, "h1"), 0o755))
	data, _ := json.Marshal(sessionspec.BundleSpec{Hash: "h1", Workflow: &sessionspec.WorkflowRuntime{ID: "ticket", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-2", Behaviors: map[string]string{"manuel": "pause"}},
	}}})
	require.NoError(t, os.WriteFile(filepath.Join(bundles, "h1", "bundle.json"), data, 0o644))
	for _, id := range []string{"ses_a", "ses_b"} {
		require.NoError(t, sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
			BundleHash: "h1", Mode: "manuel", State: domain.RunActive, EntryAgent: "orchestrator-dev"}))
	}
	cs := sqlite.NewCheckpointStore(st)
	sdir := t.TempDir()
	return reportEnv{sessions: sessions,
		reports: &report.Service{Store: sqlite.NewReportStore(st), SessionsDir: sdir},
		cp:      &checkpoint.Service{Sessions: sessions, States: cs, SessionOutputs: cs, BundlesDir: bundles, SessionsDir: sdir}}
}

func apiStatus(err error) int {
	var api *APIError
	if errors.As(err, &api) {
		return api.Status
	}
	return 0
}

// The report routes are workflow routes: the group token of the session.
func TestWorkflowReportRoutesNeedTheGroupToken(t *testing.T) {
	p := shortPaths(t)
	e := newReportEnv(t, p.Dir)
	ctx := context.Background()
	runDaemon(t, Options{Paths: p, Version: "t", Sessions: e.sessions, Checkpoints: e.cp, Reports: e.reports,
		Capability: "cap", Tick: time.Hour, IdleAfter: time.Hour})
	oh := NewClient(p).WithCapability("cap")
	issue := func(owner string) string {
		g, err := oh.IssueGrant(ctx, GrantRequest{Owner: owner, Provider: "amazon-bedrock", Region: "eu-west-1",
			Source: domain.CredentialSource{Kind: domain.CredentialBearer}, Secret: "real"})
		require.NoError(t, err)
		return g.Token
	}

	anon := NewClient(p)
	_, err := anon.WorkflowReport(ctx, "ses_a", "t", "corps")
	assert.Equal(t, http.StatusUnauthorized, apiStatus(err))
	_, err = anon.WorkflowReports(ctx, "ses_a")
	assert.Equal(t, http.StatusUnauthorized, apiStatus(err))
	_, err = anon.WorkflowReportGet(ctx, "ses_a", "rp-1", "")
	assert.Equal(t, http.StatusUnauthorized, apiStatus(err))

	other := NewClient(p).WithGroupToken(issue("g2"))
	_, err = other.WorkflowReport(ctx, "ses_a", "t", "corps")
	assert.Equal(t, http.StatusForbidden, apiStatus(err), "token of another group")

	mine := NewClient(p).WithGroupToken(issue("g1"))
	rp, err := mine.WorkflowReport(ctx, "ses_a", "Fait", "## Tests\nok")
	require.NoError(t, err)
	assert.Equal(t, "rp-1", rp.ID)
	assert.Equal(t, "orchestrator-dev", rp.Agent, "entry agent of the session")
	_, err = other.WorkflowReportGet(ctx, "ses_a", "rp-1", "")
	assert.Equal(t, http.StatusForbidden, apiStatus(err))
	got, err := mine.WorkflowReportGet(ctx, "ses_a", "rp-1", "tests")
	require.NoError(t, err)
	assert.Equal(t, "## Tests\nok", got.Body)
	_, err = mine.WorkflowReportGet(ctx, "ses_a", "rp-1", "absente")
	assert.Equal(t, http.StatusNotFound, apiStatus(err))
	list, err := mine.WorkflowReports(ctx, "ses_b")
	require.NoError(t, err)
	assert.Empty(t, list, "reports belong to their session")

	_, err = mine.WorkflowReport(ctx, "ses_a", "t", strings.Repeat("a", report.MaxBytes+1))
	assert.Equal(t, http.StatusUnprocessableEntity, apiStatus(err))
	_, err = mine.WorkflowReport(ctx, "ses_a", "t", strings.Repeat("a", reportRequestMax+1))
	assert.Equal(t, http.StatusRequestEntityTooLarge, apiStatus(err))
}

// The author is the agent oh saw in the calling tool session; the request
// cannot set it.
func TestWorkflowReportAuthorIsTheCallingAgent(t *testing.T) {
	e := newReportEnv(t, t.TempDir())
	w := &watcher{tracks: map[string]*sessionTrack{"ses_a": {own: "orchestrator-dev", agent: "developer"}},
		children: map[string]string{"ses_child": "ses_a", "ses_quiet": "ses_a"}, childAgent: map[string]string{"ses_child": "reviewer"}}
	d := &Daemon{opts: Options{Sessions: e.sessions, Checkpoints: e.cp, Reports: e.reports}, watchers: map[string]*watcher{"g1": w}}
	h := d.routes()
	post := func(body string) (*httptest.ResponseRecorder, WorkflowReport) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, apiPrefix+"/workflow/report", bytes.NewBufferString(body)))
		var out WorkflowReport
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec, out
	}

	rec, rp := post(`{"tool_session":"ses_child","title":"Review","body":"## Verdict\ncorriger","agent":"developer","Agent":"developer"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Equal(t, "rp-1", rp.ID)
	assert.Equal(t, "ses_a", rp.SessionID, "filed under the oh session")
	assert.Equal(t, "reviewer", rp.Agent, "the subagent oh saw, not the one claimed")
	assert.Equal(t, "ses_child", rp.ToolSession)

	_, rp = post(`{"tool_session":"ses_a","body":"# Coordination"}`)
	assert.Equal(t, "orchestrator-dev", rp.Agent, "own agent of the root, not the last one of the feed")

	_, rp = post(`{"tool_session":"ses_quiet","body":"x"}`)
	assert.Equal(t, "", rp.Agent, "unknown: never guessed")

	rec, _ = post(`{"tool_session":"ses_a","body":" "}`)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	rec, _ = post(`{"body":"x"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// The timeline records each report with its author.
	st, err := e.cp.State(context.Background(), "ses_a")
	require.NoError(t, err)
	var got []string
	for _, ev := range st.Timeline {
		if ev.Kind == domain.CheckpointReport {
			got = append(got, ev.ID+":"+ev.By)
		}
	}
	assert.Equal(t, []string{"rp-1:reviewer", "rp-2:orchestrator-dev", "rp-3:"}, got)

	// Without a report store: not recorded (the agent falls back on its block).
	d.opts.Reports = nil
	rec, _ = post(`{"tool_session":"ses_a","body":"x"}`)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
