package checkpoint

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// R3: verification run by oh (E1), evidence (E2), fix rounds (E5),
// conditions (CP-R7) and windows (CP-R8).

// fakeExec answers the verification commands: exit code and output per
// command line (default 0), counting the runs.
type fakeExec struct {
	mu    sync.Mutex
	exits map[string]int
	out   string
	runs  []string
	envs  [][]string
	block chan struct{} // set: a command waits for it (or the timeout)
}

func (f *fakeExec) run(ctx context.Context, _ string, args, env []string, out *TailBuffer) (int, error) {
	f.mu.Lock()
	cmd := strings.Join(args, " ")
	f.runs = append(f.runs, cmd)
	f.envs = append(f.envs, env)
	block, exit := f.block, f.exits[cmd]
	f.mu.Unlock()
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return -1, ctx.Err()
		}
	}
	out.WriteString(f.out)
	return exit, nil
}

func (f *fakeExec) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.runs)
}

// verifyWorkflow: cp-2 is verified by oh before it unlocks commit and close.
func verifyWorkflow(v *sessionspec.VerifyDef) *sessionspec.WorkflowRuntime {
	wf := lockWorkflow()
	wf.Checkpoints[1].Verify = v
	return wf
}

func newVerifyService(t *testing.T, v *sessionspec.VerifyDef) (*Service, context.Context, *fakeExec, *fakeHead) {
	t.Helper()
	svc, ctx := newTestService(t)
	writeBundle(t, svc.BundlesDir, "h_verify", verifyWorkflow(v))
	dir := t.TempDir()
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_v", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_verify", Mode: "manuel", State: domain.RunActive, LaunchPath: dir, EntryAgent: "orchestrator-dev"}))
	ex := &fakeExec{exits: map[string]int{}}
	h := &fakeHead{head: "c1"}
	svc.Exec = ex.run
	svc.Fingerprint = func(context.Context, string) string { return h.head }
	svc.Head = func(context.Context, string) (string, bool, error) { return h.head, h.dirty, nil }
	svc.Changed = func(context.Context, string, string) ([]string, error) { return []string{"calc.go"}, nil }
	svc.VerifyWait = 2 * time.Second
	return svc, ctx, ex, h
}

func askV(t *testing.T, svc *Service, ctx context.Context, id string, input map[string]any) *domain.Decision {
	t.Helper()
	in := map[string]any{"id": id, "summary": "done"}
	for k, v := range input {
		in[k] = v
	}
	d, auto, _, err := svc.Asked(ctx, "ses_v", "ses_v", adapters.PendingDecision{ID: "per_" + id, Call: &adapters.ToolCall{Input: in}})
	require.NoError(t, err)
	require.False(t, auto)
	return d
}

func waitVerify(t *testing.T, svc *Service, ctx context.Context, id string) domain.CheckpointVerify {
	t.Helper()
	var r domain.CheckpointVerify
	require.Eventually(t, func() bool {
		var ok bool
		r, ok = svc.VerifyResult(ctx, "ses_v", id)
		return ok && r.Done()
	}, 3*time.Second, 10*time.Millisecond)
	return r
}

func TestVerifyPasses(t *testing.T) {
	svc, ctx, ex, _ := newVerifyService(t, &sessionspec.VerifyDef{Cmd: []string{"make check"}, Expect: "pass", TimeoutSec: 60})
	d := askV(t, svc, ctx, "cp-2", nil)
	r := waitVerify(t, svc, ctx, "cp-2")
	assert.Equal(t, domain.VerifyPassed, r.Status, "started when the checkpoint is asked: the card shows it")
	require.NoError(t, svc.Resolve(ctx, &fakeTool{}, *d, ChoiceApprove, ""))

	res, err := svc.Reached(ctx, "ses_v", Call{ID: "cp-2"})
	require.NoError(t, err)
	assert.Equal(t, []string{"commit", "close"}, res.Unlocked)
	assert.Contains(t, res.Verify, "make check ✓")
	assert.Equal(t, 1, ex.count(), "the result of the same state of the work is reused")
	st, _ := svc.State(ctx, "ses_v")
	assert.Contains(t, st.Unlocks, sessionspec.UnlockCommit)
	assert.Equal(t, domain.CheckpointVerified, st.Timeline[len(st.Timeline)-2].Kind)
}

