package domain

import (
	"context"
	"time"
)

// Usage accounting (I6, migration v40). The daemon records what the
// sessions spend, as reported by the tool (cost in USD, tokens) and as seen
// by the credential proxy (tokens per server group), per local day. It
// survives daemon restarts and sleep/resume cycles.

// UsageDay is the accounting day of t (local time, YYYY-MM-DD).
func UsageDay(t time.Time) string { return t.Local().Format("2006-01-02") }

// SessionUsage is what a tool session spent (on a day, or in total).
type SessionUsage struct {
	Day       string
	SessionID string // tool session (the oh session, or one of its subagent sessions)
	RootID    string // oh session it works for (= SessionID for the oh session)
	ProjectID string
	GroupKey  string
	CostUSD   float64
	TokensIn  int64
	TokensOut int64
}

// EstimatedUsagePrefix starts the tool session id of the ledger rows that hold
// dollars estimated by oh, not reported by the tool: the responses cut before
// their end, for which the tool records no cost. They are summed with the
// others in the totals of the oh session (root_id); the prefix is the mark.
const EstimatedUsagePrefix = "estimate:"

// EstimatedUsageID is the tool session id of the estimated dollars of an oh
// session.
func EstimatedUsageID(rootID string) string { return EstimatedUsagePrefix + rootID }

// ProxyUsage is the traffic of a server group through the proxy.
type ProxyUsage struct {
	Requests  int64
	TokensIn  int64
	TokensOut int64
}

// UsageBy is the axis of a spending report.
type UsageBy string

const (
	UsageByProject  UsageBy = "project"
	UsageByWorkflow UsageBy = "workflow"
)

// UsageGroup is what a project or a workflow spent over a period. An empty
// Key is the spending with no known project or workflow (session deleted by
// gc, session started before workflows).
type UsageGroup struct {
	Key       string
	Sessions  int // oh sessions that spent something
	CostUSD   float64
	TokensIn  int64
	TokensOut int64
}

// UsageStore persists the usage ledger and the budget raises.
type UsageStore interface {
	// AddSession adds the deltas of u to its (day, session) row.
	AddSession(ctx context.Context, u SessionUsage) error
	// SessionTotal sums an oh session (with its subagent sessions) over all days.
	SessionTotal(ctx context.Context, rootID string) (SessionUsage, error)
	// ToolSessionTotal sums one tool session over all days.
	ToolSessionTotal(ctx context.Context, sessionID string) (SessionUsage, error)
	// DayCost sums the cost of a day, for a project ("" = all projects).
	DayCost(ctx context.Context, day, projectID string) (float64, error)
	// UsageReport sums the ledger from day since (YYYY-MM-DD, "" = everything)
	// per project or workflow, the most expensive first. The ledger is joined
	// with the sessions (outer join): a deleted session stays in the sums, with
	// the key it had in the ledger (project) or "" (workflow).
	UsageReport(ctx context.Context, by UsageBy, since string) ([]UsageGroup, error)
	// AddProxy adds proxy traffic to its (day, group) row.
	AddProxy(ctx context.Context, group, day string, u ProxyUsage) error
	// ProxyTotal sums the proxy traffic of a group over all days.
	ProxyTotal(ctx context.Context, group string) (ProxyUsage, error)
	// AddBudgetExtra raises a budget: scope "session:<id>" (day "") or a
	// daily scope ("global", "project:<id>") with its day.
	AddBudgetExtra(ctx context.Context, scope, day string, usd float64) error
	// BudgetExtra returns the raises of a scope and day.
	BudgetExtra(ctx context.Context, scope, day string) (float64, error)
}
