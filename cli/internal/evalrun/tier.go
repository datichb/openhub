package evalrun

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Reference measure by tiers (decision of 14/10): a case at 100 % on the
// last two measures of the baseline runs once, the others run Reps times;
// a case run once that fails runs up to Reps in the same run (escalation).

// maxHistory is the number of measures kept for each case in a baseline.
const maxHistory = 4

// Measure is the result of a case in a reference measure.
type Measure struct {
	At       time.Time `json:"at"`
	Note     string    `json:"note,omitempty"`
	Passed   int       `json:"passed"`
	Runs     int       `json:"runs"`
	MeanCost float64   `json:"mean_cost_usd"`
	// Hash is the content hash of the case when it was measured ("" =
	// unknown: a baseline recorded before the history).
	Hash string `json:"hash,omitempty"`
	// Assertions are the labels of the assertions measured, kept when the
	// hash is unknown.
	Assertions []string `json:"assertions,omitempty"`
}

// Full tells whether every pass of the measure passed.
func (m Measure) Full() bool { return m.Runs > 0 && m.Passed == m.Runs }

func (m Measure) String() string { return fmt.Sprintf("%d/%d", m.Passed, m.Runs) }

// CaseHistory are the last measures of a case with a model, the most
// recent first.
type CaseHistory struct {
	Case     string    `json:"case"`
	Model    string    `json:"model"`
	Measures []Measure `json:"measures"`
}

// Info describes the case in a report.
func (c *Case) Info() CaseInfo {
	info := CaseInfo{ID: c.ID, Key: c.Key(), Title: c.Title, Workflow: c.Workflow, Active: c.IsActive(), InactiveReason: c.InactiveReason}
	if h, err := c.ContentHash(); err == nil {
		info.Hash = h
	}
	return info
}

// ContentHash is a hash of the files of the case directory (case.yaml,
// fixture, hidden tests): it changes when the case does. "" when the case
// has no directory.
func (c *Case) ContentHash() (string, error) {
	if c.Dir == "" {
		return "", nil
	}
	if c.hash != "" {
		return c.hash, nil
	}
	h, err := HashDir(c.Dir)
	if err != nil {
		return "", err
	}
	c.hash = h
	return h, nil
}

