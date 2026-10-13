package evalrun

import (
	"fmt"
	"math"
	"strings"
)

// Verdicts of a comparison between the reference variant (the first one,
// which holds what is evaluated, a skill for instance) and another variant
// (without it). The rule (P4): a gain is at least +1/3 of pass rate on a
// case without a drop elsewhere, or the same pass rate and a mean cost at
// least 10 % lower; a gap of a single pass is ambiguous (run ×3 more).
const (
	VerdictGain       = "gain"
	VerdictNoGain     = "no_gain"
	VerdictAmbiguous  = "ambiguous"
	VerdictIncomplete = "incomplete"
)

// Thresholds of the verdict.
const (
	gainRate  = 1.0 / 3 // pass rate gap of a gain
	gainCost  = 0.10    // relative cost cut of a gain at the same pass rate
	rateEpsil = 1e-9
)

// Comparison compares a case between the reference variant and another one.
type Comparison struct {
	Case      string `json:"case"`
	Model     string `json:"model"`
	Reference string `json:"reference"`
	Variant   string `json:"variant"`
	RefPassed int    `json:"reference_passed"`
	RefRuns   int    `json:"reference_runs"`
	Passed    int    `json:"passed"`
	Runs      int    `json:"runs"`
	// RateGap is the pass rate of the reference less that of the variant.
	RateGap     float64 `json:"rate_gap"`
	RefMeanCost float64 `json:"reference_mean_cost_usd"`
	MeanCost    float64 `json:"mean_cost_usd"`
	// CostGap is the mean cost of the reference relative to the variant's
	// (-0.12: the reference costs 12 % less); 0 when unknown.
	CostGap     float64 `json:"cost_gap"`
	RefMeanSecs float64 `json:"reference_mean_seconds"`
	MeanSecs    float64 `json:"mean_seconds"`
	Verdict     string  `json:"verdict"`
	Reason      string  `json:"reason"`
	// Assertions compares the pass rate of each assertion (business
	// result versus loading of a skill).
	Assertions []AssertionGap `json:"assertions,omitempty"`
	// Outcome is the same comparison on the other assertions than
	// skill_loaded_before (the business result: diagnosis, flaw found,
	// tests), when the case has some: removing a skill fails its loading
	// assertion by construction.
	Outcome *OutcomeGap `json:"outcome,omitempty"`
}

// OutcomeGap compares the passes of a case on their business assertions.
type OutcomeGap struct {
	RefPassed int     `json:"reference_passed"`
	Passed    int     `json:"passed"`
	RateGap   float64 `json:"rate_gap"`
	Verdict   string  `json:"verdict"`
	Reason    string  `json:"reason"`
}

// AssertionGap is an assertion in both variants.
type AssertionGap struct {
	Name      string `json:"name"`
	RefPassed int    `json:"reference_passed"`
	RefRuns   int    `json:"reference_runs"`
	Passed    int    `json:"passed"`
	Runs      int    `json:"runs"`
}

// VariantVerdict is the verdict of a variant over every case of the run.
type VariantVerdict struct {
	Model     string `json:"model"`
	Reference string `json:"reference"`
	Variant   string `json:"variant"`
	Verdict   string `json:"verdict"`
	Reason    string `json:"reason"`
	// OutcomeVerdict is the verdict on the business assertions only
	// (Comparison.Outcome), "" when no case has a loading assertion.
	OutcomeVerdict string `json:"outcome_verdict,omitempty"`
	OutcomeReason  string `json:"outcome_reason,omitempty"`
}

// outcomeStats counts the passes of a case (variant, model) that pass on
// their other assertions than skill_loaded_before; ok is false when the
// case has none of those.
func (r *Report) outcomeStats(s CaseStats) (out CaseStats, ok bool) {
	out = s
	out.Passed = 0
	for _, p := range r.Passes {
		if p.Case != s.Case || p.Variant != s.Variant || p.Model != s.Model {
			continue
		}
		pass := p.Outcome == OutcomeCompleted
		for _, a := range p.Assertions {
			if a.Type == AssertSkillLoadedBefore {
				ok = true
				continue
			}
			pass = pass && a.Pass
		}
		if pass {
			out.Passed++
		}
	}
	out.PassRate = rate(out.Passed, out.Runs)
	return out, ok
}

