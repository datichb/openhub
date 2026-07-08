package teamstate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	repo := setupTestRepo(t)

	content := `[notification]
mattermost_webhook = "https://mattermost.example.com/hooks/abc123"
channel = "dev-ai-sessions"
enabled = true
bot_name = "TeamHub"
`
	require.NoError(t, os.WriteFile(filepath.Join(repo.path, "config.toml"), []byte(content), 0o644))

	cfg, err := repo.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "https://mattermost.example.com/hooks/abc123", cfg.Notification.MattermostWebhook)
	assert.Equal(t, "dev-ai-sessions", cfg.Notification.Channel)
	assert.True(t, cfg.Notification.Enabled)
	assert.Equal(t, "TeamHub", cfg.Notification.BotName)
}

func TestLoadConfigDefaults(t *testing.T) {
	repo := setupTestRepo(t)
	// No config.toml — should return defaults

	cfg, err := repo.LoadConfig()
	require.NoError(t, err)
	assert.False(t, cfg.Notification.Enabled)
	assert.Equal(t, "OpenHub", cfg.Notification.BotName)
	assert.Empty(t, cfg.Notification.MattermostWebhook)
}

func TestLoadConfigEmptyBotName(t *testing.T) {
	repo := setupTestRepo(t)

	content := `[notification]
mattermost_webhook = "https://example.com/hooks/xyz"
channel = "general"
enabled = true
`
	require.NoError(t, os.WriteFile(filepath.Join(repo.path, "config.toml"), []byte(content), 0o644))

	cfg, err := repo.LoadConfig()
	require.NoError(t, err)
	// Default bot name should be applied
	assert.Equal(t, "OpenHub", cfg.Notification.BotName)
}

func TestLoadConfigInvalid(t *testing.T) {
	repo := setupTestRepo(t)

	content := `this is {{{{ invalid toml`
	require.NoError(t, os.WriteFile(filepath.Join(repo.path, "config.toml"), []byte(content), 0o644))

	_, err := repo.LoadConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing config.toml")
}

func TestSaveConfig(t *testing.T) {
	repo := setupTestRepo(t)

	cfg := &TeamConfig{
		Notification: NotificationConfig{
			MattermostWebhook: "https://example.com/hooks/test",
			Channel:           "dev-team",
			Enabled:           true,
			BotName:           "MyBot",
		},
	}

	err := repo.SaveConfig(cfg)
	require.NoError(t, err)

	// Read back
	loaded, err := repo.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, cfg.Notification.MattermostWebhook, loaded.Notification.MattermostWebhook)
	assert.Equal(t, cfg.Notification.Channel, loaded.Notification.Channel)
	assert.Equal(t, cfg.Notification.Enabled, loaded.Notification.Enabled)
	assert.Equal(t, cfg.Notification.BotName, loaded.Notification.BotName)
}
