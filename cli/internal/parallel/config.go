package parallel

// Config holds the parallel execution configuration.
type Config struct {
	MaxSessions               int  `toml:"max_sessions"`                // Max concurrent sessions (default: 5, hard cap: 10)
	MaxBudgetMinutes          int  `toml:"max_budget_minutes"`          // Max total estimated minutes across all parallel tickets (default: 180, 0 = disabled)
	DefaultTicketWeightMin    int  `toml:"default_ticket_weight_min"`   // Fallback weight when ticket has no estimate (default: 60 = "unknown/M")
	PortRangeStart            int  `toml:"port_range_start"`            // Starting port for opencode serve (default: 4100)
	AutoMergeBeads            bool `toml:"auto_merge_beads"`            // Propose auto merge for Beads tickets
	AutoMergeSweep            bool `toml:"auto_merge_sweep"`            // Propose auto merge for sweep tasks (default: true)
	AutoMergeExt              bool `toml:"auto_merge_external"`         // Never for external tickets (always false)
	CleanupCompletedWorktrees bool `toml:"cleanup_completed_worktrees"` // Remove worktrees of completed sessions on shutdown (default: false)
	MaxRetries                int  `toml:"max_retries"`                 // Max recovery attempts per failed session (default: 2, cap: 5)
	RetryDelaySeconds         int  `toml:"retry_delay_seconds"`         // Seconds to wait before retrying a failed session (default: 5, cap: 60)
}

// DefaultConfig returns the default parallel configuration.
func DefaultConfig() Config {
	return Config{
		MaxSessions:               5,
		MaxBudgetMinutes:          180,
		DefaultTicketWeightMin:    60,
		PortRangeStart:            4100,
		AutoMergeBeads:            true,
		AutoMergeSweep:            true,
		AutoMergeExt:              false,
		CleanupCompletedWorktrees: false,
		MaxRetries:                2,
		RetryDelaySeconds:         5,
	}
}

// Validate checks the config for sanity.
func (c *Config) Validate() {
	if c.MaxSessions <= 0 {
		c.MaxSessions = 5
	}
	if c.MaxSessions > 10 {
		c.MaxSessions = 10
	}
	if c.MaxBudgetMinutes < 0 {
		c.MaxBudgetMinutes = 0
	}
	if c.DefaultTicketWeightMin <= 0 {
		c.DefaultTicketWeightMin = 60
	}
	if c.DefaultTicketWeightMin > 480 {
		c.DefaultTicketWeightMin = 480
	}
	if c.PortRangeStart <= 0 {
		c.PortRangeStart = 4100
	}
	if c.MaxRetries < 0 {
		c.MaxRetries = 0
	}
	if c.MaxRetries > 5 {
		c.MaxRetries = 5
	}
	if c.RetryDelaySeconds < 0 {
		c.RetryDelaySeconds = 0
	}
	if c.RetryDelaySeconds > 60 {
		c.RetryDelaySeconds = 60
	}
	// External merge is never allowed
	c.AutoMergeExt = false
}

// TotalBudget calculates the total estimated minutes for a set of tickets.
// Tickets with 0 or missing estimates use DefaultTicketWeightMin.
func (c *Config) TotalBudget(ticketEstimates map[string]int, tickets []string) int {
	total := 0
	for _, tid := range tickets {
		est := ticketEstimates[tid]
		if est <= 0 {
			est = c.DefaultTicketWeightMin
		}
		total += est
	}
	return total
}
