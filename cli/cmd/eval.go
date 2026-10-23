package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/evalrun"
	"github.com/datichb/openhub/cli/internal/hubcontent"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/runsvc"
	checkpointsvc "github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/services/launch"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// `oh eval` (P0.5): behavioural evaluations of the shipped workflows with a
// real model. Each pass runs in its own oh home (OH_HOME and HOME are
// temporary: the user's database, keychain and daemon are never touched),
// on a throwaway clone of the case fixture: a child `oh eval pass` launches
// the workflow without interface, answers its decisions through the
// SessionService (checkpoints, permissions, questions), then writes the
// session trace. The runner (internal/evalrun) checks the assertions and
// writes the reports.

// Environment of the evaluations.
const (
	envEvalModels = "OH_EVAL_MODELS"
	envEvalReps   = "OH_EVAL_REPS"
	envEvalBudget = "OH_EVAL_BUDGET_USD"
	envEvalToken  = "OH_EVAL_BEDROCK_TOKEN"
	envEvalPar    = "OH_EVAL_PARALLEL"
	envEvalTotal  = "OH_EVAL_BUDGET_TOTAL"
	// defaultEvalModel is Haiku (cheap); OH_EVAL_MODELS adds Sonnet.
	defaultEvalModel = "claude-haiku-4-5"
	// defaultEvalBudget is the cost cap of a pass (USD);
	// defaultEvalBudgetSonnet when a Sonnet model runs (a ticket session
	// of Sonnet goes past 1 $, seen in L3).
	defaultEvalBudget       = 1.0
	defaultEvalBudgetSonnet = 2.5
	// evalRegion is the Bedrock region of the evaluations.
	evalRegion = "eu-west-1"
)

func init() {
	cmd := &cobra.Command{
		Use:   "eval",
		Short: i18n.T("cmd.eval.short"),
		Long:  i18n.T("cmd.eval.long"),
		Args:  cobra.NoArgs,
		RunE:  runEval,
	}
	addEvalFlags(cmd.Flags())
	// The orchestrator only reads files: the passes have their own home.
	withInitLevel(cmd, levelLight)

	pass := &cobra.Command{
		Use:    "pass",
		Short:  "Run one evaluation pass (internal, run by oh eval in a temporary OH_HOME)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE:   runEvalPass,
	}
	pass.Flags().String("spec", "", "pass specification (JSON)")
	withInitLevel(pass, levelFull)
	cmd.AddCommand(pass)
	rootCmd.AddCommand(cmd)
}

// addEvalFlags defines the flags of oh eval.
func addEvalFlags(f *pflag.FlagSet) {
	f.String("cases", "", i18n.T("cmd.eval.flags.cases"))
	f.StringSlice("case", nil, i18n.T("cmd.eval.flags.case"))
	f.StringSlice("models", nil, i18n.T("cmd.eval.flags.models"))
	f.Int("reps", 0, i18n.T("cmd.eval.flags.reps"))
	f.Float64("budget", 0, i18n.T("cmd.eval.flags.budget"))
	f.Int("parallel", 0, i18n.T("cmd.eval.flags.parallel"))
	f.Float64("budget-total", 0, i18n.T("cmd.eval.flags.budget_total"))
	f.String("out", "", i18n.T("cmd.eval.flags.out"))
	f.String("baseline", "", i18n.T("cmd.eval.flags.baseline"))
	f.String("record-baseline", "", i18n.T("cmd.eval.flags.record_baseline"))
	f.String("note", "", i18n.T("cmd.eval.flags.note"))
	f.Bool("keep", false, i18n.T("cmd.eval.flags.keep"))
	f.Bool("list", false, i18n.T("cmd.eval.flags.list"))
	f.Bool("strict", false, i18n.T("cmd.eval.flags.strict"))
	f.String("rescore", "", i18n.T("cmd.eval.flags.rescore"))
	f.StringArray("variant", nil, i18n.T("cmd.eval.flags.variant"))
	f.Bool("tiered", false, i18n.T("cmd.eval.flags.tiered"))
	f.StringSlice("reps-case", nil, i18n.T("cmd.eval.flags.reps_case"))
	f.Bool("escalate", false, i18n.T("cmd.eval.flags.escalate"))
	f.Bool("plan", false, i18n.T("cmd.eval.flags.plan"))
	f.StringArray("costs", nil, i18n.T("cmd.eval.flags.costs"))
	f.StringArray("history", nil, i18n.T("cmd.eval.flags.history"))
	f.Int("stable-measures", evalrun.DefaultStableMeasures, i18n.T("cmd.eval.flags.stable_measures"))
}

