package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
)

// ProjectStore implements domain.ProjectStore backed by SQLite.
type ProjectStore struct {
	db *sql.DB
}

// NewProjectStore creates a ProjectStore from a shared Store.
func NewProjectStore(s *Store) *ProjectStore {
	return &ProjectStore{db: s.DB()}
}

// Ensure interface compliance at compile time.
var _ domain.ProjectStore = (*ProjectStore)(nil)

func (ps *ProjectStore) List(ctx context.Context, status domain.ProjectStatus) ([]domain.Project, error) {
	query := `SELECT id, name, path, language, tracker, provider, model, model_overrides, mcp_config, provider_config, labels, agents, mcp, status, created_at, updated_at FROM projects`
	var args []interface{}
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, string(status))
	}
	query += " ORDER BY name ASC"

	rows, err := ps.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	defer rows.Close()

	var projects []domain.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, *p)
	}
	return projects, rows.Err()
}

func (ps *ProjectStore) Get(ctx context.Context, id string) (*domain.Project, error) {
	row := ps.db.QueryRow(
		`SELECT id, name, path, language, tracker, provider, model, model_overrides, mcp_config, provider_config, labels, agents, mcp, status, created_at, updated_at FROM projects WHERE id = ?`,
		id,
	)
	p, err := scanProjectRow(row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting project %s: %w", id, err)
	}
	return p, nil
}

func (ps *ProjectStore) GetByPath(ctx context.Context, path string) (*domain.Project, error) {
	row := ps.db.QueryRow(
		`SELECT id, name, path, language, tracker, provider, model, model_overrides, mcp_config, provider_config, labels, agents, mcp, status, created_at, updated_at FROM projects WHERE path = ?`,
		path,
	)
	p, err := scanProjectRow(row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting project by path %s: %w", path, err)
	}
	return p, nil
}

