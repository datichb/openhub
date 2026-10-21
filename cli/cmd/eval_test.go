package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/evalrun"
	"github.com/datichb/openhub/cli/internal/i18n"
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

	assert.Equal(t, int64(9090909), evalMaxTokens("claude-haiku-4-5", 1))
	assert.Equal(t, int64(454545), evalMaxTokens("claude-sonnet-4-6", 1))
	assert.Equal(t, int64(181818), evalMaxTokens("eu.anthropic.claude-opus-5-5", 1))
	assert.Zero(t, evalMaxTokens("claude-haiku-4-5", 0))

	assert.InDelta(t, 1.0, defaultPassBudget([]string{"claude-haiku-4-5"}), 1e-9)
	assert.InDelta(t, 2.5, defaultPassBudget([]string{"claude-haiku-4-5", "eu.anthropic.claude-sonnet-4-6"}), 1e-9, "a Sonnet session goes past $1")
}

// The token cap of a pass follows its real cost, cache included: the
// proxy counts input-equivalent tokens (output ×5, cache read ×0.1, cache
// write ×1.25), and the cap divides the budget by the input price. An Opus
// pass that read 100k tokens of cache, wrote 20k and produced 3k of output
// cost about $0.20 (EU 5.5 prices: 0.022 + 0.11 + 0.066): it stays under the
// cap of $1 and passes the cap of $0.15. A raw count would have cut it at a
// tenth of the budget, or let 7 times the budget through.
func TestEvalTokenCapFollowsTheRealCost(t *testing.T) {
	used := credproxy.Usage{CacheReadTokens: 100_000, CacheWriteTokens: 20_000, OutputTokens: 3_000}
	assert.InDelta(t, 50_000, used.Weighted(), 1e-6)
	const opus = "eu.anthropic.claude-opus-5-5"
	assert.Less(t, used.Weighted(), float64(evalMaxTokens(opus, 1)), "under the cap of $1")
	assert.Greater(t, used.Weighted(), float64(evalMaxTokens(opus, 0.15)), "over the cap of $0.15")

	assert.Greater(t, evalTokenPrice(opus), evalTokenPrice("claude-sonnet-5-5"))
	assert.Greater(t, evalTokenPrice("claude-sonnet-5-5"), evalTokenPrice("claude-haiku-5-5"))
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
	t.Parallel()
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

// writeEvalCase writes a case of a quick workflow in root.
func writeEvalCase(t *testing.T, root, id string) {
	t.Helper()
	dir := filepath.Join(root, "quick", strings.ToLower(id))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, evalrun.FixtureDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, evalrun.CaseFile), []byte("id: "+id+"\nworkflow: quick\nassertions:\n  - type: no_denied_loop\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, evalrun.FixtureDir, "README"), []byte(id), 0o644))
}

// runEvalCmd runs oh eval with these arguments on a command of its own.
func runEvalCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	c := &cobra.Command{Use: "eval", RunE: runEval}
	addEvalFlags(c.Flags())
	var out, errOut bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errOut)
	c.SetArgs(args)
	err := c.Execute()
	return out.String(), err
}

// oh eval --tiered --plan computes the repetitions of each case from the
// history of the baseline and estimates the cost, without any key nor
// pass; --reps-case forces a case.
func TestEvalPlanDryRun(t *testing.T) {
	t.Setenv(envEvalReps, "")
	t.Setenv(envEvalToken, "")
	t.Setenv("OH_E2E_BEDROCK_TOKEN", "")
	root := t.TempDir()
	for _, id := range []string{"S1", "S2", "S3"} {
		writeEvalCase(t, root, id)
	}
	cases, err := evalrun.LoadCases(root)
	require.NoError(t, err)
	baseline := filepath.Join(t.TempDir(), "baseline.json")
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	var base *evalrun.Report
	for d, res := range []map[string][2]int{{"S1": {3, 3}, "S2": {3, 3}}, {"S1": {3, 3}, "S2": {2, 3}, "S3": {3, 3}}} {
		r := &evalrun.Report{StartedAt: at.AddDate(0, 0, d), Models: []string{defaultEvalModel}, Reps: 3}
		for _, c := range cases {
			r.Cases = append(r.Cases, c.Info())
			if v, ok := res[c.ID]; ok {
				r.Stats = append(r.Stats, evalrun.CaseStats{Case: c.ID, Model: defaultEvalModel, Passed: v[0], Runs: v[1], MeanCost: 0.1})
			}
		}
		base = r.Baseline("ref").WithHistory(base)
	}
	require.NoError(t, base.WriteJSON(baseline))

	out, err := runEvalCmd(t, "--cases", root, "--baseline", baseline, "--tiered", "--plan", "--budget-total", "1")
	require.NoError(t, err)
	for _, want := range []string{
		"| S1 | quick | 1 | 100 % sur les deux dernières mesures (3/3, 3/3) | $0.100 |",
		"| S2 | quick | 3 | dernières mesures 2/3, 3/3 |",
		"| S3 | quick | 3 | une seule mesure (3/3) |",
		"- Total : 7 passages · ≈ $0.70",
		"- Si tous les cas à moins de ×3 échouent (escalade) : 9 passages · ≈ $0.90",
		"- Plafond global : $1.00 (suffisant)",
	} {
		assert.Contains(t, out, want)
	}
	out, err = runEvalCmd(t, "--cases", root, "--baseline", baseline, "--tiered", "--plan", "--reps-case", "S1=3,S3=1", "--stable-measures", "1", "--escalate=false")
	require.NoError(t, err)
	assert.Contains(t, out, "| S1 | quick | 3 | imposé (--reps-case) |")
	assert.Contains(t, out, "| S3 | quick | 1 | imposé (--reps-case) |")
	assert.NotContains(t, out, "escalade")

	_, err = runEvalCmd(t, "--cases", root, "--tiered", "--plan")
	assert.ErrorContains(t, err, "--baseline")
	_, err = runEvalCmd(t, "--cases", root, "--baseline", baseline, "--plan", "--reps-case", "S9=1")
	assert.ErrorContains(t, err, "S9")
}

func TestEvalPlanSummary(t *testing.T) {
	assert.Equal(t, 1, defaultEvalReps(false))
	assert.Equal(t, 3, defaultEvalReps(true))
	plan := []evalrun.CasePlan{{Case: "S1", Reps: 1}, {Case: "S2", Reps: 3}, {Case: "S3", Reps: 1}}
	assert.Equal(t, "×1 : 2, ×3 : 1, "+i18n.Tf("cmd.eval.plan_escalate", 3), planSummary(plan, 3))
}
