package session

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

type transcriptTool struct {
	fakeTool
	entries []adapters.TranscriptEntry
	err     error
}

func (f *transcriptTool) Transcript(context.Context, adapters.ServerHandle, string) ([]adapters.TranscriptEntry, error) {
	return f.entries, f.err
}

// The fake secrets are split so that no secret scanner (GitHub push
// protection) takes this file for a leak.
const (
	traceAWSKey   = "AKIA" + "ABCDEFGHIJKLMNOP"
	traceProxyTok = "ohs_0123456789abcdef0123456789abcdef"
	traceGitLab   = "glpat" + "-abcdefghij0123456789"
	traceLiteral  = "my-own-literal-secret"
	traceBedrock  = "ABSK" + "QmVkcm9ja0FQSUtleS1hYmNkZWZnaGlqa2xt"
)

// A fake session with every kind of source: the JSONL holds all of it,
// oldest first, and no secret is left.
func TestTraceIsCompleteAndMasked(t *testing.T) {
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	project := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(project, "go.mod"), []byte("module x\n\ngo 1.22\n"), 0o644))

	cps := sqlite.NewCheckpointStore(st)
	tool := &transcriptTool{}
	svc := &Service{Sessions: sqlite.NewSessionStore(st), Decisions: sqlite.NewDecisionStore(st), Servers: sqlite.NewServerStore(st),
		Adapter: tool, SessionsDir: t.TempDir(), BundlesDir: t.TempDir()}
	svc.Checkpoints = &checkpoint.Service{Sessions: svc.Sessions, States: cps, SessionOutputs: cps, BundlesDir: svc.BundlesDir}
	usage := sqlite.NewUsageStore(st)

	t0 := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	at := func(s int) time.Time { return t0.Add(time.Duration(s) * time.Second) }
	require.NoError(t, svc.Servers.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: "fake", ProjectID: "p1", PID: os.Getpid(), URL: "http://x", Status: domain.ServerReady}))
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_root", ProjectID: "p1", Status: domain.SessionStatusRunning, StartedAt: t0,
		GroupKey: "g1", EntryAgent: "conductor", WorkflowID: "ticket", Mode: "manuel", State: domain.RunIdle, LaunchPath: project, Cost: 0.05,
		Outputs: map[string]any{"verdict": "corriger"}}))
	require.NoError(t, usage.AddSession(ctx, domain.SessionUsage{Day: "2026-10-13", SessionID: "ses_root", RootID: "ses_root", ProjectID: "p1", CostUSD: 0.04, TokensIn: 100, TokensOut: 20}))

	tool.entries = []adapters.TranscriptEntry{
		{Time: at(1), SessionID: "ses_root", Kind: adapters.EntryPrompt, Role: "user", Text: "Ticket pt-1. AWS key " + traceAWSKey},
		{Time: at(2), SessionID: "ses_root", Kind: adapters.EntryTool, Agent: "conductor", Call: &adapters.ToolCall{ID: "c1", Action: sessionspec.ActionSkill, Skill: "dev-standards", Status: adapters.CallOK}},
		{Time: at(3), SessionID: "ses_root", Kind: adapters.EntryTool, Agent: "conductor", Child: "ses_child",
			Call: &adapters.ToolCall{ID: "c2", Action: sessionspec.ActionSubagent, Status: adapters.CallOK, Input: map[string]any{"agent": "developer", "prompt": "fix it"}}},
		{Time: at(4), SessionID: "ses_child", ParentID: "ses_root", Kind: adapters.EntryPrompt, Role: "user", Text: "fix it"},
		{Time: at(5), SessionID: "ses_child", ParentID: "ses_root", Kind: adapters.EntryTool, Agent: "developer",
			Call:   &adapters.ToolCall{ID: "c3", Action: sessionspec.ActionShell, Status: adapters.CallOK, Input: map[string]any{"command": "go test ./..."}},
			Output: "ok   x  0.1s\nexport GITLAB_TOKEN=" + traceGitLab},
		{Time: at(6), SessionID: "ses_child", ParentID: "ses_root", Kind: adapters.EntryTool, Agent: "developer",
			Call: &adapters.ToolCall{ID: "c4", Action: sessionspec.ActionShell, Status: adapters.CallFailed, Error: "The user has specified a rule which prevents you from using this specific tool call",
				Input: map[string]any{"command": "git commit -m x", "api_key": traceLiteral}}},
		{Time: at(7), SessionID: "ses_child", ParentID: "ses_root", Kind: adapters.EntryUsage, Agent: "developer", Cost: 0.01, TokensIn: 50, TokensOut: 5},
		{Time: at(8), SessionID: "ses_child", ParentID: "ses_root", Kind: adapters.EntryEnd, Outcome: "succeeded"},
		{Time: at(9), SessionID: "ses_root", Kind: adapters.EntryText, Agent: "conductor", Text: "Done. Proxy token " + traceProxyTok},
		{Time: at(10), SessionID: "ses_root", Kind: adapters.EntryUsage, Agent: "conductor", Cost: 0.02, TokensIn: 70, TokensOut: 9},
	}
	resolved := at(12)
	require.NoError(t, svc.Decisions.Upsert(ctx, &domain.Decision{ID: "permission:ses_root:p1", SessionID: "ses_root", GroupKey: "g1", Kind: domain.DecisionPermission,
		ToolRef: "p1", CreatedAt: at(11), Payload: domain.DecisionPayload{Action: "shell", Resources: []string{"curl -H 'Authorization: Bearer abcdef123456' x"}, Data: map[string]any{domain.DataToolSession: "ses_child"}}}))
	_, err := svc.Decisions.Resolve(ctx, "permission:ses_root:p1", domain.ResolvedByOh, &domain.DecisionResolution{Decision: "reject"}, resolved)
	require.NoError(t, err)
	require.NoError(t, svc.Decisions.Upsert(ctx, &domain.Decision{ID: "checkpoint:ses_root:cp-2", SessionID: "ses_root", GroupKey: "g1", Kind: domain.DecisionCheckpoint,
		ToolRef: "cp-2", CreatedAt: at(13), Payload: domain.DecisionPayload{Title: "Commit", Data: map[string]any{checkpoint.DataCheckpoint: "cp-2", checkpoint.DataApprover: []string{"Grace Hopper"}}}}))
	_, err = cps.UpdateCheckpointState(ctx, "ses_root", func(s *domain.CheckpointState) error {
		s.Timeline = append(s.Timeline, domain.CheckpointEvent{At: at(14), Kind: domain.CheckpointPassed, ID: "cp-2", By: "oh",
			Identity: "Ada Lovelace <ada@example.com>", Approver: []string{"Grace Hopper"}, ApproverMismatch: true})
		return nil
	})
	require.NoError(t, err)

	tr, err := svc.Trace(ctx, "ses_root", TraceOptions{Usage: usage, Secrets: []string{traceLiteral}})
	require.NoError(t, err)
	assert.Equal(t, TranscriptLive, tr.Transcript)
	var buf bytes.Buffer
	require.NoError(t, tr.WriteJSONL(&buf))
	out := buf.String()
	for _, secret := range []string{traceAWSKey, traceProxyTok, traceGitLab, traceLiteral, "abcdef123456"} {
		assert.NotContains(t, out, secret)
	}
	assert.Contains(t, out, Masked)

	lines := strings.Split(strings.TrimSpace(out), "\n")
	evs, err := ReadTraceJSONL(strings.NewReader(out))
	require.NoError(t, err)
	require.Len(t, evs, len(lines))
	kinds := map[string]int{}
	for i, e := range evs {
		var raw map[string]any
		require.NoError(t, json.Unmarshal([]byte(lines[i]), &raw), "one JSON object per line")
		assert.Equal(t, i+1, e.Seq)
		kinds[e.Kind]++
		if i > 1 && i < len(evs)-1 {
			assert.False(t, e.Time.Before(evs[i-1].Time), "oldest first: %d", i)
		}
	}
	assert.Equal(t, TraceSession, evs[0].Kind)
	assert.Equal(t, TraceSummary, evs[len(evs)-1].Kind)
	for k, n := range map[string]int{TracePrompt: 2, TraceSkill: 1, TraceSubagent: 1, TraceTool: 2, TraceUsage: 2, TraceEnd: 1, TraceText: 1, TraceDecision: 2, TraceCheckpoint: 1, TraceOutput: 1} {
		assert.Equal(t, n, kinds[k], k)
	}
	byCall := map[string]TraceEvent{}
	var perm TraceEvent
	for _, e := range evs {
		if e.CallID != "" {
			byCall[e.CallID] = e
		}
		switch {
		case e.Kind == TraceCheckpoint:
			// CP-R9: who validated, when, and out of approver.
			assert.Equal(t, "Ada Lovelace <ada@example.com>", e.Identity)
			assert.True(t, e.ApproverMismatch)
			assert.Equal(t, []string{"Grace Hopper"}, e.Approver)
			assert.False(t, e.Time.IsZero())
		case e.DecisionKind == string(domain.DecisionCheckpoint):
			assert.Equal(t, []string{"Grace Hopper"}, e.Approver, "read back from the stored payload")
		}
		if e.DecisionKind == string(domain.DecisionPermission) {
			perm = e
		}
	}
	assert.Equal(t, "developer", byCall["c2"].Subagent)
	assert.Equal(t, "ses_child", byCall["c2"].Child)
	assert.True(t, byCall["c3"].InSubagent)
	assert.True(t, byCall["c3"].Verification, "go test is a verification of a Go project")
	assert.True(t, byCall["c4"].Denied)
	assert.Equal(t, Masked, byCall["c4"].Input["api_key"])
	assert.Equal(t, "developer", perm.Agent, "the agent of the subagent session that asked")
	require.NotNil(t, perm.Granted)
	assert.False(t, *perm.Granted, "rejected permission")
	assert.True(t, perm.InSubagent)
	sum := evs[len(evs)-1].Data
	assert.InDelta(t, 0.05, sum["cost_usd"], 1e-9, "session total (with what the transcript misses)")
	assert.InDelta(t, 0.03, sum["transcript_cost_usd"], 1e-9)
	assert.InDelta(t, 0.04, sum["ledger_cost_usd"], 1e-9)
	assert.EqualValues(t, 2, sum["denied"])

	// The snapshot saved with the live read serves once the server is gone.
	_, err = os.Stat(filepath.Join(svc.SessionsDir, "ses_root", transcriptFile))
	require.NoError(t, err)
	snap, _ := os.ReadFile(filepath.Join(svc.SessionsDir, "ses_root", transcriptFile))
	assert.NotContains(t, string(snap), traceAWSKey, "the snapshot is masked")
	require.NoError(t, svc.Servers.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: "fake", ProjectID: "p1", Status: domain.ServerStopped}))
	tr, err = svc.Trace(ctx, "ses_root", TraceOptions{})
	require.NoError(t, err)
	assert.Equal(t, TranscriptSnapshot, tr.Transcript)
	assert.Len(t, tr.Events, len(evs))
}

