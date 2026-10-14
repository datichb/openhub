package evalrun

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	Case    string `json:"case"`
	Variant string `json:"variant,omitempty"`
	Model   string `json:"model"`
	Rep     int    `json:"rep"`
}

type passJob struct {
	c       *Case
	variant Variant
	model   string
	rep     int
	// ci, mi, vi are the ranks of the case, the model and the variant: the
	// order of the report.
	ci, mi, vi int
}

// before tells whether j comes before k in the report: case, model,
// repetition, variant.
func (j passJob) before(k passJob) bool {
	switch {
	case j.ci != k.ci:
		return j.ci < k.ci
	case j.mi != k.mi:
		return j.mi < k.mi
	case j.rep != k.rep:
		return j.rep < k.rep
	}
	return j.vi < k.vi
}

// jobQueue hands the passes to the workers; an escalation puts its passes
// in front. pop returns -1 once every pass is done.
type jobQueue struct {
	mu      sync.Mutex
	cond    *sync.Cond
	items   []int
	pending int // queued or running
}

func newJobQueue(n int) *jobQueue {
	q := &jobQueue{pending: n}
	q.cond = sync.NewCond(&q.mu)
	for i := 0; i < n; i++ {
		q.items = append(q.items, i)
	}
	return q
}

func (q *jobQueue) pop() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && q.pending > 0 {
		q.cond.Wait()
	}
	if len(q.items) == 0 {
		return -1
	}
	i := q.items[0]
	q.items = q.items[1:]
	return i
}

// front queues passes before the others.
func (q *jobQueue) front(ids []int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(append([]int{}, ids...), q.items...)
	q.pending += len(ids)
	q.cond.Broadcast()
}

func (q *jobQueue) done() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pending--
	if q.pending <= 0 {
		q.cond.Broadcast()
	}
}

// CasePlan is the number of repetitions of a case in a run with
// repetitions per case (oh eval --tiered, --reps-case).
type CasePlan struct {
	Case string `json:"case"`
	Reps int    `json:"reps"`
	// Reason says why (tiered plan, forced).
	Reason string `json:"reason,omitempty"`
	// Escalated: a pass failed below Report.EscalateTo, so the case ran up
	// to it.
	Escalated bool `json:"escalated,omitempty"`
}

// repsOf is the number of repetitions planned for a case.
func (r *Runner) repsOf(id string, def int) int {
	for _, p := range r.Plan {
		if p.Case == id && p.Reps > 0 {
			return p.Reps
		}
	}
	return def
}

// label names the pass in the logs.
func (j passJob) label() string {
	if j.variant.Name != "" {
		return fmt.Sprintf("%s [%s] %s #%d", j.c.ID, j.variant.Name, j.model, j.rep)
	}
	return fmt.Sprintf("%s %s #%d", j.c.ID, j.model, j.rep)
}