func runEval(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	dir, err := evalCasesDir(flagString(cmd, "cases"))
	if err != nil {
		return err
	}
	cases, err := evalrun.LoadCases(dir)
	if err != nil {
		return err
	}
	if list, _ := cmd.Flags().GetBool("list"); list {
		printEvalCases(cmd.OutOrStdout(), cases)
		return nil
	}
	if dir := flagString(cmd, "rescore"); dir != "" {
		return rescoreEval(cmd, dir, cases)
	}
	ids, _ := cmd.Flags().GetStringSlice("case")
	sel, err := evalrun.Select(cases, ids)
	if err != nil {
		return err
	}
	models, _ := cmd.Flags().GetStringSlice("models")
	if len(models) == 0 {
		models = evalModels()
	}
	tiered, _ := cmd.Flags().GetBool("tiered")
	reps, _ := cmd.Flags().GetInt("reps")
	if reps <= 0 {
		reps = envInt(envEvalReps, defaultEvalReps(tiered))
	}
	budget, _ := cmd.Flags().GetFloat64("budget")
	if budget <= 0 {
		budget = envFloat(envEvalBudget, defaultPassBudget(models))
	}
	parallel, _ := cmd.Flags().GetInt("parallel")
	if parallel <= 0 {
		parallel = envInt(envEvalPar, 1)
	}
	total, _ := cmd.Flags().GetFloat64("budget-total")
	if total <= 0 {
		total = envFloat(envEvalTotal, 0)
	}
	variants, err := evalrun.ParseVariants(mustStringArray(cmd, "variant"))
	if err != nil {
		return err
	}
	if len(variants) > 0 && flagString(cmd, "record-baseline") != "" {
		return errors.New(i18n.T("cmd.eval.variant_baseline"))
	}
	history, err := evalCostSources(mustStringArray(cmd, "history"))
	if err != nil {
		return err
	}
	plan, escalateTo, base, err := evalPlan(cmd, cases, sel, models, reps, tiered, history)
	if err != nil {
		return err
	}
	if dry, _ := cmd.Flags().GetBool("plan"); dry {
		costs, err := evalCostSources(mustStringArray(cmd, "costs"))
		if err != nil {
			return err
		}
		est := evalrun.EstimateRun(sel, plan, models, reps, len(variants), escalateTo, &evalrun.CostBook{Base: base, Extra: append(costs, history...), Budget: budget})
		est.Tiered, est.BudgetTotal = tiered, total
		fmt.Fprint(cmd.OutOrStdout(), est.Markdown())
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	out := flagString(cmd, "out")
	if out == "" {
		out = filepath.Join(os.TempDir(), "oh-eval", time.Now().Format("20060102-150405"))
	}
	if out, err = filepath.Abs(out); err != nil {
		return err
	}
	work, err := os.MkdirTemp(os.TempDir(), "ohe-") // short: it holds the daemon sockets
	if err != nil {
		return err
	}
	keep, _ := cmd.Flags().GetBool("keep")
	if !keep {
		defer func() { _ = evalrun.RemoveAll(work) }()
	}
	errOut := cmd.ErrOrStderr()
	if err := prepareEvalVariants(variants, sel, work, errOut); err != nil {
		return err
	}
	token, err := evalBedrockToken(ctx)
	if err != nil {
		return err
	}
	if plan != nil {
		fmt.Fprintln(errOut, i18n.Tf("cmd.eval.start_plan", len(sel), strings.Join(models, ", "), planSummary(plan, escalateTo), budget, out))
	} else {
		fmt.Fprintln(errOut, i18n.Tf("cmd.eval.start", len(sel), strings.Join(models, ", "), reps, budget, out))
	}
	if total > 0 {
		fmt.Fprintln(errOut, i18n.Tf("cmd.eval.budget_total", total))
	}
	r := &evalrun.Runner{
		Exec: &childEvalExecutor{exe: exe, token: token, goEnv: goEnvironment(ctx)}, Models: models, Reps: reps, BudgetUSD: budget,
		WorkRoot: work, OutDir: out, Keep: keep, Parallel: parallel, Variants: variants, BudgetTotalUSD: total, Retry: evalrun.DefaultRetry,
		Plan: plan, EscalateTo: escalateTo,
		Log: func(format string, args ...any) { fmt.Fprintf(errOut, "  "+format+"\n", args...) },
	}
	rep, runErr := r.Run(ctx, sel)
	if rep == nil {
		return runErr
	}
	rep.Note = flagString(cmd, "note")
	if err := writeEvalReports(cmd, rep, out); err != nil {
		return err
	}
	if runErr != nil {
		return runErr
	}
	if strict, _ := cmd.Flags().GetBool("strict"); strict {
		for _, p := range rep.Passes {
			if !p.Pass {
				return &ExitError{Code: ExitFailure}
			}
		}
	}
	return nil
}

// writeEvalReports writes report.json and report.md in out (compared with
// --baseline), records the baseline when asked, and prints the report.
func writeEvalReports(cmd *cobra.Command, rep *evalrun.Report, out string) error {
	var base *evalrun.Report
	if p := flagString(cmd, "baseline"); p != "" {
		if b, err := evalrun.ReadReport(p); err == nil {
			base = b
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := rep.WriteJSON(filepath.Join(out, "report.json")); err != nil {
		return err
	}
	md := rep.Markdown(base)
	if err := os.WriteFile(filepath.Join(out, "report.md"), []byte(md), 0o644); err != nil {
		return err
	}
	errOut := cmd.ErrOrStderr()
	if p := flagString(cmd, "record-baseline"); p != "" {
		// The previous baseline at that path gives its measures to the
		// history of the new one (tiered reference).
		prev, err := evalrun.ReadReport(p)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := rep.Baseline(rep.Note).WithHistory(prev).WriteJSON(p); err != nil {
			return err
		}
		fmt.Fprintln(errOut, i18n.Tf("cmd.eval.baseline_written", p))
	}
	fmt.Fprint(cmd.OutOrStdout(), md)
	fmt.Fprintln(errOut, i18n.Tf("cmd.eval.done", filepath.Join(out, "report.md"), rep.TotalCost))
	return nil
}

// rescoreEval checks the assertions again on the traces of a previous run
// (no model call).
func rescoreEval(cmd *cobra.Command, dir string, cases []*evalrun.Case) error {
	rep, err := evalrun.ReadReport(filepath.Join(dir, "report.json"))
	if err != nil {
		return err
	}
	out, err := evalrun.Rescore(rep, dir, cases)
	if err != nil {
		return err
	}
	if note := flagString(cmd, "note"); note != "" {
		out.Note = note
	}
	return writeEvalReports(cmd, out, dir)
}

// prepareEvalVariants writes the hub of each variant in work and checks it
// (skills, workflows of the cases, bundles) before any pass: a variant that
// breaks the hub stops the run here.
func prepareEvalVariants(variants []evalrun.Variant, cases []*evalrun.Case, work string, errOut io.Writer) error {
	if len(variants) == 0 {
		return nil
	}
	embedded := ""
	for _, v := range variants {
		if v.Source == "" {
			embedded = filepath.Join(work, "variants", "_embedded")
			if err := hubcontent.Extract(embedded); err != nil {
				return err
			}
			break
		}
	}
	var workflows []string
	for _, c := range cases {
		if c.IsActive() {
			workflows = append(workflows, c.Workflow)
		}
	}
	for i := range variants {
		v := &variants[i]
		if err := evalrun.PrepareVariant(v, embedded, work); err != nil {
			return err
		}
		if err := evalrun.CheckHub(v.Hub, workflows); err != nil {
			return fmt.Errorf("%s\n%w", i18n.Tf("cmd.eval.variant_invalid", v.Name), err)
		}
		src := v.Source
		if src == "" {
			src = i18n.T("cmd.eval.variant_embedded")
		}
		line := i18n.Tf("cmd.eval.variant", v.Name, src)
		if len(v.Removed) > 0 {
			line += " · " + strings.Join(v.Removed, " ; ")
		}
		fmt.Fprintln(errOut, line)
	}
	return nil
}

// defaultEvalReps is the number of repetitions of a case when neither
// --reps nor OH_EVAL_REPS sets it: 3 in a tiered reference (the
// repetitions of an unstable case), else 1.
func defaultEvalReps(tiered bool) int {
	if tiered {
		return 3
	}
	return 1
}

// evalPlan computes the repetitions per case of a run: from the baseline
// with --tiered, then --reps-case; nil when every case runs reps times. It
// also returns the escalation target (0: none) and the baseline read
// (nil: none). The measures of the history reports count after those of
// the baseline, by date.
func evalPlan(cmd *cobra.Command, all, sel []*evalrun.Case, models []string, reps int, tiered bool, history []evalrun.CostSource) ([]evalrun.CasePlan, int, *evalrun.Report, error) {
	var base *evalrun.Report
	if p := flagString(cmd, "baseline"); p != "" {
		b, err := evalrun.ReadReport(p)
		switch {
		case err == nil:
			base = b
		case tiered:
			return nil, 0, nil, errors.New(i18n.Tf("cmd.eval.tiered_baseline", err))
		case !errors.Is(err, os.ErrNotExist):
			return nil, 0, nil, err
		}
	} else if tiered {
		return nil, 0, nil, errors.New(i18n.Tf("cmd.eval.tiered_baseline", "no --baseline"))
	}
	forced, err := evalrun.ParseRepsCase(mustStringSlice(cmd, "reps-case"), all)
	if err != nil {
		return nil, 0, nil, err
	}
	var plan []evalrun.CasePlan
	if tiered {
		measured := base
		if len(history) > 0 {
			extra := make([]*evalrun.Report, len(history))
			for i, h := range history {
				extra[i] = h.Report
			}
			measured = base.WithReports(extra)
		}
		stable, _ := cmd.Flags().GetInt("stable-measures")
		plan = evalrun.TieredPlan(measured, sel, models, reps, stable)
	}
	plan = evalrun.ApplyReps(plan, forced, sel, reps)
	escalate := tiered
	if cmd.Flags().Changed("escalate") {
		escalate, _ = cmd.Flags().GetBool("escalate")
	}
	escalateTo := 0
	if escalate {
		escalateTo = reps
	}
	return plan, escalateTo, base, nil
}

// planSummary sums up the repetitions per case of a run (×1: 30 cases…).
func planSummary(plan []evalrun.CasePlan, escalateTo int) string {
	counts := map[int]int{}
	maxReps := 0
	for _, p := range plan {
		counts[p.Reps]++
		maxReps = max(maxReps, p.Reps)
	}
	var parts []string
	for n := 1; n <= maxReps; n++ {
		if counts[n] > 0 {
			parts = append(parts, fmt.Sprintf("×%d : %d", n, counts[n]))
		}
	}
	if escalateTo > 0 {
		parts = append(parts, i18n.Tf("cmd.eval.plan_escalate", escalateTo))
	}
	return strings.Join(parts, ", ")
}

// evalCostSources reads the reports given with --costs (glob patterns).
func evalCostSources(patterns []string) ([]evalrun.CostSource, error) {
	var out []evalrun.CostSource
	for _, pat := range patterns {
		files, err := filepath.Glob(pat)
		if err != nil {
			return nil, err
		}
		if len(files) == 0 {
			files = []string{pat}
		}
		for _, f := range files {
			r, err := evalrun.ReadReport(f)
			if err != nil {
				return nil, err
			}
			out = append(out, evalrun.CostSource{Name: filepath.Base(filepath.Dir(f)) + "/" + filepath.Base(f), Report: r})
		}
	}
	return out, nil
}

func mustStringSlice(cmd *cobra.Command, name string) []string {
	v, _ := cmd.Flags().GetStringSlice(name)
	return v
}

func mustStringArray(cmd *cobra.Command, name string) []string {
	v, _ := cmd.Flags().GetStringArray(name)
	return v
}

func flagString(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}

// evalCasesDir finds the evals directory: the flag, else evals/ or ../evals
// from the current directory (repository root or cli/).
func evalCasesDir(flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	for _, d := range []string{"evals", filepath.Join("..", "evals")} {
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			return d, nil
		}
	}
	return "", errors.New(i18n.T("cmd.eval.no_cases"))
}

func printEvalCases(w io.Writer, cases []*evalrun.Case) {
	for _, c := range cases {
		state := ""
		if !c.IsActive() {
			state = "  (" + i18n.Tf("cmd.eval.inactive", c.InactiveReason) + ")"
		}
		fmt.Fprintf(w, "%-4s %-9s %s%s\n", c.ID, c.Workflow, c.Title, state)
	}
}

// evalModels are the models of a run: OH_EVAL_MODELS (comma separated),
// else Haiku.
func evalModels() []string {
	var out []string
	for _, m := range strings.Split(os.Getenv(envEvalModels), ",") {
		if m = strings.TrimSpace(m); m != "" {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		out = []string{defaultEvalModel}
	}
	return out
}

// defaultPassBudget is the cost cap of a pass when neither --budget nor
// OH_EVAL_BUDGET_USD sets it: higher when a Sonnet model runs.
func defaultPassBudget(models []string) float64 {
	for _, m := range models {
		if strings.Contains(strings.ToLower(m), "sonnet") {
			return defaultEvalBudgetSonnet
		}
	}
	return defaultEvalBudget
}

func envInt(key string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key))); err == nil && n > 0 {
		return n
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if n, err := strconv.ParseFloat(strings.TrimSpace(os.Getenv(key)), 64); err == nil && n > 0 {
		return n
	}
	return def
}

// evalBedrockToken returns the Bedrock API key of the evaluations:
// OH_EVAL_BEDROCK_TOKEN (or OH_E2E_BEDROCK_TOKEN), else the key stored by
// the session tool. It only goes to the credential proxy of each pass.
func evalBedrockToken(ctx context.Context) (string, error) {
	for _, k := range []string{envEvalToken, "OH_E2E_BEDROCK_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v, nil
		}
	}
	ad, _, err := detectTool(ctx)
	if err != nil {
		return "", errors.New(i18n.T("cmd.eval.no_credential"))
	}
	if r, ok := ad.(adapters.StoredKeyReader); ok {
		if k, err := r.StoredProviderKey(ctx, runsvc.ToolProviderID(ad, "bedrock")); err == nil && k != "" {
			return k, nil
		}
	}
	return "", errors.New(i18n.T("cmd.eval.no_credential"))
}

