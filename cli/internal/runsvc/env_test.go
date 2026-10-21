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
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
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
	st := sqlitetest.New(t)
	sqlitetest.Seed(t, st).Project("p1", "/p1")
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

func TestBeadsAllowKeptForResumes(t *testing.T) {
	ctx := context.Background()
	st := sqlitetest.New(t)
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	sessions := sqlite.NewSessionStore(st)
	var calls []SessionEnvRequest
	svc := &Service{Adapter: &envAdapter{}, Sessions: sessions, SessionsDir: t.TempDir(),
		SessionEnv: func(_ context.Context, r SessionEnvRequest) (map[string]string, error) {
			calls = append(calls, r)
			return nil, nil
		}}
	require.NoError(t, svc.saveBeadsAllow("ses_a", []string{"show", "update"}))
	require.NoError(t, svc.saveBeadsAllow("ses_b", []string{}))
	require.NoError(t, svc.saveBeadsAllow("ses_c", nil))
	for _, id := range []string{"ses_a", "ses_b", "ses_c"} {
		require.NoError(t, sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", GroupKey: "g1", Runtime: "container",
			WorkflowID: "ticket", LaunchPath: "/p1", State: domain.RunIdle, Status: domain.SessionStatusRunning}))
	}
	require.NoError(t, svc.reapplyGroupEnv(ctx, &domain.Server{GroupKey: "g1", ProjectID: "p1"}, ""))
	got := map[string]SessionEnvRequest{}
	for _, c := range calls {
		got[c.SessionID] = c
	}
	assert.Equal(t, []string{"show", "update"}, got["ses_a"].BeadsAllow)
	assert.NotNil(t, got["ses_b"].BeadsAllow, "explicit empty list")
	assert.Empty(t, got["ses_b"].BeadsAllow)
	assert.Nil(t, got["ses_c"].BeadsAllow, "not declared")
	assert.Equal(t, "container", string(got["ses_a"].Runtime))
	assert.Equal(t, "ticket", got["ses_a"].WorkflowID)

	svc.removeStaticEnv("ses_a")
	allow, err := svc.loadBeadsAllow("ses_a")
	require.NoError(t, err)
	assert.Nil(t, allow)
}

func TestWithMachineShellEnv(t *testing.T) {
	t.Setenv("PATH", "/opt/tools/bin:/usr/bin")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	env := withMachineShellEnv(sessionspec.RuntimeLocal, map[string]string{"FOO": "bar", "HOME": "/custom"}, nil)
	assert.Equal(t, "/opt/tools/bin:/usr/bin", env["PATH"])
	assert.Equal(t, "/custom", env["HOME"], "static variables win")
	assert.Equal(t, "bar", env["FOO"])
	assert.NotContains(t, env, "AWS_SECRET_ACCESS_KEY")

	static := map[string]string{"FOO": "bar"}
	assert.Equal(t, static, withMachineShellEnv(sessionspec.RuntimeContainer, static, []string{"/shims"}), "container: machine PATH is meaningless inside")
}

// QB1: the fake bd comes first on the PATH of local sessions (kept in the
// static environment: resumes and subagents get it too).
func TestWithMachineShellEnvPathFirst(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")
	env := withMachineShellEnv(sessionspec.RuntimeLocal, nil, []string{"/oh/run/bin"})
	assert.Equal(t, "/oh/run/bin"+string(os.PathListSeparator)+"/usr/bin", env["PATH"])
}

// S-28: the shell of a local session gets the same allow-list as the tool
// server: the base list, plus [execution] env_allow. A secret of the user's
// shell does not reach the agents, a variable they allowed does.
func TestWithMachineShellEnvAllowList(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("HTTPS_PROXY", "http://proxy:3128")
	t.Setenv("LC_ALL", "C")
	t.Setenv("GEMINI_API_KEY", "g-secret")
	t.Setenv("GH_TOKEN", "gh-secret")
	t.Setenv("SSH_AUTH_SOCK", "/tmp/agent")
	t.Setenv("MYCO_REGION", "eu")
	t.Setenv("ANTHROPIC_API_KEY", "a-secret")

	env := withMachineShellEnv(sessionspec.RuntimeLocal, nil, nil)
	assert.Equal(t, "http://proxy:3128", env["HTTPS_PROXY"])
	assert.Equal(t, "C", env["LC_ALL"])
	for _, k := range []string{"GEMINI_API_KEY", "GH_TOKEN", "SSH_AUTH_SOCK", "MYCO_REGION", "ANTHROPIC_API_KEY"} {
		assert.NotContains(t, env, k)
	}

	env = withMachineShellEnv(sessionspec.RuntimeLocal, nil, nil, "SSH_AUTH_SOCK", "MYCO_*", "GEMINI_API_KEY", "ANTHROPIC_API_KEY")
	assert.Equal(t, "/tmp/agent", env["SSH_AUTH_SOCK"])
	assert.Equal(t, "eu", env["MYCO_REGION"])
	assert.Equal(t, "g-secret", env["GEMINI_API_KEY"], "the user decides")
	assert.NotContains(t, env, "GH_TOKEN")
	assert.NotContains(t, env, "ANTHROPIC_API_KEY", "the credentials oh manages stay behind the proxy")
}
