package runsvc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// envAdapter records SetSessionEnv calls (other methods are not used).
type envAdapter struct {
	adapters.ToolAdapter
	got map[string]map[string]string
}

func (e *envAdapter) SetSessionEnv(_ context.Context, _ adapters.ServerHandle, id string, env map[string]string) error {
	if e.got == nil {
		e.got = map[string]map[string]string{}
	}
	e.got[id] = env
	return nil
}

func TestSessionEnvBuildPersistReapply(t *testing.T) {
	ctx := context.Background()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	sessions := sqlite.NewSessionStore(st)

	ad := &envAdapter{}
	var calls []SessionEnvRequest
	svc := &Service{Adapter: ad, Sessions: sessions, SessionsDir: t.TempDir(),
		SessionEnv: func(_ context.Context, r SessionEnvRequest) (map[string]string, error) {
			calls = append(calls, r)
			return map[string]string{"OH_GATEWAY_TOKEN": "tok-" + r.SessionID, EnvSessionID: "spoofed"}, nil
		}}

	env, err := svc.buildSessionEnv(ctx, map[string]string{"FOO": "bar"}, SessionEnvRequest{SessionID: "ses_a", GroupKey: "g1"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"FOO": "bar", "OH_GATEWAY_TOKEN": "tok-ses_a", EnvSessionID: "ses_a"}, env)

	require.NoError(t, svc.saveStaticEnv("ses_a", map[string]string{"FOO": "bar"}))
	info, err := os.Stat(filepath.Join(svc.SessionsDir, "ses_a", "env.json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	for _, s := range []domain.Session{
		{ID: "ses_a", ProjectID: "p1", GroupKey: "g1", State: domain.RunSleeping, Status: domain.SessionStatusRunning},
		{ID: "ses_b", ProjectID: "p1", GroupKey: "g1", State: domain.RunStopped, Status: domain.SessionStatusCompleted},
		{ID: "ses_c", ProjectID: "p1", GroupKey: "g2", State: domain.RunIdle, Status: domain.SessionStatusRunning},
	} {
		s := s
		require.NoError(t, sessions.Create(ctx, &s))
	}
	require.NoError(t, svc.reapplyGroupEnv(ctx, &domain.Server{GroupKey: "g1", ProjectID: "p1"}, ""))
	assert.Equal(t, map[string]map[string]string{"ses_a": {"FOO": "bar", "OH_GATEWAY_TOKEN": "tok-ses_a", EnvSessionID: "ses_a"}}, ad.got,
		"only open sessions of the restarted group")
	assert.True(t, calls[len(calls)-1].Resume)

	svc.removeStaticEnv("ses_a")
	got, err := svc.loadStaticEnv("ses_a")
	require.NoError(t, err)
	assert.Nil(t, got)

	svc.SessionEnv = func(context.Context, SessionEnvRequest) (map[string]string, error) {
		return nil, errors.New("no gateway")
	}
	_, err = svc.buildSessionEnv(ctx, nil, SessionEnvRequest{SessionID: "ses_x"})
	assert.ErrorContains(t, err, "no gateway")
}
