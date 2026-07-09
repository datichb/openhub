// Package domain defines core business entities and interfaces.
// This package has ZERO infrastructure dependencies — it is imported by all other layers
// but never imports them (Dependency Rule).
package domain

import (
	"context"
	"time"
)

// Project represents a registered project in the hub.
type Project struct {
	ID             string
	Name           string
	Path           string
	Language       string
	Provider       string // LLM provider override (bedrock, anthropic, openai, openrouter); empty = use hub default
	Model          string // LLM model override (claude-sonnet-4-5, etc.); empty = use hub default
	Labels         []string
	Agents         []string
	MCP            []string                // deprecated: use MCPConfig. Kept for backward compat migration.
	MCPConfig      *ProjectMCPConfig       // per-project MCP overrides (nil = inherit hub defaults)
	ModelOverrides *ProjectModelOverrides   // per-project model cascade overrides (nil = no overrides)
	Status         ProjectStatus
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ProjectModelOverrides holds per-agent and per-family model overrides at the project level.
// These override hub-level settings (hub.toml [models]) but are overridden by the agent's
// frontmatter is never overridden — the cascade is:
// project.Agents > project.Families > project.Model > hub.Agents > hub.Families > hub.Default > frontmatter
type ProjectModelOverrides struct {
	Families map[string]string `json:"families,omitempty"` // family name → model
	Agents   map[string]string `json:"agents,omitempty"`   // agent-id → model
}

// ProjectMCPConfig holds per-project MCP server overrides.
// When non-nil and Services is non-empty, it REPLACES the hub-level MCP list for this project.
// Credentials (TokenKey) and options (WriteEnabled) can be overridden per service;
// empty/nil values inherit from hub.toml.
type ProjectMCPConfig struct {
	Services []ProjectMCPService `json:"services,omitempty"`
}

// ProjectMCPService represents a single MCP service configuration at the project level.
type ProjectMCPService struct {
	Name         string `json:"name"`                    // "figma", "gitlab", "gslides"
	TokenKey     string `json:"token_key,omitempty"`     // keychain key override (empty = inherit hub)
	WriteEnabled *bool  `json:"write_enabled,omitempty"` // nil = inherit hub, true/false = override
}

// ProjectStatus represents the lifecycle state of a project.
type ProjectStatus string

const (
	ProjectStatusActive   ProjectStatus = "active"
	ProjectStatusArchived ProjectStatus = "archived"
)

// ProjectStore defines the contract for project persistence.
type ProjectStore interface {
	// List returns projects filtered by status. Empty status returns all.
	List(ctx context.Context, status ProjectStatus) ([]Project, error)
	// Get retrieves a project by ID. Returns ErrNotFound if absent.
	Get(ctx context.Context, id string) (*Project, error)
	// GetByPath retrieves a project by its filesystem path.
	GetByPath(ctx context.Context, path string) (*Project, error)
	// Create inserts a new project.
	Create(ctx context.Context, p *Project) error
	// Update modifies an existing project.
	Update(ctx context.Context, p *Project) error
	// Delete removes a project by ID.
	Delete(ctx context.Context, id string) error
}