func TestVerifyFails(t *testing.T) {
	svc, ctx, ex, h := newVerifyService(t, &sessionspec.VerifyDef{Cmd: []string{"make check"}, Expect: "pass", TimeoutSec: 60})
	ex.exits["make check"] = 2
	ex.out = "ok  pkg/a\n--- FAIL: TestSub\nFAIL pkg/calc\n"
	d := askV(t, svc, ctx, "cp-2", nil)
	r := waitVerify(t, svc, ctx, "cp-2")
	assert.Equal(t, domain.VerifyFailed, r.Status)
	assert.Contains(t, r.Excerpt, "--- FAIL: TestSub")

	err := svc.Resolve(ctx, &fakeTool{}, *d, ChoiceApprove, "")
	require.Error(t, err, "a failed verification is not validated by a plain approval")
	assert.Contains(t, err.Error(), "cp-2")

	// Validated in the tool (no override there): refused by the verification.
	_, err = reachValidated(t, svc, ctx, "ses_v", "cp-2")
	var ve *VerifyError
	require.ErrorAs(t, err, &ve)
	assert.ErrorIs(t, err, ErrVerifyFailed)
	assert.Contains(t, err.Error(), "--- FAIL: TestSub", "the agent gets an excerpt")
	st, _ := svc.State(ctx, "ses_v")
	assert.NotContains(t, st.Unlocks, sessionspec.UnlockCommit, "nothing unlocked")
	assert.NotContains(t, st.Approved, "cp-2", "the validation of the failed state is dropped")
	assert.False(t, st.HasPassed("cp-2"))

	// Fixed: a new state of the work runs the verification again.
	ex.exits["make check"] = 0
	h.head = "c2"
	_, err = reachValidated(t, svc, ctx, "ses_v", "cp-2")
	require.NoError(t, err)
	assert.Equal(t, 2, ex.count())
}

func TestVerifyOverride(t *testing.T) {
	svc, ctx, ex, _ := newVerifyService(t, &sessionspec.VerifyDef{Cmd: []string{"make check"}, Expect: "pass", TimeoutSec: 60})
	ex.exits["make check"] = 1
	d := askV(t, svc, ctx, "cp-2", nil)
	waitVerify(t, svc, ctx, "cp-2")
	tool := &fakeTool{}
	require.NoError(t, svc.Resolve(ctx, tool, *d, ChoiceOverride, "flaky test, known"))
	assert.Equal(t, []string{"once"}, tool.replies)

	res, err := svc.Reached(ctx, "ses_v", Call{ID: "cp-2"})
	require.NoError(t, err, "passed despite the failure")
	assert.Equal(t, "flaky test, known", res.Message)
	st, _ := svc.State(ctx, "ses_v")
	assert.Contains(t, st.Unlocks, sessionspec.UnlockCommit)
	var overridden *domain.CheckpointEvent
	for i, e := range st.Timeline {
		if e.Kind == domain.CheckpointOverridden {
			overridden = &st.Timeline[i]
		}
	}
	require.NotNil(t, overridden, "the override is traced")
	assert.Equal(t, "cp-2", overridden.ID)
	assert.Equal(t, string(domain.ResolvedByOh), overridden.By)
	assert.Equal(t, "flaky test, known", overridden.Message)
}

func TestVerifyExpectFailAndMustTouch(t *testing.T) {
	svc, ctx, ex, h := newVerifyService(t, &sessionspec.VerifyDef{Cmd: []string{"go test ./..."}, Expect: "fail", TimeoutSec: 60, MustTouch: []string{"*_test.go"}})
	_, err := reachValidated(t, svc, ctx, "ses_v", "cp-2")
	require.ErrorIs(t, err, ErrVerifyFailed, "must_touch: no test file changed")
	assert.Equal(t, 0, ex.count(), "nothing run")

	svc.Changed = func(context.Context, string, string) ([]string, error) {
		return []string{"calc.go", "calc_test.go"}, nil
	}
	h.head = "c2"
	_, err = reachValidated(t, svc, ctx, "ses_v", "cp-2")
	require.ErrorIs(t, err, ErrVerifyFailed, "the tests pass while a failure is expected")

	ex.exits["go test ./..."] = 1
	h.head = "c3"
	_, err = reachValidated(t, svc, ctx, "ses_v", "cp-2")
	require.NoError(t, err, "the test written first fails: expected")
}