// goEnvironment keeps the Go caches of the user for the passes (their HOME
// is temporary): the sessions and the hidden tests build without download.
func goEnvironment(ctx context.Context) []string {
	out, err := exec.CommandContext(ctx, "go", "env", "GOCACHE", "GOMODCACHE", "GOPATH").Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	keys := []string{"GOCACHE", "GOMODCACHE", "GOPATH"}
	var env []string
	for i, l := range lines {
		if i < len(keys) && strings.TrimSpace(l) != "" {
			env = append(env, keys[i]+"="+strings.TrimSpace(l))
		}
	}
	return env
}

// ── Pass specification (parent → child) ───────────────────────────────────

// evalPassSpec is what the child `oh eval pass` runs.
type evalPassSpec struct {
	CaseID    string            `json:"case_id"`
	Workflow  string            `json:"workflow"`
	Mode      string            `json:"mode,omitempty"`
	Inputs    map[string]string `json:"inputs,omitempty"`
	Text      string            `json:"text,omitempty"`
	Tickets   []string          `json:"tickets,omitempty"`
	Project   string            `json:"project"`
	Model     string            `json:"model"`
	BudgetUSD float64           `json:"budget_usd"`
	Timeout   time.Duration     `json:"timeout_ns"`
	Answers   evalrun.Answers   `json:"answers"`
	OutDir    string            `json:"out_dir"`
	// Previous are sessions recorded in the project before the launch (R5).
	Previous []evalrun.PreviousSession `json:"previous,omitempty"`
	// Fanout enables the fan-out in the pass (R6).
	Fanout *evalrun.FanoutSetup `json:"fanout,omitempty"`
}

