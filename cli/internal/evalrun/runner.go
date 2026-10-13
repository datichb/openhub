package evalrun

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
)

// Pass outcomes.
const (
	OutcomeCompleted = "completed" // the session ended its work (idle, no decision left)
	OutcomeTimeout   = "timeout"   // the pass ran out of time
	OutcomeBudget    = "budget"    // the budget of the pass was spent
	OutcomeFailed    = "failed"    // the session failed
	OutcomeError     = "error"     // the pass could not run (setup, launch)
	OutcomeInactive  = "inactive"  // the case is not active
)

// Pass is one run of a case with a model.
type Pass struct {
	Case      *Case
	Model     string
	Rep       int // 1-based
	BudgetUSD float64
	// Dir is the work directory of the pass (removed afterwards unless
	// kept); Project is the throwaway clone of the fixture in it.
	Dir     string
	Project string
	// OutDir receives the trace and the result of the pass.
	OutDir string
}

// AnsweredDecision is a decision answered during a pass.
type AnsweredDecision struct {
	Kind       string `json:"kind"`
	Checkpoint string `json:"checkpoint,omitempty"`
	Decision   string `json:"decision,omitempty"`
	Error      string `json:"error,omitempty"`
}

// PassOutput is what an Executor reports.
type PassOutput struct {
	SessionID string             `json:"session_id,omitempty"`
	Outcome   string             `json:"outcome"`
	Error     string             `json:"error,omitempty"`
	CostUSD   float64            `json:"cost_usd"`
	TokensIn  int64              `json:"tokens_in"`
	TokensOut int64              `json:"tokens_out"`
	Duration  time.Duration      `json:"duration_ns"`
	Decisions []AnsweredDecision `json:"decisions,omitempty"`
	// TracePath is the JSONL trace of the session ("" = none).
	TracePath string `json:"trace_path,omitempty"`
	// Throttle is the evidence of a provider rate limit the executor found
	// (logs of the pass), "" when none.
	Throttle string `json:"throttle,omitempty"`
}

// Executor runs the session of a pass on its prepared clone.
type Executor interface {
	Execute(ctx context.Context, p Pass) (*PassOutput, error)
}

// ExecutorFunc adapts a function.
type ExecutorFunc func(ctx context.Context, p Pass) (*PassOutput, error)

// Execute implements Executor.
func (f ExecutorFunc) Execute(ctx context.Context, p Pass) (*PassOutput, error) { return f(ctx, p) }

// PassResult is the outcome of a pass and of its assertions.
type PassResult struct {
	Case       string             `json:"case"`
	Model      string             `json:"model"`
	Rep        int                `json:"rep"`
	Outcome    string             `json:"outcome"`
	Pass       bool               `json:"pass"`
	Error      string             `json:"error,omitempty"`
	SessionID  string             `json:"session_id,omitempty"`
	CostUSD    float64            `json:"cost_usd"`
	TokensIn   int64              `json:"tokens_in"`
	TokensOut  int64              `json:"tokens_out"`
	Seconds    float64            `json:"seconds"`
	Assertions []AssertionResult  `json:"assertions"`
	Decisions  []AnsweredDecision `json:"decisions,omitempty"`
	Trace      string             `json:"trace,omitempty"`
	// Retries are the attempts given up because of a provider rate limit
	// (throttling), before this result.
	Retries []RetriedAttempt `json:"retries,omitempty"`
}

// RetriedAttempt is an attempt of a pass stopped by a provider rate limit
// and run again after a wait.
type RetriedAttempt struct {
	Attempt  int     `json:"attempt"`
	Outcome  string  `json:"outcome"`
	Throttle string  `json:"throttle"`
	CostUSD  float64 `json:"cost_usd"`
	Seconds  float64 `json:"seconds"`
	WaitSecs float64 `json:"wait_seconds"`
	// Dir holds the files of the attempt, relative to the output directory.
	Dir string `json:"dir,omitempty"`
}

