package evalrun

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
)

// Report is the outcome of an evaluation run.
type Report struct {
	StartedAt  time.Time    `json:"started_at"`
	FinishedAt time.Time    `json:"finished_at"`
	Models     []string     `json:"models"`
	Reps       int          `json:"reps"`
	BudgetUSD  float64      `json:"budget_usd_per_pass"`
	Cases      []CaseInfo   `json:"cases"`
	Passes     []PassResult `json:"passes"`
	Stats      []CaseStats  `json:"stats"`
	TotalCost  float64      `json:"total_cost_usd"`
	// Note is free text recorded with a baseline (environment, caveats).
	Note string `json:"note,omitempty"`
}

// CaseInfo describes a case of the run.
type CaseInfo struct {
	ID             string `json:"id"`
	Key            string `json:"key"`
	Title          string `json:"title"`
	Workflow       string `json:"workflow"`
	Active         bool   `json:"active"`
	InactiveReason string `json:"inactive_reason,omitempty"`
}

// CaseStats aggregates the passes of a case with a model.
type CaseStats struct {
	Case     string         `json:"case"`
	Model    string         `json:"model"`
	Runs     int            `json:"runs"`
	Passed   int            `json:"passed"`
	PassRate float64        `json:"pass_rate"`
	Outcomes map[string]int `json:"outcomes"`
	// Assertions: pass rate of each assertion (by label).
	Assertions []AssertionStat `json:"assertions"`
	MeanCost   float64         `json:"mean_cost_usd"`
	TotalCost  float64         `json:"total_cost_usd"`
	MeanSecs   float64         `json:"mean_seconds"`
}

// AssertionStat is the pass rate of an assertion over the passes.
type AssertionStat struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Passed   int     `json:"passed"`
	Runs     int     `json:"runs"`
	PassRate float64 `json:"pass_rate"`
}

func (r *Report) compute() {
	type key struct{ c, m string }
	idx := map[key]*CaseStats{}
	var order []key
	r.TotalCost = 0
	for _, p := range r.Passes {
		k := key{p.Case, p.Model}
		s := idx[k]
		if s == nil {
			s = &CaseStats{Case: p.Case, Model: p.Model, Outcomes: map[string]int{}}
			idx[k] = s
			order = append(order, k)
		}
		s.Runs++
		if p.Pass {
			s.Passed++
		}
		s.Outcomes[p.Outcome]++
		s.TotalCost += p.CostUSD
		s.MeanSecs += p.Seconds
		r.TotalCost += p.CostUSD
		for i, a := range p.Assertions {
			if i >= len(s.Assertions) {
				s.Assertions = append(s.Assertions, AssertionStat{Name: a.Name, Type: a.Type})
			}
			s.Assertions[i].Runs++
			if a.Pass {
				s.Assertions[i].Passed++
			}
		}
	}
	r.Stats = nil
	for _, k := range order {
		s := idx[k]
		s.PassRate = rate(s.Passed, s.Runs)
		s.MeanCost = s.TotalCost / float64(s.Runs)
		s.MeanSecs /= float64(s.Runs)
		for i := range s.Assertions {
			s.Assertions[i].PassRate = rate(s.Assertions[i].Passed, s.Assertions[i].Runs)
		}
		r.Stats = append(r.Stats, *s)
	}
}

