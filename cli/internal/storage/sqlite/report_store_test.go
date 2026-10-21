package sqlite

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestReportStore(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	seedProject(t, st, "p1")
	sessions := NewSessionStore(st)
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning}))
	rs := NewReportStore(st)

	at := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	r := &domain.Report{SessionID: "ses_a", Agent: "developer", ToolSession: "ses_child", Title: "Fait", Bytes: 10, SHA256: "x",
		Sections: []string{"Résumé", "Tests"}, CreatedAt: at}
	require.NoError(t, rs.AddReport(ctx, r, 3, 100))
	assert.Equal(t, 1, r.Seq)
	assert.Equal(t, "rp-1", r.ID())

	// Concurrent reports get distinct numbers and stop at the limit.
	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- rs.AddReport(ctx, &domain.Report{SessionID: "ses_a", Agent: "reviewer", Bytes: 10, CreatedAt: at}, 3, 100)
		}()
	}
	wg.Wait()
	close(errs)
	ok, limited := 0, 0
	for err := range errs {
		if err == nil {
			ok++
		} else {
			assert.ErrorIs(t, err, domain.ErrReportLimit)
			limited++
		}
	}
	assert.Equal(t, 2, ok)
	assert.Equal(t, 3, limited)

	list, err := rs.ListReports(ctx, "ses_a")
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, []int{1, 2, 3}, []int{list[0].Seq, list[1].Seq, list[2].Seq})
	assert.Equal(t, []string{"Résumé", "Tests"}, list[0].Sections)
	assert.True(t, list[0].CreatedAt.Equal(at))

	// Byte limit.
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_b", ProjectID: "p1", Status: domain.SessionStatusRunning}))
	require.NoError(t, rs.AddReport(ctx, &domain.Report{SessionID: "ses_b", Bytes: 60}, 0, 100))
	assert.ErrorIs(t, rs.AddReport(ctx, &domain.Report{SessionID: "ses_b", Bytes: 41}, 0, 100), domain.ErrReportLimit)

	// Unknown session.
	assert.ErrorIs(t, rs.AddReport(ctx, &domain.Report{SessionID: "ses_x", Bytes: 1}, 0, 0), domain.ErrNotFound)

	got, err := rs.GetReport(ctx, "ses_a", 1)
	require.NoError(t, err)
	assert.Equal(t, "developer", got.Agent)
	assert.Equal(t, "ses_child", got.ToolSession)
	_, err = rs.GetReport(ctx, "ses_a", 9)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	// Import keeps the number and never replaces a report.
	added, err := rs.ImportReport(ctx, &domain.Report{SessionID: "ses_b", Seq: 7, Agent: "developer", CreatedAt: at})
	require.NoError(t, err)
	assert.True(t, added)
	added, err = rs.ImportReport(ctx, &domain.Report{SessionID: "ses_b", Seq: 7, Agent: "intrus", CreatedAt: at})
	require.NoError(t, err)
	assert.False(t, added)
	got, err = rs.GetReport(ctx, "ses_b", 7)
	require.NoError(t, err)
	assert.Equal(t, "developer", got.Agent)

	require.NoError(t, rs.DeleteReport(ctx, "ses_b", 7))
	_, err = rs.GetReport(ctx, "ses_b", 7)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	// The reports go with their session.
	require.NoError(t, sessions.Delete(ctx, "ses_a"))
	list, err = rs.ListReports(ctx, "ses_a")
	require.NoError(t, err)
	assert.Empty(t, list)
}
