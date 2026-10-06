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
	"sync"
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
	return r.replyText(ctx, t, id)
}

// replyText returns the text of the assistant messages of a session.
func (r *e2eRun) replyText(ctx context.Context, t *testing.T, id string) string {
	t.Helper()
	c := NewClient(r.handle.URL, r.handle.Password)
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

// P1-T09: an agent reads a skill annex stored in the bundle (outside the
// project) through the expanded BundleRootVar path, without any
// external_directory permission prompt.
func TestE2EAgentReadsSkillAnnexOutsideProject(t *testing.T) {
	b := e2eBundle(t)
	b.Root = filepath.Dir(b.SkillsDir)
	annex := filepath.Join(b.SkillsDir, "alpha", "templates", "codeword.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(annex), 0o755))
	require.NoError(t, os.WriteFile(annex, []byte("The codeword is PAPAYA-47.\n"), 0o444))
	b.Agents[0].Body += "\n\nWhen asked for the codeword, read the file `" + sessionspec.BundleRootVar + "/skills/alpha/templates/codeword.md` with the read tool and answer with the codeword only."
	b.Agents[0].Permissions = []sessionspec.PermissionRule{{Action: sessionspec.ActionRead, Resource: "*", Effect: sessionspec.EffectAllow}}

	r := startE2E(t, b, false)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	id := sessionspec.NewSessionID()
	require.NoError(t, r.adapter.CreateSession(ctx, r.handle, sessionspec.SessionSpec{SessionID: id, Title: "e2e", EntryAgent: "lead", Location: r.project}))
	require.NoError(t, r.adapter.SendPrompt(ctx, r.handle, id, "What is the codeword?"))
	c := NewClient(r.handle.URL, r.handle.Password)

	asked := make(chan []string, 1)
	go func() {
		for ctx.Err() == nil {
			if perms, err := c.Permissions(ctx, id); err == nil && len(perms) > 0 {
				asked <- append([]string{perms[0].Action}, perms[0].Resources...)
				cancel()
				return
			}
			time.Sleep(time.Second)
		}
	}()
	err := c.Wait(ctx, id)
	select {
	case p := <-asked:
		t.Fatalf("permission asked while reading the annex: %v", p)
	default:
	}
	require.NoError(t, err)
	assert.Contains(t, r.replyText(context.Background(), t, id), "PAPAYA-47")
}

// pendingOf polls the pending decisions of a session until one of kind shows up.
func (r *e2eRun) pendingOf(t *testing.T, ctx context.Context, id string, kind adapters.DecisionKind) adapters.PendingDecision {
	t.Helper()
	for ctx.Err() == nil {
		list, err := r.adapter.Pending(ctx, r.handle, id)
		require.NoError(t, err)
		for _, d := range list {
			if d.Kind == kind {
				return d
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("no pending %s decision", kind)
	return adapters.PendingDecision{}
}

func (r *e2eRun) text(t *testing.T, ctx context.Context, id string) string {
	t.Helper()
	txt, err := NewClient(r.handle.URL, r.handle.Password).AssistantText(ctx, id)
	require.NoError(t, err)
	return txt
}

// Headless decisions (S3/S4, P3-T08): a permission and an agent question
// answered through the API, then the agent goes on.
func TestE2EHeadlessDecisions(t *testing.T) {
	r := startE2E(t, e2eBundle(t), false)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	c := NewClient(r.handle.URL, r.handle.Password)
	evs, err := r.adapter.Events(ctx, r.handle)
	require.NoError(t, err)
	var seenMu sync.Mutex
	seen := map[string]map[adapters.EventKind]bool{}
	go func() {
		for ev := range evs {
			seenMu.Lock()
			if seen[ev.SessionID] == nil {
				seen[ev.SessionID] = map[adapters.EventKind]bool{}
			}
			seen[ev.SessionID][ev.Kind] = true
			seenMu.Unlock()
		}
	}()
	asked := func(id string, k adapters.EventKind) bool {
		seenMu.Lock()
		defer seenMu.Unlock()
		return seen[id][k]
	}

	id := sessionspec.NewSessionID()
	require.NoError(t, r.adapter.CreateSession(ctx, r.handle, sessionspec.SessionSpec{SessionID: id, Title: "e2e", EntryAgent: "lead", Location: r.project,
		SessionRules: []sessionspec.PermissionRule{{Action: "shell", Resource: "*", Effect: sessionspec.EffectAsk}}}))
	require.NoError(t, r.adapter.SendPrompt(ctx, r.handle, id, "Run the shell command `echo e2e-perm-ok` and reply with its exact output."))
	p := r.pendingOf(t, ctx, id, adapters.DecisionPermission)
	assert.Equal(t, "shell", p.Action)
	require.NoError(t, r.adapter.Reply(ctx, r.handle, adapters.DecisionReply{SessionID: id, ID: p.ID, Kind: adapters.DecisionPermission, Decision: "once", Message: "approved by oh e2e"}))
	// Answering twice: the second answer is refused (first answer wins).
	err = r.adapter.Reply(ctx, r.handle, adapters.DecisionReply{SessionID: id, ID: p.ID, Kind: adapters.DecisionPermission, Decision: "reject"})
	assert.ErrorIs(t, err, adapters.ErrRequestGone, "%v", err)
	require.NoError(t, c.Wait(ctx, id))
	assert.Contains(t, r.text(t, ctx, id), "e2e-perm-ok")

	q := sessionspec.NewSessionID()
	require.NoError(t, r.adapter.CreateSession(ctx, r.handle, sessionspec.SessionSpec{SessionID: q, Title: "e2e", EntryAgent: "lead", Location: r.project}))
	require.NoError(t, r.adapter.SendPrompt(ctx, r.handle, q, "Use your question tool to ask me which colour I prefer, with exactly two options: Blue and Red. Then reply with the single word I chose."))
	form := r.pendingOf(t, ctx, q, adapters.DecisionQuestion)
	require.NotEmpty(t, form.Fields)
	f := form.Fields[0]
	value := "Blue"
	for _, o := range f.Options {
		if strings.EqualFold(o.Label, "blue") || strings.EqualFold(o.Value, "blue") {
			value = o.Value
		}
	}
	var answer any = value
	if f.Type == "multiselect" {
		answer = []string{value}
	}
	t.Logf("question field %+v", f)
	require.NoError(t, r.adapter.Reply(ctx, r.handle, adapters.DecisionReply{SessionID: q, ID: form.ID, Kind: adapters.DecisionQuestion, Answer: map[string]any{f.Key: answer}}))
	require.NoError(t, c.Wait(ctx, q))
	assert.Contains(t, strings.ToLower(r.text(t, ctx, q)), "blue")

	// The decision events carry their session (permission.asked, form.created
	// nest it in data.request / data.form).
	for _, s := range []string{id, q} {
		assert.True(t, asked(s, adapters.EventDecisionAsked), "decision asked event for %s", s)
		assert.True(t, asked(s, adapters.EventDecisionReplied), "decision replied event for %s", s)
	}
}