// Runner runs the passes of a set of cases.
type Runner struct {
	Exec   Executor
	Models []string
	Reps   int
	// BudgetUSD is the default cost cap of a pass.
	BudgetUSD float64
	// WorkRoot holds the work directories of the passes; OutDir the traces
	// and reports.
	WorkRoot string
	OutDir   string
	// Keep leaves the work directories in place (debugging).
	Keep bool
	// Parallel is the number of passes run at the same time (< 1: one).
	Parallel int
	// BudgetTotalUSD caps the cost of the run (0: none): once the passes
	// done cost that much, no new pass starts (running passes finish).
	BudgetTotalUSD float64
	// Retry runs again a pass stopped by a provider rate limit.
	Retry RetryPolicy
	// Sleep waits before a retry (tests); nil: a timer bound to ctx.
	Sleep func(ctx context.Context, d time.Duration) error
	Log   func(format string, args ...any)
	Now   func() time.Time

	logMu sync.Mutex
}

func (r *Runner) logf(format string, args ...any) {
	if r.Log != nil {
		r.logMu.Lock()
		defer r.logMu.Unlock()
		r.Log(format, args...)
	}
}

func passWord(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

// runAttempt runs one attempt of a pass on a fresh clone. It also returns
// the evidence of a provider rate limit that stopped it ("" = none).
func (r *Runner) runAttempt(ctx context.Context, c *Case, model string, rep int) (res PassResult, throttle string) {
	res = PassResult{Case: c.ID, Model: model, Rep: rep}
	budget := c.BudgetUSD
	if budget <= 0 {
		budget = r.BudgetUSD
	}
	if err := os.MkdirAll(r.WorkRoot, 0o700); err != nil {
		res.Outcome, res.Error = OutcomeError, err.Error()
		return res, ""
	}
	dir, err := os.MkdirTemp(r.WorkRoot, "p-")
	if err != nil {
		res.Outcome, res.Error = OutcomeError, err.Error()
		return res, ""
	}
	if !r.Keep {
		defer func() { _ = RemoveAll(dir) }()
	}
	out := r.passOutDir(c, model, rep)
	if err := os.MkdirAll(out, 0o755); err != nil {
		res.Outcome, res.Error = OutcomeError, err.Error()
		return res, ""
	}
	p := Pass{Case: c, Model: model, Rep: rep, BudgetUSD: budget, Dir: dir, Project: filepath.Join(dir, "project"), OutDir: out}
	base, err := PrepareClone(ctx, c, p.Project)
	if err != nil {
		res.Outcome, res.Error = OutcomeError, "setup: "+err.Error()
		return res, ""
	}
	start := time.Now()
	po, err := r.Exec.Execute(ctx, p)
	if err != nil {
		res.Outcome, res.Error = OutcomeError, err.Error()
		res.Seconds = time.Since(start).Seconds()
		return res, Throttling(res.Error)
	}
	res.Outcome, res.Error, res.SessionID = po.Outcome, po.Error, po.SessionID
	res.CostUSD, res.TokensIn, res.TokensOut, res.Decisions = po.CostUSD, po.TokensIn, po.TokensOut, po.Decisions
	res.Seconds = time.Since(start).Seconds()
	if po.Duration > 0 {
		res.Seconds = po.Duration.Seconds()
	}
	var events []sessionsvc.TraceEvent
	if po.TracePath != "" {
		if f, err := os.Open(po.TracePath); err == nil {
			events, err = sessionsvc.ReadTraceJSONL(f)
			f.Close()
			if err != nil {
				res.Error = strings.TrimSpace(res.Error + "; trace: " + err.Error())
			}
		}
		if rel, err := filepath.Rel(r.OutDir, po.TracePath); err == nil {
			res.Trace = rel
		} else {
			res.Trace = po.TracePath
		}
	}
	if res.CostUSD == 0 && len(events) > 0 {
		if s := events[len(events)-1]; s.Kind == sessionsvc.TraceSummary {
			res.CostUSD, _ = s.Data["cost_usd"].(float64)
		}
	}
	changed, err := ChangedFiles(ctx, p.Project, base)
	if err != nil {
		res.Error = strings.TrimSpace(res.Error + "; changes: " + err.Error())
	}
	ev := Evidence{Events: events, Changed: changed, HiddenTests: func() (bool, string, error) {
		return RunHiddenTests(ctx, c, p.Project)
	}}
	if len(events) == 0 {
		res.Assertions = make([]AssertionResult, len(c.Assertions))
		for i, a := range c.Assertions {
			res.Assertions[i] = AssertionResult{Type: a.Type, Name: a.Label(), Detail: "no trace"}
		}
	} else {
		res.Assertions = Check(c.Assertions, ev)
	}
	res.Pass = res.Outcome == OutcomeCompleted
	for _, a := range res.Assertions {
		res.Pass = res.Pass && a.Pass
	}
	return res, throttleEvidence(po, events)
}

// RemoveAll removes a work directory, the read-only directories it holds
// included (the bundles of a pass home).
func RemoveAll(dir string) error {
	if err := os.RemoveAll(dir); err == nil {
		return nil
	}
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			_ = os.Chmod(p, 0o700)
		}
		return nil
	})
	return os.RemoveAll(dir)
}

