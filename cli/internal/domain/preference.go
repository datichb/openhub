package domain

import (
	"context"
	"encoding/json"
	"time"
)

// Preference scopes: "global", "project:<id>", "team:<id>".
const (
	PreferenceScopeGlobal = "global"
	prefScopeProject      = "project:"
	prefScopeTeam         = "team:"
)

// ProjectPreferenceScope returns the scope of a project's preferences.
func ProjectPreferenceScope(projectID string) string { return prefScopeProject + projectID }

// TeamPreferenceScope returns the scope of a team's preferences.
func TeamPreferenceScope(teamID string) string { return prefScopeTeam + teamID }

// Preference is a JSON value stored per scope and key (pinned workflows,
// UI settings).
type Preference struct {
	Scope     string
	Key       string
	Value     json.RawMessage
	UpdatedAt time.Time
}

// PreferenceStore persists preferences.
type PreferenceStore interface {
	// Get returns ErrNotFound when the key is not set.
	Get(ctx context.Context, scope, key string) (*Preference, error)
	Set(ctx context.Context, scope, key string, value json.RawMessage) error
	Delete(ctx context.Context, scope, key string) error
	List(ctx context.Context, scope string) ([]Preference, error)
}

// WorkflowUse is the last use of a workflow, derived from the sessions.
type WorkflowUse struct {
	WorkflowID string
	LastUsed   time.Time
}

// WorkflowUsageReader lists recently used workflows (most recent first).
type WorkflowUsageReader interface {
	// RecentWorkflows returns distinct workflow IDs of the sessions, filtered
	// by project when projectID is not empty.
	RecentWorkflows(ctx context.Context, projectID string, limit int) ([]WorkflowUse, error)
}