// passGap is the gap of pass rate as a number of passes (on the smaller
// number of runs).
func passGap(c Comparison) float64 {
	return c.RateGap * float64(min(c.RefRuns, c.Runs))
}

// CompareStats compares the stats of a case for the reference variant and
// another one, and gives the verdict.
func CompareStats(ref, v CaseStats) Comparison {
	c := Comparison{Case: ref.Case, Model: ref.Model, Reference: ref.Variant, Variant: v.Variant,
		RefPassed: ref.Passed, RefRuns: ref.Runs, Passed: v.Passed, Runs: v.Runs,
		RefMeanCost: ref.MeanCost, MeanCost: v.MeanCost, RefMeanSecs: ref.MeanSecs, MeanSecs: v.MeanSecs}
	byName := map[string]AssertionStat{}
	for _, a := range v.Assertions {
		byName[a.Name] = a
	}
	for _, a := range ref.Assertions {
		o := byName[a.Name]
		c.Assertions = append(c.Assertions, AssertionGap{Name: a.Name, RefPassed: a.Passed, RefRuns: a.Runs, Passed: o.Passed, Runs: o.Runs})
	}
	if ref.Runs == 0 || v.Runs == 0 {
		c.Verdict, c.Reason = VerdictIncomplete, "pas de passage d'un côté"
		return c
	}
	c.RateGap = ref.PassRate - v.PassRate
	if v.MeanCost > 0 && ref.MeanCost > 0 {
		c.CostGap = ref.MeanCost/v.MeanCost - 1
	}
	gap := passGap(c)
	switch {
	case c.RateGap >= gainRate-rateEpsil && gap <= 1+rateEpsil:
		c.Verdict, c.Reason = VerdictAmbiguous, fmt.Sprintf("écart d'un seul passage (%+.0f %%) : relancer ×3", 100*c.RateGap)
	case c.RateGap >= gainRate-rateEpsil:
		c.Verdict, c.Reason = VerdictGain, fmt.Sprintf("réussite %+.0f %%", 100*c.RateGap)
	case c.RateGap < -rateEpsil:
		c.Verdict, c.Reason = VerdictNoGain, fmt.Sprintf("recul : réussite %+.0f %%", 100*c.RateGap)
	case c.RateGap > rateEpsil:
		c.Verdict, c.Reason = VerdictNoGain, fmt.Sprintf("réussite %+.0f %%, moins que +33 %%", 100*c.RateGap)
	case c.CostGap != 0 && c.CostGap <= -gainCost+rateEpsil:
		c.Verdict, c.Reason = VerdictGain, fmt.Sprintf("réussite égale, coût moyen %+.0f %%", 100*c.CostGap)
	case c.CostGap != 0:
		c.Verdict, c.Reason = VerdictNoGain, fmt.Sprintf("réussite égale, coût moyen %+.0f %% (il faut −10 %%)", 100*c.CostGap)
	default:
		c.Verdict, c.Reason = VerdictNoGain, "réussite égale, coût inconnu"
	}
	return c
}

