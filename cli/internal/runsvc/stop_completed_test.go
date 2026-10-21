package runsvc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

// T4: a session completed by the end of its workflow already ran its end
// (team claims, daemon): stopping it afterwards does not run it again.
func TestStopCompletedSessionDoesNotEndItTwice(t *testing.T) {
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	sessions := sqlite.NewSessionStore(st)
	for id, state := range map[string]domain.RunState{"ses_done": domain.RunCompleted, "ses_open": domain.RunIdle} {
		require.NoError(t, sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: state}))
	}
	var ended []string
	svc := &Service{Sessions: sessions, Servers: sqlite.NewServerStore(st),
		OnSessionEnd: func(_ context.Context, s domain.Session) { ended = append(ended, s.ID) }}
	require.NoError(t, svc.StopSession(ctx, "ses_done"))
	require.NoError(t, svc.StopSession(ctx, "ses_open"))
	assert.Equal(t, []string{"ses_open"}, ended)
	s, err := sessions.Get(ctx, "ses_done")
	require.NoError(t, err)
	assert.Equal(t, domain.RunStopped, s.State)
}