func rate(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

// Stat returns the stats of a case with a model.
func (r *Report) Stat(caseID, model string) (CaseStats, bool) {
	for _, s := range r.Stats {
		if s.Case == caseID && s.Model == model {
			return s, true
		}
	}
	return CaseStats{}, false
}

// WriteJSON writes the report.
func (r *Report) WriteJSON(path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// ReadReport reads a report (or a baseline) written by WriteJSON.
func ReadReport(path string) (*Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &r, nil
}

// Baseline is the reference measure kept in the repository: the report
// without the per-pass details that change at every run (session ids,
// trace paths), the assertions of each pass kept.
func (r *Report) Baseline(note string) *Report {
	b := *r
	b.Note = note
	b.Passes = make([]PassResult, len(r.Passes))
	for i, p := range r.Passes {
		p.SessionID, p.Trace = "", ""
		b.Passes[i] = p
	}
	return &b
}

// Markdown renders the report, compared with a baseline when given.
func (r *Report) Markdown(base *Report) string {
	var b strings.Builder
	b.WriteString("# oh eval\n\n")
	fmt.Fprintf(&b, "- Début : %s · durée : %s\n", r.StartedAt.Format(time.RFC3339), r.FinishedAt.Sub(r.StartedAt).Round(time.Second))
	fmt.Fprintf(&b, "- Modèles : %s · répétitions : %d · plafond par passage : $%.2f\n", strings.Join(r.Models, ", "), r.Reps, r.BudgetUSD)
	fmt.Fprintf(&b, "- Coût total : $%.3f · passages : %d\n", r.TotalCost, len(r.Passes))
	if r.Note != "" {
		fmt.Fprintf(&b, "- Note : %s\n", r.Note)
	}
	b.WriteString("\n## Taux de réussite\n\n")
	if base != nil {
		b.WriteString("| Cas | Workflow | Modèle | Réussite | Référence | Issues | Coût moyen | Durée moyenne |\n|---|---|---|---|---|---|---|---|\n")
	} else {
		b.WriteString("| Cas | Workflow | Modèle | Réussite | Issues | Coût moyen | Durée moyenne |\n|---|---|---|---|---|---|---|\n")
	}
	wf := map[string]string{}
	for _, c := range r.Cases {
		wf[c.ID] = c.Workflow
	}
	for _, s := range r.Stats {
		ref := ""
		if base != nil {
			ref = "—"
			if bs, ok := base.Stat(s.Case, s.Model); ok {
				ref = fmt.Sprintf("%d/%d", bs.Passed, bs.Runs)
				if d := s.PassRate - bs.PassRate; d < -0.001 {
					ref += " ▼"
				} else if d > 0.001 {
					ref += " ▲"
				}
			}
			ref = " " + ref + " |"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %d/%d (%.0f %%) |%s %s | $%.3f | %.0f s |\n", s.Case, wf[s.Case], shortModel(s.Model),
			s.Passed, s.Runs, 100*s.PassRate, ref, outcomes(s.Outcomes), s.MeanCost, s.MeanSecs)
	}
	var inactive []CaseInfo
	for _, c := range r.Cases {
		if !c.Active {
			inactive = append(inactive, c)
		}
	}
	if len(inactive) > 0 {
		b.WriteString("\nCas inactifs :\n\n")
		for _, c := range inactive {
			fmt.Fprintf(&b, "- %s (%s) : %s\n", c.ID, c.Workflow, c.InactiveReason)
		}
	}
	b.WriteString("\n## Assertions\n")
	for _, s := range r.Stats {
		fmt.Fprintf(&b, "\n### %s · %s\n\n| Assertion | Réussite |\n|---|---|\n", s.Case, shortModel(s.Model))
		for _, a := range s.Assertions {
			fmt.Fprintf(&b, "| %s | %d/%d |\n", strings.ReplaceAll(a.Name, "|", "\\|"), a.Passed, a.Runs)
		}
	}
	var failures []string
	for _, p := range r.Passes {
		if p.Pass {
			continue
		}
		var why []string
		if p.Outcome != OutcomeCompleted {
			why = append(why, "issue "+p.Outcome)
		}
		if p.Error != "" {
			why = append(why, oneLine(p.Error, 300))
		}
		for _, a := range p.Assertions {
			if !a.Pass {
				why = append(why, a.Name+" : "+oneLine(a.Detail, 200))
			}
		}
		failures = append(failures, fmt.Sprintf("- %s · %s · #%d : %s", p.Case, shortModel(p.Model), p.Rep, strings.Join(why, " ; ")))
	}
	if len(failures) > 0 {
		b.WriteString("\n## Échecs\n\n")
		b.WriteString(strings.Join(failures, "\n"))
		b.WriteString("\n")
	}
	return b.String()
}

func outcomes(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s ×%d", k, m[k])
	}
	return strings.Join(parts, ", ")
}

func shortModel(m string) string {
	if i := strings.LastIndex(m, "/"); i >= 0 {
		m = m[i+1:]
	}
	return m
}

func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > n {
		s = s[:n] + "…"
	}
	return s
}

// cloneAssertion tells the assertions that look at the clone of a pass
// (gone after the run): they are not checked again by Rescore.
func cloneAssertion(t string) bool { return t == AssertChangedWithin || t == AssertHiddenTests }

// Rescore checks the assertions of the cases again on the traces of a
// report (outDir holds them), without running anything: tuning an assertion
// costs no model call. The assertions that need the clone keep their result
// (matched by type and rank).
func Rescore(r *Report, outDir string, cases []*Case) (*Report, error) {
	by := map[string]*Case{}
	for _, c := range cases {
		by[c.ID] = c
	}
	out := *r
	out.Passes = make([]PassResult, len(r.Passes))
	for i, p := range r.Passes {
		c, ok := by[p.Case]
		if !ok {
			return nil, fmt.Errorf("pass of an unknown case %s", p.Case)
		}
		var events []sessionsvc.TraceEvent
		if p.Trace != "" {
			path := p.Trace
			if !filepath.IsAbs(path) {
				path = filepath.Join(outDir, path)
			}
			f, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			events, err = sessionsvc.ReadTraceJSONL(f)
			f.Close()
			if err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
		}
		prev := map[string][]AssertionResult{}
		for _, a := range p.Assertions {
			prev[a.Type] = append(prev[a.Type], a)
		}
		rank := map[string]int{}
		p.Assertions = make([]AssertionResult, len(c.Assertions))
		for j, a := range c.Assertions {
			k := rank[a.Type]
			rank[a.Type]++
			switch {
			case cloneAssertion(a.Type) && k < len(prev[a.Type]):
				res := prev[a.Type][k]
				res.Name = a.Label()
				p.Assertions[j] = res
			case cloneAssertion(a.Type):
				p.Assertions[j] = AssertionResult{Type: a.Type, Name: a.Label(), Detail: "not checked again (needs the clone)"}
			case len(events) == 0:
				p.Assertions[j] = AssertionResult{Type: a.Type, Name: a.Label(), Detail: "no trace"}
			default:
				p.Assertions[j] = Check([]Assertion{a}, Evidence{Events: events})[0]
			}
		}
		p.Pass = p.Outcome == OutcomeCompleted
		for _, a := range p.Assertions {
			p.Pass = p.Pass && a.Pass
		}
		out.Passes[i] = p
	}
	out.Cases = nil
	for _, c := range cases {
		out.Cases = append(out.Cases, CaseInfo{ID: c.ID, Key: c.Key(), Title: c.Title, Workflow: c.Workflow, Active: c.IsActive(), InactiveReason: c.InactiveReason})
	}
	out.compute()
	return &out, nil
}
