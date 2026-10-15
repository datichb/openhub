package session

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
)

// Digest of a trace (R5, source `session.trace()` of a workflow input): a
// short text a later session (retro) reads instead of the whole trace —
// the session (title, state, duration), its checkpoints and those asked
// several times, refused decisions (with the user's instruction) and
// calls, skills, delegations, outputs, totals and cost per agent. Secrets
// are already masked in t.

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
		asked              = map[string]int{}
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
			if e.Event == string(domain.CheckpointWaiting) {
				asked[e.Checkpoint]++
			}
			line := e.Checkpoint + " " + e.Event
			if e.Identity != "" {
				line += " par " + e.Identity
			}
			if e.ApproverMismatch {
				line += " (hors valideur, attendu : " + strings.Join(e.Approver, ", ") + ")"
			}
			if e.Text != "" {
				line += " — " + clipText(oneLine(e.Text), 160)
			}
			cps = append(cps, line)
		case TraceDecision:
			if e.Resolution != "" && e.Resolution != "approve" && e.Resolution != "once" && e.Resolution != "always" {
				line := fmt.Sprintf("%s %s → %s", e.DecisionKind, firstNonEmpty(e.Checkpoint, e.Title), e.Resolution)
				if m, _ := e.Data["message"].(string); m != "" {
					line += " — « " + clipText(oneLine(m), 160) + " »"
				}
				refused = append(refused, line)
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
		if t, ok := sessionEv.Data["title"].(string); ok && t != "" {
			fmt.Fprintf(&b, "Titre : %s\n", clipText(oneLine(t), 160))
		}
		if st, _ := sessionEv.Data["state"].(string); st != "" {
			fmt.Fprintf(&b, "État : %s\n", st)
		}
		if d, ok := sessionDuration(sessionEv.Data); ok {
			fmt.Fprintf(&b, "Durée : %s\n", d)
		}
	}
	if summary != nil {
		d := summary.Data
		fmt.Fprintf(&b, "Totaux : %v $, %v appels d'outils, %v délégations, %v décisions, %v refus\n",
			roundCost(d["cost_usd"]), d["tool_calls"], d["subagent_calls"], d["decisions"], d["denied"])
		if per := agentCosts(d["per_agent"]); per != "" {
			fmt.Fprintf(&b, "Coût par agent : %s\n", per)
		}
	}
	section(&b, "Checkpoints", cps)
	section(&b, "Checkpoints demandés plusieurs fois", repeated(asked))
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

// sessionDuration is the time from the start to the end of the session.
func sessionDuration(d map[string]any) (string, bool) {
	start, ok1 := traceTime(d["started_at"])
	end, ok2 := traceTime(d["ended_at"])
	if !ok1 || !ok2 || end.Before(start) {
		return "", false
	}
	return end.Sub(start).Round(time.Second).String(), true
}

func traceTime(v any) (time.Time, bool) {
	switch x := v.(type) {
	case time.Time:
		return x, !x.IsZero()
	case string:
		t, err := time.Parse(time.RFC3339Nano, x)
		return t, err == nil
	}
	return time.Time{}, false
}

// agentCosts lists the cost of each agent, most expensive first.
func agentCosts(v any) string {
	per, ok := v.(map[string]map[string]any)
	if !ok {
		return ""
	}
	type ac struct {
		agent string
		cost  float64
	}
	var list []ac
	for agent, m := range per {
		if c, ok := m["cost"].(float64); ok && agent != "" {
			list = append(list, ac{agent, c})
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].cost != list[j].cost {
			return list[i].cost > list[j].cost
		}
		return list[i].agent < list[j].agent
	})
	parts := make([]string, len(list))
	for i, x := range list {
		parts[i] = fmt.Sprintf("%s %.2f $", x.agent, x.cost)
	}
	return strings.Join(parts, ", ")
}

// repeated lists the checkpoints asked more than once (retries).
func repeated(asked map[string]int) []string {
	var out []string
	for _, id := range sortedKeys(asked) {
		if asked[id] > 1 {
			out = append(out, fmt.Sprintf("%s ×%d", id, asked[id]))
		}
	}
	return out
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
