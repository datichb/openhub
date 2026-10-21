package evalrun

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
)

// twoCases are two active cases and an inactive one.
func twoCases(t *testing.T) []*Case {
	t.Helper()
	root := t.TempDir()
	writeCase(t, root, "quick", "s7", "id: S7\nworkflow: quick\nassertions:\n  - type: ran\n    command: go test*\n")
	writeCase(t, root, "quick", "s10", "id: S10\nworkflow: quick\nassertions:\n  - type: changed_within\n    paths: [\"*.go\"]\n")
	writeCase(t, root, "ticket", "s8", "id: S8\nworkflow: ticket\nactive: false\ninactive_reason: plus tard\nassertions:\n  - type: no_denied_loop\n")
	cases, err := LoadCases(root)
	require.NoError(t, err)
	return cases
}

// simulated is a pass whose result only depends on its case, model and
// repetition; delay makes the passes end out of order.
func simulated(delay func(p Pass) time.Duration) ExecutorFunc {
	return func(ctx context.Context, p Pass) (*PassOutput, error) {
		time.Sleep(delay(p))
		tr := &sessionsvc.Trace{Events: []sessionsvc.TraceEvent{ev(sessionsvc.TraceSession, nil)}}
		if p.Rep%2 == 1 {
			tr.Events = append(tr.Events, shell("developer", "go test ./...", false))
		}
		path := filepath.Join(p.OutDir, "trace.jsonl")
		f, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		if err := tr.WriteJSONL(f); err != nil {
			return nil, err
		}
		cost := float64(len(p.Case.ID)+len(p.Model)+p.Rep) / 100
		return &PassOutput{SessionID: "ses_" + p.Case.ID, Outcome: OutcomeCompleted, CostUSD: cost, TracePath: path,
			Duration: time.Duration(p.Rep) * time.Second}, nil
	}
}

// noGit prepares a pass without git: the folder of the project, and an
// empty commit, so that no changed file is listed. For the tests of the
// scheduling, the budgets and the retries, whose executors do not read the
// project; PrepareClone keeps its own tests (git).
func noGit(_ context.Context, _ *Case, dest string) (string, error) {
	return "", os.MkdirAll(dest, 0o755)
}

func fixedNow() time.Time { return time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC) }

// A parallel run writes the same reports as a sequential one: the passes
// in the order case, model, repetition, whatever the order they end in.
func TestParallelReportsMatchSequential(t *testing.T) {
	cases := twoCases(t)
	models := []string{"claude-haiku-4-5", "claude-sonnet-4-6"}
	run := func(parallel int, delay func(p Pass) time.Duration) (*Report, []byte, string) {
		out := t.TempDir()
		r := &Runner{Exec: simulated(delay), Models: models, Reps: 3, BudgetUSD: 1, WorkRoot: t.TempDir(), OutDir: out, Parallel: parallel, Now: fixedNow, Prepare: noGit}
		rep, err := r.Run(context.Background(), cases)
		require.NoError(t, err)
		data, err := json.Marshal(rep)
		require.NoError(t, err)
		return rep, data, rep.Markdown(nil)
	}
	seq, seqJSON, seqMD := run(1, func(Pass) time.Duration { return 0 })
	// The last passes end first.
	par, parJSON, parMD := run(4, func(p Pass) time.Duration { return time.Duration(4-p.Rep) * 30 * time.Millisecond })
	require.Len(t, seq.Passes, 12)
	assert.Equal(t, "S7", par.Passes[0].Case)
	assert.Equal(t, 1, par.Passes[0].Rep)
	assert.Equal(t, "S10", par.Passes[11].Case)
	assert.JSONEq(t, string(seqJSON), string(parJSON))
	assert.Equal(t, seqMD, parMD)
}

