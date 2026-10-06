package session

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

type fakeTool struct {
	adapters.ToolAdapter
	mu       sync.Mutex
	replies  []adapters.DecisionReply
	controls []adapters.ControlOp
	replyErr error
	result   adapters.SessionResult
}

func (f *fakeTool) Capabilities() adapters.Capabilities {
	return adapters.Capabilities{HeadlessDecisions: true}
}

func (f *fakeTool) Reply(_ context.Context, _ adapters.ServerHandle, r adapters.DecisionReply) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.replies = append(f.replies, r)
	return f.replyErr
}

func (f *fakeTool) Control(_ context.Context, _ adapters.ServerHandle, _ string, op adapters.ControlOp) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.controls = append(f.controls, op)
	return nil
}

func (f *fakeTool) Results(context.Context, adapters.ServerHandle, string) (adapters.SessionResult, error) {
	return f.result, nil
}

type fixture struct {
	svc  *Service
	tool *fakeTool
	ctx  context.Context
}

func newFixture(t *testing.T, strict bool) *fixture {
	t.Helper()
	svc, ctx := newTestService(t)
	root := t.TempDir()
	svc.BundlesDir = filepath.Join(root, "bundles")
	svc.SessionsDir = filepath.Join(root, "sessions")
	dir := filepath.Join(svc.BundlesDir, "hash1")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	data, _ := json.Marshal(sessionspec.BundleSpec{Hash: "hash1", EntryAgent: "lead", StrictIsolation: strict,
		Agents: []sessionspec.AgentDef{{ID: "lead", Mode: "primary"}}})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bundle.json"), data, 0o644))

	tool := &fakeTool{}
	svc.Adapter = tool
	require.NoError(t, svc.Servers.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: "fake", ProjectID: "p1", PID: os.Getpid(), URL: "http://x", Status: domain.ServerReady}))
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning,
		GroupKey: "g1", BundleHash: "hash1", EntryAgent: "lead", State: domain.RunWaiting}))
	return &fixture{svc: svc, tool: tool, ctx: ctx}
}

func (f *fixture) raise(t *testing.T, d domain.Decision) string {
	t.Helper()
	require.NoError(t, f.svc.Raise(f.ctx, &d))
	return d.ID
}

func TestDecidePermission(t *testing.T) {
	f := newFixture(t, false)
	id := f.raise(t, domain.Decision{SessionID: "ses_a", Kind: domain.DecisionPermission, ToolRef: "per_1", Payload: domain.DecisionPayload{Action: "shell"}})

	assert.ErrorContains(t, f.svc.Decide(f.ctx, Reply{DecisionID: id, Decision: "maybe"}), "invalid permission decision")
	require.NoError(t, f.svc.Decide(f.ctx, Reply{DecisionID: id, Decision: "once", Message: "ok"}))
	require.Len(t, f.tool.replies, 1)
	assert.Equal(t, adapters.DecisionReply{SessionID: "ses_a", ID: "per_1", Kind: adapters.DecisionPermission, Decision: "once", Message: "ok"}, f.tool.replies[0])
	d, _ := f.svc.Decisions.Get(f.ctx, id)
	assert.Equal(t, domain.ResolvedByOh, d.ResolvedBy)
	assert.Equal(t, "ok", d.Resolution.Message)

	// First answer wins: a second answer is refused, the tool is not called.
	err := f.svc.Decide(f.ctx, Reply{DecisionID: id, Decision: "reject"})
	var re *ResolvedError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, domain.ResolvedByOh, re.By)
	assert.ErrorIs(t, err, ErrAlreadyResolved)
	assert.Len(t, f.tool.replies, 1)
}

