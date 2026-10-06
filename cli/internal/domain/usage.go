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

// ProxyUsage is the traffic of a server group through the proxy.
type ProxyUsage struct {
	Requests  int64
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
