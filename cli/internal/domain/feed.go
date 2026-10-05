package domain

import "time"

// FeedKind classifies an entry of a session live feed (P3-T10).
type FeedKind string

const (
	FeedAgent    FeedKind = "agent"    // the current agent changed (Agent)
	FeedText     FeedKind = "text"     // the agent wrote a message (Text)
	FeedTool     FeedKind = "tool"     // a tool call (Tool, Title; Status when done)
	FeedDelegate FeedKind = "delegate" // a delegation to a subagent (Agent)
	FeedDecision FeedKind = "decision" // a decision is waiting (Title)
	FeedState    FeedKind = "state"    // the agent loop started / ended (Status)
	FeedUsage    FeedKind = "usage"    // cost update (Cost)
)

// FeedItem is one line of a session live feed. Subagent activity is
// reported on the oh session that delegated it (Agent tells who).
type FeedItem struct {
	Time      time.Time `json:"time"`
	SessionID string    `json:"session_id"`
	Kind      FeedKind  `json:"kind"`
	Agent     string    `json:"agent,omitempty"`
	Tool      string    `json:"tool,omitempty"`
	Title     string    `json:"title,omitempty"`
	Text      string    `json:"text,omitempty"`
	Status    string    `json:"status,omitempty"` // tool: ok | failed; state: started | succeeded | failed | interrupted
	Cost      float64   `json:"cost,omitempty"`
}

// SessionChange tells that a session or its decisions changed: clients
// reload what they show (badge, Sessions view, inbox).
type SessionChange struct {
	Time      time.Time `json:"time"`
	SessionID string    `json:"session_id"`
	GroupKey  string    `json:"group_key,omitempty"`
	State     RunState  `json:"state,omitempty"`
	Decisions bool      `json:"decisions,omitempty"` // the open decisions changed
}
