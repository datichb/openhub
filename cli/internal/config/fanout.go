package config

// FanoutConfig configures the fan-out of the workflows (R6, experimental):
// one child session per ticket, each in its own worktree, merged by oh.
type FanoutConfig struct {
	// Experimental enables the fan-out on this machine (off by default):
	// without it, a launch with the fan-out input is refused.
	Experimental bool `mapstructure:"experimental" toml:"experimental,omitempty"`
	// Parallel caps the child sessions at the same time (1 to 3; 0 = 1:
	// one after the other).
	Parallel int `mapstructure:"parallel" toml:"parallel,omitempty"`
	// BudgetUSD is the global budget of a fan-out, parent and children
	// together (F-04; 0 = DefaultFanoutBudgetUSD).
	BudgetUSD float64 `mapstructure:"budget_usd" toml:"budget_usd,omitempty"`
}

// DefaultFanoutBudgetUSD is the global budget of a fan-out when none is set.
const DefaultFanoutBudgetUSD = 10.0

// EffectiveParallel is Parallel bounded to [1, 3].
func (f FanoutConfig) EffectiveParallel() int {
	return min(max(f.Parallel, 1), 3)
}

// EffectiveBudget is BudgetUSD, or the default.
func (f FanoutConfig) EffectiveBudget() float64 {
	if f.BudgetUSD > 0 {
		return f.BudgetUSD
	}
	return DefaultFanoutBudgetUSD
}
