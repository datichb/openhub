//go:build integration && e2e

// End-to-end tests with a real model (Claude Haiku on Bedrock EU).
// Run with: go test -tags "integration e2e" ./internal/adapters/opencodev2/...
// Credentials: OH_E2E_BEDROCK_TOKEN, or the Bedrock key stored by `opencode auth`.
package opencodev2

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

const e2eModel = "amazon-bedrock/eu.anthropic.claude-haiku-4-5-20251001-v1:0"

func bedrockToken(t *testing.T) string {
	t.Helper()
	if tok := os.Getenv("OH_E2E_BEDROCK_TOKEN"); tok != "" {
		return tok
	}
	out, err := exec.Command("opencode", "auth", "export").Output()
	if err != nil {
		t.Skip("no Bedrock credential available")
	}
	var creds []struct {
		IntegrationID string `json:"integrationID"`
		Value         struct {
			Key string `json:"key"`
		} `json:"value"`
	}
	if json.Unmarshal(out, &creds) != nil {
		t.Skip("cannot parse opencode credentials")
	}
	for _, c := range creds {
		if c.IntegrationID == "amazon-bedrock" && c.Value.Key != "" {
			return c.Value.Key
		}
	}
	t.Skip("no Bedrock API key in opencode credentials")
	return ""
}

type e2eRun struct {
	adapter *Adapter
	handle  adapters.ServerHandle
	project string
	trace   string
}

func startE2E(t *testing.T, b sessionspec.BundleSpec, disablePlugin bool) *e2eRun {
	t.Helper()
	tok := bedrockToken(t)
	return startE2EWith(t, b, disablePlugin, sessionspec.ProviderSpec{ID: "amazon-bedrock", Region: "eu-west-1", SessionToken: tok})
}

func startE2EWith(t *testing.T, b sessionspec.BundleSpec, disablePlugin bool, prov sessionspec.ProviderSpec) *e2eRun {
	t.Helper()
	a := newContractAdapter(t)
	a.DisablePlugin = disablePlugin
	root := t.TempDir()
	a.PluginTrace = filepath.Join(root, "trace.jsonl")
	project := filepath.Join(root, "proj")
	require.NoError(t, os.MkdirAll(project, 0o755))
	require.NoError(t, exec.Command("git", "init", "-q", project).Run())

	m := sessionspec.ParseModelRef(e2eModel)
	b.DefaultModel = &m
	h, err := a.StartServer(context.Background(), adapters.ServerGroup{
		Bundle:   b,
		Provider: prov,
		DataDir:  filepath.Join(root, "data"),
		WorkDir:  project,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = a.StopServer(context.Background(), h) })
	return &e2eRun{adapter: a, handle: h, project: project, trace: a.PluginTrace}
}

