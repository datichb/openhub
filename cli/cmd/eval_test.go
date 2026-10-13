package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/evalrun"
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

	assert.InDelta(t, 1.0, defaultPassBudget([]string{"claude-haiku-4-5"}), 1e-9)
	assert.InDelta(t, 2.5, defaultPassBudget([]string{"claude-haiku-4-5", "eu.anthropic.claude-sonnet-4-6"}), 1e-9, "a Sonnet session goes past $1")
}

// The rate limit of a pass is read on the error lines of its logs, never
// on the commands of the agents.
func TestEvalPassThrottle(t *testing.T) {
	dir := t.TempDir()
	home, user := filepath.Join(dir, "h"), filepath.Join(dir, "u")
	passLog := filepath.Join(dir, "pass.log")
	require.NoError(t, os.WriteFile(passLog, []byte("eval pass: session ses_1\n"), 0o644))
	ocLog := filepath.Join(user, ".local", "share", "tool", "log")
	require.NoError(t, os.MkdirAll(ocLog, 0o755))
	cmdLine := `level=INFO message="spawning process" args="grep 'Too many requests' x"` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(ocLog, "a.log"), []byte(cmdLine), 0o644))
	assert.Empty(t, passThrottle(passLog, home, user))

	srv := filepath.Join(home, "servers", "g1", "data")
	require.NoError(t, os.MkdirAll(srv, 0o755))
	line := `level=ERROR service=llm error="ThrottlingException: Too many tokens, please wait before trying again."`
	require.NoError(t, os.WriteFile(filepath.Join(srv, "oh-server.log"), []byte("level=INFO ready\n"+line+"\n"), 0o644))
	assert.Contains(t, passThrottle(passLog, home, user), "ThrottlingException")
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
	t.Setenv("TMPDIR", "/real/tmp")
	e := &childEvalExecutor{goEnv: []string{"GOCACHE=/real/cache"}}
	env := strings.Join(e.passEnv("/tmp/h", "/tmp/u", "/tmp/t"), "\n")
	assert.Contains(t, env, "OH_HOME=/tmp/h")
	assert.Contains(t, env, "HOME=/tmp/u")
	assert.Contains(t, env, "TMPDIR=/tmp/t", "a temporary directory of its own: passes run side by side")
	assert.Contains(t, env, "GOCACHE=/real/cache")
	for _, bad := range []string{"/real/oh", "OH_PASSPHRASE", "/real/xdg", "/real/tmp"} {
		assert.NotContains(t, env, bad)
	}
}

// The variants are prepared from the embedded hub and checked before any
// pass: a variant that breaks a workflow of the cases stops the run.
func TestPrepareEvalVariants(t *testing.T) {
	work := t.TempDir()
	cases := []*evalrun.Case{{ID: "S6", Workflow: "debug"}}
	vs, err := evalrun.ParseVariants([]string{"base", "sans-23=-skill:debugger-phase-2-3"})
	require.NoError(t, err)
	var log bytes.Buffer
	require.NoError(t, prepareEvalVariants(vs, cases, work, &log))
	assert.Equal(t, filepath.Join(work, "variants", "base"), vs[0].Hub)
	assert.FileExists(t, filepath.Join(vs[0].Hub, "skills", "quality", "debugger-phase-2-3.md"))
	assert.NoFileExists(t, filepath.Join(vs[1].Hub, "skills", "quality", "debugger-phase-2-3.md"))
	assert.Contains(t, log.String(), "debugger-phase-2-3")

	vs, err = evalrun.ParseVariants([]string{"base", "sans-mode=-skill:orchestrator-dev-feedback-mode"})
	require.NoError(t, err)
	err = prepareEvalVariants(vs, []*evalrun.Case{{ID: "S9", Workflow: "review-feedback"}}, work, &log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sans-mode")
}
