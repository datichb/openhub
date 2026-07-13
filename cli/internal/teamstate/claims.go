package teamstate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// Claim represents a ticket reservation by a team member.
type Claim struct {
	TicketID     string    `toml:"-"` // derived from filename
	Project      string    `toml:"-"` // derived from directory
	ClaimedBy    string    `toml:"claimed_by"`
	ClaimedAt    time.Time `toml:"claimed_at"`
	Worktree     string    `toml:"worktree,omitempty"`      // associated branch
	Status       string    `toml:"status"`                  // in_progress | review | blocked
	LastActivity time.Time `toml:"last_activity,omitempty"` // last session/commit activity
}

// ListClaims returns all active claims for a project.
// If project is empty, returns claims across all projects.
func (r *Repo) ListClaims(project string) ([]Claim, error) {
	if project != "" {
		return r.listClaimsForProject(project)
	}
	// List all projects
	projectsDir := filepath.Join(r.path, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading projects dir: %w", err)
	}
	var all []Claim
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		claims, err := r.listClaimsForProject(e.Name())
		if err != nil {
			return nil, err
		}
		all = append(all, claims...)
	}
	return all, nil
}

// GetClaim retrieves a specific claim by project and ticket ID.
func (r *Repo) GetClaim(project, ticketID string) (*Claim, error) {
	path := r.claimFilePath(project, ticketID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrClaimNotFound
		}
		return nil, fmt.Errorf("reading claim %s/%s: %w", project, ticketID, err)
	}
	var c Claim
	if err := toml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing claim %s/%s: %w", project, ticketID, err)
	}
	c.TicketID = ticketID
	c.Project = project
	return &c, nil
}

// CreateClaim reserves a ticket for a member.
// Returns ErrClaimExists if already claimed (warning — not blocking).
func (r *Repo) CreateClaim(ctx context.Context, c Claim) (*Claim, error) {
	// Pull before checking
	if err := r.Pull(ctx); err != nil && err != ErrNotCloned {
		return nil, err
	}

	// Check if already claimed
	existing, err := r.GetClaim(c.Project, c.TicketID)
	if err == nil {
		// Already claimed — return existing for warning
		return existing, ErrClaimExists
	}

	// Ensure project claims directory
	dir := filepath.Join(r.path, "projects", c.Project, "claims")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating claims dir: %w", err)
	}

	// Write claim file
	if c.ClaimedAt.IsZero() {
		c.ClaimedAt = time.Now().UTC()
	}
	if c.Status == "" {
		c.Status = "in_progress"
	}

	data, err := toml.Marshal(&c)
	if err != nil {
		return nil, fmt.Errorf("marshaling claim: %w", err)
	}

	path := r.claimFilePath(c.Project, c.TicketID)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nil, fmt.Errorf("writing claim: %w", err)
	}

	// Commit and push
	relPath := r.claimRelPath(c.Project, c.TicketID)
	msg := fmt.Sprintf("claim: %s takes %s/%s", c.ClaimedBy, c.Project, c.TicketID)
	if err := r.CommitAndPush(ctx, msg, relPath); err != nil {
		return nil, err
	}

	return nil, nil
}

// ReleaseClaim removes a claim (ticket is done or abandoned).
func (r *Repo) ReleaseClaim(ctx context.Context, project, ticketID string) error {
	if err := r.Pull(ctx); err != nil && err != ErrNotCloned {
		return err
	}

	path := r.claimFilePath(project, ticketID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return ErrClaimNotFound
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing claim file: %w", err)
	}

	relPath := r.claimRelPath(project, ticketID)
	msg := fmt.Sprintf("release: %s/%s", project, ticketID)
	return r.CommitAndPush(ctx, msg, relPath)
}

// TransferClaim changes the owner of an existing claim.
func (r *Repo) TransferClaim(ctx context.Context, project, ticketID, newOwner string) error {
	if err := r.Pull(ctx); err != nil && err != ErrNotCloned {
		return err
	}

	c, err := r.GetClaim(project, ticketID)
	if err != nil {
		return err
	}

	previousOwner := c.ClaimedBy
	c.ClaimedBy = newOwner

	data, marshalErr := toml.Marshal(c)
	if marshalErr != nil {
		return fmt.Errorf("marshaling claim: %w", marshalErr)
	}

	path := r.claimFilePath(project, ticketID)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing claim: %w", err)
	}

	relPath := r.claimRelPath(project, ticketID)
	msg := fmt.Sprintf("transfer: %s/%s from %s to %s", project, ticketID, previousOwner, newOwner)
	return r.CommitAndPush(ctx, msg, relPath)
}

// claimFilePath returns the absolute path to a claim TOML file.
func (r *Repo) claimFilePath(project, ticketID string) string {
	// Sanitize ticketID for filesystem safety
	safe := strings.ReplaceAll(ticketID, "/", "_")
	return filepath.Join(r.path, "projects", project, "claims", safe+".toml")
}

// claimRelPath returns the repo-relative path to a claim file.
func (r *Repo) claimRelPath(project, ticketID string) string {
	safe := strings.ReplaceAll(ticketID, "/", "_")
	return filepath.Join("projects", project, "claims", safe+".toml")
}

func (r *Repo) listClaimsForProject(project string) ([]Claim, error) {
	dir := filepath.Join(r.path, "projects", project, "claims")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading claims for %s: %w", project, err)
	}

	var claims []Claim
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		ticketID := strings.TrimSuffix(e.Name(), ".toml")
		c, err := r.GetClaim(project, ticketID)
		if err != nil {
			continue // skip malformed
		}
		claims = append(claims, *c)
	}
	return claims, nil
}
