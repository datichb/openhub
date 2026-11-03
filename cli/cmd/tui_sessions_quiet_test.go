package cmd

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/hubcontent"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The Sessions view reads its data through the real wiring: whatever the
// sessions are, it must not raise an error nor log a warning (the TUI turns
// both into red toasts).
func TestTUISessionsBackendIsQuiet(t *testing.T) {
	i18ntest.Set(t, "en")
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	config.Reset()
	t.Cleanup(config.Reset)
	require.NoError(t, hubcontent.Extract(hubcontent.HubContentDir()), "the hub the sessions were launched from")
	s, err := sqlite.Open(filepath.Join(home, "oh.db"))
	require.NoError(t, err)
	prev := store
	store = s
	t.Cleanup(func() { store = prev; s.Close() })
	a := &app.App{Config: &config.Config{}, IO: app.DefaultIOStreams()}
	wireStores(a, s)
	application = a
	t.Cleanup(func() { application = nil })

	var logs bytes.Buffer
	prevLog := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prevLog) })

	ctx := context.Background()
	ts := &tuiSessions{a: a, teams: map[string]string{}, seen: map[string]bool{}, watchers: map[chan struct{}]struct{}{}, ends: map[string]string{}}

	check := func(label string) {
		t.Helper()
		sessions, err := ts.List(ctx, "", true)
		require.NoError(t, err, "%s: List", label)
		for _, r := range sessions {
			_, err := ts.NextSteps(ctx, r.ID)
			assert.NoError(t, err, "%s: NextSteps %s", label, r.ID)
			_, err = ts.SessionTrace(ctx, r.ID)
			assert.NoError(t, err, "%s: SessionTrace %s", label, r.ID)
			_, err = ts.SessionReports(ctx, r.ID)
			assert.NoError(t, err, "%s: SessionReports %s", label, r.ID)
			if r.FanoutRole != "" {
				_, err = ts.Fanout(ctx, r.ID)
				assert.NoError(t, err, "%s: Fanout %s", label, r.ID)
			}
			for _, d := range r.Decisions {
				if d.Kind == views.DecisionKindCheckpoint {
					_, err = ts.CheckpointCard(ctx, d.ID)
					assert.NoError(t, err, "%s: CheckpointCard %s", label, d.ID)
				}
			}
		}
		_, err = ts.BatchItems(ctx, "")
		assert.NoError(t, err, "%s: BatchItems", label)
		assert.NotContains(t, logs.String(), "level=WARN", label)
		assert.NotContains(t, logs.String(), "level=ERROR", label)
	}

	check("no session")

	sqlitetest.Seed(t, s).Project("p1", t.TempDir()).Project("p2", filepath.Join(home, "missing"))
	now := time.Now()
	ended := now.Add(-time.Hour)
	title := "a title"
	mk := func(id, project string, state domain.RunState, wf string) *domain.Session {
		return &domain.Session{ID: id, ProjectID: project, State: state, WorkflowID: wf, EntryAgent: "dev", StartedAt: now.Add(-2 * time.Hour),
			Status: domain.SessionStatusRunning, Title: &title, Mode: "semi-auto", Runtime: "local", LaunchPath: t.TempDir()}
	}
	finished := mk("ses_done", "p1", domain.RunCompleted, "feature")
	finished.EndedAt, finished.Status = &ended, domain.SessionStatusCompleted
	sessions := []*domain.Session{
		mk("ses_run", "p1", domain.RunActive, "feature"),
		mk("ses_wait", "p1", domain.RunWaiting, "feature"),
		mk("ses_agent", "p1", domain.RunIdle, ""),
		mk("ses_unknown_wf", "p1", domain.RunSleeping, "deleted-workflow"),
		mk("ses_missing_project_dir", "p2", domain.RunActive, "feature"),
		mk("ses_queued", "p1", domain.RunQueued, "feature"),
		finished,
		mk("ses_failed", "p1", domain.RunFailed, "feature"),
	}
	shipped, err := filepath.Glob(filepath.Join(hubcontent.HubContentDir(), "workflows", "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, shipped)
	for _, f := range shipped {
		id := strings.TrimSuffix(filepath.Base(f), ".yaml")
		for _, st := range []domain.RunState{domain.RunActive, domain.RunCompleted} {
			sess := mk("ses_"+id+"_"+string(st), "p1", st, id)
			sess.Outputs = map[string]any{"branch": "feat/x", "tickets": []string{"oh-1"}}
			sessions = append(sessions, sess)
		}
	}
	for _, sess := range sessions {
		require.NoError(t, a.Sessions.Create(ctx, sess))
	}
	for i, kind := range []domain.DecisionKind{domain.DecisionPermission, domain.DecisionQuestion, domain.DecisionCheckpoint, domain.DecisionBudget, domain.DecisionError} {
		sid := []string{"ses_wait", "ses_wait", "ses_wait", "ses_run", "ses_failed"}[i]
		d := &domain.Decision{ID: domain.DecisionID(kind, sid, "ref"), SessionID: sid, Kind: kind, CreatedAt: now.Add(-time.Duration(i+1) * time.Minute),
			Payload: domain.DecisionPayload{Title: "t", Message: "m", Data: map[string]any{}}}
		require.NoError(t, a.Decisions.Upsert(ctx, d))
	}
	check("sessions")
}
