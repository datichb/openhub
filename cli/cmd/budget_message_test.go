package cmd

import (
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/limits"
)

// A28: `oh budget unset` (or a value of 0/off) says the restriction was
// removed, not "saved".
func TestBudgetSavedMessage(t *testing.T) {
	useLocale(t, "en")
	var l limits.Limits
	if got := budgetSavedMessage(l, limits.FieldDailyBudget, ""); !strings.Contains(got, "removed from hub.toml") {
		t.Fatalf("unset hub: %q", got)
	}
	if got := budgetSavedMessage(l, limits.FieldDailyBudget, "demo"); !strings.Contains(got, "removed") || !strings.Contains(got, "demo") {
		t.Fatalf("unset project: %q", got)
	}
	_ = l.Set(limits.FieldDailyBudget, "5")
	if got := budgetSavedMessage(l, limits.FieldDailyBudget, ""); !strings.Contains(got, "saved in hub.toml") {
		t.Fatalf("set hub: %q", got)
	}
}