func TestDecideAnsweredInTheTool(t *testing.T) {
	f := newFixture(t, false)
	id := f.raise(t, domain.Decision{SessionID: "ses_a", Kind: domain.DecisionPermission, ToolRef: "per_1"})
	f.tool.replyErr = adapters.ErrRequestGone
	err := f.svc.Decide(f.ctx, Reply{DecisionID: id, Decision: "once"})
	var re *ResolvedError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, domain.ResolvedByTool, re.By)
	d, _ := f.svc.Decisions.Get(f.ctx, id)
	assert.Equal(t, domain.ResolvedByTool, d.ResolvedBy)

	// Any other failure leaves the decision open.
	id2 := f.raise(t, domain.Decision{SessionID: "ses_a", Kind: domain.DecisionPermission, ToolRef: "per_2"})
	f.tool.replyErr = errors.New("network down")
	assert.ErrorContains(t, f.svc.Decide(f.ctx, Reply{DecisionID: id2, Decision: "once"}), "network down")
	d, _ = f.svc.Decisions.Get(f.ctx, id2)
	assert.True(t, d.Open())
}

func TestDecideAlwaysForbiddenWhenStrict(t *testing.T) {
	f := newFixture(t, true)
	id := f.raise(t, domain.Decision{SessionID: "ses_a", Kind: domain.DecisionPermission, ToolRef: "per_1"})
	assert.ErrorIs(t, f.svc.Decide(f.ctx, Reply{DecisionID: id, Decision: "always"}), ErrAlwaysForbidden)
	assert.Empty(t, f.tool.replies)
	require.NoError(t, f.svc.Decide(f.ctx, Reply{DecisionID: id, Decision: "reject"}))

	g := newFixture(t, false)
	id = g.raise(t, domain.Decision{SessionID: "ses_a", Kind: domain.DecisionPermission, ToolRef: "per_1"})
	require.NoError(t, g.svc.Decide(g.ctx, Reply{DecisionID: id, Decision: "always"}))
}

func TestDecideQuestionErrorAndResolver(t *testing.T) {
	f := newFixture(t, false)
	fields := []domain.DecisionField{
		{Key: "color", Type: "string", Required: true, Options: []domain.DecisionOption{{Value: "Blue"}, {Value: "Red"}}},
		{Key: "count", Type: "integer"},
		{Key: "ok", Type: "boolean"},
		{Key: "tags", Type: "multiselect", Options: []domain.DecisionOption{{Value: "a"}, {Value: "b"}}},
	}
	id := f.raise(t, domain.Decision{SessionID: "ses_a", Kind: domain.DecisionQuestion, ToolRef: "frm_1", Payload: domain.DecisionPayload{Fields: fields}})

	_, err := ParseAnswers(fields, map[string]string{"color": "Green"})
	assert.ErrorContains(t, err, "not one of")
	_, err = ParseAnswers(fields, map[string]string{"count": "3"})
	assert.ErrorContains(t, err, "required")
	_, err = ParseAnswers(fields, map[string]string{"color": "Blue", "nope": "x"})
	assert.ErrorContains(t, err, "unknown field")
	ans, err := ParseAnswers(fields, map[string]string{"color": "Blue", "count": "3", "ok": "oui", "tags": "a, b"})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"color": "Blue", "count": int64(3), "ok": true, "tags": []string{"a", "b"}}, ans)

	require.NoError(t, f.svc.Decide(f.ctx, Reply{DecisionID: id, Answer: ans}))
	assert.Equal(t, ans, f.tool.replies[0].Answer)

	errID := f.raise(t, domain.Decision{SessionID: "ses_a", Kind: domain.DecisionError, ToolRef: "evt_1"})
	require.NoError(t, f.svc.Decide(f.ctx, Reply{DecisionID: errID}))
	d, _ := f.svc.Decisions.Get(f.ctx, errID)
	assert.Equal(t, "dismiss", d.Resolution.Decision)

	cp := f.raise(t, domain.Decision{SessionID: "ses_a", Kind: domain.DecisionCheckpoint, ToolRef: "cp-1"})
	assert.ErrorIs(t, f.svc.Decide(f.ctx, Reply{DecisionID: cp, Decision: "commit"}), ErrUnsupportedKind)
	var got Reply
	f.svc.Resolvers = map[domain.DecisionKind]Resolver{domain.DecisionCheckpoint: func(_ context.Context, _ domain.Decision, r Reply) error {
		got = r
		return nil
	}}
	require.NoError(t, f.svc.Decide(f.ctx, Reply{DecisionID: cp, Decision: "commit"}))
	assert.Equal(t, "commit", got.Decision)
}

