package session

import (
	"fmt"
	"sort"
	"strings"
)

// Digest of a trace (R5, source `session.trace()` of a workflow input): a
// short text a later session (retro) reads instead of the whole trace —
// the session, its checkpoints, refused decisions and calls, skills,
// delegations, outputs and totals. Secrets are already masked in t.

// digestMaxItems bounds each list of the digest.
const digestMaxItems = 12

// Digest returns the summary of t.
func (t *Trace) Digest() string {
	if t == nil || len(t.Events) == 0 {
		return ""
	}
	var (
		b                  strings.Builder
		cps, refused       []string
		skills, subagents  = map[string]int{}, map[string]int{}
		denied             []string
		outputs            []string
		summary, sessionEv *TraceEvent
	)
	for i := range t.Events {
		e := &t.Events[i]
		switch e.Kind {
		case TraceSession:
			sessionEv = e
		case TraceSummary:
			summary = e
		case TraceCheckpoint:
			line := e.Checkpoint + " " + e.Event
			if e.Text != "" {
				line += " — " + clipText(oneLine(e.Text), 160)
			}
			cps = append(cps, line)
		case TraceDecision:
			if e.Resolution != "" && e.Resolution != "approve" && e.Resolution != "once" && e.Resolution != "always" {
				refused = append(refused, fmt.Sprintf("%s %s → %s", e.DecisionKind, firstNonEmpty(e.Checkpoint, e.Title), e.Resolution))
			}
		case TraceSkill:
			skills[e.Skill]++
		case TraceSubagent:
			subagents[e.Subagent]++
		case TraceOutput:
			outputs = append(outputs, fmt.Sprintf("%s = %v", e.Title, e.Data["value"]))
		}
		if e.Kind == TraceTool && e.Denied {
			denied = append(denied, clipText(oneLine(e.Tool+" "+mainInput(e)), 160))
		}
	}
	if sessionEv != nil {
		fmt.Fprintf(&b, "Session %s", sessionEv.ToolSession)
		if wf, _ := sessionEv.Data["workflow"].(string); wf != "" {
			fmt.Fprintf(&b, " (workflow %s", wf)
			if m, _ := sessionEv.Data["mode"].(string); m != "" {
				fmt.Fprintf(&b, ", mode %s", m)
			}
			b.WriteString(")")
		}
		b.WriteString("\n")
	}
	if summary != nil {
		d := summary.Data
		fmt.Fprintf(&b, "Totaux : %v $, %v appels d'outils, %v délégations, %v décisions, %v refus\n",
			roundCost(d["cost_usd"]), d["tool_calls"], d["subagent_calls"], d["decisions"], d["denied"])
	}
	section(&b, "Checkpoints", cps)
	section(&b, "Décisions refusées ou corrigées", refused)
	section(&b, "Appels refusés", denied)
	section(&b, "Skills chargées", counted(skills))
	section(&b, "Délégations", counted(subagents))
	section(&b, "Sorties", outputs)
	return strings.TrimRight(b.String(), "\n")
}

func section(b *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "%s :\n", title)
	for i, it := range items {
		if i == digestMaxItems {
			fmt.Fprintf(b, "- … (%d de plus)\n", len(items)-digestMaxItems)
			break
		}
		fmt.Fprintf(b, "- %s\n", it)
	}
}

func counted(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		if k != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = k
		if m[k] > 1 {
			out[i] = fmt.Sprintf("%s ×%d", k, m[k])
		}
	}
	return out
}

func mainInput(e *TraceEvent) string {
	for _, k := range []string{"command", "filePath", "path", "pattern"} {
		if v, ok := e.Input[k].(string); ok {
			return v
		}
	}
	return ""
}

func roundCost(v any) string {
	if f, ok := v.(float64); ok {
		return fmt.Sprintf("%.2f", f)
	}
	return fmt.Sprint(v)
}
