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

	"github.com/datichb/openhub/cli/internal/adapters/opencodev2"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
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