func (ps *ProjectStore) Create(ctx context.Context, p *domain.Project) error {
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = p.CreatedAt
	}

	_, err := ps.db.Exec(
		`INSERT INTO projects (id, name, path, language, tracker, provider, model, model_overrides, mcp_config, provider_config, labels, agents, mcp, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Path, p.Language, "", p.Provider, p.Model, marshalModelOverrides(p.ModelOverrides),
		marshalMCPConfig(p.MCPConfig), marshalProviderConfig(p.ProviderConfig),
		joinStrings(p.Labels), joinStrings(p.Agents), joinStrings(p.MCP),
		string(p.Status), p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("creating project: %w", err)
	}
	return nil
}

func (ps *ProjectStore) Update(ctx context.Context, p *domain.Project) error {
	p.UpdatedAt = time.Now()
	result, err := ps.db.Exec(
		`UPDATE projects SET name=?, path=?, language=?, tracker=?, provider=?, model=?, model_overrides=?, mcp_config=?, provider_config=?, labels=?, agents=?, mcp=?, status=?, updated_at=?
		 WHERE id=?`,
		p.Name, p.Path, p.Language, "", p.Provider, p.Model, marshalModelOverrides(p.ModelOverrides),
		marshalMCPConfig(p.MCPConfig), marshalProviderConfig(p.ProviderConfig),
		joinStrings(p.Labels), joinStrings(p.Agents), joinStrings(p.MCP),
		string(p.Status), p.UpdatedAt, p.ID,
	)
	if err != nil {
		return fmt.Errorf("updating project %s: %w", p.ID, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (ps *ProjectStore) Delete(ctx context.Context, id string) error {
	result, err := ps.db.Exec("DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting project %s: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// --- helpers ---

func scanProject(rows *sql.Rows) (*domain.Project, error) {
	var p domain.Project
	var labels, agents, mcp, status, tracker, modelOverrides, mcpConfig, providerConfig string
	err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.Language, &tracker, &p.Provider, &p.Model,
		&modelOverrides, &mcpConfig, &providerConfig, &labels, &agents, &mcp, &status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scanning project: %w", err)
	}
	p.Labels = splitStrings(labels)
	p.Agents = splitStrings(agents)
	p.MCP = splitStrings(mcp)
	p.Status = domain.ProjectStatus(status)
	p.ModelOverrides = unmarshalModelOverrides(modelOverrides)
	p.MCPConfig = unmarshalMCPConfig(mcpConfig, p.MCP)
	p.ProviderConfig = unmarshalProviderConfig(providerConfig)
	return &p, nil
}

func scanProjectRow(row *sql.Row) (*domain.Project, error) {
	var p domain.Project
	var labels, agents, mcp, status, tracker, modelOverrides, mcpConfig, providerConfig string
	err := row.Scan(&p.ID, &p.Name, &p.Path, &p.Language, &tracker, &p.Provider, &p.Model,
		&modelOverrides, &mcpConfig, &providerConfig, &labels, &agents, &mcp, &status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.Labels = splitStrings(labels)
	p.Agents = splitStrings(agents)
	p.MCP = splitStrings(mcp)
	p.Status = domain.ProjectStatus(status)
	p.ModelOverrides = unmarshalModelOverrides(modelOverrides)
	p.MCPConfig = unmarshalMCPConfig(mcpConfig, p.MCP)
	p.ProviderConfig = unmarshalProviderConfig(providerConfig)
	return &p, nil
}

func joinStrings(s []string) string {
	return strings.Join(s, ",")
}

func splitStrings(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// marshalModelOverrides serializes project model overrides to JSON for storage.
// Returns "" for nil overrides (no overrides set).
func marshalModelOverrides(mo *domain.ProjectModelOverrides) string {
	if mo == nil {
		return ""
	}
	// Skip serialization if both maps are empty
	if len(mo.Families) == 0 && len(mo.Agents) == 0 {
		return ""
	}
	data, err := json.Marshal(mo)
	if err != nil {
		return ""
	}
	return string(data)
}

// unmarshalModelOverrides deserializes project model overrides from JSON storage.
// Returns nil for empty strings (no overrides).
func unmarshalModelOverrides(s string) *domain.ProjectModelOverrides {
	if s == "" {
		return nil
	}
	var mo domain.ProjectModelOverrides
	if err := json.Unmarshal([]byte(s), &mo); err != nil {
		return nil
	}
	// Return nil if both maps are empty after unmarshal
	if len(mo.Families) == 0 && len(mo.Agents) == 0 {
		return nil
	}
	return &mo
}

// marshalMCPConfig serializes project MCP config to JSON for storage.
// Returns "" for nil config (no overrides).
func marshalMCPConfig(mc *domain.ProjectMCPConfig) string {
	if mc == nil {
		return ""
	}
	if len(mc.Services) == 0 {
		return ""
	}
	data, err := json.Marshal(mc)
	if err != nil {
		return ""
	}
	return string(data)
}

// unmarshalMCPConfig deserializes project MCP config from JSON storage.
// If mcpConfig is empty but the legacy mcp field has values, migrates them
// to a ProjectMCPConfig with services listed (no credential overrides).
func unmarshalMCPConfig(mcpConfig string, legacyMCP []string) *domain.ProjectMCPConfig {
	if mcpConfig != "" {
		var mc domain.ProjectMCPConfig
		if err := json.Unmarshal([]byte(mcpConfig), &mc); err != nil {
			return nil
		}
		if len(mc.Services) == 0 {
			return nil
		}
		return &mc
	}

	// Backward compat: migrate legacy mcp field (comma-separated names)
	if len(legacyMCP) > 0 {
		mc := &domain.ProjectMCPConfig{}
		for _, name := range legacyMCP {
			mc.Services = append(mc.Services, domain.ProjectMCPService{Name: name})
		}
		return mc
	}

	return nil
}

// marshalProviderConfig serializes project provider config to JSON for storage.
func marshalProviderConfig(pc *domain.ProjectProviderConfig) string {
	if pc == nil {
		return ""
	}
	if pc.AWSProfile == "" && pc.AWSRegion == "" && pc.AuthMode == "" && pc.TokenKey == "" {
		return ""
	}
	data, err := json.Marshal(pc)
	if err != nil {
		return ""
	}
	return string(data)
}

// unmarshalProviderConfig deserializes project provider config from JSON storage.
func unmarshalProviderConfig(s string) *domain.ProjectProviderConfig {
	if s == "" {
		return nil
	}
	var pc domain.ProjectProviderConfig
	if err := json.Unmarshal([]byte(s), &pc); err != nil {
		return nil
	}
	if pc.AWSProfile == "" && pc.AWSRegion == "" && pc.AuthMode == "" && pc.TokenKey == "" {
		return nil
	}
	return &pc
}
