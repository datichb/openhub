package domain

import (
	"context"
	"time"
)

// AgentEvent records a single agent execution within a session.
// Enables per-agent metrics: success rate, duration, skill usage, cost.
type AgentEvent struct {
	ID           string
	SessionID    string
	ProjectID    string
	AgentName    string
	SkillsLoaded []string // Bucket A + B skills loaded during the session
	StartedAt    time.Time
	CompletedAt  *time.Time
	Status       AgentEventStatus // "success" | "failed" | "cancelled"
	TokensIn     int64
	TokensOut    int64
	CostUSD      float64 // estimated cost in USD
	ErrorMessage string  // populated if Status == "failed"
}

// AgentEventStatus represents the outcome of an agent execution.
type AgentEventStatus string

const (
	AgentEventSuccess   AgentEventStatus = "success"
	AgentEventFailed    AgentEventStatus = "failed"
	AgentEventCancelled AgentEventStatus = "cancelled"
)

// AgentMetrics holds aggregated statistics for a single agent.
type AgentMetrics struct {
	AgentName      string
	TotalRuns      int
	SuccessCount   int
	FailureCount   int
	SuccessRate    float64 // 0-100
	AvgDurationSec float64
	TotalTokensIn  int64
	TotalTokensOut int64
	TotalCostUSD   float64
	TopSkills      []string // most frequently loaded skills
}

// AgentEventStore defines persistence operations for agent telemetry.
type AgentEventStore interface {
	// Create records a new agent event.
	Create(ctx context.Context, e *AgentEvent) error
	// Update modifies an existing agent event (e.g. on completion).
	Update(ctx context.Context, e *AgentEvent) error
	// ListBySession returns all events for a session.
	ListBySession(ctx context.Context, sessionID string) ([]AgentEvent, error)
	// Metrics returns aggregated metrics per agent for the given project (empty = all).
	Metrics(ctx context.Context, projectID string) ([]AgentMetrics, error)
}