func TestVerifyPendingThenDone(t *testing.T) {
	svc, ctx, ex, _ := newVerifyService(t, &sessionspec.VerifyDef{Cmd: []string{"make check"}, Expect: "pass", TimeoutSec: 60})
	ex.block = make(chan struct{})
	svc.VerifyWait = 50 * time.Millisecond
	_, err := reachValidated(t, svc, ctx, "ses_v", "cp-2")
	require.ErrorIs(t, err, ErrVerifyPending)
	assert.Contains(t, err.Error(), "cp-2")
	close(ex.block)
	waitVerify(t, svc, ctx, "cp-2")
	_, err = svc.Reached(ctx, "ses_v", Call{ID: "cp-2"})
	require.NoError(t, err, "the validation is kept while the verification runs")
	assert.Equal(t, 1, ex.count())
}

func TestVerifyTimeout(t *testing.T) {
	svc, ctx, ex, _ := newVerifyService(t, &sessionspec.VerifyDef{Cmd: []string{"make check"}, Expect: "fail", TimeoutSec: 1})
	ex.block = make(chan struct{})
	defer close(ex.block)
	_, err := reachValidated(t, svc, ctx, "ses_v", "cp-2")
	require.ErrorIs(t, err, ErrVerifyFailed, "a timeout is never the expected failure")
	r, _ := svc.VerifyResult(ctx, "ses_v", "cp-2")
	require.Len(t, r.Commands, 1)
	assert.True(t, r.Commands[0].TimedOut)
}

func TestVerifyWithoutCommandOrLocation(t *testing.T) {
	svc, ctx, ex, _ := newVerifyService(t, &sessionspec.VerifyDef{From: "stack", Kinds: []string{"build", "test"}, Expect: "pass", TimeoutSec: 60})
	_, err := reachValidated(t, svc, ctx, "ses_v", "cp-2")
	require.NoError(t, err, "a project without a detected stack passes")
	r, _ := svc.VerifyResult(ctx, "ses_v", "cp-2")
	assert.Equal(t, domain.VerifyNone, r.Status)
	assert.Equal(t, 0, ex.count())

	_, _ = svc.Sessions.Get(ctx, "ses_v")
	sess := &domain.Session{ID: "ses_v", LaunchPath: "/no/such/dir"}
	r = svc.runVerify(ctx, sess, sessionspec.CheckpointDef{ID: "cp-2", Verify: &sessionspec.VerifyDef{Cmd: []string{"make"}}})
	assert.Equal(t, domain.VerifyUnavailable, r.Status, "location missing: the verification cannot run, never « nothing to verify » (R3-remote)")
	assert.False(t, r.OK())
}

func TestVerifyStackGo(t *testing.T) {
	svc, ctx, ex, _ := newVerifyService(t, &sessionspec.VerifyDef{From: "stack", Kinds: []string{"build", "test", "typecheck"}, Cmd: []string{"./scripts/extra.sh"}, Expect: "pass", TimeoutSec: 60})
	sess, err := svc.Sessions.Get(ctx, "ses_v")
	require.NoError(t, err)
	writeFile(t, sess.LaunchPath, "go.mod", "module x\n")
	_, err = reachValidated(t, svc, ctx, "ses_v", "cp-2")
	require.NoError(t, err)
	assert.Equal(t, []string{"go build ./...", "go test ./...", "./scripts/extra.sh"}, ex.runs)
	for _, kv := range ex.envs[0] {
		assert.False(t, strings.HasPrefix(kv, sessionspec.EnvGroupToken+"="), "cleaned environment")
	}
}

