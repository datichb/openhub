package parallel

// Config holds the parallel execution configuration.
type Config struct {
	MaxSessions    int    `toml:"max_sessions"`      // Max concurrent sessions (default: 3)
	PortRangeStart int    `toml:"port_range_start"`  // Starting port for opencode serve (default: 4100)
	AutoMergeBeads bool   `toml:"auto_merge_beads"`  // Propose auto merge for Beads tickets
	AutoMergeExt   bool   `toml:"auto_merge_external"` // Never for external tickets (always false)
}

// DefaultConfig returns the default parallel configuration.
func DefaultConfig() Config {
	return Config{
		MaxSessions:    3,
		PortRangeStart: 4100,
		AutoMergeBeads: true,
		AutoMergeExt:   false,
	}
}

// Validate checks the config for sanity.
func (c *Config) Validate() {
	if c.MaxSessions <= 0 {
		c.MaxSessions = 3
	}
	if c.MaxSessions > 10 {
		c.MaxSessions = 10
	}
	if c.PortRangeStart <= 0 {
		c.PortRangeStart = 4100
	}
	// External merge is never allowed
	c.AutoMergeExt = false
}
