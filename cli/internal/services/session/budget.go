package session

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/limits"
)

// BudgetResolver answers the budget decisions of the restrictions (I6):
// "raise" adds an amount (Reply.Message, USD; default: the configured
// budget once more) to the session's or the day's allowance; "stop" stops
// the session. The daemon lets the session go on as soon as the allowance
// covers what it spent.
func BudgetResolver(usage domain.UsageStore, stop func(ctx context.Context, sessionID string) error) Resolver {
	return func(ctx context.Context, d domain.Decision, r Reply) error {
		switch r.Decision {
		case "stop":
			return stop(ctx, d.SessionID)
		case "raise":
			amount, err := RaiseAmount(d, r.Message)
			if err != nil {
				return err
			}
			scope, _ := d.Payload.Data[limits.DataBudgetScope].(string)
			day, _ := d.Payload.Data[limits.DataBudgetDay].(string)
			if scope == "" {
				return fmt.Errorf("budget decision %s has no scope", d.ID)
			}
			return usage.AddBudgetExtra(ctx, scope, day, amount)
		}
		return fmt.Errorf("invalid choice %q (raise, stop)", r.Decision)
	}
}

// RaiseAmount is the amount of a raise: text in USD, or by default enough
// to cover what was spent plus the configured budget once more.
func RaiseAmount(d domain.Decision, text string) (float64, error) {
	text = strings.TrimPrefix(strings.TrimSpace(text), "$")
	if text != "" {
		v, err := strconv.ParseFloat(text, 64)
		if err != nil || v <= 0 {
			return 0, fmt.Errorf("invalid raise %q (amount in USD expected)", text)
		}
		return v, nil
	}
	unit, _ := d.Payload.Data[limits.DataBudgetUnit].(float64)
	spent, _ := d.Payload.Data[limits.DataBudgetSpent].(float64)
	allowance, _ := d.Payload.Data[limits.DataBudgetUSD].(float64)
	if unit <= 0 {
		return 0, fmt.Errorf("budget decision %s has no configured budget: give an amount", d.ID)
	}
	return max(0, spent-allowance) + unit, nil
}
