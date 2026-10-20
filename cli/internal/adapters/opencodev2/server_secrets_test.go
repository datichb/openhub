package opencodev2

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/testenv"
)

// Regression test of the leak of 2026-09-29: the Bedrock key was in the
// environment of oh, the tool server inherited it and an agent read it with
// `env` in its shell. Since then an allow-list (internal/envfilter) filters
// what the server inherits. These tests go through the real ways the
// environment of the server is built, with fake credentials of every kind
// in the process environment: the value of none of them may appear in the
// environment of the server, under any name.

// wideEnvAllow is what a user in a hurry writes: the suffix patterns are not
// patterns (PREFIX_* only), AWS_* is one.
var wideEnvAllow = []string{"AWS_*", "*_TOKEN", "*_KEY"}

// dumpEnvServer is a "server" that writes its environment to a file and
// stops. It returns the binary and a function that reads the environment
// it was given.
func dumpEnvServer(t *testing.T) (bin string, dumped func() []string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("a shell script stands for the server")
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "env.txt")
	bin = filepath.Join(dir, "fake-opencode")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\nenv > '"+out+"'\nexit 1\n"), 0o755))
	return bin, func() []string {
		data, err := os.ReadFile(out)
		require.NoError(t, err, "the server was started")
		return strings.Split(strings.TrimSpace(string(data)), "\n")
	}
}

func TestServerEnvironmentHoldsNoCredential(t *testing.T) {
	secrets := testenv.SetFakeSecrets(t)
	t.Setenv("LANG", "fr_FR.UTF-8")

	for name, extra := range map[string][]string{"base list": nil, "wide env_allow": wideEnvAllow} {
		t.Run(name, func(t *testing.T) {
			// buildEnv: the environment of a server run on the machine.
			env := buildEnv(os.Environ(), ServerOptions{DataDir: "/data", EnvAllow: extra}, "pw")
			assert.Empty(t, testenv.Leaks(secrets, env))
			assert.Contains(t, env, "LANG=fr_FR.UTF-8", "a variable of the base list passes")
			assert.Contains(t, env, "OPENCODE_SERVER_PASSWORD=pw", "oh's settings are there")

			// StartServer: the process really started.
			bin, dumped := dumpEnvServer(t)
			root := t.TempDir()
			_, err := StartServer(t.Context(), ServerOptions{Binary: bin, WorkDir: root, DataDir: filepath.Join(root, "data"),
				EnvAllow: extra, ReadyTimeout: 20 * time.Second})
			require.ErrorIs(t, err, errServerExited)
			got := dumped()
			assert.Empty(t, testenv.Leaks(secrets, got))
			assert.Contains(t, got, "LANG=fr_FR.UTF-8")
			for _, n := range testenv.FakeSecretNames {
				assert.NotContains(t, envNames(got), n, "no variable of this name either: oh sets none itself here")
			}
		})
	}
}

// The adapter hands the env_allow of the session to the server, and the
// credentials that oh itself provides (the session token of the credential
// proxy) are the only ones in the environment.
func TestAdapterServerEnvironmentHoldsNoCredential(t *testing.T) {
	secrets := testenv.SetFakeSecrets(t)
	t.Setenv("LANG", "fr_FR.UTF-8")

	for name, extra := range map[string][]string{"base list": nil, "wide env_allow": wideEnvAllow} {
		t.Run(name, func(t *testing.T) {
			bin, dumped := dumpEnvServer(t)
			root := t.TempDir()
			a := &Adapter{Binary: bin, CacheDir: t.TempDir(), DisablePlugin: true}
			_, err := a.StartServer(t.Context(), adapters.ServerGroup{
				Key:     sessionspec.GroupKey{BundleHash: "h", ProjectID: "p1"},
				Bundle:  sessionspec.BundleSpec{Root: root, EntryAgent: "lead", Agents: []sessionspec.AgentDef{{ID: "lead", Mode: "primary"}}},
				DataDir: filepath.Join(root, "data"), WorkDir: root,
				Provider: sessionspec.ProviderSpec{ID: "amazon-bedrock", Region: "eu-west-1", BaseURL: "http://127.0.0.1:5555/amazon-bedrock", SessionToken: "ohs_session"},
				Env:      map[string]string{"AWS_BEARER_TOKEN_BEDROCK": "ohs_session"},
				EnvAllow: extra,
			})
			require.Error(t, err)
			got := dumped()
			assert.Empty(t, testenv.Leaks(secrets, got))
			assert.Contains(t, got, "LANG=fr_FR.UTF-8")
			assert.Contains(t, got, "AWS_BEARER_TOKEN_BEDROCK=ohs_session", "the token of the proxy session, not the one of the user")
		})
	}
}

// In a container or a job the server environment is built from nothing: oh's
// settings only. Run is the way the runtime starts the process; the
// environment it is given is the one of the container.
func TestRuntimeServerEnvironmentHoldsNoCredential(t *testing.T) {
	secrets := testenv.SetFakeSecrets(t)
	t.Setenv("LANG", "fr_FR.UTF-8")

	for name, extra := range map[string][]string{"base list": nil, "wide env_allow": wideEnvAllow} {
		t.Run(name, func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skip("the runtime command is a shell command")
			}
			assert.Empty(t, testenv.LeaksInMap(secrets, runtimeEnv(ServerOptions{
				EnvAllow: extra, ConfigContent: `{"a":1}`, Env: map[string]string{"AWS_BEARER_TOKEN_BEDROCK": "ohs_session"},
			}, "pw", "/opt/oh/data")))

			root := t.TempDir()
			var given map[string]string
			_, err := StartServer(t.Context(), ServerOptions{
				WorkDir: root, DataDir: filepath.Join(root, "data"), EnvAllow: extra, ReadyTimeout: 20 * time.Second,
				Env:   map[string]string{"AWS_BEARER_TOKEN_BEDROCK": "ohs_session"},
				Paths: ohruntime.PathMap{{Host: root, Inner: root}}, // the whole tree is visible as such
				Run: func(_ []string, env map[string]string, _ string, _ int) (*exec.Cmd, error) {
					given = env
					return exec.Command("/bin/sh", "-c", "exit 1"), nil
				},
			})
			require.ErrorIs(t, err, errServerExited)
			require.NotEmpty(t, given)
			assert.Empty(t, testenv.LeaksInMap(secrets, given))
			assert.NotContains(t, given, "LANG", "nothing of the machine is inherited, not even the base list")
			assert.Equal(t, "ohs_session", given["AWS_BEARER_TOKEN_BEDROCK"])
		})
	}
}

func envNames(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		n, _, _ := strings.Cut(kv, "=")
		out = append(out, n)
	}
	return out
}
