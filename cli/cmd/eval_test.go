package cmd

import (
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
)

func TestEvalSettings(t *testing.T) {
	t.Setenv(envEvalModels, "")
	assert.Equal(t, []string{defaultEvalModel}, evalModels())
	t.Setenv(envEvalModels, " claude-haiku-4-5 , claude-sonnet-4-6,")
	assert.Equal(t, []string{"claude-haiku-4-5", "claude-sonnet-4-6"}, evalModels())
	t.Setenv(envEvalReps, "3")
	assert.Equal(t, 3, envInt(envEvalReps, 1))
	t.Setenv(envEvalReps, "x")
	assert.Equal(t, 1, envInt(envEvalReps, 1))
	t.Setenv(envEvalBudget, "0.25")
	assert.InDelta(t, 0.25, envFloat(envEvalBudget, 1), 1e-9)

	assert.Equal(t, int64(200000), evalMaxTokens("claude-haiku-4-5", 1))
	assert.Equal(t, int64(66667), evalMaxTokens("claude-sonnet-4-6", 1))
	assert.Zero(t, evalMaxTokens("claude-haiku-4-5", 0))
}

// The hub.toml of a pass home: Bedrock EU, no client, no notification, the
// session budget of the pass.
func TestEvalHubConfig(t *testing.T) {
	var cfg config.Config
	err := toml.Unmarshal([]byte(evalHubConfig(0.5)), &cfg)
	require.NoError(t, err)
	assert.Equal(t, "bedrock", cfg.LLM.DefaultProvider)
	assert.Equal(t, evalRegion, cfg.Provider.Bedrock.AWSRegion)
	assert.Equal(t, "none", cfg.Session.Attach)
	assert.Equal(t, "off", cfg.Session.Notify)
	assert.InDelta(t, 0.5, cfg.Limits.SessionBudgetUSD, 1e-9)
}

// A pass never sees the oh settings nor the HOME of the user.
func TestEvalPassEnv(t *testing.T) {
	t.Setenv("OH_HOME", "/real/oh")
	t.Setenv("OH_PASSPHRASE", "secret")
	t.Setenv("XDG_CONFIG_HOME", "/real/xdg")
	e := &childEvalExecutor{goEnv: []string{"GOCACHE=/real/cache"}}
	env := strings.Join(e.passEnv("/tmp/h", "/tmp/u"), "\n")
	assert.Contains(t, env, "OH_HOME=/tmp/h")
	assert.Contains(t, env, "HOME=/tmp/u")
	assert.Contains(t, env, "GOCACHE=/real/cache")
	for _, bad := range []string{"/real/oh", "OH_PASSPHRASE", "/real/xdg"} {
		assert.NotContains(t, env, bad)
	}
}
