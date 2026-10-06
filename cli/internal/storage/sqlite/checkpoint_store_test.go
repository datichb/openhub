package sqlite

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestCheckpointStore(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	require.NoError(t, NewSessionStore(st).Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning}))
	cs := NewCheckpointStore(st)

	got, err := cs.GetCheckpointState(ctx, "ses_a")
	require.NoError(t, err)
	assert.Equal(t, domain.CheckpointState{}, got)
	_, err = cs.GetCheckpointState(ctx, "ses_x")
	assert.ErrorIs(t, err, domain.ErrNotFound)

	at := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	_, err = cs.UpdateCheckpointState(ctx, "ses_a", func(s *domain.CheckpointState) error {
		s.Passed = map[string]time.Time{"cp-1": at}
		return nil
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := cs.UpdateCheckpointState(ctx, "ses_a", func(s *domain.CheckpointState) error { s.Consecutive++; return nil })
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
	got, err = cs.GetCheckpointState(ctx, "ses_a")
	require.NoError(t, err)
	assert.Equal(t, 10, got.Consecutive, "no update lost")
	assert.True(t, got.HasPassed("cp-1"))
	assert.True(t, got.Passed["cp-1"].Equal(at))
}

func TestSetSessionOutput(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	sessions := NewSessionStore(st)
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning}))
	cs := NewCheckpointStore(st)
	require.NoError(t, cs.SetSessionOutput(ctx, "ses_a", "branch", "feat/x"))
	require.NoError(t, cs.SetSessionOutput(ctx, "ses_a", "tickets", []string{"bd-1", "bd-2"}))
	require.NoError(t, cs.SetSessionOutput(ctx, "ses_a", "a.b", "dotted key"))

	// A later full update of the row keeps them.
	s, err := sessions.Get(ctx, "ses_a")
	require.NoError(t, err)
	s.Outputs = nil
	s.State = domain.RunIdle
	require.NoError(t, sessions.Update(ctx, s))
	s, err = sessions.Get(ctx, "ses_a")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"branch": "feat/x", "tickets": []any{"bd-1", "bd-2"}, "a.b": "dotted key"}, s.Outputs)
	assert.ErrorIs(t, cs.SetSessionOutput(ctx, "ses_x", "k", "v"), domain.ErrNotFound)
}
