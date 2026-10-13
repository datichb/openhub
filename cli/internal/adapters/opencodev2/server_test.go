package opencodev2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func envMap(env []string) map[string]string {
	m := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		m[k] = v
	}
	return m
}

func TestBuildEnvStripsSecretsAndToolVars(t *testing.T) {
	parent := []string{
		"PATH=/usr/bin", "HOME=/home/u",
		"AWS_BEARER_TOKEN_BEDROCK=real-secret", "ANTHROPIC_API_KEY=k", "GITLAB_TOKEN=g",
		"OPENCODE_CONFIG=/x.json", "OPENCODE_SERVER_PASSWORD=old", "XDG_DATA_HOME=/old",
		"XDG_CONFIG_HOME=/cfg",
	}
	opts := ServerOptions{
		DataDir:       "/data",
		ConfigContent: `{"a":1}`,
		Env:           map[string]string{"AWS_BEARER_TOKEN_BEDROCK": "session-token"},
	}
	m := envMap(buildEnv(parent, opts, "pw"))

	assert.Equal(t, "/usr/bin", m["PATH"])
	assert.Equal(t, "/home/u", m["HOME"])
	assert.Equal(t, "session-token", m["AWS_BEARER_TOKEN_BEDROCK"], "only the explicit proxy token reaches the tool")
	assert.NotContains(t, m, "ANTHROPIC_API_KEY")
	assert.NotContains(t, m, "GITLAB_TOKEN")
	assert.NotContains(t, m, "OPENCODE_CONFIG")
	assert.Equal(t, "pw", m["OPENCODE_SERVER_PASSWORD"])
	assert.Equal(t, "1", m["OPENCODE_DISABLE_PROJECT_CONFIG"])
	assert.Equal(t, "true", m["OPENCODE_DISABLE_AUTOUPDATE"])
	assert.Equal(t, "/data", m["XDG_DATA_HOME"])
	assert.Equal(t, `{"a":1}`, m["OPENCODE_CONFIG_CONTENT"])
	assert.Equal(t, "/cfg", m["XDG_CONFIG_HOME"], "config home kept unless strict isolation")
}

func TestBuildEnvStrictConfigHome(t *testing.T) {
	m := envMap(buildEnv([]string{"XDG_CONFIG_HOME=/cfg"}, ServerOptions{DataDir: "/d", ConfigHome: "/iso"}, "pw"))
	assert.Equal(t, "/iso", m["XDG_CONFIG_HOME"])
	assert.NotContains(t, m, "OPENCODE_CONFIG_CONTENT")
}

func TestRandomPasswordAndFreePort(t *testing.T) {
	a, b := RandomPassword(), RandomPassword()
	assert.Len(t, a, 64)
	assert.NotEqual(t, a, b)
	p, err := FreePort()
	assert.NoError(t, err)
	assert.Greater(t, p, 0)
}

func TestStartServerValidatesOptions(t *testing.T) {
	_, err := StartServer(t.Context(), ServerOptions{})
	assert.Error(t, err)
}

// A port picked by oh and taken before the server binds it: another port,
// three times at most; any other startup failure, or a port given, is not
// tried again.
func TestStartServerRetriesATakenPort(t *testing.T) {
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	fake := func(name, msg string) string {
		bin := filepath.Join(dir, name)
		script := "#!/bin/sh\necho \"$5\" >> " + calls + "\necho \"" + msg + "\" >&2\nexit 1\n"
		require.NoError(t, os.WriteFile(bin, []byte(script), 0o755))
		return bin
	}
	ports := func() []string {
		data, _ := os.ReadFile(calls)
		_ = os.Remove(calls)
		return strings.Fields(string(data))
	}
	opts := func(bin string) ServerOptions {
		return ServerOptions{Binary: bin, WorkDir: dir, DataDir: filepath.Join(dir, "data"), ReadyTimeout: 20 * time.Second}
	}

	_, err := StartServer(t.Context(), opts(fake("taken", "error: Failed to start server. Is port in use? (EADDRINUSE)")))
	require.ErrorIs(t, err, errPortTaken)
	assert.Len(t, ports(), portAttempts)

	_, err = StartServer(t.Context(), opts(fake("broken", "error: no provider")))
	require.ErrorIs(t, err, errServerExited)
	assert.NotErrorIs(t, err, errPortTaken)
	assert.Contains(t, err.Error(), "no provider", "the log tail")
	assert.Len(t, ports(), 1)

	o := opts(filepath.Join(dir, "taken"))
	o.Port = 4321
	_, err = StartServer(t.Context(), o)
	require.ErrorIs(t, err, errPortTaken)
	assert.Equal(t, []string{"4321"}, ports(), "a port given is not changed")

	assert.True(t, portTaken("Error: listen EADDRINUSE: address already in use 127.0.0.1:4096"))
	assert.True(t, portTaken("Bind for 0.0.0.0:4096 failed: port is already allocated"))
	assert.False(t, portTaken("error: no provider"))
}