// Run runs every pass, up to Parallel at the same time, and returns the
// report: the passes in the order of a sequential run (case, model,
// repetition, variant: the variants of a repetition run side by side)
// whatever the order they end in. The run goes on after a
// failed pass; ctx cancellation stops it (the passes not started are left
// out). With Plan, each case runs its own number of repetitions; with
// EscalateTo, a case whose pass fails below it runs up to it in the same
// run (its new passes go first in the queue).
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
	rep := &Report{StartedAt: now(), Models: r.Models, Reps: reps, BudgetUSD: r.BudgetUSD, BudgetTotalUSD: r.BudgetTotalUSD, Variants: r.Variants}
	variants := r.Variants
	if len(variants) == 0 {
		variants = []Variant{{}}
	}
	var jobs []passJob
	planned := map[string]int{}
	maxJobs := 0
	for ci, c := range cases {
		rep.Cases = append(rep.Cases, c.Info())
		if !c.IsActive() {
			continue
		}
		n := r.repsOf(c.ID, reps)
		planned[c.ID] = n
		if r.Plan != nil {
			p := CasePlan{Case: c.ID, Reps: n}
			for _, q := range r.Plan {
				if q.Case == c.ID {
					p.Reason = q.Reason
				}
			}
			rep.Plan = append(rep.Plan, p)
		}
		for mi, m := range r.Models {
			for i := 1; i <= n; i++ {
				for vi, v := range variants {
					jobs = append(jobs, passJob{c: c, variant: v, model: m, rep: i, ci: ci, mi: mi, vi: vi})
				}
			}
			maxJobs += max(n, r.EscalateTo) * len(variants)
		}
	}
	if r.EscalateTo > 0 {
		rep.EscalateTo = r.EscalateTo
	}
	workers := r.Parallel
	if workers < 1 {
		workers = 1
	}
	if workers > maxJobs {
		workers = maxJobs
	}
	if workers > 1 {
		r.logf("%d passes, %d at a time", len(jobs), workers)
	}

	var (
		mu        sync.Mutex
		spent     float64
		done      int
		results   = make([]*PassResult, len(jobs), maxJobs)
		skipped   = make([]bool, len(jobs), maxJobs)
		escalated = map[string]bool{} // case + model
		queue     = newJobQueue(len(jobs))
		wg        sync.WaitGroup
	)
	// overBudget tells whether the run spent its cost cap (mu held).
	overBudget := func() bool { return r.BudgetTotalUSD > 0 && spent >= r.BudgetTotalUSD }
	// escalate queues the missing repetitions of a case after a failed
	// pass (mu held); they go first.
	escalate := func(j passJob) {
		key := j.c.ID + "\x00" + j.model
		n := planned[j.c.ID]
		if r.EscalateTo <= n || escalated[key] {
			return
		}
		escalated[key] = true
		var ids []int
		for i := n + 1; i <= r.EscalateTo; i++ {
			for vi, v := range variants {
				jobs = append(jobs, passJob{c: j.c, variant: v, model: j.model, rep: i, ci: j.ci, mi: j.mi, vi: vi})
				results, skipped = append(results, nil), append(skipped, false)
				ids = append(ids, len(jobs)-1)
			}
		}
		for k := range rep.Plan {
			if rep.Plan[k].Case == j.c.ID {
				rep.Plan[k].Escalated = true
			}
		}
		r.logf("%s: failed, %d more pass(es) up to ×%d", j.label(), len(ids), r.EscalateTo)
		queue.front(ids)
	}
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := queue.pop()
				if i < 0 {
					return
				}
				mu.Lock()
				j := jobs[i]
				if overBudget() {
					skipped[i] = true
					mu.Unlock()
					queue.done()
					continue
				}
				mu.Unlock()
				if ctx.Err() != nil {
					queue.done()
					continue
				}
				if workers > 1 {
					r.logf("→ %s", j.label())
				}
				res := r.runPass(ctx, i, j, func(cost float64) bool {
					mu.Lock()
					defer mu.Unlock()
					spent += cost
					return !overBudget()
				})
				mu.Lock()
				done++
				n, total := done, len(jobs)
				results[i] = &res
				if !res.Pass && ctx.Err() == nil {
					escalate(j)
					total = len(jobs)
				}
				mu.Unlock()
				r.logf("[%d/%d] %s: %s %s ($%.3f, %.0fs)", n, total, j.label(), res.Outcome, passWord(res.Pass), res.CostUSD, res.Seconds)
				queue.done()
			}
		}()
	}
	wg.Wait()

	order := make([]int, len(jobs))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return jobs[order[a]].before(jobs[order[b]]) })
	for _, i := range order {
		switch {
		case results[i] != nil:
			rep.Passes = append(rep.Passes, *results[i])
		case skipped[i]:
			rep.Skipped = append(rep.Skipped, SkippedPass{Case: jobs[i].c.ID, Variant: jobs[i].variant.Name, Model: jobs[i].model, Rep: jobs[i].rep})
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
		res, throttle := r.runAttempt(ctx, j)
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
			r.logf("%s: keeping the files of attempt %d: %v", j.label(), attempt, err)
		}
		retries = append(retries, RetriedAttempt{Attempt: attempt, Outcome: res.Outcome, Throttle: throttle, CostUSD: res.CostUSD,
			Seconds: res.Seconds, WaitSecs: wait.Seconds(), Dir: dir})
		r.logf("%s: rate limited (%s), attempt %d in %s", j.label(), oneLine(throttle, 120), attempt+1, wait)
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

// passOutDir is the output directory of a pass:
// passes/<case>/[<variant>/]<model>/<rep>.
func (r *Runner) passOutDir(j passJob) string {
	parts := []string{r.OutDir, "passes", j.c.ID}
	if j.variant.Name != "" {
		parts = append(parts, safeName(j.variant.Name))
	}
	return filepath.Join(append(parts, safeName(j.model), fmt.Sprintf("%d", j.rep))...)
}

// setAside moves the files of a throttled attempt to attempt-<n>/ in the
// pass output directory; it returns that directory relative to OutDir.
func (r *Runner) setAside(j passJob, attempt int) (string, error) {
	out := r.passOutDir(j)
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
