package runsvc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

type policyDaemon struct {
	DaemonClient
	policies map[string]daemon.QuitPolicy
}

func (p *policyDaemon) SetPolicy(_ context.Context, group string, pol daemon.QuitPolicy) error {
	p.policies[group] = pol
	return nil
}

func TestQuitPlanAndApply(t *testing.T) {
	ctx := context.Background()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	sessions := sqlite.NewSessionStore(st)
	for _, s := range []domain.Session{
		{ID: "ses_work1", GroupKey: "g1", State: domain.RunActive},
		{ID: "ses_work2", GroupKey: "g2", State: domain.RunActive},
		{ID: "ses_work3", GroupKey: "g2", State: domain.RunActive},
		{ID: "ses_wait", GroupKey: "g3", State: domain.RunWaiting},
		{ID: "ses_sleep", GroupKey: "g4", State: domain.RunSleeping},
		{ID: "ses_done", GroupKey: "g5", State: domain.RunStopped},
		{ID: "legacy", State: domain.RunActive},
	} {
		s.ProjectID, s.Status = "p1", domain.SessionStatusRunning
		require.NoError(t, sessions.Create(ctx, &s))
	}
	dc := &policyDaemon{policies: map[string]daemon.QuitPolicy{}}
	svc := &Service{Sessions: sessions, Servers: sqlite.NewServerStore(st), Daemon: func(context.Context) (DaemonClient, error) { return dc, nil }}

	plan, err := svc.QuitPlan(ctx)
	require.NoError(t, err)
	assert.Len(t, plan.Working, 3)
	require.Len(t, plan.Resting, 1)
	assert.Equal(t, "ses_wait", plan.Resting[0].ID)
	assert.ElementsMatch(t, []string{"g1", "g2", "g3"}, plan.Groups)

	require.NoError(t, svc.ApplyQuit(ctx, plan, map[string]QuitChoice{
		"ses_work1": QuitStop, "ses_work2": QuitBackground, // ses_work3: default (finish)
	}))
	assert.Equal(t, map[string]daemon.QuitPolicy{
		"g1": daemon.PolicySleepWhenIdle, "g2": daemon.PolicyBackground, "g3": daemon.PolicySleepWhenIdle,
	}, dc.policies, "a group with a session sent to the background keeps running")
	s, _ := sessions.Get(ctx, "ses_work1")
	assert.Equal(t, domain.RunStopped, s.State)
	s, _ = sessions.Get(ctx, "ses_work3")
	assert.Equal(t, domain.RunActive, s.State)
}