// VerdictOf is the verdict of a variant over its comparisons (one model):
// a gain on a case without a drop elsewhere; a drop of a single pass next
// to a gain, or a gap of a single pass, is ambiguous.
func VerdictOf(cmps []Comparison) VariantVerdict {
	var out VariantVerdict
	if len(cmps) > 0 {
		out.Model, out.Reference, out.Variant = cmps[0].Model, cmps[0].Reference, cmps[0].Variant
	}
	var gains, ambiguous, drops, smallDrops []string
	for _, c := range cmps {
		switch {
		case c.Verdict == VerdictGain:
			gains = append(gains, c.Case)
		case c.Verdict == VerdictAmbiguous:
			ambiguous = append(ambiguous, c.Case)
		case c.Verdict == VerdictIncomplete:
		case c.RateGap < -rateEpsil && -passGap(c) <= 1+rateEpsil:
			smallDrops = append(smallDrops, c.Case)
		case c.RateGap < -rateEpsil:
			drops = append(drops, c.Case)
		}
	}
	list := func(s []string) string { return strings.Join(s, ", ") }
	switch {
	case len(drops) > 0:
		out.Verdict, out.Reason = VerdictNoGain, "recul sur "+list(drops)
	case len(gains) > 0 && len(smallDrops) > 0:
		out.Verdict, out.Reason = VerdictAmbiguous, "gain sur "+list(gains)+", recul d'un passage sur "+list(smallDrops)+" : relancer ×3"
	case len(gains) > 0:
		out.Verdict, out.Reason = VerdictGain, "gain sur "+list(gains)
	case len(ambiguous) > 0:
		out.Verdict, out.Reason = VerdictAmbiguous, "écart d'un seul passage sur "+list(ambiguous)+" : relancer ×3"
	case len(cmps) == 0:
		out.Verdict, out.Reason = VerdictIncomplete, "aucun cas comparé"
	default:
		out.Verdict, out.Reason = VerdictNoGain, "aucun gain"
	}
	return out
}

// compare fills the comparisons of a run with variants: each variant
// against the first one, case by case, then over every case.
func (r *Report) compare() {
	r.Comparisons, r.Verdicts = nil, nil
	if len(r.Variants) < 2 {
		return
	}
	ref := r.Variants[0].Name
	stats := map[[3]string]CaseStats{}
	for _, s := range r.Stats {
		stats[[3]string{s.Case, s.Variant, s.Model}] = s
	}
	for _, m := range r.Models {
		for _, v := range r.Variants[1:] {
			var cmps []Comparison
			for _, c := range r.Cases {
				if !c.Active {
					continue
				}
				rs, ok1 := stats[[3]string{c.ID, ref, m}]
				vs, ok2 := stats[[3]string{c.ID, v.Name, m}]
				if !ok1 && !ok2 {
					continue
				}
				rs.Case, rs.Model, rs.Variant = c.ID, m, ref
				vs.Case, vs.Model, vs.Variant = c.ID, m, v.Name
				cmp := CompareStats(rs, vs)
				ro, ok1 := r.outcomeStats(rs)
				vo, ok2 := r.outcomeStats(vs)
				if ok1 || ok2 {
					oc := CompareStats(ro, vo)
					cmp.Outcome = &OutcomeGap{RefPassed: oc.RefPassed, Passed: oc.Passed, RateGap: oc.RateGap, Verdict: oc.Verdict, Reason: oc.Reason}
				}
				cmps = append(cmps, cmp)
			}
			r.Comparisons = append(r.Comparisons, cmps...)
			vv := VerdictOf(cmps)
			vv.Model, vv.Reference, vv.Variant = m, ref, v.Name
			var outcomes []Comparison
			for _, c := range cmps {
				if c.Outcome != nil {
					o := c
					o.RefPassed, o.Passed, o.RateGap, o.Verdict, o.Reason = c.Outcome.RefPassed, c.Outcome.Passed, c.Outcome.RateGap, c.Outcome.Verdict, c.Outcome.Reason
					outcomes = append(outcomes, o)
				} else {
					outcomes = append(outcomes, c)
				}
			}
			if len(outcomes) > 0 && hasOutcome(cmps) {
				ov := VerdictOf(outcomes)
				vv.OutcomeVerdict, vv.OutcomeReason = ov.Verdict, ov.Reason
			}
			r.Verdicts = append(r.Verdicts, vv)
		}
	}
}

func hasOutcome(cmps []Comparison) bool {
	for _, c := range cmps {
		if c.Outcome != nil {
			return true
		}
	}
	return false
}

// verdictText is the verdict in the Markdown report.
func verdictText(v string) string {
	switch v {
	case VerdictGain:
		return "**gain**"
	case VerdictAmbiguous:
		return "ambigu : relancer ×3"
	case VerdictIncomplete:
		return "incomplet"
	}
	return "pas de gain"
}