func safeName(s string) string {
	return strings.NewReplacer("/", "_", ":", "_", " ", "_").Replace(s)
}

// ── Workspace ─────────────────────────────────────────────────────────────

// gitEnv is the environment of the git commands run on a clone.
func gitEnv() []string {
	return append(os.Environ(), "GIT_AUTHOR_NAME=oh eval", "GIT_AUTHOR_EMAIL=eval@oh.invalid",
		"GIT_COMMITTER_NAME=oh eval", "GIT_COMMITTER_EMAIL=eval@oh.invalid", "GIT_CONFIG_NOSYSTEM=1")
}

func run(ctx context.Context, dir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir, cmd.Env = dir, env
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if err != nil {
		return out.String(), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(out.String()))
	}
	return out.String(), nil
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	return run(ctx, dir, gitEnv(), "git", args...)
}

// PrepareClone makes a throwaway git clone of the case fixture at dest:
// the fixture committed on main, the Beads tickets, then the setup
// commands (each committed when it changes files). It returns the commit the
// session starts from.
func PrepareClone(ctx context.Context, c *Case, dest string) (string, error) {
	if err := copyTree(filepath.Join(c.Dir, FixtureDir), dest); err != nil {
		return "", err
	}
	steps := [][]string{{"init", "-q", "-b", "main"}, {"config", "user.name", "oh eval"}, {"config", "user.email", "eval@oh.invalid"},
		{"config", "commit.gpgsign", "false"}, {"add", "-A"}, {"commit", "-q", "--no-verify", "-m", "fixture"}}
	for _, s := range steps {
		if _, err := git(ctx, dest, s...); err != nil {
			return "", err
		}
	}
	commit := func(msg string) error {
		if st, _ := git(ctx, dest, "status", "--porcelain"); strings.TrimSpace(st) == "" {
			return nil
		}
		if _, err := git(ctx, dest, "add", "-A"); err != nil {
			return err
		}
		_, err := git(ctx, dest, "commit", "-q", "--no-verify", "-m", msg)
		return err
	}
	if c.Beads != nil {
		if err := setupBeads(ctx, c.Beads, dest); err != nil {
			return "", fmt.Errorf("beads: %w", err)
		}
		// bd init writes a .gitignore: part of the starting point.
		if err := commit("setup: beads"); err != nil {
			return "", err
		}
	}
	for _, s := range c.Setup {
		if _, err := run(ctx, dest, gitEnv(), "sh", "-c", s); err != nil {
			return "", err
		}
		if err := commit("setup: " + firstLine(s)); err != nil {
			return "", err
		}
	}
	head, err := git(ctx, dest, "rev-parse", "HEAD")
	return strings.TrimSpace(head), err
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return s
}