func TestVerifyEnv(t *testing.T) {
	parent := []string{"PATH=/bin", "HOME=/h", "GOPATH=/g", sessionspec.EnvGroupToken + "=t", "GITLAB_TOKEN=x", "AWS_SECRET_ACCESS_KEY=y", "TOOL_SERVER_PASSWORD=z"}
	env := verifyEnv(parent, nil)
	assert.Equal(t, []string{"PATH=/bin", "HOME=/h", "GOPATH=/g", "CI=true", "NO_COLOR=1", "TERM=dumb"}, env)

	// R3-remote: run as the account of the agents, with its own folders.
	env = verifyEnv(append(parent, "USER=root", "TMPDIR=/root/tmp", "XDG_CACHE_HOME=/root/.cache"), &Account{UID: 1000, GID: 1000, Home: "/home/oh"})
	assert.Equal(t, []string{"PATH=/bin", "GOPATH=/g", "HOME=/home/oh", "XDG_CONFIG_HOME=/home/oh/.config", "XDG_CACHE_HOME=/home/oh/.cache",
		"TMPDIR=/home/oh/tmp", "CI=true", "NO_COLOR=1", "TERM=dumb"}, env)
}

func TestExecVerifyAndTail(t *testing.T) {
	out := &TailBuffer{Max: 8}
	exit, err := execVerify(context.Background(), t.TempDir(), []string{"sh", "-c", "echo 0123456789abcdef; exit 3"}, verifyEnv(nil, nil), out, nil)
	require.NoError(t, err)
	assert.Equal(t, 3, exit)
	assert.Equal(t, "9abcdef\n", string(out.Bytes()), "the end of the output is kept")

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _ = execVerify(ctx, t.TempDir(), []string{"sh", "-c", "sleep 30 & sleep 30"}, verifyEnv([]string{"PATH=/bin:/usr/bin"}, nil), &TailBuffer{}, nil)
	assert.Less(t, time.Since(start), 10*time.Second, "the process group is killed on timeout")

	long := strings.Repeat("line\n", 100) + "last"
	ex := excerpt([]byte(long))
	assert.Len(t, strings.Split(ex, "\n"), excerptLines)
	assert.True(t, strings.HasSuffix(ex, "last"))
}

// E2: the evidence of the agent is checked against the shell calls observed.
func TestEvidence(t *testing.T) {
	svc, ctx, _, _ := newVerifyService(t, nil)
	shell := func(id, cmd, status string) {
		in := map[string]any{}
		if cmd != "" {
			in["command"] = cmd
		}
		_, _, err := svc.OnCall(ctx, "ses_v", adapters.ToolCall{ID: id, Action: sessionspec.ActionShell, Status: status, Input: in})
		require.NoError(t, err)
	}
	shell("s1", "go  test ./...", adapters.CallCalled)
	shell("s1", "", adapters.CallOK)
	shell("s2", "go vet ./...", adapters.CallCalled)
	shell("s2", "", adapters.CallFailed)
	d := askV(t, svc, ctx, "cp-2", map[string]any{"evidence": []any{
		map[string]any{"cmd": "go test ./...", "exit": float64(0)},
		map[string]any{"cmd": "go vet ./...", "exit": float64(0)},
		map[string]any{"cmd": "npm test", "exit": float64(0)},
	}})
	assert.Equal(t, []string{"go vet ./...", "npm test"}, d.Payload.Data[DataUnobserved])
	card := svc.Card(ctx, "ses_v", "cp-2")
	require.Len(t, card.Evidence, 3)
	assert.True(t, card.Evidence[0].Observed)
	assert.True(t, card.Evidence[1].Contradicted, "failed while the agent says 0")
	assert.False(t, card.Evidence[2].Observed, "preuve non observée")
	st, _ := svc.State(ctx, "ses_v")
	n := 0
	for _, e := range st.Timeline {
		if e.Kind == domain.CheckpointUnobserved {
			n++
		}
	}
	assert.Equal(t, 2, n)

	res, err := reachValidated(t, svc, ctx, "ses_v", "cp-2")
	require.NoError(t, err)
	assert.Empty(t, res.Unobserved, "no evidence in this call")
	res, err = svc.Reached(ctx, "ses_v", Call{ID: "cp-1", Evidence: []Evidence{{Cmd: "make lint"}}})
	require.Error(t, err, "cp-1 pauses in manuel")
	_ = res
}