// No more than N passes run at the same time, and N do.
func TestParallelScheduling(t *testing.T) {
	cases := twoCases(t)
	var (
		mu       sync.Mutex
		inflight int
		maxSeen  int
		calls    int
	)
	full := make(chan struct{})
	var once sync.Once
	exec := ExecutorFunc(func(ctx context.Context, p Pass) (*PassOutput, error) {
		mu.Lock()
		inflight++
		calls++
		if inflight > maxSeen {
			maxSeen = inflight
		}
		if inflight == 3 {
			once.Do(func() { close(full) })
		}
		mu.Unlock()
		select {
		case <-full:
		case <-time.After(5 * time.Second):
		}
		time.Sleep(10 * time.Millisecond)
		mu.Lock()
		inflight--
		mu.Unlock()
		return &PassOutput{Outcome: OutcomeCompleted}, nil
	})
	var logs []string
	r := &Runner{Exec: exec, Models: []string{"m"}, Reps: 4, WorkRoot: t.TempDir(), OutDir: t.TempDir(), Parallel: 3, Prepare: noGit,
		Log: func(format string, args ...any) { logs = append(logs, fmt.Sprintf(format, args...)) }}
	rep, err := r.Run(context.Background(), cases)
	require.NoError(t, err)
	assert.Equal(t, 8, calls)
	assert.Equal(t, 3, maxSeen)
	require.Len(t, rep.Passes, 8)
	assert.Contains(t, logs, "8 passes, 3 at a time")
	// The counter is the rank of the end of the pass, taken before the
	// line is written: the lines of concurrent passes arrive in any order.
	for n := 1; n <= 8; n++ {
		prefix := fmt.Sprintf("[%d/8] ", n)
		found := 0
		for _, l := range logs {
			if strings.HasPrefix(l, prefix) {
				found++
			}
		}
		assert.Equal(t, 1, found, "one line starts with %q", prefix)
	}

	// More workers than passes, and a cancelled run starts nothing.
	r.Parallel = 50
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls = 0
	rep, err = r.Run(ctx, cases)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, calls)
	assert.Empty(t, rep.Passes)
}

// Once the passes done cost the cap of the run, no new pass starts; the
// report lists them.
func TestBudgetTotal(t *testing.T) {
	cases := twoCases(t)
	exec := ExecutorFunc(func(ctx context.Context, p Pass) (*PassOutput, error) {
		return &PassOutput{Outcome: OutcomeCompleted, CostUSD: 0.4}, nil
	})
	r := &Runner{Exec: exec, Models: []string{"m"}, Reps: 4, WorkRoot: t.TempDir(), OutDir: t.TempDir(), BudgetTotalUSD: 1, Prepare: noGit}
	rep, err := r.Run(context.Background(), cases)
	require.NoError(t, err)
	require.Len(t, rep.Passes, 3, "0.4 + 0.4 < 1: the third starts, then the cap is reached")
	assert.Len(t, rep.Skipped, 5)
	assert.Equal(t, SkippedPass{Case: "S7", Model: "m", Rep: 4}, rep.Skipped[0])
	assert.InDelta(t, 1.2, rep.TotalCost, 1e-9)
	md := rep.Markdown(nil)
	assert.Contains(t, md, "Plafond global : $1.00 · atteint : 5 passage(s) non lancé(s)")
	assert.Contains(t, md, "## Passages non lancés (plafond global atteint)\n\n- S7 · m · #4\n- S10 · m · #1\n")

	// In parallel, the running passes finish (one more may have started
	// before the cap was reached); none starts after it.
	r.Parallel = 2
	rep, err = r.Run(context.Background(), cases)
	require.NoError(t, err)
	assert.Contains(t, []int{3, 4}, len(rep.Passes))
	assert.Len(t, rep.Skipped, 8-len(rep.Passes))
	assert.GreaterOrEqual(t, rep.TotalCost, 1.0)

	// Without a cap, every pass runs.
	r.BudgetTotalUSD = 0
	rep, err = r.Run(context.Background(), cases)
	require.NoError(t, err)
	assert.Len(t, rep.Passes, 8)
	assert.Empty(t, rep.Skipped)
}