// Files written by the child in the pass output directory.
const (
	evalResultFile = "result.json"
	evalTraceFile  = "trace.jsonl"
	evalLogFile    = "pass.log"
)

// childEvalExecutor runs each pass in a child oh with its own home.
type childEvalExecutor struct {
	exe   string
	token string
	goEnv []string
}

// evalHubConfig is the hub.toml of a pass home.
func evalHubConfig(budget float64) string {
	return fmt.Sprintf(`[cli]
language = "fr"

[llm]
default_provider = "bedrock"

[provider.bedrock]
aws_region = %q
auth_mode = "bearer"

[session]
attach = "none"
notify = "off"
idle_sleep_minutes = 240

[limits]
session_budget_usd = %g

[gc]
enabled = false
`, evalRegion, budget)
}

// passEnv is the environment of a pass: the user's, without its oh
// settings, with a temporary oh home, HOME and TMPDIR of its own (the
// passes run side by side with --parallel).
func (e *childEvalExecutor) passEnv(home, user, tmp string) []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(k, "OH_") || k == "HOME" || k == "TMPDIR" || k == "XDG_CONFIG_HOME" || k == "XDG_DATA_HOME" || k == "XDG_STATE_HOME" || k == "XDG_CACHE_HOME" {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "OH_HOME="+home, "HOME="+user, "TMPDIR="+tmp, "OH_SESSION_ATTACH=none", "OH_SKIP_DB_BACKUP=1",
		"GIT_AUTHOR_NAME=oh eval", "GIT_AUTHOR_EMAIL=eval@oh.invalid", "GIT_COMMITTER_NAME=oh eval", "GIT_COMMITTER_EMAIL=eval@oh.invalid")
	return append(env, e.goEnv...)
}