// bdTimeout bounds one bd command of the setup (bd init starts Dolt).
const bdTimeout = 2 * time.Minute

func setupBeads(ctx context.Context, b *BeadsSetup, dir string) error {
	bd := func(args ...string) error {
		cctx, cancel := context.WithTimeout(ctx, bdTimeout)
		defer cancel()
		_, err := run(cctx, dir, gitEnv(), "bd", args...)
		return err
	}
	prefix := b.Prefix
	if prefix == "" {
		prefix = "ev"
	}
	if err := bd("init", "--non-interactive", "--quiet", "--prefix", prefix, "--skip-hooks", "--skip-agents", "--setup-exclude"); err != nil {
		return err
	}
	_ = bd("config", "set", "dolt.local-only", "true")
	for _, t := range b.Tickets {
		args := []string{"create", t.Title, "--id", t.ID, "--force"}
		if t.Type != "" {
			args = append(args, "-t", t.Type)
		}
		if t.Description != "" {
			args = append(args, "-d", t.Description)
		}
		if t.Acceptance != "" {
			args = append(args, "--acceptance", t.Acceptance)
		}
		if t.Parent != "" {
			args = append(args, "--parent", t.Parent)
		}
		if len(t.Labels) > 0 {
			args = append(args, "-l", strings.Join(t.Labels, ","))
		}
		if err := bd(args...); err != nil {
			return err
		}
	}
	return nil
}

// ChangedFiles lists the files of the clone changed since base: committed,
// staged, modified or new (Beads and git internals, and the Beads gate
// lock, left out).
func ChangedFiles(ctx context.Context, dir, base string) ([]string, error) {
	seen := map[string]bool{}
	add := func(out string) {
		for _, l := range strings.Split(out, "\n") {
			l = strings.TrimSpace(l)
			if l == "" || strings.HasPrefix(l, ".beads/") || strings.HasPrefix(l, ".git/") || l == ".beads.gate.lock" {
				continue
			}
			seen[l] = true
		}
	}
	out, err := git(ctx, dir, "diff", "--name-only", base)
	if err != nil {
		return nil, err
	}
	add(out)
	out, err = git(ctx, dir, "diff", "--name-only", base, "HEAD")
	if err == nil {
		add(out)
	}
	out, err = git(ctx, dir, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	add(out)
	list := make([]string, 0, len(seen))
	for f := range seen {
		list = append(list, f)
	}
	sort.Strings(list)
	return list, nil
}

// HiddenTestCommand is the command of the hidden tests of a case: the case
// setting, else by stack (go.mod: go test ./..., package.json: npm test,
// else make test).
func HiddenTestCommand(c *Case, dir string) string {
	if c.HiddenTests != "" {
		return c.HiddenTests
	}
	switch {
	case exists(filepath.Join(dir, "go.mod")):
		return "go test ./..."
	case exists(filepath.Join(dir, "package.json")):
		return "npm test --silent"
	case exists(filepath.Join(dir, "pyproject.toml")):
		return "python3 -m pytest -q"
	}
	return "make test"
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// hiddenTestTimeout bounds the hidden tests of a pass.
const hiddenTestTimeout = 5 * time.Minute

// RunHiddenTests copies the hidden tests of a case over its clone (never
// seen by the session) and runs them.
func RunHiddenTests(ctx context.Context, c *Case, dir string) (pass bool, detail string, err error) {
	if err := copyTree(filepath.Join(c.Dir, HiddenTestsDir), dir); err != nil {
		return false, "", err
	}
	cctx, cancel := context.WithTimeout(ctx, hiddenTestTimeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "sh", "-c", HiddenTestCommand(c, dir))
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	tail := lastLines(out.String(), 15)
	var ee *exec.ExitError
	switch {
	case err == nil:
		return true, "", nil
	case errors.As(err, &ee):
		return false, tail, nil
	}
	return false, tail, err
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// copyTree copies the files of src into dst (created), keeping modes.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}
