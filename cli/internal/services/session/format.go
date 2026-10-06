package session

import (
	"sort"
	"strconv"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// Plain-text renderings shared by the CLI and the TUI (the TUI adds colors).

// KindIcon is the glyph of a decision kind (10 §0 legend).
func KindIcon(k domain.DecisionKind) string {
	switch k {
	case domain.DecisionCheckpoint:
		return "⏸"
	case domain.DecisionQuestion:
		return "?"
	case domain.DecisionPermission:
		return "!"
	case domain.DecisionBudget:
		return "$"
	case domain.DecisionError:
		return "✗"
	}
	return "•"
}

// StateIcon is the glyph of a run state.
func StateIcon(s domain.RunState) string {
	switch s {
	case domain.RunActive, domain.RunPreparing:
		return "●"
	case domain.RunWaiting:
		return "⏸"
	case domain.RunIdle:
		return "○"
	case domain.RunSleeping:
		return "◌"
	case domain.RunFailed:
		return "✗"
	case domain.RunCompleted, domain.RunStopped:
		return "✔"
	}
	return "·"
}

// DecisionSummary describes a decision in one line (no secret: commands and
// paths come from the agent request).
func DecisionSummary(d domain.Decision) string {
	p := d.Payload
	switch d.Kind {
	case domain.DecisionPermission:
		s := p.Action
		if len(p.Resources) > 0 {
			s += " " + strings.Join(p.Resources, ", ")
		}
		return oneLine(s)
	case domain.DecisionQuestion:
		// The form title is often generic ("Questions"): the first field says more.
		if len(p.Fields) > 0 {
			s := oneLine(firstNonEmpty(p.Fields[0].Description, p.Fields[0].Title, p.Title, p.Fields[0].Key))
			if len(p.Fields) > 1 {
				s += " (+" + strconv.Itoa(len(p.Fields)-1) + ")"
			}
			return s
		}
		if p.Title != "" {
			return oneLine(p.Title)
		}
	case domain.DecisionBudget:
		return i18n.T("cmd.session.decision.budget")
	case domain.DecisionError:
		if p.Message != "" {
			return oneLine(p.Message)
		}
		return i18n.T("cmd.session.decision.error")
	}
	return oneLine(firstNonEmpty(p.Title, p.Message, string(d.Kind)))
}

// DecisionBadges renders open decisions as "! 1  ? 2".
func DecisionBadges(ds []domain.Decision) string {
	order := []domain.DecisionKind{domain.DecisionCheckpoint, domain.DecisionQuestion, domain.DecisionPermission, domain.DecisionBudget, domain.DecisionError}
	counts := map[domain.DecisionKind]int{}
	for _, d := range ds {
		counts[d.Kind]++
	}
	var parts []string
	for _, k := range order {
		if n := counts[k]; n > 0 {
			parts = append(parts, KindIcon(k)+" "+strconv.Itoa(n))
			delete(counts, k)
		}
	}
	rest := make([]string, 0, len(counts))
	for k, n := range counts {
		rest = append(rest, KindIcon(k)+" "+strconv.Itoa(n))
	}
	sort.Strings(rest)
	parts = append(parts, rest...)
	return strings.Join(parts, "  ")
}

// FeedLine renders a feed item without its time: "agent › text".
func FeedLine(it domain.FeedItem) string {
	who := it.Agent
	prefix := ""
	if who != "" {
		prefix = who + " › "
	}
	switch it.Kind {
	case domain.FeedAgent:
		return "→ " + it.Agent
	case domain.FeedDelegate:
		s := "⤷ " + it.Agent
		if it.Title != "" {
			s += " (" + it.Title + ")"
		}
		return s
	case domain.FeedText:
		return prefix + "« " + oneLine(it.Text) + " »"
	case domain.FeedTool:
		s := prefix + it.Tool
		if it.Title != "" {
			s += " " + it.Title
		}
		switch it.Status {
		case "ok":
			s += " ✔"
		case "failed":
			s += " ✗"
		}
		return s
	case domain.FeedDecision:
		return prefix + i18n.T("cmd.session.feed.decision."+it.Title)
	case domain.FeedState:
		if t := i18n.T("cmd.session.feed.state." + it.Status); t != "cmd.session.feed.state."+it.Status {
			return t
		}
		return "· " + it.Status
	case domain.FeedUsage:
		return ""
	}
	return prefix + it.Text
}

func oneLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i] + " …"
	}
	return s
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}