// Execute implements evalrun.Executor.
func (e *childEvalExecutor) Execute(ctx context.Context, p evalrun.Pass) (*evalrun.PassOutput, error) {
	home, user, tmp := filepath.Join(p.Dir, "h"), filepath.Join(p.Dir, "u"), filepath.Join(p.Dir, "t")
	for _, d := range []string{home, user, tmp} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return nil, err
		}
	}
	if err := os.WriteFile(filepath.Join(home, "hub.toml"), []byte(evalHubConfig(p.BudgetUSD)), 0o600); err != nil {
		return nil, err
	}
	gitcfg := "[user]\n\tname = oh eval\n\temail = eval@oh.invalid\n[commit]\n\tgpgsign = false\n[init]\n\tdefaultBranch = main\n"
	if err := os.WriteFile(filepath.Join(user, ".gitconfig"), []byte(gitcfg), 0o600); err != nil {
		return nil, err
	}
	if p.HubDir != "" {
		if err := hubcontent.Install(p.HubDir, filepath.Join(home, "hub")); err != nil {
			return nil, err
		}
	}
	c := p.Case
	spec := evalPassSpec{CaseID: c.ID, Workflow: c.Workflow, Mode: c.Mode, Inputs: c.Inputs, Text: c.Text, Tickets: c.Tickets,
		Project: p.Project, Model: p.Model, BudgetUSD: p.BudgetUSD, Timeout: c.TimeoutOrDefault(), Answers: c.Answers, OutDir: p.OutDir, Previous: c.Previous, Fanout: c.Fanout}
	data, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}
	specPath := filepath.Join(p.Dir, "pass.json")
	if err := os.WriteFile(specPath, data, 0o600); err != nil {
		return nil, err
	}
	logf, err := os.Create(filepath.Join(p.OutDir, evalLogFile))
	if err != nil {
		return nil, err
	}
	defer logf.Close()
	env := e.passEnv(home, user, tmp)
	if c.Reports == evalrun.ReportsOff {
		env = append(env, envWorkflowReports+"=off")
	}
	// The child stops its session and its daemon itself; this margin lets it.
	cctx, cancel := context.WithTimeout(ctx, c.TimeoutOrDefault()+5*time.Minute)
	defer cancel()
	child := exec.CommandContext(cctx, e.exe, "eval", "pass", "--spec", specPath)
	child.Dir, child.Env = p.Project, env
	child.Stdin = strings.NewReader(e.token + "\n")
	child.Stdout, child.Stderr = logf, logf
	child.WaitDelay = 30 * time.Second
	runErr := child.Run()
	e.cleanup(home, env)

	out := &evalrun.PassOutput{}
	data, err = os.ReadFile(filepath.Join(p.OutDir, evalResultFile))
	if err != nil {
		tail, _ := os.ReadFile(filepath.Join(p.OutDir, evalLogFile))
		msg := strings.TrimSpace(lastLinesOf(string(tail), 12))
		if runErr != nil {
			msg = runErr.Error() + ": " + msg
		}
		out = &evalrun.PassOutput{Outcome: evalrun.OutcomeError, Error: msg}
	} else if err := json.Unmarshal(data, out); err != nil {
		return nil, err
	}
	if out.Outcome != evalrun.OutcomeCompleted && out.Outcome != evalrun.OutcomeBudget {
		out.Throttle = passThrottle(filepath.Join(p.OutDir, evalLogFile), home, user)
	}
	return out, nil
}