// E5: past limits.max_fix_rounds, the developer is locked and the commit
// checkpoint is marked unresolved, until the user steps in or the ticket
// is closed.
func TestFixRoundsLimit(t *testing.T) {
	svc, ctx := newTestService(t)
	wf := lockWorkflow()
	wf.MaxFixRounds = 2
	writeBundle(t, svc.BundlesDir, "h_rounds", wf)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_r", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_rounds", Mode: "manuel", State: domain.RunActive, LaunchPath: "/work", Tickets: []string{"pt-1"}}))
	call := func(agent string) bool {
		changed, _, err := svc.OnCall(ctx, "ses_r", adapters.ToolCall{ID: agent, Action: sessionspec.ActionSubagent, Status: adapters.CallCalled, Input: map[string]any{"agent": agent}})
		require.NoError(t, err)
		return changed
	}
	for i := 0; i < 2; i++ { // implementation + 1 fix round
		assert.False(t, call("developer"))
		assert.False(t, call("reviewer"), "round %d", i+1)
	}
	assert.False(t, call("developer"))
	assert.True(t, call("reviewer"), "the review of the 2nd fix round reaches the cap: no 3rd fix")
	st, _ := svc.State(ctx, "ses_r")
	assert.Equal(t, "pt-1", st.Unresolved)
	rules, err := svc.Rules(ctx, "ses_r")
	require.NoError(t, err)
	assert.Contains(t, rules, sessionspec.PermissionRule{Action: sessionspec.ActionSubagent, Resource: "developer*", Effect: sessionspec.EffectDeny})

	d, auto, _, err := svc.Asked(ctx, "ses_r", "ses_r", adapters.PendingDecision{ID: "per_cp2", Call: &adapters.ToolCall{Input: map[string]any{"id": "cp-2"}}})
	require.NoError(t, err)
	require.False(t, auto)
	assert.Equal(t, "pt-1", d.Payload.Data[DataUnresolved], "the commit checkpoint is marked unresolved")
	assert.Equal(t, "pt-1", svc.Card(ctx, "ses_r", "cp-2").Unresolved)

	tool := &fakeTool{}
	require.NoError(t, svc.Resolve(ctx, tool, *d, ChoiceFix, "one more try"))
	assert.Equal(t, 1, tool.refresh, "the developer is unlocked")
	st, _ = svc.State(ctx, "ses_r")
	assert.Empty(t, st.Unresolved)
	rules, _ = svc.Rules(ctx, "ses_r")
	assert.NotContains(t, rules, sessionspec.PermissionRule{Action: sessionspec.ActionSubagent, Resource: "developer*", Effect: sessionspec.EffectDeny})

	for i := 0; i < 3; i++ {
		call("developer")
		call("reviewer")
	}
	st, _ = svc.State(ctx, "ses_r")
	require.Equal(t, "pt-1", st.Unresolved)
	changed, err := svc.BeadsDone(ctx, "ses_r", BeadsClose, []string{"pt-1"})
	require.NoError(t, err)
	assert.True(t, changed)
	st, _ = svc.State(ctx, "ses_r")
	assert.Empty(t, st.Unresolved, "a closed ticket resets its rounds")
}

