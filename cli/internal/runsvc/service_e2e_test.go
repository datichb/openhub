//go:build integration && e2e

package runsvc

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/limits"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/sessionctx"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

func bedrockKey(t *testing.T) string {
	if k := os.Getenv("OH_E2E_BEDROCK_TOKEN"); k != "" {
		return k
	}
	out, err := exec.Command("opencode", "auth", "export").Output()
	if err != nil {
		t.Skip("no Bedrock credential")
	}
	var creds []struct {
		IntegrationID string               `json:"integrationID"`
		Value         struct{ Key string } `json:"value"`
	}
	_ = json.Unmarshal(out, &creds)
	for _, c := range creds {
		if c.IntegrationID == "amazon-bedrock" && c.Value.Key != "" {
			return c.Value.Key
		}
	}
	t.Skip("no Bedrock API key")
	return ""
}

func TestE2EStartSessionThroughDaemonProxy(t *testing.T) {
	key := bedrockKey(t)
	f := newFixture(t, mapSecrets{"openhub.team.core.provider.bedrock.token": key})
	req := f.request("Say hello in three words.")
	req.TeamID = "core"
	req.ProviderCfg.AWSRegion = "eu-west-1"
	req.AllowedModels = []string{"eu.anthropic.claude-haiku-*"}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	r, err := f.svc.StartSession(ctx, req)
	require.NoError(t, err)

	c := opencodev2.NewClient(r.Server.URL, r.Server.Password)
	require.NoError(t, c.Wait(ctx, r.SessionID))
	s, err := c.GetSession(ctx, r.SessionID)
	require.NoError(t, err)
	assert.Greater(t, s.Tokens.Output, int64(0), "the model answered through the daemon proxy")

	var msgs struct {
		Data []struct {
			Content []struct{ Type, Text string } `json:"content"`
		} `json:"data"`
	}
	out, err := exec.Command("curl", "-s", "-u", "opencode:"+r.Server.Password, r.Server.URL+"/api/session/"+r.SessionID+"/message").Output()
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(out, &msgs))
	var text strings.Builder
	for _, m := range msgs.Data {
		for _, p := range m.Content {
			if p.Type == "text" {
				text.WriteString(p.Text)
			}
		}
	}
	assert.True(t, strings.HasPrefix(strings.TrimSpace(text.String()), "LEAD:"), "reply: %q", text.String())

	// E10: the daemon watcher tracked the session (idle after the turn, usage recorded).
	require.Eventually(t, func() bool {
		sess, err := f.svc.Sessions.Get(ctx, r.SessionID)
		return err == nil && sess.State == domain.RunIdle && sess.TokensOut > 0 && sess.Cost > 0
	}, 20*time.Second, 200*time.Millisecond)
}

// The daemon applies the session environment to the sub-sessions of the
// subagents (opencode does not pass it on), in time for their first shell.
func TestE2ESubagentGetsSessionEnvironment(t *testing.T) {
	key := bedrockKey(t)
	f := newFixture(t, mapSecrets{"openhub.team.core.provider.bedrock.token": key})
	b := *f.bundle
	b.Spec.Agents = []sessionspec.AgentDef{
		{ID: "lead", Description: "lead", Mode: "primary", Body: "You are LEAD. Delegate every request to the `helper` subagent, then reply DONE.",
			Permissions: []sessionspec.PermissionRule{{Action: sessionspec.ActionSubagent, Resource: "helper", Effect: sessionspec.EffectAllow}}},
		{ID: "helper", Description: "helper", Mode: "subagent", Body: "You are HELPER. Run exactly the shell command you are given with the shell tool, then reply with its output.",
			Permissions: []sessionspec.PermissionRule{{Action: sessionspec.ActionShell, Resource: "*", Effect: sessionspec.EffectAllow}}},
	}
	b.Spec.SubagentGraph = map[string][]string{"lead": {"helper"}}
	b.Spec.Hash = "testbundlesubenv01"
	out := filepath.Join(f.project, "sub-env.txt")
	req := f.request("Ask the helper subagent to run this shell command: echo \"VAL=${OH_E2E_VAR:-missing} SID=${OH_SESSION_ID:-missing}\" > " + out)
	req.Bundle = &b
	req.TeamID = "core"
	req.ProviderCfg.AWSRegion = "eu-west-1"
	req.SessionEnv = map[string]string{"OH_E2E_VAR": "inherited"}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	r, err := f.svc.StartSession(ctx, req)
	require.NoError(t, err)
	require.NoError(t, opencodev2.NewClient(r.Server.URL, r.Server.Password).Wait(ctx, r.SessionID))
	data, err := os.ReadFile(out)
	require.NoError(t, err, "the helper did not run the command")
	assert.Equal(t, "VAL=inherited SID="+r.SessionID, strings.TrimSpace(string(data)))
}

