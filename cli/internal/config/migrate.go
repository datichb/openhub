package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
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
