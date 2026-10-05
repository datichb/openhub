package opencodev2

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
