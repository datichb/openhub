package config

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
)

// MigrateTeamToTeams performs the automatic migration from the legacy [team]
// section to the [[teams]] array. It modifies the Config in-place and returns
// true if a migration occurred (caller should persist to disk).
//
// Migration rules:
//   - If Teams is already populated → no migration needed (return false)
//   - If Team is zero-value (no state_repo) → no migration needed (return false)
//   - Otherwise: derive an ID from the StateRepo, create a Teams[0] entry,
//     and clear the legacy Team field
func MigrateTeamToTeams(c *Config) bool {
	if len(c.Teams) > 0 {
		return false // already migrated
	}
	if c.Team.StateRepo == "" {
		return false // no team configured
	}

	// Derive an ID from the repo URL
	id := repoNameFromRemote(c.Team.StateRepo)

	// Build the new entry
	entry := TeamConfig{
		ID:        id,
		Name:      "", // will use ID as display name
		Enabled:   c.Team.Enabled,
		StateRepo: c.Team.StateRepo,
		StatePath: c.Team.StatePath,
		MemberID:  c.Team.MemberID,
	}

	// If StatePath is the legacy default, update to the new convention
	if entry.StatePath == DefaultTeamStatePath() || entry.StatePath == "" {
		entry.StatePath = filepath.Join(HubDir(), "team-states", id)
	}

	c.Teams = []TeamConfig{entry}

	// Clear legacy field (it will no longer be written to hub.toml)
	c.Team = TeamConfig{}

	return true
}

