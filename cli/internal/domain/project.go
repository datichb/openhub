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
	ID        string
	Name      string
	Path      string
	Language  string
	Provider  string // LLM provider override (bedrock, anthropic, openai, openrouter); empty = use hub default
	Model     string // LLM model override (claude-sonnet-4-5, etc.); empty = use hub default
	Labels    []string
	Agents    []string
	MCP       []string
	Status    ProjectStatus
	CreatedAt time.Time
	UpdatedAt time.Time
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