func TestTraceMasking(t *testing.T) {
	m := newMasker([]string{"short", "literal-value-42"})
	for in, want := range map[string]string{
		"TOKEN=abcdefgh1234":                         "TOKEN=" + Masked,
		`{"password": "hunter2hunter2"}`:             `{"password": "` + Masked + `"}`,
		"Authorization: Bearer xyz.abc.def123":       "Authorization: Bearer " + Masked,
		"https://user:p4ssw0rd@gitlab.example.com/x": "https://user:" + Masked + "@gitlab.example.com/x",
		"key " + traceBedrock:                        "key " + Masked,
		"echo $GITLAB_TOKEN":                         "echo $GITLAB_TOKEN",
		"token={env:OH_E2E_MCP_TOKEN}":               "token={env:OH_E2E_MCP_TOKEN}",
		"value literal-value-42 here":                "value " + Masked + " here",
		"short words stay":                           "short words stay",
		"no secret here":                             "no secret here",
	} {
		assert.Equal(t, want, m.text(in), in)
	}
}

func TestWildcard(t *testing.T) {
	assert.True(t, matchesAny("go test ./...", []string{"go test*"}))
	assert.True(t, matchesAny("make test-unit", []string{"make test*"}))
	assert.True(t, matchesAny("cd /p && go test -v", []string{"go test*"}))
	assert.False(t, matchesAny("go run .", []string{"go test*", "go vet*"}))
	assert.True(t, wildcard("a*b*c", "axxbyyc"))
	assert.False(t, wildcard("a*b*c", "axxbyy"))
}

func TestTraceUnknownSession(t *testing.T) {
	svc, ctx := newTestService(t)
	_, err := svc.Trace(ctx, "ses_nope", TraceOptions{})
	assert.ErrorIs(t, err, ErrNoSession)
}
