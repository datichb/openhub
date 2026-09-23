// Package sweep implements the sweep mode: large-scale, goal-driven parallel
// execution. A high-level goal is decomposed into concrete sub-tasks via a
// pluggable splitting strategy, executed in parallel via the coordinator, then
// collected, merged, and optionally verified.
package sweep

// Strategy identifies the decomposition algorithm used by the Splitter.
type Strategy string

const (
	// StrategyManual decomposes from an explicit user-provided list.
	StrategyManual Strategy = "manual"

	// StrategyByFile groups files matched by glob patterns into batches.
	StrategyByFile Strategy = "by-file"

	// StrategyByPackage creates one task per Go package (or JS workspace).
	StrategyByPackage Strategy = "by-package"

	// StrategyLLM delegates decomposition to an LLM planner via llm.Completer.
	StrategyLLM Strategy = "llm"
)

// ValidStrategies returns the list of valid strategy names.
func ValidStrategies() []Strategy {
	return []Strategy{StrategyManual, StrategyByFile, StrategyByPackage, StrategyLLM}
}

// IsValidStrategy returns true if s is a recognized strategy.
func IsValidStrategy(s Strategy) bool {
	for _, v := range ValidStrategies() {
		if v == s {
			return true
		}
	}
	return false
}

// SweepConfig holds sweep-specific configuration.
type SweepConfig struct {
	// MaxSplits is the hard cap on task decomposition (default: 10).
	MaxSplits int

	// BranchPrefix is prepended to task IDs for branch naming (default: "sweep/").
	BranchPrefix string

	// VerifyStrategy selects post-sweep verification: none, tests, lint, build, all, custom.
	VerifyStrategy string

	// VerifyCmd is the shell command when VerifyStrategy == "custom".
	VerifyCmd string
}

// DefaultSweepConfig returns sensible defaults for sweep mode.
func DefaultSweepConfig() SweepConfig {
	return SweepConfig{
		MaxSplits:      10,
		BranchPrefix:   "sweep/",
		VerifyStrategy: "none",
	}
}
