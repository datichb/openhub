package evalrun

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
)

// RetryPolicy runs again a pass stopped by a provider rate limit: up to Max
// more attempts, the n-th after Base·2^(n-1), at most Cap.
type RetryPolicy struct {
	Max  int
	Base time.Duration
	Cap  time.Duration
}

// DefaultRetry is the policy of oh eval: three more attempts, after 30 s,
// 1 min then 2 min.
var DefaultRetry = RetryPolicy{Max: 3, Base: 30 * time.Second, Cap: 4 * time.Minute}

// Wait is the wait before the attempt after the n-th throttled one
// (1-based); slot spreads the passes throttled together.
func (p RetryPolicy) Wait(n, slot int) time.Duration {
	d := p.Base
	for i := 1; i < n && d < p.Cap; i++ {
		d *= 2
	}
	if p.Cap > 0 && d > p.Cap {
		d = p.Cap
	}
	return d + time.Duration(slot%4)*d/8
}

// SkippedPass is a pass not started because the cost cap of the run was
// reached.
type SkippedPass struct {
	Case  string `json:"case"`
	Model string `json:"model"`
	Rep   int    `json:"rep"`
}

type passJob struct {
	c     *Case
	model string
	rep   int
}

// Run runs every pass, up to Parallel at the same time, and returns the
// report: the passes in the order of a sequential run (case, model,
// repetition) whatever the order they end in. The run goes on after a
// failed pass; ctx cancellation stops it (the passes not started are left
// out).
func (r *Runner) Run(ctx context.Context, cases []*Case) (*Report, error) {
	if r.Exec == nil {
		return nil, errors.New("evalrun: no executor")
	}
	reps := r.Reps
	if reps < 1 {
		reps = 1
	}
	now := r.Now
	if now == nil {
		now = time.Now
	}
	rep := &Report{StartedAt: now(), Models: r.Models, Reps: reps, BudgetUSD: r.BudgetUSD, BudgetTotalUSD: r.BudgetTotalUSD}
	var jobs []passJob
	for _, c := range cases {
		rep.Cases = append(rep.Cases, CaseInfo{ID: c.ID, Key: c.Key(), Title: c.Title, Workflow: c.Workflow, Active: c.IsActive(), InactiveReason: c.InactiveReason})
		if !c.IsActive() {
			continue
		}
		for _, m := range r.Models {
			for i := 1; i <= reps; i++ {
				jobs = append(jobs, passJob{c, m, i})
			}
		}
	}
	workers := r.Parallel
	if workers < 1 {
		workers = 1
	}
	if workers > len(jobs) {
		workers = len(jobs)
	}
	if workers > 1 {
		r.logf("%d passes, %d at a time", len(jobs), workers)
	}

	var (
		mu      sync.Mutex
		spent   float64
		done    int
		results = make([]*PassResult, len(jobs))
		skipped = make([]bool, len(jobs))
		next    = make(chan int)
		wg      sync.WaitGroup
	)
	// overBudget tells whether the run spent its cost cap (mu held).
	overBudget := func() bool { return r.BudgetTotalUSD > 0 && spent >= r.BudgetTotalUSD }
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				j := jobs[i]
				mu.Lock()
				if overBudget() {
					skipped[i] = true
					mu.Unlock()
					continue
				}
				mu.Unlock()
				if ctx.Err() != nil {
					continue
				}
				if workers > 1 {
					r.logf("→ %s %s #%d", j.c.ID, j.model, j.rep)
				}
				res := r.runPass(ctx, i, j, func(cost float64) bool {
					mu.Lock()
					defer mu.Unlock()
					spent += cost
					return !overBudget()
				})
				mu.Lock()
				done++
				n := done
				mu.Unlock()
				r.logf("[%d/%d] %s %s #%d: %s %s ($%.3f, %.0fs)", n, len(jobs), j.c.ID, j.model, j.rep, res.Outcome, passWord(res.Pass), res.CostUSD, res.Seconds)
				results[i] = &res
			}
		}()
	}
feed:
	for i := range jobs {
		select {
		case <-ctx.Done():
			break feed
		case next <- i:
		}
	}
	close(next)
	wg.Wait()

	for i, res := range results {
		switch {
		case res != nil:
			rep.Passes = append(rep.Passes, *res)
		case skipped[i]:
			rep.Skipped = append(rep.Skipped, SkippedPass{Case: jobs[i].c.ID, Model: jobs[i].model, Rep: jobs[i].rep})
		}
	}
	if n := len(rep.Skipped); n > 0 {
		r.logf("cost cap of the run reached ($%.3f of $%.2f): %d passes not started", spent, r.BudgetTotalUSD, n)
	}
	rep.FinishedAt = now()
	rep.compute()
	return rep, ctx.Err()
}

