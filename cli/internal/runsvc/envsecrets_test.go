package runsvc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/testenv"
)

// Regression test of the leak of 2026-09-29: the Bedrock key was in the
// environment of oh and an agent read it with `env` in its shell. Since then
// the shell of a local session gets an allow-list of the environment
// (internal/envfilter), and a container or a job gets none of it. This test
// starts sessions through the service, with fake credentials of every kind
// in the process environment, and looks for their values everywhere the
// service hands an environment on: the server group, the environment of the
// session (what the shells of the agents get) and the copy kept on disk for
// the resumes.

// wideEnvAllow is what a user in a hurry writes: the suffix patterns are not
// patterns (PREFIX_* only), AWS_* is one.
var wideEnvAllow = []string{"AWS_*", "*_TOKEN", "*_KEY"}

func TestSessionEnvironmentHoldsNoCredential(t *testing.T) {
	secrets := testenv.SetFakeSecrets(t)
	t.Setenv("LANG", "fr_FR.UTF-8")

	for _, kind := range []sessionspec.RuntimeKind{sessionspec.RuntimeLocal, sessionspec.RuntimeContainer} {
		for name, extra := range map[string][]string{"base list": nil, "wide env_allow": wideEnvAllow} {
			t.Run(string(kind)+"/"+name, func(t *testing.T) {
				f := newRTFixture(t)
				f.svc.SessionsDir = t.TempDir()
				req := f.request(f.project)
				req.Runtime = kind
				req.EnvAllow = extra
				res, err := f.svc.StartSession(context.Background(), req)
				require.NoError(t, err)

				// The server group: the environment oh adds to the tool's.
				require.Len(t, f.ad.started, 1)
				sg := f.ad.started[0]
				assert.Empty(t, testenv.LeaksInMap(secrets, sg.Env))
				assert.Equal(t, extra, sg.EnvAllow, "the names the user allowed, never values")

				// The shell of the agents.
				require.Len(t, f.ad.created, 1)
				env := f.ad.created[0].SessionEnv
				assert.Empty(t, testenv.LeaksInMap(secrets, env))
				if kind == sessionspec.RuntimeLocal {
					assert.Equal(t, "fr_FR.UTF-8", env["LANG"], "a variable of the base list passes on the machine")
				} else {
					assert.NotContains(t, env, "LANG", "a container inherits nothing of the machine")
				}
				assert.Equal(t, res.SessionID, env[EnvSessionID])

				// What is kept for the resumes.
				data, err := os.ReadFile(filepath.Join(f.svc.SessionsDir, res.SessionID, "env.json"))
				if kind == sessionspec.RuntimeLocal {
					require.NoError(t, err)
					assert.Empty(t, testenv.LeaksIn(secrets, string(data)))
				}
				static, err := f.svc.loadStaticEnv(res.SessionID)
				require.NoError(t, err)
				assert.Empty(t, testenv.LeaksInMap(secrets, static))
			})
		}
	}
}

// The part of the shell environment that comes from the machine, on its own:
// the wide env_allow lets no credential through, and a name the user wrote
// passes (his decision, `oh doctor` warns), except the credentials oh manages.
func TestMachineShellEnvironmentHoldsNoCredential(t *testing.T) {
	secrets := testenv.SetFakeSecrets(t)
	got := withMachineShellEnv(sessionspec.RuntimeLocal, map[string]string{"FOO": "bar"}, []string{"/oh/run/bin"}, wideEnvAllow...)
	assert.Empty(t, testenv.LeaksInMap(secrets, got))
	got = withMachineShellEnv(sessionspec.RuntimeLocal, nil, nil, "GH_*", "GEMINI_API_KEY")
	assert.Equal(t, secrets["GH_TOKEN"], got["GH_TOKEN"], "a name the user wrote passes: his decision (oh doctor warns)")
	assert.Equal(t, secrets["GEMINI_API_KEY"], got["GEMINI_API_KEY"])
	for _, n := range []string{"AWS_BEARER_TOKEN_BEDROCK", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GITHUB_TOKEN", "GITLAB_TOKEN"} {
		assert.NotContains(t, got, n)
	}
}