func TestControlAndServerState(t *testing.T) {
	f := newFixture(t, false)
	require.NoError(t, f.svc.Send(f.ctx, "ses_a", " run the tests ", SendOptions{Synthetic: true, Delivery: adapters.DeliveryQueue}))
	require.NoError(t, f.svc.Interrupt(f.ctx, "ses_a"))
	require.NoError(t, f.svc.Compact(f.ctx, "ses_a"))
	require.NoError(t, f.svc.SwitchModel(f.ctx, "ses_a", "amazon-bedrock/anthropic.claude-haiku-4-5"))
	require.Len(t, f.tool.controls, 4)
	assert.Equal(t, adapters.ControlOp{Kind: adapters.ControlSynthetic, Text: "run the tests", Delivery: adapters.DeliveryQueue}, f.tool.controls[0])
	assert.Equal(t, adapters.ControlInterrupt, f.tool.controls[1].Kind)
	assert.Equal(t, adapters.ControlCompact, f.tool.controls[2].Kind)
	assert.Equal(t, "anthropic.claude-haiku-4-5", f.tool.controls[3].Model.Model)
	sess, _ := f.svc.Sessions.Get(f.ctx, "ses_a")
	assert.Equal(t, "amazon-bedrock/anthropic.claude-haiku-4-5", sess.Model)

	assert.Error(t, f.svc.Send(f.ctx, "ses_a", "  ", SendOptions{}))
	assert.ErrorContains(t, f.svc.SwitchModel(f.ctx, "ses_a", "haiku"), "invalid model")

	require.NoError(t, f.svc.Servers.SetStatus(f.ctx, "g1", domain.ServerSleeping))
	assert.ErrorIs(t, f.svc.Interrupt(f.ctx, "ses_a"), ErrServerNotRunning)
}

func TestResultsLiveThenSnapshot(t *testing.T) {
	f := newFixture(t, false)
	f.tool.result = adapters.SessionResult{SessionID: "ses_a", Title: "Export CSV", Agent: "lead", Branch: "feat/csv", Cost: 0.84, TokensIn: 1000, TokensOut: 200,
		Changes: []adapters.FileChange{{File: "a.go", Status: "modified", Additions: 10, Deletions: 2, Patch: "--- a/a.go\n+++ b/a.go\n"}, {File: "b.go", Additions: 5}}}
	r, err := f.svc.Results(f.ctx, "ses_a")
	require.NoError(t, err)
	assert.True(t, r.Live)
	assert.Equal(t, 15, r.Additions)
	assert.Len(t, r.Files, 2)
	assert.Contains(t, r.Patch, "+++ b/a.go")
	md := MRDescription(r)
	assert.Contains(t, md, "## Export CSV")
	assert.Contains(t, md, "`a.go` (+10 −2)")
	assert.Contains(t, md, "feat/csv")
	assert.NotEmpty(t, Recap(r))

	require.NoError(t, f.svc.Servers.SetStatus(f.ctx, "g1", domain.ServerSleeping))
	r, err = f.svc.Results(f.ctx, "ses_a")
	require.NoError(t, err)
	assert.False(t, r.Live)
	assert.Equal(t, "feat/csv", r.Branch)
	assert.Contains(t, r.Patch, "+++ b/a.go")

	// No snapshot: usage from the session row.
	require.NoError(t, f.svc.Sessions.Create(f.ctx, &domain.Session{ID: "ses_b", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", Cost: 0.5, State: domain.RunSleeping}))
	r, err = f.svc.Results(f.ctx, "ses_b")
	require.NoError(t, err)
	assert.InDelta(t, 0.5, r.Cost, 1e-9)
	assert.Empty(t, r.Files)
}
