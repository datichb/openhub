package opencodev2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
)

func cliConfigOf(t *testing.T, env []string) map[string]any {
	t.Helper()
	v := lookupEnv(env, cliConfigEnv)
	require.NotEmpty(t, v)
	cfg := map[string]any{}
	require.NoError(t, json.Unmarshal([]byte(v), &cfg))
	return cfg
}

// A43: the clients opened by oh have no session tabs (tabs remembered per
// directory, whatever the server, piled up in the open window).
func TestSingleSessionAttachTurnsTabsOff(t *testing.T) {
	a := &Adapter{Binary: "/bin/opencode"}
	h := adapters.ServerHandle{URL: "http://127.0.0.1:4096", Password: "pw"}
	argv, env := a.SingleSessionAttachCommand(h, "ses_1", []string{"HOME=/h"})
	plainArgv, plainEnv := a.AttachCommand(h, "ses_1")
	assert.Equal(t, plainArgv, argv)
	assert.Subset(t, env, plainEnv)
	assert.Equal(t, map[string]any{"tabs": map[string]any{"mode": "off"}}, cliConfigOf(t, env))
}

func TestSingleSessionAttachKeepsUserCLIConfig(t *testing.T) {
	a := &Adapter{}
	base := []string{cliConfigEnv + `={"theme":{"name":"x"},"tabs":{"enabled":true,"layout":"vertical"}}`}
	_, env := a.SingleSessionAttachCommand(adapters.ServerHandle{}, "ses_1", base)
	cfg := cliConfigOf(t, env)
	assert.Equal(t, map[string]any{"name": "x"}, cfg["theme"])
	assert.Equal(t, map[string]any{"mode": "off", "layout": "vertical"}, cfg["tabs"])

	_, env = a.SingleSessionAttachCommand(adapters.ServerHandle{}, "ses_1", []string{cliConfigEnv + "=not json"})
	assert.Equal(t, map[string]any{"tabs": map[string]any{"mode": "off"}}, cliConfigOf(t, env))
}