// passThrottle looks for a provider rate limit in the logs of a pass: its
// own log, the logs of its tool servers (oh home) and those the session
// tool wrote in the HOME of the pass.
func passThrottle(passLog, home, user string) string {
	files := []string{passLog}
	for _, root := range []string{filepath.Join(home, "servers"), user} {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(strings.TrimRight(d.Name(), ".0123456789"), ".log") {
				if info, ierr := d.Info(); ierr == nil && info.Size() <= evalMaxLogBytes {
					files = append(files, path)
				}
			}
			return nil //nolint:nilerr // an unreadable entry is skipped
		})
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, l := range strings.Split(string(data), "\n") {
			// Error lines only: the logs also hold the commands of the agents.
			if !evalErrorLine.MatchString(l) || strings.Contains(l, "spawning process") {
				continue
			}
			if t := evalrun.Throttling(l); t != "" {
				return t
			}
		}
	}
	return ""
}

// evalMaxLogBytes bounds the size of a log read for a rate limit.
const evalMaxLogBytes = 32 << 20

var evalErrorLine = regexp.MustCompile(`(?i)(level=(error|warn)|\berror\b|exception|eval pass:)`)

// cleanup stops what a pass may have left running in its home (child
// killed by the timeout): the tool servers it recorded and its daemon.
func (e *childEvalExecutor) cleanup(home string, env []string) {
	st, err := sqlite.Open(filepath.Join(home, "oh.db"))
	if err == nil {
		if list, err := sqlite.NewServerStore(st).List(context.Background()); err == nil {
			for _, s := range list {
				if s.PID > 0 && s.Status != domain.ServerStopped {
					if p, err := os.FindProcess(s.PID); err == nil {
						_ = p.Kill()
					}
				}
			}
		}
		st.Close()
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	stop := exec.CommandContext(ctx, e.exe, "daemon", "stop", "--force")
	stop.Env = env
	_ = stop.Run()
}

func lastLinesOf(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// ── Child: one pass in its own home ───────────────────────────────────────

// evalSecrets is the secret store of a pass: the provider key only, in
// memory (the keychain of the user is never read nor written).
type evalSecrets map[string]string

func (s evalSecrets) Get(_ context.Context, key string) (string, error) { return s[key], nil }
func (s evalSecrets) Set(_ context.Context, key, value string) error    { s[key] = value; return nil }
func (s evalSecrets) Delete(_ context.Context, key string) error        { delete(s, key); return nil }
func (s evalSecrets) List(context.Context) ([]string, error) {
	out := make([]string, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	return out, nil
}

// Pass loop timings.
var (
	evalPoll   = 2 * time.Second
	evalSettle = 20 * time.Second // idle without decision this long: the session is done
)

// defaultMaxDecisions bounds the decisions answered in one pass.
const defaultMaxDecisions = 40

func runEvalPass(cmd *cobra.Command, _ []string) error {
	specPath := flagString(cmd, "spec")
	data, err := os.ReadFile(specPath)
	if err != nil {
		return err
	}
	var spec evalPassSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return err
	}
	token, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	token = strings.TrimSpace(token)
	res := &evalrun.PassOutput{Outcome: evalrun.OutcomeError}
	start := time.Now()
	defer func() {
		res.Duration = time.Since(start)
		if data, err := json.MarshalIndent(res, "", "  "); err == nil {
			_ = os.WriteFile(filepath.Join(spec.OutDir, evalResultFile), data, 0o644)
		}
	}()
	if token == "" {
		res.Error = "no provider credential given to the pass"
		return errors.New(res.Error)
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), spec.Timeout)
	defer cancel()
	if err := runEvalSession(ctx, cmd, spec, token, res); err != nil {
		res.Error = strings.TrimSpace(res.Error + " " + err.Error())
		fmt.Fprintln(cmd.ErrOrStderr(), "eval pass:", err)
	}
	return nil
}

func runEvalSession(ctx context.Context, cmd *cobra.Command, spec evalPassSpec, token string, res *evalrun.PassOutput) error {
	a := MustApp()
	a.WithSecretStore(evalSecrets{"openhub.provider.bedrock.token": token})
	project := &domain.Project{ID: "eval", Name: "eval-" + strings.ToLower(spec.CaseID), Path: spec.Project, Provider: "bedrock", Model: spec.Model, Status: domain.ProjectStatusActive}
	if err := a.Projects.Create(ctx, project); err != nil {
		return fmt.Errorf(i18n.T("cmd.eval.registering_the_project"), err)
	}
	errOut := cmd.ErrOrStderr()
	if err := evalPrevious(ctx, a, project, spec.Previous); err != nil {
		return fmt.Errorf(i18n.T("cmd.eval.recording_the_previous_sessions"), err)
	}
	if spec.Fanout != nil {
		// R6: the fan-out of the pass, its global budget the one of the pass.
		a.Config.Fanout = config.FanoutConfig{Experimental: true, Parallel: spec.Fanout.Parallel, BudgetUSD: spec.BudgetUSD}
	}
	opts := launch.Options{Workflow: spec.Workflow, Project: project, Inputs: spec.Inputs, Mode: spec.Mode, Text: spec.Text, Tickets: spec.Tickets,
		Attach: string(sessionspec.AttachNone), Location: "base", Dirty: runsvc.DirtyAllow, OneSession: true}
	opts.Progress = launchProgress(nil, errOut)
	p, err := launch.Prepare(ctx, launchDeps(a), opts)
	if err != nil {
		return fmt.Errorf(i18n.T("cmd.eval.preparing_the_launch"), err)
	}
	p.Plan.Request.Base.Attach, p.Plan.Request.Base.Headless = sessionspec.AttachNone, true
	// Hard cap at the credential proxy (input-equivalent tokens, cache
	// included), on top of the session budget (cost, checked at each
	// request) and of the cost watched below.
	p.Plan.Request.Base.MaxTokens = evalMaxTokens(spec.Model, spec.BudgetUSD)
	results, err := p.Svc.Start(ctx, p.Plan)
	if err != nil || len(results) == 0 {
		return fmt.Errorf(i18n.T("cmd.eval.starting_the_session"), err)
	}
	sid := results[0].SessionID
	res.SessionID = sid
	fmt.Fprintln(errOut, "eval pass: session", sid)
	defer stopEvalPass(p.Svc, sid)

	ssvc, err := newSessionService(ctx, a)
	if err != nil {
		return err
	}
	var fan *evalFanout
	if p.Plan.Request.Base.Fanout != nil {
		fan = startEvalFanout(ctx, a, sid, spec, errOut)
		defer fan.stop()
	}
	res.Outcome = evalLoop(ctx, ssvc, sid, spec.Answers, spec.BudgetUSD, res, errOut, fan)
	if ctx.Err() != nil && res.Outcome == evalrun.OutcomeError {
		res.Outcome = evalrun.OutcomeTimeout
	}
	// The trace is read before the session stops (live transcript).
	tctx, tcancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer tcancel()
	if sess, err := a.Sessions.Get(tctx, sid); err == nil {
		res.CostUSD, res.TokensIn, res.TokensOut = sess.Cost, sess.TokensIn, sess.TokensOut
	}
	fan.collect(tctx, ssvc, res, spec.OutDir, token)
	tr, err := ssvc.Trace(tctx, sid, sessionsvc.TraceOptions{Usage: sqlite.NewUsageStore(store), Secrets: []string{token}})
	if err != nil {
		return fmt.Errorf("trace: %w", err)
	}
	path := filepath.Join(spec.OutDir, evalTraceFile)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	appendNextEvents(tctx, a, tr, sid)
	if err := tr.WriteJSONL(f); err != nil {
		return err
	}
	res.TracePath = path
	if n := len(tr.Events); n > 0 && res.CostUSD == 0 {
		res.CostUSD, _ = tr.Events[n-1].Data["cost_usd"].(float64)
	}
	return nil
}

// evalWiki decides the wiki proposals on the card of a checkpoint (case
// answers.wiki: accept, refuse or pending) before the checkpoint is
// answered, as a user would on the card.
func evalWiki(ctx context.Context, ssvc *sessionsvc.Service, sid, decisionID, cp, choice string, log io.Writer) []evalrun.AnsweredDecision {
	card, err := ssvc.CheckpointCard(ctx, decisionID)
	if err != nil || !card.WikiCommit {
		return nil
	}
	var out []evalrun.AnsweredDecision
	for _, p := range card.Wiki {
		rec := evalrun.AnsweredDecision{Kind: "wiki", Checkpoint: cp, Decision: choice + " " + p.ID()}
		if _, err := ssvc.WikiDecide(ctx, sid, p.ID(), choice, ""); err != nil {
			rec.Error = err.Error()
		}
		fmt.Fprintf(log, "eval pass: wiki %s %s → %s %s\n", cp, p.ID(), choice, rec.Error)
		out = append(out, rec)
	}
	return out
}

// evalLoop answers the decisions of the session until it is done (idle
// without decision for evalSettle, or ended), out of time or out of budget.
func evalLoop(ctx context.Context, ssvc *sessionsvc.Service, sid string, answers evalrun.Answers, budget float64, res *evalrun.PassOutput, log io.Writer, fan *evalFanout) string {
	maxDecisions := answers.MaxDecisions
	if maxDecisions <= 0 {
		maxDecisions = defaultMaxDecisions
	}
	var idleSince time.Time
	// asks counts the asks of each checkpoint (answers.sequence).
	asks, counted := map[string]int{}, map[string]bool{}
	t := time.NewTicker(evalPoll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return evalrun.OutcomeTimeout
		case <-t.C:
		}
		items, err := ssvc.Inbox(ctx, domain.DecisionFilter{SessionID: sid})
		if err != nil && ctx.Err() != nil {
			return evalrun.OutcomeTimeout
		}
		// R6: the decisions of the children of a fan-out too.
		for _, child := range fan.sessions(ctx) {
			if more, err := ssvc.Inbox(ctx, domain.DecisionFilter{SessionID: child}); err == nil {
				items = append(items, more...)
			}
		}
		for _, it := range items {
			d := it.Decision
			cp, _ := d.Payload.Data["checkpoint"].(string)
			ans := answers.DecideAt(d, asks[cp])
			if d.Kind == domain.DecisionCheckpoint && !counted[d.ID] {
				counted[d.ID] = true
				asks[cp]++
			}
			if d.Kind == domain.DecisionCheckpoint && answers.Wiki != "" {
				res.Decisions = append(res.Decisions, evalWiki(ctx, ssvc, sid, d.ID, cp, answers.Wiki, log)...)
			}
			rec := evalrun.AnsweredDecision{Kind: string(d.Kind), Checkpoint: cp, Decision: ans.Decision}
			err := ssvc.Decide(ctx, sessionsvc.Reply{DecisionID: d.ID, Decision: ans.Decision, Message: ans.Message, Answer: ans.Answer})
			if err != nil && d.Kind == domain.DecisionCheckpoint && errors.Is(err, checkpointsvc.ErrOverrideRequired) {
				// As a user would: the verification of oh failed, so the
				// approval is refused; the work goes back to the agent
				// (never an override).
				rec.Decision = "reject"
				err = ssvc.Decide(ctx, sessionsvc.Reply{DecisionID: d.ID, Decision: "reject", Message: evalrun.VerifyFailedNote})
			}
			if err != nil {
				rec.Error = err.Error()
			}
			fmt.Fprintf(log, "eval pass: %s %s → %s %s\n", d.Kind, cp, ans.Decision, rec.Error)
			res.Decisions = append(res.Decisions, rec)
			if ans.Stop {
				res.Error = "pass budget spent"
				return evalrun.OutcomeBudget
			}
			if len(res.Decisions) >= maxDecisions {
				res.Error = fmt.Sprintf("more than %d decisions", maxDecisions)
				return evalrun.OutcomeError
			}
		}
		sess, err := ssvc.Sessions.Get(ctx, sid)
		if err != nil {
			continue
		}
		if spent := sess.Cost + fan.cost(ctx); budget > 0 && spent >= budget {
			res.Error = fmt.Sprintf("pass budget spent ($%.3f of $%.2f)", spent, budget)
			return evalrun.OutcomeBudget
		}
		switch sess.State {
		case domain.RunCompleted, domain.RunStopped:
			return evalrun.OutcomeCompleted
		case domain.RunFailed:
			return evalrun.OutcomeFailed
		case domain.RunIdle:
			if len(items) > 0 || fan.running(ctx) {
				// R6: the parent waits idle while its children run.
				idleSince = time.Time{}
				continue
			}
			if idleSince.IsZero() {
				idleSince = time.Now()
			} else if time.Since(idleSince) >= evalSettle {
				return evalrun.OutcomeCompleted
			}
		default:
			idleSince = time.Time{}
		}
	}
}

