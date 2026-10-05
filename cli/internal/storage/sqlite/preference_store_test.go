package sqlite

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestPreferenceStoreCRUD(t *testing.T) {
	st := NewPreferenceStore(openTemp(t))
	ctx := context.Background()

	_, err := st.Get(ctx, domain.PreferenceScopeGlobal, "start.pins")
	assert.ErrorIs(t, err, domain.ErrNotFound)

	require.NoError(t, st.Set(ctx, domain.PreferenceScopeGlobal, "start.pins", json.RawMessage(`["ticket"]`)))
	require.NoError(t, st.Set(ctx, domain.PreferenceScopeGlobal, "start.pins", json.RawMessage(`["ticket","review"]`)))
	require.NoError(t, st.Set(ctx, domain.ProjectPreferenceScope("p1"), "start.pins", json.RawMessage(`["cadrage"]`)))
	assert.Error(t, st.Set(ctx, domain.PreferenceScopeGlobal, "bad", json.RawMessage(`{`)))

	p, err := st.Get(ctx, domain.PreferenceScopeGlobal, "start.pins")
	require.NoError(t, err)
	assert.JSONEq(t, `["ticket","review"]`, string(p.Value))
	assert.WithinDuration(t, time.Now(), p.UpdatedAt, time.Minute)

	list, err := st.List(ctx, "project:p1")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "start.pins", list[0].Key)

	require.NoError(t, st.Delete(ctx, domain.PreferenceScopeGlobal, "start.pins"))
	require.NoError(t, st.Delete(ctx, domain.PreferenceScopeGlobal, "start.pins"))
	_, err = st.Get(ctx, domain.PreferenceScopeGlobal, "start.pins")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestPreferenceStoreRecentWorkflows(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	projects := NewProjectStore(s)
	for _, id := range []string{"p1", "p2"} {
		require.NoError(t, projects.Create(ctx, &domain.Project{ID: id, Name: id, Path: "/tmp/" + id}))
	}
	sessions := NewSessionStore(s)
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for i, row := range []struct{ id, project, wf string }{
		{"s1", "p1", "ticket"},
		{"s2", "p1", "review"},
		{"s3", "p2", "audit"},
		{"s4", "p1", "ticket"}, // most recent use of ticket
		{"s5", "p1", ""},       // legacy session without workflow
	} {
		require.NoError(t, sessions.Create(ctx, &domain.Session{
			ID: row.id, ProjectID: row.project, WorkflowID: row.wf, Status: domain.SessionStatusCompleted,
			StartedAt: base.Add(time.Duration(i) * time.Hour),
		}))
	}
	st := NewPreferenceStore(s)

	all, err := st.RecentWorkflows(ctx, "", 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"ticket", "audit", "review"}, useIDs(all))
	assert.True(t, all[0].LastUsed.Equal(base.Add(3*time.Hour)), "last use: %v", all[0].LastUsed)

	p1, err := st.RecentWorkflows(ctx, "p1", 1)
	require.NoError(t, err)
	assert.Equal(t, []string{"ticket"}, useIDs(p1))
}

func useIDs(us []domain.WorkflowUse) []string {
	out := make([]string, 0, len(us))
	for _, u := range us {
		out = append(out, u.WorkflowID)
	}
	return out
}

// Migrations reserved by parallel branches run even when a higher version
// was applied first (a branch merged out of order).
func TestMigrationsAppliedOutOfOrder(t *testing.T) {
	s := openTemp(t)
	_, err := s.DB().Exec(`DROP TABLE preferences; DELETE FROM schema_migrations WHERE version = 32;
		INSERT INTO schema_migrations (version) VALUES (99)`)
	require.NoError(t, err)
	require.NoError(t, s.migrate())
	var n int
	require.NoError(t, s.DB().QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 32`).Scan(&n))
	assert.Equal(t, 1, n)
	_, err = s.DB().Exec(`SELECT scope FROM preferences`)
	assert.NoError(t, err, "v32 applied after v99")
	require.NoError(t, s.migrate(), "idempotent")
}