// runPass runs a pass, again after a wait while a provider rate limit
// stops it (Retry). spend counts the cost of an attempt in the run and
// tells whether the run may still spend.
func (r *Runner) runPass(ctx context.Context, slot int, j passJob, spend func(cost float64) bool) PassResult {
	var retries []RetriedAttempt
	for attempt := 1; ; attempt++ {
		res, throttle := r.runAttempt(ctx, j.c, j.model, j.rep)
		canSpend := spend(res.CostUSD)
		if throttle == "" || attempt > r.Retry.Max || ctx.Err() != nil || !canSpend {
			if throttle != "" {
				why := fmt.Sprintf("rate limited (throttling), %d attempt(s): %s", attempt, throttle)
				if !canSpend {
					why += " (cost cap of the run reached)"
				}
				if res.Error != "" {
					why = res.Error + "; " + why
				}
				res.Error = why
			}
			res.Retries = retries
			return res
		}
		wait := r.Retry.Wait(attempt, slot)
		dir, err := r.setAside(j, attempt)
		if err != nil {
			r.logf("%s %s #%d: keeping the files of attempt %d: %v", j.c.ID, j.model, j.rep, attempt, err)
		}
		retries = append(retries, RetriedAttempt{Attempt: attempt, Outcome: res.Outcome, Throttle: throttle, CostUSD: res.CostUSD,
			Seconds: res.Seconds, WaitSecs: wait.Seconds(), Dir: dir})
		r.logf("%s %s #%d: rate limited (%s), attempt %d in %s", j.c.ID, j.model, j.rep, oneLine(throttle, 120), attempt+1, wait)
		if err := r.sleep(ctx, wait); err != nil {
			res.Retries = retries
			return res
		}
	}
}

func (r *Runner) sleep(ctx context.Context, d time.Duration) error {
	if r.Sleep != nil {
		return r.Sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// passOutDir is the output directory of a pass.
func (r *Runner) passOutDir(c *Case, model string, rep int) string {
	return filepath.Join(r.OutDir, "passes", c.ID, safeName(model), fmt.Sprintf("%d", rep))
}

// setAside moves the files of a throttled attempt to attempt-<n>/ in the
// pass output directory; it returns that directory relative to OutDir.
func (r *Runner) setAside(j passJob, attempt int) (string, error) {
	out := r.passOutDir(j.c, j.model, j.rep)
	dst := filepath.Join(out, fmt.Sprintf("attempt-%d", attempt))
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "attempt-") {
			continue
		}
		if err := os.Rename(filepath.Join(out, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return "", err
		}
	}
	return filepath.Rel(r.OutDir, dst)
}

// ── Provider rate limits ──────────────────────────────────────────────────

// throttlePattern matches the rate limit errors of Bedrock (and of the
// other providers): ThrottlingException, "Too many requests/tokens, please
// wait", HTTP 429.
var throttlePattern = regexp.MustCompile(`(?i)(throttlingexception|throttled|too many requests|too many tokens|toomanyrequests|rate exceeded|status_?(code)?["':= ]+429\b|\b429 too many)`)

// notThrottle are the refusals that look like a rate limit but are not one:
// the token cap of the pass, enforced by the credential proxy with a 429.
var notThrottle = regexp.MustCompile(`(?i)(token budget exhausted|budget spent|pass budget)`)

// Throttling returns the line of text that shows a provider rate limit,
// "" when none.
func Throttling(text string) string {
	for _, l := range strings.Split(text, "\n") {
		if throttlePattern.MatchString(l) && !notThrottle.MatchString(l) {
			return oneLine(l, 300)
		}
	}
	return ""
}

// throttleEvidence looks for a rate limit in what an attempt left: the
// executor finding, its error, the errors of the trace.
func throttleEvidence(po *PassOutput, events []sessionsvc.TraceEvent) string {
	if po.Outcome == OutcomeCompleted || po.Outcome == OutcomeBudget {
		return ""
	}
	if po.Throttle != "" {
		return po.Throttle
	}
	if t := Throttling(po.Error); t != "" {
		return t
	}
	for _, e := range events {
		if e.Error != "" {
			if t := Throttling(e.Error); t != "" {
				return t
			}
		}
	}
	return ""
}