// A pass stopped by a provider rate limit runs again after a growing wait;
// other failures do not; the report records the attempts.
func TestThrottlingRetry(t *testing.T) {
	cases := twoCases(t)[:1] // S7
	attempts := map[int]int{}
	var mu sync.Mutex
	exec := ExecutorFunc(func(ctx context.Context, p Pass) (*PassOutput, error) {
		mu.Lock()
		attempts[p.Rep]++
		n := attempts[p.Rep]
		mu.Unlock()
		require.NoError(t, os.WriteFile(filepath.Join(p.OutDir, "pass.log"), []byte(fmt.Sprint("attempt ", n)), 0o644))
		switch {
		case p.Rep == 1 && n <= 2: // throttled twice, then done
			return &PassOutput{Outcome: OutcomeFailed, CostUSD: 0.01, Throttle: "ThrottlingException: Too many tokens, please wait before trying again."}, nil
		case p.Rep == 2: // always throttled
			return &PassOutput{Outcome: OutcomeError, Error: "session failed: 429 Too Many Requests"}, nil
		case p.Rep == 3: // the token cap of the pass: not a rate limit
			return &PassOutput{Outcome: OutcomeError, Error: "credproxy: session token budget exhausted (429)"}, nil
		case p.Rep == 4: // another failure
			return &PassOutput{Outcome: OutcomeFailed, Error: "model refused"}, nil
		}
		return &PassOutput{Outcome: OutcomeCompleted, CostUSD: 0.1}, nil
	})
	var waits []time.Duration
	out := t.TempDir()
	r := &Runner{Exec: exec, Models: []string{"m"}, Reps: 4, WorkRoot: t.TempDir(), OutDir: out, Prepare: noGit,
		Retry: RetryPolicy{Max: 3, Base: 30 * time.Second, Cap: 4 * time.Minute},
		Sleep: func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil }}
	rep, err := r.Run(context.Background(), cases)
	require.NoError(t, err)
	assert.Equal(t, map[int]int{1: 3, 2: 4, 3: 1, 4: 1}, attempts)
	// Rep 2 is the second job: its waits are spread by an eighth.
	sp := func(d time.Duration) time.Duration { return d + d/8 }
	assert.Equal(t, []time.Duration{30 * time.Second, time.Minute, sp(30 * time.Second), sp(time.Minute), sp(2 * time.Minute)}, waits)

	p1 := rep.Passes[0]
	assert.Equal(t, OutcomeCompleted, p1.Outcome)
	require.Len(t, p1.Retries, 2)
	p1.Retries[0].Seconds = 0
	assert.Equal(t, RetriedAttempt{Attempt: 1, Outcome: OutcomeFailed, Throttle: "ThrottlingException: Too many tokens, please wait before trying again.",
		CostUSD: 0.01, WaitSecs: 30, Dir: filepath.Join("passes", "S7", "m", "1", "attempt-1")}, p1.Retries[0])
	data, err := os.ReadFile(filepath.Join(out, p1.Retries[1].Dir, "pass.log"))
	require.NoError(t, err)
	assert.Equal(t, "attempt 2", string(data), "the files of an attempt are kept apart")
	data, err = os.ReadFile(filepath.Join(out, "passes", "S7", "m", "1", "pass.log"))
	require.NoError(t, err)
	assert.Equal(t, "attempt 3", string(data))

	p2 := rep.Passes[1]
	assert.Len(t, p2.Retries, 3)
	assert.Contains(t, p2.Error, "rate limited (throttling), 4 attempt(s): session failed: 429 Too Many Requests")
	assert.Empty(t, rep.Passes[2].Retries)
	assert.NotContains(t, rep.Passes[2].Error, "rate limited")
	assert.Empty(t, rep.Passes[3].Retries)
	assert.InDelta(t, 0.12, rep.TotalCost, 1e-9, "the cost of the attempts run again counts")
	md := rep.Markdown(nil)
	assert.Contains(t, md, "## Relances (limite de débit du fournisseur)")
	assert.Contains(t, md, "- S7 · m · #1, tentative 1 : ThrottlingException: Too many tokens, please wait before trying again. (issue failed, $0.010, attente 30 s)")

	// The cost cap of the run stops the retries.
	attempts = map[int]int{}
	r.Reps, r.BudgetTotalUSD = 1, 0.015
	rep, err = r.Run(context.Background(), cases)
	require.NoError(t, err)
	assert.Equal(t, 2, attempts[1])
	assert.Contains(t, rep.Passes[0].Error, "cost cap of the run reached")
}