func (r *e2eRun) ask(t *testing.T, agent, prompt string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	id := sessionspec.NewSessionID()
	require.NoError(t, r.adapter.CreateSession(ctx, r.handle, sessionspec.SessionSpec{
		SessionID: id, Title: "e2e", EntryAgent: agent, Location: r.project,
	}))
	require.NoError(t, r.adapter.SendPrompt(ctx, r.handle, id, prompt))
	c := NewClient(r.handle.URL, r.handle.Password)
	require.NoError(t, c.Wait(ctx, id))
	var msgs struct {
		Data []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"data"`
	}
	require.NoError(t, c.do(ctx, "GET", "/api/session/"+id+"/message", nil, nil, &msgs))
	var b strings.Builder
	for _, m := range msgs.Data {
		for _, p := range m.Content {
			if p.Type == "text" {
				b.WriteString(p.Text)
			}
		}
	}
	return b.String()
}

func e2eBundle(t *testing.T) sessionspec.BundleSpec {
	b := contractBundle(t, t.TempDir())
	b.Agents[0].Body = "You are LEAD. Always start every reply with the exact prefix 'LEAD:'."
	return b
}

func TestE2EPluginInjectsAgentPrompt(t *testing.T) {
	r := startE2E(t, e2eBundle(t), false)
	reply := r.ask(t, "lead", "Say hello in three words.")
	assert.True(t, strings.HasPrefix(strings.TrimSpace(reply), "LEAD:"), "reply: %q", reply)

	f, err := os.Open(r.trace)
	require.NoError(t, err, "plugin trace written")
	defer f.Close()
	sc := bufio.NewScanner(f)
	require.True(t, sc.Scan())
	var tr struct {
		Agent string   `json:"agent"`
		Parts []string `json:"parts"`
	}
	require.NoError(t, json.Unmarshal(sc.Bytes(), &tr))
	assert.Equal(t, "lead", tr.Agent)
	require.GreaterOrEqual(t, len(tr.Parts), 2)
	assert.True(t, strings.HasPrefix(tr.Parts[0], "You are an AI agent running in OpenCode"), "opencode base prompt kept: %q", tr.Parts[0])
	assert.True(t, strings.HasPrefix(tr.Parts[1], "You are LEAD."), "agent body injected after the base prompt")
}

func TestE2EModelSeesOnlyBundleSkills(t *testing.T) {
	r := startE2E(t, e2eBundle(t), false)
	reply := r.ask(t, "lead", "Without calling any tool, list the exact IDs of every skill available in your skill tool description, comma separated, nothing else.")
	low := strings.ToLower(reply)
	assert.Contains(t, low, "alpha", reply)
	assert.Contains(t, low, "beta", reply)
	assert.NotContains(t, low, "opencode", reply)
	assert.NotContains(t, low, "report", reply)
}

func TestE2EFallbackWithoutPlugin(t *testing.T) {
	r := startE2E(t, e2eBundle(t), true)
	reply := r.ask(t, "lead", "Say hello in three words.")
	assert.True(t, strings.HasPrefix(strings.TrimSpace(reply), "LEAD:"), "reply: %q", reply)
}

func TestE2EThroughCredentialProxy(t *testing.T) {
	real := bedrockToken(t)
	p := credproxy.New()
	require.NoError(t, p.Start("127.0.0.1:0"))
	t.Cleanup(func() { _ = p.Close(context.Background()) })
	tok, err := p.Issue(credproxy.Grant{
		SessionID:     "e2e",
		Provider:      credproxy.ProviderBedrock,
		Upstream:      credproxy.BedrockUpstream("eu-west-1", credproxy.BearerAuth{Token: real}),
		AllowedModels: []string{"eu.anthropic.claude-haiku-*"},
	})
	require.NoError(t, err)

	b := e2eBundle(t)
	b.Agents[0].Body = "You are LEAD. Run the shell command `env` exactly once, then reply with only the lines of its output that contain the word BEDROCK, verbatim."
	b.Agents[0].Permissions = []sessionspec.PermissionRule{{Action: "shell", Resource: "env*", Effect: "allow"}}
	r := startE2EWith(t, b, false, sessionspec.ProviderSpec{
		ID: "amazon-bedrock", Region: "eu-west-1", BaseURL: p.BaseURL(credproxy.ProviderBedrock), SessionToken: tok,
	})

	reply := r.ask(t, "lead", "go")
	assert.NotContains(t, reply, real, "the real key must never reach the agent")
	assert.Contains(t, reply, "ohs_", "the agent only sees the session token: %q", reply)

	u, ok := p.Usage(tok)
	require.True(t, ok)
	assert.Greater(t, u.Requests, int64(0))
	assert.Greater(t, u.InputTokens, int64(0), "usage counted from the Bedrock stream")
	assert.Greater(t, u.OutputTokens, int64(0))
	t.Logf("proxy usage: %+v", u)

	// The server process environment does not hold the real key either.
	ps, err := exec.Command("ps", "eww", "-p", fmt.Sprint(r.handle.PID)).Output()
	require.NoError(t, err)
	assert.NotContains(t, string(ps), real)
}

// Requires OH_E2E_AWS_PROFILE: an AWS profile with Bedrock access in eu-west-1.
func TestE2EThroughCredentialProxySigV4(t *testing.T) {
	profile := os.Getenv("OH_E2E_AWS_PROFILE")
	if profile == "" {
		t.Skip("OH_E2E_AWS_PROFILE not set")
	}
	auth, err := credproxy.NewSigV4FromProfile(context.Background(), profile, "eu-west-1")
	require.NoError(t, err)
	p := credproxy.New()
	require.NoError(t, p.Start("127.0.0.1:0"))
	t.Cleanup(func() { _ = p.Close(context.Background()) })
	tok, err := p.Issue(credproxy.Grant{Provider: credproxy.ProviderBedrock, Upstream: credproxy.BedrockUpstream("eu-west-1", auth)})
	require.NoError(t, err)

	r := startE2EWith(t, e2eBundle(t), false, sessionspec.ProviderSpec{
		ID: "amazon-bedrock", Region: "eu-west-1", BaseURL: p.BaseURL(credproxy.ProviderBedrock), SessionToken: tok,
	})
	reply := r.ask(t, "lead", "Say hello in three words.")
	assert.True(t, strings.HasPrefix(strings.TrimSpace(reply), "LEAD:"), "reply: %q", reply)
}
