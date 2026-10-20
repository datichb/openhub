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

// S-28: the environment of the server is an allow-list. A secret the user
// has in the shell, whatever its name, is not handed to the tool and to the
// shells of the agents.
func TestBuildEnvAllowListDropsSecretsOutsideTheList(t *testing.T) {
	parent := []string{
		"PATH=/usr/bin", "HOME=/home/u", "USER=u", "SHELL=/bin/zsh", "TERM=xterm", "LANG=fr_FR.UTF-8", "LC_ALL=C",
		"TMPDIR=/tmp/x", "TZ=Europe/Paris", "XDG_CACHE_HOME=/cache", "HTTPS_PROXY=http://proxy:3128", "NO_PROXY=localhost",
		"SSL_CERT_FILE=/etc/ca.pem", "NODE_EXTRA_CA_CERTS=/etc/ca2.pem", "OH_HOME=/oh",
		"GEMINI_API_KEY=g-secret", "GH_TOKEN=gh-secret", "NPM_TOKEN=npm-secret", "SLACK_BOT_TOKEN=s-secret",
		"DATABASE_URL=postgres://u:p@h/db", "SSH_AUTH_SOCK=/tmp/agent", "OH_LLM_KEY=oh-secret", "ZDOTDIR=/z", "BASH_ENV=/b", "ENV=/e",
	}
	m := envMap(buildEnv(parent, ServerOptions{DataDir: "/data"}, "pw"))
	for _, k := range []string{"PATH", "HOME", "USER", "SHELL", "TERM", "LANG", "LC_ALL", "TMPDIR", "TZ", "XDG_CACHE_HOME",
		"HTTPS_PROXY", "NO_PROXY", "SSL_CERT_FILE", "NODE_EXTRA_CA_CERTS", "OH_HOME"} {
		assert.Contains(t, m, k)
	}
	for _, k := range []string{"GEMINI_API_KEY", "GH_TOKEN", "NPM_TOKEN", "SLACK_BOT_TOKEN", "DATABASE_URL", "SSH_AUTH_SOCK", "OH_LLM_KEY",
		"ZDOTDIR", "BASH_ENV", "ENV"} {
		assert.NotContains(t, m, k)
	}
	for k, v := range m {
		assert.NotContains(t, v, "secret", k)
	}
	assert.Equal(t, "/data", m["XDG_DATA_HOME"])
}

func TestBuildEnvAllowAddsNamesAndPatterns(t *testing.T) {
	parent := []string{
		"PATH=/usr/bin", "SSH_AUTH_SOCK=/tmp/agent", "MYCO_REGION=eu", "MYCO_TEAM=t", "MYCOX=no", "GEMINI_API_KEY=g-secret", "OTHER=1",
		"ANTHROPIC_API_KEY=a-secret", "AWS_REGION=eu-west-1", "AWS_SECRET_ACCESS_KEY=aws-secret",
	}
	m := envMap(buildEnv(parent, ServerOptions{DataDir: "/data", EnvAllow: []string{"SSH_AUTH_SOCK", "MYCO_*", "GEMINI_API_KEY", "ANTHROPIC_API_KEY", "AWS_*"}}, "pw"))
	assert.Equal(t, "/tmp/agent", m["SSH_AUTH_SOCK"], "a name added by env_allow")
	assert.Equal(t, "eu", m["MYCO_REGION"], "a pattern added by env_allow")
	assert.Equal(t, "t", m["MYCO_TEAM"])
	assert.NotContains(t, m, "MYCOX", "the pattern is a prefix with its underscore")
	assert.NotContains(t, m, "OTHER")
	assert.Equal(t, "g-secret", m["GEMINI_API_KEY"], "the user decides for a variable oh does not manage")
	assert.Equal(t, "eu-west-1", m["AWS_REGION"])
	assert.NotContains(t, m, "ANTHROPIC_API_KEY", "the credentials oh manages stay behind the proxy")
	assert.NotContains(t, m, "AWS_SECRET_ACCESS_KEY")
}

// The tool variables stay under oh's control whatever env_allow says.
func TestBuildEnvAllowDoesNotLetToolVariablesIn(t *testing.T) {
	parent := []string{"OPENCODE_CONFIG=/x", "OPENCODE_SERVER_PASSWORD=old", "XDG_DATA_HOME=/old", "XDG_CONFIG_HOME=/cfg"}
	m := envMap(buildEnv(parent, ServerOptions{DataDir: "/d", ConfigHome: "/iso", EnvAllow: []string{"OPENCODE_*", "XDG_DATA_HOME"}}, "pw"))
	assert.NotContains(t, m, "OPENCODE_CONFIG")
	assert.Equal(t, "pw", m["OPENCODE_SERVER_PASSWORD"])
	assert.Equal(t, "/d", m["XDG_DATA_HOME"])
	assert.Equal(t, "/iso", m["XDG_CONFIG_HOME"])
}