// CP-R7: when: adds a pause to an automatic checkpoint, makes a conditional
// one pause only when it holds, and never removes a mandatory pause.
func TestWhenConditions(t *testing.T) {
	svc, ctx := newTestService(t)
	wf := &sessionspec.WorkflowRuntime{ID: "feature", DefaultMode: "semi-auto", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-spec", Behaviors: map[string]string{"semi-auto": "conditional"}, When: &sessionspec.WhenDef{Paths: []string{"docs/specs/**"}}},
		{ID: "cp-1", Behaviors: map[string]string{"semi-auto": "auto"}, When: &sessionspec.WhenDef{Labels: []string{"risky"}, Outputs: []string{"mr"}}},
		{ID: "cp-2", Behaviors: map[string]string{"semi-auto": "pause"}, Mandatory: true, When: &sessionspec.WhenDef{Entry: []string{"nobody"}}},
		{ID: "cp-3", Behaviors: map[string]string{"semi-auto": "auto"}},
	}}
	writeBundle(t, svc.BundlesDir, "h_when", wf)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_w", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_when", Mode: "semi-auto", State: domain.RunActive, LaunchPath: "/work", Tickets: []string{"pt-1"}}))
	changed := []string{"calc.go"}
	labels := []string{"bug"}
	svc.Changed = func(context.Context, string, string) ([]string, error) { return changed, nil }
	svc.Labels = func(_ context.Context, _, ticket string) ([]string, error) {
		if ticket != "pt-1" {
			return nil, errors.New("unknown")
		}
		return labels, nil
	}
	asks := func(id string) bool {
		_, auto, _, err := svc.Asked(ctx, "ses_w", "ses_w", adapters.PendingDecision{ID: "per_" + id, Call: &adapters.ToolCall{Input: map[string]any{"id": id}}})
		require.NoError(t, err)
		eff, err := svc.Evaluate(ctx, "ses_w", bundle.CheckpointAction(), map[string]any{"id": id})
		require.NoError(t, err)
		assert.Equal(t, auto, eff == string(sessionspec.EffectAllow), id)
		return !auto
	}
	rules, err := svc.Rules(ctx, "ses_w")
	require.NoError(t, err)
	assert.Equal(t, sessionspec.EffectAsk, rules[0].Effect, "oh evaluates the conditions: the tool asks")

	assert.False(t, asks("cp-spec"), "conditional, condition false: no pause")
	assert.False(t, asks("cp-1"))
	assert.True(t, asks("cp-2"), "a mandatory pause is never removed")
	assert.False(t, asks("cp-3"))

	changed = []string{"docs/specs/login.md"}
	labels = []string{"Risky"}
	assert.True(t, asks("cp-spec"), "changed path")
	assert.True(t, asks("cp-1"), "ticket label")
	_, err = reachValidated(t, svc, ctx, "ses_w", "cp-1")
	require.NoError(t, err, "a pause added by when: needs a validation")

	labels = nil
	require.NoError(t, svc.Declare(ctx, "ses_w", Output{ID: "mr", Type: "merge_request", Value: "!3"}))
	assert.True(t, asks("cp-1"), "declared output")
}

// CP-R8: the commit window closes once the work is committed.
func TestCommitWindowClosesAtCommit(t *testing.T) {
	svc, ctx, _, h := newVerifyService(t, nil)
	_, err := reachValidated(t, svc, ctx, "ses_v", "cp-2")
	require.NoError(t, err)
	shellOK := func(id string) bool {
		_, _, err := svc.OnCall(ctx, "ses_v", adapters.ToolCall{ID: id, Action: sessionspec.ActionShell, Status: adapters.CallCalled, Input: map[string]any{"command": "git commit -m x"}})
		require.NoError(t, err)
		changed, _, err := svc.OnCall(ctx, "ses_v", adapters.ToolCall{ID: id, Action: sessionspec.ActionShell, Status: adapters.CallOK})
		require.NoError(t, err)
		return changed
	}
	assert.False(t, shellOK("s1"), "HEAD did not move")
	h.head, h.dirty = "c2", true
	assert.False(t, shellOK("s2"), "more to commit (one commit per subtask)")
	h.dirty = false
	assert.True(t, shellOK("s3"), "all committed: the window closes")
	st, _ := svc.State(ctx, "ses_v")
	assert.NotContains(t, st.Unlocks, sessionspec.UnlockCommit)
	assert.Contains(t, st.Unlocks, sessionspec.UnlockClose, "closing stays open until the ticket is closed")
	rules, _ := svc.Rules(ctx, "ses_v")
	assert.Contains(t, rules, sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: "git commit*", Effect: sessionspec.EffectDeny})
}

func TestVerifyAskInReadOnlyWorkflow(t *testing.T) {
	c := sessionspec.CheckpointDef{ID: "cp-x", Behaviors: map[string]string{"auto": "auto", "manuel": "skip"}, Verify: &sessionspec.VerifyDef{Ask: true}}
	assert.Equal(t, sessionspec.CheckpointPause, c.Behavior("auto"), "risk: read: the commands run once validated")
	assert.Equal(t, sessionspec.CheckpointSkip, c.Behavior("manuel"))
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}