// markdownComparison renders the comparison of the variants.
func (r *Report) markdownComparison(b *strings.Builder) {
	if len(r.Variants) < 2 {
		return
	}
	ref := r.Variants[0].Name
	b.WriteString("\n## Comparaison avec la référence\n\n")
	fmt.Fprintf(b, "Référence : **%s**, la variante qui contient ce qu'on évalue. Gain : au moins +1/3 de réussite sur un cas sans recul ailleurs, ou réussite égale et coût moyen inférieur d'au moins 10 %% ; un écart d'un seul passage est ambigu (relancer ×3). Écart : la référence moins la variante ; écart de coût : le coût moyen de la référence rapporté à celui de la variante.\n\n", ref)
	for i, v := range r.Variants {
		role := ""
		if i == 0 {
			role = " (référence)"
		}
		fmt.Fprintf(b, "- **%s**%s : %s", v.Name, role, v.Describe())
		if len(v.Removed) > 0 {
			fmt.Fprintf(b, " ; retiré : %s", strings.Join(v.Removed, " ; "))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n| Cas | Modèle | Variante | Réussite | " + ref + " | Écart | Coût moyen | " + ref + " | Écart de coût | Durée | " + ref + " | Verdict |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, c := range r.Comparisons {
		cost := "—"
		if c.CostGap != 0 {
			cost = fmt.Sprintf("%+.0f %%", 100*c.CostGap)
		}
		fmt.Fprintf(b, "| %s | %s | %s | %d/%d | %d/%d | %+.0f %% | $%.3f | $%.3f | %s | %.0f s | %.0f s | %s : %s |\n",
			c.Case, shortModel(c.Model), c.Variant, c.Passed, c.Runs, c.RefPassed, c.RefRuns, 100*c.RateGap,
			c.MeanCost, c.RefMeanCost, cost, c.MeanSecs, c.RefMeanSecs, verdictText(c.Verdict), c.Reason)
	}
	if hasOutcome(r.Comparisons) {
		b.WriteString("\n### Résultat hors chargement des skills\n\nLes mêmes passages, sans les assertions `skill_loaded_before` (forcément en échec sans le skill) : le résultat métier.\n\n")
		b.WriteString("| Cas | Modèle | Variante | Réussite | " + ref + " | Écart | Verdict |\n|---|---|---|---|---|---|---|\n")
		for _, c := range r.Comparisons {
			if o := c.Outcome; o != nil {
				fmt.Fprintf(b, "| %s | %s | %s | %d/%d | %d/%d | %+.0f %% | %s : %s |\n", c.Case, shortModel(c.Model), c.Variant, o.Passed, c.Runs, o.RefPassed, c.RefRuns,
					100*o.RateGap, verdictText(o.Verdict), o.Reason)
			}
		}
	}
	b.WriteString("\n### Verdict\n\n")
	for _, v := range r.Verdicts {
		fmt.Fprintf(b, "- %s contre %s · %s : %s (%s)", v.Reference, v.Variant, shortModel(v.Model), verdictText(v.Verdict), v.Reason)
		if v.OutcomeVerdict != "" {
			fmt.Fprintf(b, " ; hors chargement des skills : %s (%s)", verdictText(v.OutcomeVerdict), v.OutcomeReason)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n### Assertions par variante\n")
	for _, c := range r.Comparisons {
		fmt.Fprintf(b, "\n#### %s · %s : %s / %s\n\n| Assertion | %s | %s |\n|---|---|---|\n", c.Case, shortModel(c.Model), c.Reference, c.Variant, c.Reference, c.Variant)
		for _, a := range c.Assertions {
			mark := ""
			if a.RefRuns > 0 && a.Runs > 0 && math.Abs(rate(a.RefPassed, a.RefRuns)-rate(a.Passed, a.Runs)) > rateEpsil {
				mark = " ◆"
			}
			fmt.Fprintf(b, "| %s%s | %d/%d | %d/%d |\n", strings.ReplaceAll(a.Name, "|", "\\|"), mark, a.RefPassed, a.RefRuns, a.Passed, a.Runs)
		}
	}
}