// HashDir hashes the files of a directory: their paths and contents, in
// order (system files such as .DS_Store left out).
func HashDir(dir string) (string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() != ".DS_Store" {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	h := sha256.New()
	for _, f := range files {
		rel, err := filepath.Rel(dir, f)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00", filepath.ToSlash(rel))
		in, err := os.Open(f)
		if err != nil {
			return "", err
		}
		n, err := io.Copy(h, in)
		in.Close()
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "\x00%d\x00", n)
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// ownHistory is the measure of each case of the report (variants left
// out).
func (r *Report) ownHistory() []CaseHistory {
	hash := map[string]string{}
	for _, c := range r.Cases {
		hash[c.ID] = c.Hash
	}
	var out []CaseHistory
	for _, s := range r.Stats {
		if s.Variant != "" || s.Runs == 0 {
			continue
		}
		m := Measure{At: r.StartedAt, Note: r.Note, Passed: s.Passed, Runs: s.Runs, MeanCost: s.MeanCost, Hash: hash[s.Case]}
		if m.Hash == "" {
			m.Assertions = []string{}
			for _, a := range s.Assertions {
				m.Assertions = append(m.Assertions, a.Name)
			}
		}
		out = append(out, CaseHistory{Case: s.Case, Model: s.Model, Measures: []Measure{m}})
	}
	return out
}

// mergeHistory puts the measures of newer before those of older, for each
// case and model (a measure of older taken at the same time as one of
// newer is the same: a rescore), at most maxHistory each. The cases of
// older only are kept.
func mergeHistory(newer, older []CaseHistory) []CaseHistory {
	type key struct{ c, m string }
	old := map[key][]Measure{}
	for _, h := range older {
		old[key{h.Case, h.Model}] = h.Measures
	}
	var out []CaseHistory
	seen := map[key]bool{}
	for _, h := range newer {
		k := key{h.Case, h.Model}
		seen[k] = true
		ms := append([]Measure{}, h.Measures...)
		for _, m := range old[k] {
			dup := false
			for _, n := range h.Measures {
				dup = dup || n.At.Equal(m.At)
			}
			if !dup {
				ms = append(ms, m)
			}
		}
		if len(ms) > maxHistory {
			ms = ms[:maxHistory]
		}
		out = append(out, CaseHistory{Case: h.Case, Model: h.Model, Measures: ms})
	}
	for _, h := range older {
		if !seen[key{h.Case, h.Model}] {
			out = append(out, h)
		}
	}
	return out
}

// WithHistory adds the measures of the previous baseline (its history, or
// its stats when it was recorded without one) after those of r.
func (r *Report) WithHistory(prev *Report) *Report {
	if prev == nil {
		return r
	}
	older := prev.History
	if len(older) == 0 {
		older = prev.ownHistory()
	}
	r.History = mergeHistory(r.History, older)
	return r
}

// Measures are the measures of a case with a model, the most recent first:
// the history of the baseline, else its stats (a baseline recorded before
// the history).
func (r *Report) Measures(caseID, model string) []Measure {
	for _, h := range r.History {
		if h.Case == caseID && h.Model == model {
			return append([]Measure{}, h.Measures...)
		}
	}
	for _, o := range r.ownHistory() {
		if o.Case == caseID && o.Model == model {
			return o.Measures
		}
	}
	return nil
}

// sameCase tells whether a measure was taken on the case as it is now:
// same content hash, else (no hash recorded) same assertions.
func sameCase(m Measure, c *Case) (same bool, why string) {
	if m.Hash != "" {
		h, err := c.ContentHash()
		if err != nil || h == "" || h == m.Hash {
			return true, ""
		}
		return false, "cas modifié"
	}
	if m.Assertions == nil {
		return true, ""
	}
	if len(m.Assertions) != len(c.Assertions) {
		return false, "assertions modifiées"
	}
	for i, a := range c.Assertions {
		if a.Label() != m.Assertions[i] {
			return false, "assertions modifiées"
		}
	}
	return true, ""
}

// DefaultStableMeasures is the number of last measures at 100 % that let a
// case run once in a tiered reference.
const DefaultStableMeasures = 2

// tierOf tells whether a case may run once: 100 % on its last n measures,
// all taken on the case as it is now.
func tierOf(ms []Measure, c *Case, n int) (once bool, reason string) {
	if len(ms) == 0 {
		return false, "absent de la référence"
	}
	last := ms[:min(n, len(ms))]
	for i, m := range last {
		if ok, why := sameCase(m, c); !ok {
			switch i {
			case 0:
				return false, why + " depuis la dernière mesure"
			case 1:
				return false, why + " depuis l'avant-dernière mesure"
			}
			return false, fmt.Sprintf("%s depuis la mesure n° %d", why, i+1)
		}
	}
	list := make([]string, len(last))
	full := true
	for i, m := range last {
		list[i] = m.String()
		full = full && m.Full()
	}
	switch {
	case len(ms) < n && len(ms) == 1:
		return false, "une seule mesure (" + list[0] + ")"
	case len(ms) < n:
		return false, fmt.Sprintf("%d mesures sur %d (%s)", len(ms), n, strings.Join(list, ", "))
	case !full && n == 1:
		return false, "dernière mesure " + list[0]
	case !full:
		return false, "dernières mesures " + strings.Join(list, ", ")
	case n == 1:
		return true, "100 % sur la dernière mesure (" + list[0] + ")"
	case n == 2:
		return true, "100 % sur les deux dernières mesures (" + strings.Join(list, ", ") + ")"
	}
	return true, fmt.Sprintf("100 %% sur les %d dernières mesures (%s)", n, strings.Join(list, ", "))
}

// TieredPlan computes the repetitions of the active cases from the
// baseline: once for a case at 100 % on its last stable measures (0: the
// default, two) with every model, full otherwise.
func TieredPlan(base *Report, cases []*Case, models []string, full, stable int) []CasePlan {
	if stable < 1 {
		stable = DefaultStableMeasures
	}
	var out []CasePlan
	for _, c := range cases {
		if !c.IsActive() {
			continue
		}
		p := CasePlan{Case: c.ID, Reps: 1}
		var full1, ok1 []string
		for _, m := range models {
			var ms []Measure
			if base != nil {
				ms = base.Measures(c.ID, m)
			}
			once, why := tierOf(ms, c, stable)
			if len(models) > 1 {
				why = shortModel(m) + " : " + why
			}
			if once {
				ok1 = append(ok1, why)
			} else {
				full1 = append(full1, why)
			}
		}
		p.Reason = strings.Join(ok1, " ; ")
		if len(full1) > 0 {
			p.Reps, p.Reason = full, strings.Join(full1, " ; ")
		}
		out = append(out, p)
	}
	return out
}

// ParseRepsCase reads the repetitions given per case (S1=1, s5=3).
func ParseRepsCase(items []string, cases []*Case) (map[string]int, error) {
	known := map[string]bool{}
	for _, c := range cases {
		known[c.ID] = true
	}
	out := map[string]int{}
	for _, it := range items {
		for _, part := range strings.Split(it, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, n, ok := strings.Cut(part, "=")
			id = strings.ToUpper(strings.TrimSpace(id))
			reps, err := strconv.Atoi(strings.TrimSpace(n))
			if !ok || err != nil || reps < 1 {
				return nil, fmt.Errorf("repetitions of a case: %q is not <case>=<n ≥ 1>", part)
			}
			if !known[id] {
				return nil, fmt.Errorf("repetitions of a case: unknown case %s", id)
			}
			out[id] = reps
		}
	}
	return out, nil
}

// ApplyReps sets the repetitions given per case over a plan (nil: the
// other cases run def), for the active cases.
func ApplyReps(plan []CasePlan, forced map[string]int, cases []*Case, def int) []CasePlan {
	if len(forced) == 0 {
		return plan
	}
	if plan == nil {
		for _, c := range cases {
			if c.IsActive() {
				plan = append(plan, CasePlan{Case: c.ID, Reps: def})
			}
		}
	}
	for i := range plan {
		if n, ok := forced[plan[i].Case]; ok {
			plan[i].Reps, plan[i].Reason = n, "imposé (--reps-case)"
		}
	}
	return plan
}

// ── Cost estimate (oh eval --plan) ────────────────────────────────────────

// CostSource is a report whose mean costs feed an estimate.
type CostSource struct {
	Name   string
	Report *Report
}

// CostBook estimates the mean cost of a pass of a case: the baseline, then
// the other reports (most recent first), then the mean of its workflow,
// then the mean of the baseline, else the cost cap of a pass.
type CostBook struct {
	Base   *Report
	Extra  []CostSource
	Budget float64
}

func sameModel(a, b string) bool { return shortModel(a) == shortModel(b) }

// Lookup returns the mean cost of a pass of the case with the model and
// where it comes from.
func (b *CostBook) Lookup(c *Case, model string) (cost float64, source string) {
	if b.Base != nil {
		if ms := b.Base.Measures(c.ID, model); len(ms) > 0 && ms[0].MeanCost > 0 {
			return ms[0].MeanCost, "référence du " + ms[0].At.Format("2006-01-02")
		}
	}
	extra := append([]CostSource{}, b.Extra...)
	sort.SliceStable(extra, func(i, j int) bool { return extra[i].Report.StartedAt.After(extra[j].Report.StartedAt) })
	for _, e := range extra {
		for _, s := range e.Report.Stats {
			if s.Case == c.ID && s.Variant == "" && sameModel(s.Model, model) && s.Runs > 0 && s.MeanCost > 0 {
				return s.MeanCost, e.Name
			}
		}
	}
	all := []*Report{}
	if b.Base != nil {
		all = append(all, b.Base)
	}
	for _, e := range extra {
		all = append(all, e.Report)
	}
	var sum float64
	seen := map[string]bool{}
	for _, r := range all {
		wf := map[string]string{}
		for _, ci := range r.Cases {
			wf[ci.ID] = ci.Workflow
		}
		for _, s := range r.Stats {
			if wf[s.Case] == c.Workflow && s.Variant == "" && sameModel(s.Model, model) && s.Runs > 0 && s.MeanCost > 0 && !seen[s.Case] {
				seen[s.Case] = true
				sum += s.MeanCost
			}
		}
	}
	if n := len(seen); n > 0 {
		return sum / float64(n), fmt.Sprintf("moyenne du workflow %s (%d cas)", c.Workflow, n)
	}
	if b.Base != nil {
		var n int
		sum = 0
		for _, s := range b.Base.Stats {
			if s.Variant == "" && sameModel(s.Model, model) && s.Runs > 0 {
				sum += s.MeanCost
				n++
			}
		}
		if n > 0 {
			return sum / float64(n), "moyenne de la référence"
		}
	}
	return b.Budget, "plafond par passage"
}

// PlanLine is a case of an estimate.
type PlanLine struct {
	Case     string
	Workflow string
	Reps     int
	Reason   string
	Passes   int
	MeanCost float64 // per pass, first model
	Source   string
	Cost     float64
	// WorstPasses and WorstCost count the escalation of a failed case.
	WorstPasses int
	WorstCost   float64
}

// Estimate is what a run would launch and cost (oh eval --plan).
type Estimate struct {
	Models      []string
	Variants    int
	Reps        int
	EscalateTo  int
	BudgetTotal float64
	Tiered      bool
	Lines       []PlanLine
	Passes      int
	Cost        float64
	WorstPasses int
	WorstCost   float64
}

// EstimateRun estimates the passes and the cost of a run of the active
// cases: plan (nil: reps for every case), every model, variants (0 or 1:
// none), escalation up to escalateTo.
func EstimateRun(cases []*Case, plan []CasePlan, models []string, reps, variants, escalateTo int, book *CostBook) *Estimate {
	if variants < 1 {
		variants = 1
	}
	e := &Estimate{Models: models, Variants: variants, Reps: reps, EscalateTo: escalateTo}
	byCase := map[string]CasePlan{}
	for _, p := range plan {
		byCase[p.Case] = p
	}
	for _, c := range cases {
		if !c.IsActive() {
			continue
		}
		p, ok := byCase[c.ID]
		if !ok || p.Reps < 1 {
			p = CasePlan{Case: c.ID, Reps: reps}
		}
		l := PlanLine{Case: c.ID, Workflow: c.Workflow, Reps: p.Reps, Reason: p.Reason}
		worst := max(p.Reps, escalateTo)
		for i, m := range models {
			cost, src := book.Lookup(c, m)
			if i == 0 {
				l.MeanCost, l.Source = cost, src
			}
			l.Passes += p.Reps * variants
			l.Cost += float64(p.Reps*variants) * cost
			l.WorstPasses += worst * variants
			l.WorstCost += float64(worst*variants) * cost
		}
		e.Lines = append(e.Lines, l)
		e.Passes += l.Passes
		e.Cost += l.Cost
		e.WorstPasses += l.WorstPasses
		e.WorstCost += l.WorstCost
	}
	return e
}

// Markdown renders the estimate.
func (e *Estimate) Markdown() string {
	var b strings.Builder
	b.WriteString("# oh eval --plan\n\n")
	fmt.Fprintf(&b, "- Modèles : %s", strings.Join(e.Models, ", "))
	if e.Variants > 1 {
		fmt.Fprintf(&b, " · %d versions du hub", e.Variants)
	}
	if e.Tiered {
		fmt.Fprintf(&b, " · référence par niveaux : ×1 si 100 %% sur les deux dernières mesures, ×%d sinon", e.Reps)
	} else {
		fmt.Fprintf(&b, " · répétitions : %d", e.Reps)
	}
	if e.EscalateTo > 0 {
		fmt.Fprintf(&b, " · escalade jusqu'à ×%d", e.EscalateTo)
	}
	b.WriteString("\n- Aucun passage lancé (estimation depuis les coûts moyens enregistrés)\n\n")
	b.WriteString("| Cas | Workflow | Passages | Raison | Coût moyen | Source du coût | Estimation |\n|---|---|---|---|---|---|---|\n")
	counts := map[int]int{}
	for _, l := range e.Lines {
		counts[l.Reps]++
		fmt.Fprintf(&b, "| %s | %s | %d | %s | $%.3f | %s | $%.2f |\n", l.Case, l.Workflow, l.Passes, strings.ReplaceAll(l.Reason, "|", "\\|"), l.MeanCost, l.Source, l.Cost)
	}
	var tiers []int
	for n := range counts {
		tiers = append(tiers, n)
	}
	sort.Ints(tiers)
	var parts []string
	for _, n := range tiers {
		parts = append(parts, fmt.Sprintf("×%d : %d cas", n, counts[n]))
	}
	fmt.Fprintf(&b, "\n- Cas : %d (%s)\n", len(e.Lines), strings.Join(parts, ", "))
	fmt.Fprintf(&b, "- Total : %d passages · ≈ $%.2f\n", e.Passes, e.Cost)
	if e.WorstPasses > e.Passes {
		fmt.Fprintf(&b, "- Si tous les cas à moins de ×%d échouent (escalade) : %d passages · ≈ $%.2f\n", e.EscalateTo, e.WorstPasses, e.WorstCost)
	}
	if e.BudgetTotal > 0 {
		state := "suffisant"
		if e.Cost > e.BudgetTotal {
			state = "dépassé : des passages ne seront pas lancés"
		} else if e.WorstCost > e.BudgetTotal {
			state = "suffisant sans escalade, dépassé si toutes les escalades ont lieu"
		}
		fmt.Fprintf(&b, "- Plafond global : $%.2f (%s)\n", e.BudgetTotal, state)
	}
	return b.String()
}

// repsLine is the repetitions of the report header: Reps, or the
// repetitions per case of a plan.
func (r *Report) repsLine() string {
	if len(r.Plan) == 0 {
		return strconv.Itoa(r.Reps)
	}
	counts := map[int]int{}
	var tiers []int
	var escalated []string
	for _, p := range r.Plan {
		if counts[p.Reps] == 0 {
			tiers = append(tiers, p.Reps)
		}
		counts[p.Reps]++
		if p.Escalated {
			escalated = append(escalated, p.Case)
		}
	}
	sort.Ints(tiers)
	parts := make([]string, len(tiers))
	for i, n := range tiers {
		parts[i] = fmt.Sprintf("×%d : %d cas", n, counts[n])
	}
	s := "par cas (" + strings.Join(parts, ", ") + ")"
	if r.EscalateTo > 0 {
		s += fmt.Sprintf(" · escalade jusqu'à ×%d", r.EscalateTo)
		if len(escalated) > 0 {
			s += " : " + strings.Join(escalated, ", ")
		}
	}
	return s
}

// markdownPlan lists the repetitions per case of a plan and why.
func (r *Report) markdownPlan(b *strings.Builder) {
	if len(r.Plan) == 0 {
		return
	}
	b.WriteString("\n## Répétitions par cas\n\n| Cas | Prévu | Lancé | Raison |\n|---|---|---|---|\n")
	runs := map[string]int{}
	for _, p := range r.Passes {
		if p.Variant == "" || (len(r.Variants) > 0 && p.Variant == r.Variants[0].Name) {
			runs[p.Case]++
		}
	}
	for _, p := range r.Plan {
		reason := p.Reason
		if p.Escalated {
			reason = joinNonEmpty(reason, fmt.Sprintf("échec : escaladé jusqu'à ×%d", r.EscalateTo))
		}
		fmt.Fprintf(b, "| %s | ×%d | %d | %s |\n", p.Case, p.Reps, runs[p.Case], strings.ReplaceAll(reason, "|", "\\|"))
	}
}

func joinNonEmpty(a, b string) string {
	if a == "" {
		return b
	}
	return a + " ; " + b
}

// WithReports returns a copy of the baseline whose history also holds the
// measures of other reports (the passes of a lot between two references,
// variants left out), by date, the most recent first. Only the tiered plan
// reads it: a recorded baseline keeps the references only.
func (r *Report) WithReports(extra []*Report) *Report {
	out := *r
	src := r.History
	if len(src) == 0 {
		src = r.ownHistory()
	}
	type key struct{ c, m string }
	idx := map[key]int{}
	var hist []CaseHistory
	add := func(h CaseHistory) {
		k := key{h.Case, h.Model}
		i, ok := idx[k]
		if !ok {
			idx[k] = len(hist)
			hist = append(hist, CaseHistory{Case: h.Case, Model: h.Model})
			i = len(hist) - 1
		}
		for _, m := range h.Measures {
			dup := false
			for _, n := range hist[i].Measures {
				dup = dup || n.At.Equal(m.At)
			}
			if !dup {
				hist[i].Measures = append(hist[i].Measures, m)
			}
		}
	}
	for _, h := range src {
		add(h)
	}
	for _, e := range extra {
		for _, h := range e.ownHistory() {
			add(h)
		}
	}
	for i := range hist {
		sort.SliceStable(hist[i].Measures, func(a, b int) bool { return hist[i].Measures[a].At.After(hist[i].Measures[b].At) })
	}
	out.History = hist
	return &out
}

// PlannedRuns is the number of passes a case of the report must have with a
// model: its plan (else Reps), or EscalateTo once one of them failed.
func (r *Report) PlannedRuns(s CaseStats) int {
	want := r.Reps
	escalated := false
	for _, p := range r.Plan {
		if p.Case == s.Case {
			want, escalated = p.Reps, p.Escalated
		}
	}
	if r.EscalateTo > want && (escalated || s.Passed < s.Runs) {
		want = r.EscalateTo
	}
	return want
}