// evalTokenPrice is an upper bound of the price of one uncached input token
// (USD, EU prices of the 5.x generation, Opus at the former $5/M). The
// credential proxy counts tokens weighted by their relative price
// (credproxy.WeightedTokens: output ×5, cache read ×0.1, cache write ×1.25),
// so its budget is a number of input-equivalent tokens and this price turns
// a cost cap into one, cache included.
func evalTokenPrice(model string) float64 {
	switch m := strings.ToLower(model); {
	case strings.Contains(m, "opus"):
		return 5.5e-6
	case strings.Contains(m, "sonnet"):
		return 2.2e-6
	}
	return 0.11e-6 // Haiku
}

// evalMaxTokens is the proxy token budget of a pass, in input-equivalent
// tokens: what its cost cap buys at the input price (0 = no cap).
func evalMaxTokens(model string, budget float64) int64 {
	if budget <= 0 {
		return 0
	}
	return int64(math.Round(budget / evalTokenPrice(model)))
}

// stopEvalPass stops the session and the daemon of the pass home.
func stopEvalPass(svc *runsvc.Service, sid string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := svc.StopSession(ctx, sid); err != nil {
		fmt.Fprintln(os.Stderr, "eval pass: stopping the session:", err)
	}
	if capability, _, err := daemonCapability(ctx); err == nil {
		_ = daemon.NewClient(daemon.Paths{Dir: ohRunDir()}).WithCapability(capability).Shutdown(ctx, true)
	}
}

// evalPrevious records the previous sessions of a case in the project of
// the pass, finished, oldest first (R5: `from: session.output(…)`).
func evalPrevious(ctx context.Context, a *app.App, project *domain.Project, prev []evalrun.PreviousSession) error {
	start := time.Now().Add(-time.Duration(len(prev)+1) * time.Hour)
	for i, p := range prev {
		at := start.Add(time.Duration(i) * time.Hour)
		end := at.Add(30 * time.Minute)
		s := &domain.Session{ID: fmt.Sprintf("ses_eval_prev_%d", i+1), ProjectID: project.ID, WorkflowID: p.Workflow, StartedAt: at, EndedAt: &end,
			Status: domain.SessionStatusCompleted, State: domain.RunCompleted, LaunchPath: project.Path, Outputs: p.Outputs}
		if err := a.Sessions.Create(ctx, s); err != nil {
			return err
		}
	}
	return nil
}