func TestThrottlingDetection(t *testing.T) {
	for _, s := range []string{
		"ThrottlingException: Rate exceeded",
		"AI_APICallError: Too many tokens, please wait before trying again.",
		`{"statusCode":429,"message":"x"}`,
		"upstream: 429 Too Many Requests",
	} {
		assert.NotEmpty(t, Throttling("ok\n"+s), s)
	}
	for _, s := range []string{
		"credproxy: session token budget exhausted",
		"pass budget spent ($1.02 of $1.00)",
		"session failed: model refused",
		"line 429 of calc.go",
	} {
		assert.Empty(t, Throttling(s), s)
	}
	p := RetryPolicy{Base: 30 * time.Second, Cap: 4 * time.Minute}
	assert.Equal(t, 30*time.Second, p.Wait(1, 0))
	assert.Equal(t, 4*time.Minute, p.Wait(5, 0))
	assert.Equal(t, 30*time.Second+30*time.Second/8, p.Wait(1, 1), "passes throttled together do not retry together")
}

// The work directory of a pass goes away with the read-only bundles of its
// home.
func TestRemoveAllReadOnly(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	ro := filepath.Join(dir, "h", "bundles", "abc")
	require.NoError(t, os.MkdirAll(ro, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ro, "f"), []byte("x"), 0o444))
	require.NoError(t, os.Chmod(ro, 0o555))
	require.NoError(t, RemoveAll(dir))
	assert.NoDirExists(t, dir)
}

// Runner.Prepare replaces PrepareClone: it is called once per pass, with the
// project of the pass; its error is a setup error; an empty commit lists no
// changed file instead of failing on a folder without git.
func TestRunnerPrepare(t *testing.T) {
	cases := twoCases(t)[:1]
	var mu sync.Mutex
	var dests []string
	prepare := func(ctx context.Context, c *Case, dest string) (string, error) {
		mu.Lock()
		dests = append(dests, dest)
		mu.Unlock()
		assert.Equal(t, "S7", c.ID)
		return noGit(ctx, c, dest)
	}
	var projects []string
	exec := ExecutorFunc(func(ctx context.Context, p Pass) (*PassOutput, error) {
		projects = append(projects, p.Project)
		assert.DirExists(t, p.Project)
		assert.NoDirExists(t, filepath.Join(p.Project, ".git"), "no clone")
		return &PassOutput{Outcome: OutcomeCompleted}, nil
	})
	r := &Runner{Exec: exec, Models: []string{"m"}, Reps: 2, WorkRoot: t.TempDir(), OutDir: t.TempDir(), Prepare: prepare}
	rep, err := r.Run(context.Background(), cases)
	require.NoError(t, err)
	require.Len(t, rep.Passes, 2)
	assert.Equal(t, projects, dests, "one preparation per pass, for the project it runs in")
	for _, p := range rep.Passes {
		assert.Empty(t, p.Error)
	}

	r.Prepare = func(context.Context, *Case, string) (string, error) { return "", os.ErrPermission }
	rep, err = r.Run(context.Background(), cases)
	require.NoError(t, err)
	assert.Equal(t, OutcomeError, rep.Passes[0].Outcome)
	assert.Contains(t, rep.Passes[0].Error, "setup: ")
}

// The fake preparation changes nothing to what a run reports: the report of
// a run on real clones is the one of a run without git.
func TestNoGitSameReport(t *testing.T) {
	requireGit(t)
	cases := twoCases(t)[2:] // S10: changed_within, read from the clone
	run := func(prepare func(context.Context, *Case, string) (string, error)) []byte {
		r := &Runner{Exec: simulated(func(Pass) time.Duration { return 0 }), Models: []string{"m"}, Reps: 1, BudgetUSD: 1,
			WorkRoot: t.TempDir(), OutDir: t.TempDir(), Now: fixedNow, Prepare: prepare}
		rep, err := r.Run(context.Background(), cases)
		require.NoError(t, err)
		require.Len(t, rep.Passes, 1)
		data, err := json.Marshal(rep)
		require.NoError(t, err)
		return data
	}
	assert.JSONEq(t, string(run(nil)), string(run(noGit)))
}
