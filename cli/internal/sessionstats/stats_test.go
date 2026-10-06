package sessionstats

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

func TestProviderOverTheSessionRegistry(t *testing.T) {
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	ps, ss := sqlite.NewProjectStore(st), sqlite.NewSessionStore(st)
	now := time.Date(2026, 10, 6, 15, 0, 0, 0, time.Local)
	for _, p := range []domain.Project{{ID: "a", Name: "A", Path: "/w/a"}, {ID: "b", Name: "B", Path: "/w/b"}} {
		p.Status, p.CreatedAt, p.UpdatedAt = domain.ProjectStatusActive, now, now
		require.NoError(t, ps.Create(ctx, &p))
	}
	title := "a · ticket"
	for _, s := range []domain.Session{
		{ID: "s1", ProjectID: "a", StartedAt: now.Add(-time.Hour), Cost: 0.5, TokensIn: 10, TokensOut: 20, Title: &title},
		{ID: "s2", ProjectID: "a", StartedAt: now.AddDate(0, 0, -3), Cost: 1, TokensIn: 1, LaunchPath: "/w/a-oh-ticket-x"},
		{ID: "s3", ProjectID: "b", StartedAt: now.AddDate(0, 0, -20), Cost: 2, TokensCacheRead: 5},
	} {
		s.Status = domain.SessionStatusCompleted
		require.NoError(t, ss.Create(ctx, &s))
	}
	p := New(ss, ps)
	p.Now = func() time.Time { return now }
	require.True(t, p.Available())

	week, err := p.AggregateStats("7d")
	require.NoError(t, err)
	assert.Equal(t, 2, week.TotalSessions)
	assert.InDelta(t, 1.5, week.TotalCost, 1e-9)
	assert.Equal(t, 1, week.TodaySessions)
	assert.Equal(t, 1, week.ActiveProjects)

	all, err := p.AggregateStats("all")
	require.NoError(t, err)
	assert.Equal(t, 3, all.TotalSessions)
	assert.Equal(t, int64(5), all.CacheReadTokens)

	proj, err := p.ProjectStats("/w/a", "30d")
	require.NoError(t, err)
	assert.Equal(t, 2, proj.TotalSessions, "project sessions, worktrees included")

	recent, err := p.RecentSessions(2)
	require.NoError(t, err)
	require.Len(t, recent, 2)
	assert.Equal(t, "s1", recent[0].ID)
	assert.Equal(t, "a · ticket", recent[0].Title)

	days, err := p.DailyCosts("30d")
	require.NoError(t, err)
	require.Len(t, days, 3)
	assert.Less(t, days[0].Day, days[2].Day)
}