// BackupHubToml creates a timestamped backup of the current hub.toml.
// Returns the backup path or an error.
func BackupHubToml() (string, error) {
	src := ConfigPath()
	if _, err := os.Stat(src); err != nil {
		return "", fmt.Errorf("hub.toml not found: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(filepath.Dir(src), fmt.Sprintf("hub.toml.%s.bak", timestamp))

	srcFile, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("opening hub.toml: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(backupPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("creating backup: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return "", fmt.Errorf("copying hub.toml to backup: %w", err)
	}

	return backupPath, nil
}

// RunMigrationIfNeeded checks the loaded config and performs the [team] → [[teams]]
// migration if applicable. It creates a backup of hub.toml before writing.
// Returns (migrated bool, backupPath string, error).
func RunMigrationIfNeeded(c *Config) (bool, string, error) {
	if !MigrateTeamToTeams(c) {
		return false, "", nil
	}

	// Create backup before persisting
	backupPath, err := BackupHubToml()
	if err != nil {
		// Non-fatal: proceed without backup if hub.toml doesn't exist yet
		backupPath = ""
	}

	if err := Save(c); err != nil {
		return false, backupPath, fmt.Errorf("saving migrated config: %w", err)
	}

	return true, backupPath, nil
}

// SecretMigrator is the interface needed for migrating keychain keys.
type SecretMigrator interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
}

// legacyTokenMapping maps old keychain key names to new ones (MCP services).
var legacyTokenMapping = map[string]string{
	"gitlab-token":  DefaultGitLabTokenKey,
	"jira-token":    DefaultJiraTokenKey,
	"figma-token":   DefaultFigmaTokenKey,
	"gslides-token": DefaultGslidesTokenKey,
}

// legacyProviderMapping maps old provider keychain key names to new convention.
var legacyProviderMapping = map[string]string{
	"bedrock-token-default":      "openhub.provider.bedrock.token",
	"anthropic-api-key-default":  "openhub.provider.anthropic.token",
	"openrouter-api-key-default": "openhub.provider.openrouter.token",
}

// MigrateTokenKeys renames legacy keychain keys (e.g. "gitlab-token") to the
// new convention ("openhub.mcp.gitlab.token") and updates the hub.toml token_key fields.
// Returns true if any migration occurred.
func MigrateTokenKeys(c *Config) bool {
	migrated := false

	// Check each MCP service for legacy key names
	type svcRef struct {
		token   *string
		name    string
	}
	services := []svcRef{
		{&c.MCP.Gitlab.Token, "gitlab"},
		{&c.MCP.Jira.Token, "jira"},
		{&c.MCP.Figma.Token, "figma"},
		{&c.MCP.Gslides.Token, "gslides"},
	}

	for _, svc := range services {
		oldKey := *svc.token
		if oldKey == "" {
			continue
		}
		newKey, isLegacy := legacyTokenMapping[oldKey]
		if !isLegacy {
			continue // already using new convention or custom name
		}
		// Update the config to use the new key name
		*svc.token = newKey
		migrated = true
	}

	return migrated
}

// MigrateKeychainKeys renames legacy keychain entries to the new naming convention.
// For each legacy key that has a stored value and whose new-name counterpart is empty,
// it copies the value to the new key and deletes the old one.
// This completes the migration started by MigrateTokenKeys (which only updates config references).
// Returns the number of keys migrated.
func MigrateKeychainKeys(store SecretMigrator) int {
	if store == nil {
		return 0
	}
	ctx := context.Background()
	count := 0

	// Merge both mappings (MCP services + providers)
	allMappings := make(map[string]string, len(legacyTokenMapping)+len(legacyProviderMapping))
	for old, new := range legacyTokenMapping {
		allMappings[old] = new
	}
	for old, new := range legacyProviderMapping {
		allMappings[old] = new
	}

	for oldKey, newKey := range allMappings {
		// Check if old key has a value
		oldVal, err := store.Get(ctx, oldKey)
		if err != nil || oldVal == "" {
			continue // nothing stored under old name
		}

		// Check if new key already has a value (don't overwrite)
		newVal, _ := store.Get(ctx, newKey)
		if newVal != "" {
			// New key already populated — just clean up old one
			_ = store.Delete(ctx, oldKey)
			count++
			slog.Debug("keychain migration: deleted legacy key (new key already set)", "old", oldKey, "new", newKey)
			continue
		}

		// Migrate: copy to new key, delete old key
		if err := store.Set(ctx, newKey, oldVal); err != nil {
			slog.Warn("keychain migration: failed to set new key", "key", newKey, "error", err)
			continue
		}
		if err := store.Delete(ctx, oldKey); err != nil {
			slog.Warn("keychain migration: failed to delete old key", "key", oldKey, "error", err)
		}
		count++
		slog.Debug("keychain migration: renamed", "old", oldKey, "new", newKey)
	}

	return count
}

// MigrateProjectTeamToTeamID migrates projects from the legacy ProjectTeamConfig
// (team_config JSON column) to the new TeamID column. For each project that has
// a team_config but no team_id, it parses the JSON and sets team_id based on the mode:
//   - "" or "inherit" → team_id = activeTeamID
//   - "disabled"      → team_id stays NULL
//   - "custom"        → team_id = activeTeamID (simplified — single-team setup)
//
// Returns the number of projects migrated.
func MigrateProjectTeamToTeamID(db *sql.DB, activeTeamID string) (int, error) {
	if activeTeamID == "" {
		return 0, nil // no team configured at hub level — nothing to migrate
	}

	rows, err := db.Query(`SELECT id, team_config FROM projects WHERE team_id IS NULL AND team_config != ''`)
	if err != nil {
		return 0, fmt.Errorf("querying projects for team migration: %w", err)
	}
	defer rows.Close()

	type candidate struct {
		id         string
		teamConfig string
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.teamConfig); err != nil {
			return 0, fmt.Errorf("scanning project for team migration: %w", err)
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	count := 0
	for _, c := range candidates {
		var tc domain.ProjectTeamConfig
		if err := json.Unmarshal([]byte(c.teamConfig), &tc); err != nil {
			slog.Warn("team migration: invalid team_config JSON, skipping", "project", c.id, "error", err)
			continue
		}

		switch tc.Mode {
		case domain.ProjectTeamModeDisabled:
			// disabled → team_id stays NULL, nothing to do
			continue
		case "", domain.ProjectTeamModeInherit, domain.ProjectTeamModeCustom:
			// inherit or custom → attach to the active team
			_, err := db.Exec(`UPDATE projects SET team_id = ? WHERE id = ?`, activeTeamID, c.id)
			if err != nil {
				return count, fmt.Errorf("migrating project %s team_id: %w", c.id, err)
			}
			count++
			slog.Info("team migration: set team_id", "project", c.id, "team_id", activeTeamID, "old_mode", tc.Mode)
		default:
			slog.Warn("team migration: unknown mode, skipping", "project", c.id, "mode", tc.Mode)
		}
	}

	return count, nil
}
