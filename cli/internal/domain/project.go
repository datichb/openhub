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
	MCP            []string               // deprecated: use MCPConfig. Kept for backward compat migration.
	MCPConfig      *ProjectMCPConfig      // per-project MCP overrides (nil = inherit hub defaults)
	ProviderConfig *ProjectProviderConfig // per-project provider config overrides (nil = inherit hub)
	ModelOverrides *ProjectModelOverrides // per-project model cascade overrides (nil = no overrides)
	TeamConfig     *ProjectTeamConfig     // deprecated: use TeamID. Kept for backward compat migration.
	TrackerConfig  *ProjectTrackerConfig  // per-project tracker overrides (nil = inherit team defaults)
	// ExecConfig holds the execution settings of the project (dev image,
	// default workflow and runtime). nil = defaults.
	ExecConfig *ProjectExecConfig
	// TeamID links this project to a team by its ID (matching a teams[].id entry
	// in hub.toml). nil = solo project (no team affiliation). When set, the project
	// inherits team-level configuration (MCP, tracker, models, policies).
	TeamID    *string
	Status    ProjectStatus
	CreatedAt time.Time
	UpdatedAt time.Time
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

// ProjectExecConfig holds the execution settings of a project (v5 phase 4):
// the dev environment of the container runtime and the launch defaults.
type ProjectExecConfig struct {
	// Dockerfile is the dev Dockerfile, absolute or relative to the project
	// path ("" = detected: Dockerfile.dev, dev.Dockerfile,
	// .devcontainer/Dockerfile, Dockerfile).
	Dockerfile string `json:"dockerfile,omitempty"`
	// BuildArgs are the build arguments of the dev image.
	BuildArgs map[string]string `json:"build_args,omitempty"`
	// Volumes are persistent cache volumes: absolute paths in the container,
	// or paths relative to each mounted location (e.g. "node_modules").
	Volumes []string `json:"volumes,omitempty"`
	// DefaultWorkflow is launched by `oh run` without a workflow and comes
	// first in the « Démarrer » section ("" = none).
	DefaultWorkflow string `json:"default_workflow,omitempty"`
	// DefaultRuntime is the preferred runtime (local | container | remote),
	// used when the workflow allows it ("" = settings, then workflow).
	DefaultRuntime string `json:"default_runtime,omitempty"`
}

// IsEmpty reports whether the config holds no setting.
func (c *ProjectExecConfig) IsEmpty() bool {
	return c == nil || (c.Dockerfile == "" && len(c.BuildArgs) == 0 && len(c.Volumes) == 0 &&
		c.DefaultWorkflow == "" && c.DefaultRuntime == "")
}

// Clone returns a deep copy (nil for nil).
func (c *ProjectExecConfig) Clone() *ProjectExecConfig {
	if c == nil {
		return nil
	}
	cp := *c
	if c.BuildArgs != nil {
		cp.BuildArgs = make(map[string]string, len(c.BuildArgs))
		for k, v := range c.BuildArgs {
			cp.BuildArgs[k] = v
		}
	}
	if c.Volumes != nil {
		cp.Volumes = append([]string{}, c.Volumes...)
	}
	return &cp
}

// ProjectTrackerConfig holds per-project tracker overrides.
// Empty fields inherit from the team-state TrackerConfig defaults.
// Resolution cascade: project → team → MCP → env → built-in default.
type ProjectTrackerConfig struct {
	// TrackerProject overrides the team-state TrackerProject for this project.
	// For GitLab: numeric project ID or URL-encoded path (e.g. "group/project").
	// For Jira: project key (e.g. "SRU").
	TrackerProject string `json:"tracker_project,omitempty"`
	// TrackerURL overrides the team-state TrackerURL for this project.
	TrackerURL string `json:"tracker_url,omitempty"`
	// TrackerTokenKey overrides the tracker token keychain key for this project.
	TrackerTokenKey string `json:"tracker_token_key,omitempty"`
	// TicketPattern overrides the team-state TicketPattern regex for this project.
	TicketPattern string `json:"ticket_pattern,omitempty"`
	// WriteEnabled overrides the tracker write permission for this project.
	// nil = inherit from hub/team. true/false = override.
	WriteEnabled *bool `json:"write_enabled,omitempty"`
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