// I6 session budget with a real model (v5 finalisation, Q4: the e2e R4 did
// not run): the turn that spends the budget ends, a $ decision is raised
// with the spent amount; a turn started while it is open is interrupted;
// once raised, the session answers again.
func TestE2ESessionBudget(t *testing.T) {
	key := bedrockKey(t)
	f := newFixture(t, mapSecrets{"openhub.team.core.provider.bedrock.token": key})
	req := f.request("Say hello in three words.")
	req.TeamID = "core"
	req.ProviderCfg.AWSRegion = "eu-west-1"
	req.Limits = limits.Resolve(limits.Input{Workflow: limits.Limits{SessionBudgetUSD: 0.000001}, ProjectID: "p1"})

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	r, err := f.svc.StartSession(ctx, req)
	require.NoError(t, err)
	c := opencodev2.NewClient(r.Server.URL, r.Server.Password)
	require.NoError(t, c.Wait(ctx, r.SessionID))

	decisions := sqlite.NewDecisionStore(f.store)
	usage := sqlite.NewUsageStore(f.store)
	var dec domain.Decision
	require.Eventually(t, func() bool {
		open, err := decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: r.SessionID})
		if err != nil {
			return false
		}
		for _, d := range open {
			if d.Kind == domain.DecisionBudget {
				dec = d
				return true
			}
		}
		return false
	}, 30*time.Second, 200*time.Millisecond, "budget decision raised after the turn")
	spent, _ := dec.Payload.Data[limits.DataBudgetSpent].(float64)
	assert.Greater(t, spent, 0.0, "spent amount in the decision")
	tot, err := usage.SessionTotal(ctx, r.SessionID)
	require.NoError(t, err)
	assert.InDelta(t, spent, tot.CostUSD, 1e-9, "ledger = what the decision reports")

	// While the decision is open, a new turn is interrupted.
	before := tot.TokensOut
	require.NoError(t, f.adapter.SendPrompt(ctx, adapters.ServerHandle{URL: r.Server.URL, Password: r.Server.Password, PID: r.Server.PID}, r.SessionID, "Write a 300-word story about a lighthouse."))
	require.NoError(t, c.Wait(ctx, r.SessionID))
	time.Sleep(2 * time.Second)
	held, err := usage.SessionTotal(ctx, r.SessionID)
	require.NoError(t, err)
	assert.Less(t, held.TokensOut-before, int64(300), "the story was cut short (interrupted)")

	// Raise by $1, as `oh budget raise`: the next turn is answered.
	resolve := sessionsvc.BudgetResolver(usage, func(context.Context, string) error { return nil })
	require.NoError(t, resolve(ctx, dec, sessionsvc.Reply{DecisionID: dec.ID, Decision: "raise", Message: "1"}))
	ok, err := decisions.Resolve(ctx, dec.ID, domain.ResolvedByOh, &domain.DecisionResolution{Decision: "raise"}, time.Now())
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, f.adapter.SendPrompt(ctx, adapters.ServerHandle{URL: r.Server.URL, Password: r.Server.Password, PID: r.Server.PID}, r.SessionID, "Say goodbye in three words."))
	require.NoError(t, c.Wait(ctx, r.SessionID))
	require.Eventually(t, func() bool {
		after, err := usage.SessionTotal(ctx, r.SessionID)
		return err == nil && after.TokensOut > held.TokensOut
	}, 30*time.Second, 200*time.Millisecond, "answered after the raise")
}

// QB8 (S8): a checkpoint state written in the session context is known to
// the entry agent at its next turn; a subagent does not receive it (tool
// limit, documented).
func TestE2ESessionContextReachesTheEntryAgent(t *testing.T) {
	key := bedrockKey(t)
	f := newFixture(t, mapSecrets{"openhub.team.core.provider.bedrock.token": key})
	b := *f.bundle
	b.Spec.Agents = []sessionspec.AgentDef{
		{ID: "lead", Description: "lead", Mode: "primary", Body: "You are LEAD. Answer briefly from what you know; when asked to, delegate to the `helper` subagent and repeat its answer.",
			Permissions: []sessionspec.PermissionRule{{Action: sessionspec.ActionSubagent, Resource: "helper", Effect: sessionspec.EffectAllow}}},
		{ID: "helper", Description: "helper", Mode: "subagent", Body: "You are HELPER. Answer the question only from what you know in this conversation; if you do not know, answer UNKNOWN."},
	}
	b.Spec.SubagentGraph = map[string][]string{"lead": {"helper"}}
	b.Spec.Hash = "testbundlectx0001"
	req := f.request("")
	req.Bundle = &b
	req.TeamID = "core"
	req.ProviderCfg.AWSRegion = "eu-west-1"

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	r, err := f.svc.StartSession(ctx, req)
	require.NoError(t, err)
	h := adapters.ServerHandle{URL: r.Server.URL, Password: r.Server.Password}
	w := &sessionctx.Writer{Adapter: f.adapter, Dir: f.svc.SessionsDir}
	wrote, err := w.Set(ctx, h, r.SessionID, sessionctx.Entry{Key: sessionctx.KeyCheckpoints,
		Value: map[string]any{"workflow": "e2e", "passed": []map[string]string{{"id": "cp-zebra", "label": "Zebra"}}, "next": "cp-lion"}})
	require.NoError(t, err)
	require.True(t, wrote)

	c := opencodev2.NewClient(r.Server.URL, r.Server.Password)
	ask := func(q string) string {
		t.Helper()
		require.NoError(t, f.adapter.SendPrompt(ctx, h, r.SessionID, q))
		require.NoError(t, c.Wait(ctx, r.SessionID))
		text, err := f.adapter.AssistantText(ctx, h, r.SessionID)
		require.NoError(t, err)
		return text
	}
	got := ask("According to your session context, which workflow checkpoint was passed and which one is next? Answer with the two ids only.")
	assert.Contains(t, got, "cp-zebra")
	assert.Contains(t, got, "cp-lion")

	// AssistantText holds every answer of the session: the subagent answer
	// is the new part.
	all := ask("Delegate to the helper subagent this exact question: « Which workflow checkpoint was passed? Answer with its id, or UNKNOWN. » Then repeat its answer.")
	assert.Contains(t, strings.ToLower(strings.Replace(all, got, "", 1)), "unknown", "the subagent does not receive the session context")
}
