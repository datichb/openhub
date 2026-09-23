// Package task defines the universal work-unit abstraction for parallel execution.
// It generalizes Beads tickets, sweep items, and arbitrary task identifiers.
package task

// Kind identifies the type of work unit for downstream behavior (merge policy, prompts, etc.).
type Kind string

const (
	KindTicket Kind = "ticket" // Beads ticket (bd-42, BD-123)
	KindSweep  Kind = "sweep"  // Sweep-generated task (lint-fix-1, migration-pkg-auth)
	KindCustom Kind = "custom" // Free-form identifier from user
)

// Task is the generalized work unit for parallel execution.
// It replaces raw string ticket IDs throughout the parallel infrastructure.
type Task struct {
	// ID is the unique identifier used for state lookups, branch naming, and dedup.
	// Examples: "bd-42", "sweep-lint-pkg-auth", "refactor-config-layer"
	ID string

	// Kind determines merge policy, prompt strategy, and display behavior.
	Kind Kind

	// Label is a human-readable short description shown in TUI.
	// Falls back to ID if empty.
	Label string

	// Description is the full task context passed to the prompt generator.
	// For tickets: ticket title + acceptance criteria from beads.
	// For sweeps: the decomposed sub-task description from the splitter.
	Description string

	// BranchName overrides the auto-generated branch name.
	// Empty = use BranchPattern from CoordinatorOpts (default: "feat/<ID>").
	BranchName string

	// EstimateMinutes is the expected duration (0 = use config default).
	EstimateMinutes int

	// Priority marks this task for first-merge ordering.
	Priority bool

	// Metadata holds arbitrary key-value pairs for extensibility.
	// Sweep mode stores: {"scope": "pkg/auth", "rule": "no-bare-returns"}
	// Ticket mode stores: {"beads": "true", "estimate_source": "beads"}
	Metadata map[string]string
}

// DisplayName returns Label if set, otherwise ID.
func (t Task) DisplayName() string {
	if t.Label != "" {
		return t.Label
	}
	return t.ID
}

// IsBeads returns true if this is a Beads-managed ticket.
func (t Task) IsBeads() bool {
	if t.Kind == KindTicket {
		if v, ok := t.Metadata["beads"]; ok {
			return v == "true"
		}
	}
	return false
}

// IsMergeable returns true if the task kind supports auto-merge proposals.
// Beads tickets and sweep tasks are mergeable; custom tasks are not.
func (t Task) IsMergeable() bool {
	return (t.Kind == KindTicket && t.IsBeads()) || t.Kind == KindSweep
}

// TicketsToTasks converts a legacy string slice of ticket IDs to a slice of Tasks.
// Used for backward compatibility with the existing --tickets CLI flag.
func TicketsToTasks(tickets []string, estimates map[string]int, priority string) []Task {
	tasks := make([]Task, 0, len(tickets))
	for _, tid := range tickets {
		t := Task{
			ID:       tid,
			Kind:     KindTicket,
			Priority: priority != "" && priority == tid,
			Metadata: map[string]string{"beads": "true"},
		}
		if est, ok := estimates[tid]; ok && est > 0 {
			t.EstimateMinutes = est
		}
		tasks = append(tasks, t)
	}
	return tasks
}
