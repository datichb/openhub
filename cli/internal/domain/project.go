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
	MCP            []string               // deprecated: use MCPConfig. Kept for backward compat migration.
	MCPConfig      *ProjectMCPConfig      // per-project MCP overrides (nil = inherit hub defaults)
	ProviderConfig *ProjectProviderConfig // per-project provider config overrides (nil = inherit hub)
	ModelOverrides *ProjectModelOverrides // per-project model cascade overrides (nil = no overrides)
	TeamConfig     *ProjectTeamConfig     // deprecated: use TeamID. Kept for backward compat migration.
	// TeamID links this project to a team by its ID (matching a teams[].id entry
	// in hub.toml). nil = solo project (no team affiliation). When set, the project
	// inherits team-level configuration (MCP, tracker, models, policies).
	TeamID *string
	Status ProjectStatus
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
// When non-nil and Services is non-empty, each service entry specifies an explicit
// override for that MCP server. Services not listed inherit from hub.toml.
// The Enabled field controls activation:
//   - nil   → inherit hub-level enabled state
//   - true  → force-enable regardless of hub config
//   - false → force-disable regardless of hub config
type ProjectMCPConfig struct {
	Services []ProjectMCPService `json:"services,omitempty"`
}

// ProjectMCPService represents a single MCP service configuration at the project level.
type ProjectMCPService struct {
	Name         string `json:"name"`                    // "figma", "gitlab", "gslides", "team"
	Enabled      *bool  `json:"enabled,omitempty"`       // nil = inherit hub, true/false = override
	TokenKey     string `json:"token_key,omitempty"`     // keychain key override (empty = inherit hub)
	WriteEnabled *bool  `json:"write_enabled,omitempty"` // nil = inherit hub, true/false = override
	URL          string `json:"url,omitempty"`           // per-project URL override (empty = inherit hub/team)
}

// ProjectProviderConfig holds per-project provider configuration overrides.
// Non-empty fields override the hub-level [provider.*] config.
type ProjectProviderConfig struct {
	AWSProfile string `json:"aws_profile,omitempty"` // override AWS profile for this project
	AWSRegion  string `json:"aws_region,omitempty"`  // override AWS region for this project
	AuthMode   string `json:"auth_mode,omitempty"`   // override auth mode for this project
	TokenKey   string `json:"token_key,omitempty"`   // project-specific keychain key for credentials
}

// ProjectTeamConfig holds per-project team configuration.
// Mode controls resolution:
//   - "" or "inherit" → use hub-level [team] config as-is (nil pointer also means inherit)
//   - "custom"        → use the fields below; MemberID falls back to hub MemberID if empty
//   - "disabled"      → team features explicitly off for this project (even if hub team is active)
type ProjectTeamConfig struct {
	// Mode is the resolution strategy: "inherit" | "custom" | "disabled".
	Mode string `json:"mode"`
	// StateRepo is the Git remote URL of the team-state repository.
	// Only used when Mode == "custom".
	StateRepo string `json:"state_repo,omitempty"`
	// StatePath is the local clone path for the team-state repo.
	// Only used when Mode == "custom". Auto-derived from StateRepo if empty.
	StatePath string `json:"state_path,omitempty"`
	// MemberID overrides the hub-level member_id for this project.
	// Only used when Mode == "custom". Falls back to hub MemberID when empty.
	MemberID string `json:"member_id,omitempty"`
}

// ProjectTeamMode constants for ProjectTeamConfig.Mode.
const (
	ProjectTeamModeInherit  = "inherit"
	ProjectTeamModeCustom   = "custom"
	ProjectTeamModeDisabled = "disabled"
)

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
	// GetByName retrieves a project by its display name. Returns ErrNotFound if absent.
	GetByName(ctx context.Context, name string) (*Project, error)
	// GetByPath retrieves a project by its filesystem path.
	GetByPath(ctx context.Context, path string) (*Project, error)
	// Create inserts a new project. Returns ErrAlreadyExists if the name is taken.
	Create(ctx context.Context, p *Project) error
	// Update modifies an existing project.
	Update(ctx context.Context, p *Project) error
	// Delete removes a project by ID.
	Delete(ctx context.Context, id string) error
}
