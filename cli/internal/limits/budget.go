package limits

// Budget decision data (Payload.Data).
const (
	DataBudgetLimit = "limit"     // "session" | "daily"
	DataBudgetScope = "scope"     // raise scope (limits.SessionScope, daily scope)
	DataBudgetDay   = "day"       // day of a daily raise
	DataBudgetUSD   = "limit_usd" // allowance reached (budget + raises)
	DataBudgetSpent = "spent_usd"
	DataBudgetUnit  = "unit_usd" // configured budget (default raise)
)

// BudgetSession / BudgetDaily are the DataBudgetLimit values.
const (
	BudgetSession = "session"
	BudgetDaily   = "daily"
)

// IsBudgetData reports whether the data of a budget decision comes from
// the restrictions (as opposed to an exhausted proxy token budget).
func IsBudgetData(data map[string]any) bool {
	_, ok := data[DataBudgetLimit]
	return ok
}
