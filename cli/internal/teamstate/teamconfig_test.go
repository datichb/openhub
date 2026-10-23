package teamstate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	repo := setupTestRepo(t)
	// No config.toml — should return defaults

	cfg, err := repo.LoadConfig()
	require.NoError(t, err)
	assert.False(t, cfg.Notification.Enabled)
	assert.Equal(t, "OpenHub", cfg.Notification.BotName)
	assert.Empty(t, cfg.Notification.MattermostWebhook)
}

func TestLoadConfigEmptyBotName(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	repo := setupTestRepo(t)

	content := `this is {{{{ invalid toml`
	require.NoError(t, os.WriteFile(filepath.Join(repo.path, "config.toml"), []byte(content), 0o644))

	_, err := repo.LoadConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing config.toml")
}

func TestSaveConfig(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)

	cfg := &TeamConfig{
		Notification: NotificationConfig{
			MattermostWebhook: "https://example.com/hooks/test",
			Channel:           "dev-team",
			Enabled:           true,
			BotName:           "MyBot",
		},
	}

	err := repo.SaveConfig(context.Background(), cfg)
	require.NoError(t, err)

	// Read back
	loaded, err := repo.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, cfg.Notification.MattermostWebhook, loaded.Notification.MattermostWebhook)
	assert.Equal(t, cfg.Notification.Channel, loaded.Notification.Channel)
	assert.Equal(t, cfg.Notification.Enabled, loaded.Notification.Enabled)
	assert.Equal(t, cfg.Notification.BotName, loaded.Notification.BotName)
}

// ─────────────────────────────────────────────────────────────────────────────
// BoardConfig method tests
// ─────────────────────────────────────────────────────────────────────────────

func TestBoardConfig_InitialStatus(t *testing.T) {
	t.Parallel()
	t.Run("custom columns with initial role", func(t *testing.T) {
		cfg := BoardConfig{
			Columns: []BoardColumnConfig{
				{ID: "backlog", Name: "BACKLOG", Role: ColumnRoleActive},
				{ID: "todo", Name: "TODO", Role: ColumnRoleInitial},
				{ID: "done", Name: "DONE", Role: ColumnRoleTerminal},
			},
		}
		assert.Equal(t, "todo", cfg.InitialStatus())
	})

	t.Run("no custom columns returns planned", func(t *testing.T) {
		cfg := BoardConfig{}
		assert.Equal(t, ClaimStatusPlanned, cfg.InitialStatus())
	})

	t.Run("no initial role returns first column", func(t *testing.T) {
		cfg := BoardConfig{
			Columns: []BoardColumnConfig{
				{ID: "backlog", Name: "BACKLOG", Role: ColumnRoleActive},
				{ID: "wip", Name: "WIP", Role: ColumnRoleActive},
				{ID: "done", Name: "DONE", Role: ColumnRoleTerminal},
			},
		}
		assert.Equal(t, "backlog", cfg.InitialStatus())
	})
}

func TestBoardConfig_DefaultWorkStatus(t *testing.T) {
	t.Parallel()
	t.Run("custom columns with active role", func(t *testing.T) {
		cfg := BoardConfig{
			Columns: []BoardColumnConfig{
				{ID: "todo", Name: "TODO", Role: ColumnRoleInitial},
				{ID: "dev", Name: "DEV", Role: ColumnRoleActive},
				{ID: "testing", Name: "TESTING", Role: ColumnRoleActive},
				{ID: "done", Name: "DONE", Role: ColumnRoleTerminal},
			},
		}
		assert.Equal(t, "dev", cfg.DefaultWorkStatus())
	})

	t.Run("no custom columns returns in_progress", func(t *testing.T) {
		cfg := BoardConfig{}
		assert.Equal(t, ClaimStatusInProgress, cfg.DefaultWorkStatus())
	})

	t.Run("no active role returns second column", func(t *testing.T) {
		cfg := BoardConfig{
			Columns: []BoardColumnConfig{
				{ID: "todo", Name: "TODO", Role: ColumnRoleInitial},
				{ID: "staging", Name: "STAGING", Role: ColumnRoleBlocked},
				{ID: "done", Name: "DONE", Role: ColumnRoleTerminal},
			},
		}
		assert.Equal(t, "staging", cfg.DefaultWorkStatus())
	})
}

func TestBoardConfig_TerminalStatuses(t *testing.T) {
	t.Parallel()
	t.Run("custom columns with terminal roles", func(t *testing.T) {
		cfg := BoardConfig{
			Columns: []BoardColumnConfig{
				{ID: "todo", Name: "TODO", Role: ColumnRoleInitial},
				{ID: "dev", Name: "DEV", Role: ColumnRoleActive},
				{ID: "done", Name: "DONE", Role: ColumnRoleTerminal},
				{ID: "archived", Name: "ARCHIVED", Role: ColumnRoleTerminal},
			},
		}
		assert.Equal(t, []string{"done", "archived"}, cfg.TerminalStatuses())
	})

	t.Run("no custom columns returns done", func(t *testing.T) {
		cfg := BoardConfig{}
		assert.Equal(t, []string{ClaimStatusDone}, cfg.TerminalStatuses())
	})
}

func TestBoardConfig_ActiveStatuses(t *testing.T) {
	t.Parallel()
	t.Run("custom columns with active roles", func(t *testing.T) {
		cfg := BoardConfig{
			Columns: []BoardColumnConfig{
				{ID: "todo", Name: "TODO", Role: ColumnRoleInitial},
				{ID: "dev", Name: "DEV", Role: ColumnRoleActive},
				{ID: "review", Name: "REVIEW", Role: ColumnRoleActive},
				{ID: "done", Name: "DONE", Role: ColumnRoleTerminal},
			},
		}
		assert.Equal(t, []string{"dev", "review"}, cfg.ActiveStatuses())
	})

	t.Run("no custom columns returns default active", func(t *testing.T) {
		cfg := BoardConfig{}
		assert.Equal(t, []string{ClaimStatusInProgress, ClaimStatusReview}, cfg.ActiveStatuses())
	})
}

func TestBoardConfig_IsValidStatus(t *testing.T) {
	t.Parallel()
	cfg := BoardConfig{
		Columns: []BoardColumnConfig{
			{ID: "todo", Name: "TODO", Role: ColumnRoleInitial},
			{ID: "dev", Name: "DEV", Role: ColumnRoleActive},
			{ID: "testing", Name: "TESTING", Role: ColumnRoleActive},
			{ID: "done", Name: "DONE", Role: ColumnRoleTerminal},
		},
	}

	t.Run("custom status accepted", func(t *testing.T) {
		assert.True(t, cfg.IsValidStatus("testing"))
	})

	t.Run("built-in status accepted when in columns", func(t *testing.T) {
		assert.True(t, cfg.IsValidStatus("done"))
	})

	t.Run("planned always accepted (legacy compat)", func(t *testing.T) {
		assert.True(t, cfg.IsValidStatus(ClaimStatusPlanned))
	})

	t.Run("unknown rejected", func(t *testing.T) {
		assert.False(t, cfg.IsValidStatus("nonexistent"))
	})
}

func TestBoardConfig_Validate(t *testing.T) {
	t.Parallel()
	t.Run("valid config", func(t *testing.T) {
		cfg := BoardConfig{
			Columns: []BoardColumnConfig{
				{ID: "todo", Name: "TODO", Role: ColumnRoleInitial},
				{ID: "dev", Name: "DEV", Role: ColumnRoleActive},
				{ID: "done", Name: "DONE", Role: ColumnRoleTerminal},
			},
		}
		assert.NoError(t, cfg.Validate())
	})

	t.Run("empty columns valid (defaults used)", func(t *testing.T) {
		cfg := BoardConfig{}
		assert.NoError(t, cfg.Validate())
	})

	t.Run("less than 2 columns", func(t *testing.T) {
		cfg := BoardConfig{
			Columns: []BoardColumnConfig{
				{ID: "todo", Name: "TODO", Role: ColumnRoleTerminal},
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least 2 columns required")
	})

	t.Run("duplicate IDs", func(t *testing.T) {
		cfg := BoardConfig{
			Columns: []BoardColumnConfig{
				{ID: "todo", Name: "TODO", Role: ColumnRoleInitial},
				{ID: "todo", Name: "TODO 2", Role: ColumnRoleTerminal},
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate column ID")
	})

	t.Run("no terminal column", func(t *testing.T) {
		cfg := BoardConfig{
			Columns: []BoardColumnConfig{
				{ID: "todo", Name: "TODO", Role: ColumnRoleInitial},
				{ID: "dev", Name: "DEV", Role: ColumnRoleActive},
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least 1 column must have role")
	})

	t.Run("uppercase ID rejected", func(t *testing.T) {
		cfg := BoardConfig{
			Columns: []BoardColumnConfig{
				{ID: "Todo", Name: "TODO", Role: ColumnRoleInitial},
				{ID: "done", Name: "DONE", Role: ColumnRoleTerminal},
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be lowercase")
	})

	t.Run("empty name rejected", func(t *testing.T) {
		cfg := BoardConfig{
			Columns: []BoardColumnConfig{
				{ID: "todo", Name: "", Role: ColumnRoleInitial},
				{ID: "done", Name: "DONE", Role: ColumnRoleTerminal},
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must have a display name")
	})

	t.Run("multiple initial columns rejected", func(t *testing.T) {
		cfg := BoardConfig{
			Columns: []BoardColumnConfig{
				{ID: "todo", Name: "TODO", Role: ColumnRoleInitial},
				{ID: "backlog", Name: "BACKLOG", Role: ColumnRoleInitial},
				{ID: "done", Name: "DONE", Role: ColumnRoleTerminal},
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at most 1 column can have role")
	})

	t.Run("unknown role rejected", func(t *testing.T) {
		cfg := BoardConfig{
			Columns: []BoardColumnConfig{
				{ID: "todo", Name: "TODO", Role: "weird"},
				{ID: "done", Name: "DONE", Role: ColumnRoleTerminal},
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown role")
	})
}

func TestBoardConfig_HasCustomColumns(t *testing.T) {
	t.Parallel()
	t.Run("empty returns false", func(t *testing.T) {
		cfg := BoardConfig{}
		assert.False(t, cfg.HasCustomColumns())
	})

	t.Run("with columns returns true", func(t *testing.T) {
		cfg := BoardConfig{
			Columns: []BoardColumnConfig{
				{ID: "todo", Name: "TODO", Role: ColumnRoleInitial},
				{ID: "done", Name: "DONE", Role: ColumnRoleTerminal},
			},
		}
		assert.True(t, cfg.HasCustomColumns())
	})
}

func TestDefaultBoardConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultBoardConfig()

	require.Len(t, cfg.Columns, 6)

	expectedIDs := []string{"todo", "in_progress", "review", "validation", "done", "blocked"}
	expectedRoles := []string{ColumnRoleInitial, ColumnRoleActive, ColumnRoleActive, ColumnRoleActive, ColumnRoleTerminal, ColumnRoleBlocked}

	for i, col := range cfg.Columns {
		assert.Equal(t, expectedIDs[i], col.ID, "column %d ID", i)
		assert.Equal(t, expectedRoles[i], col.Role, "column %d role", i)
		assert.NotEmpty(t, col.Name, "column %d should have a display name", i)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// ClaimConfig lease settings
// ─────────────────────────────────────────────────────────────────────────────

func TestClaimConfigDefaults(t *testing.T) {
	t.Parallel()
	repo := setupTestRepo(t)

	t.Run("no config.toml", func(t *testing.T) {
		cfg, err := repo.LoadConfig()
		require.NoError(t, err)
		assert.Equal(t, 24, cfg.Claim.LeaseHours)
		assert.Equal(t, 60, cfg.Claim.HeartbeatMinutes)
	})

	load := func(t *testing.T, content string) ClaimConfig {
		t.Helper()
		r := setupTestRepo(t)
		require.NoError(t, os.WriteFile(filepath.Join(r.path, "config.toml"), []byte(content), 0o644))
		cfg, err := r.LoadConfig()
		require.NoError(t, err)
		return cfg.Claim
	}

	t.Run("claim table without the new keys", func(t *testing.T) {
		got := load(t, "[claim]\ndone_retention_days = 10\n")
		assert.Equal(t, 10, got.DoneRetentionDays)
		assert.Equal(t, 24, got.LeaseHours)
		assert.Equal(t, 60, got.HeartbeatMinutes)
	})
	t.Run("explicit values", func(t *testing.T) {
		got := load(t, "[claim]\nlease_hours = 48\nheartbeat_minutes = 30\n")
		assert.Equal(t, 48, got.LeaseHours)
		assert.Equal(t, 30, got.HeartbeatMinutes)
	})
	t.Run("a heartbeat of 0 disables it", func(t *testing.T) {
		got := load(t, "[claim]\nheartbeat_minutes = 0\n")
		assert.Equal(t, 0, got.HeartbeatMinutes)
		assert.Equal(t, time.Duration(0), got.HeartbeatInterval())
	})
	t.Run("out of bounds values are brought back", func(t *testing.T) {
		assert.Equal(t, MaxLeaseHours, load(t, "[claim]\nlease_hours = 100000\n").LeaseHours)
		assert.Equal(t, 24, load(t, "[claim]\nlease_hours = -3\n").LeaseHours)
		assert.Equal(t, MinHeartbeatMinutes, load(t, "[claim]\nheartbeat_minutes = 1\n").HeartbeatMinutes)
		assert.Equal(t, MaxHeartbeatMinutes, load(t, "[claim]\nlease_hours = 168\nheartbeat_minutes = 99999\n").HeartbeatMinutes)
		assert.Equal(t, 0, load(t, "[claim]\nheartbeat_minutes = -5\n").HeartbeatMinutes)
	})
	t.Run("the heartbeat is at most half the lease", func(t *testing.T) {
		got := load(t, "[claim]\nlease_hours = 2\nheartbeat_minutes = 90\n")
		assert.Equal(t, 60, got.HeartbeatMinutes)
		got = load(t, "[claim]\nlease_hours = 1\n") // default heartbeat of 60 minutes
		assert.Equal(t, 30, got.HeartbeatMinutes)
	})
}

func TestClaimConfigDurations(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 24*time.Hour, ClaimConfig{}.LeaseDuration(), "unset: default")
	assert.Equal(t, 6*time.Hour, ClaimConfig{LeaseHours: 6}.LeaseDuration())
	assert.Equal(t, 45*time.Minute, ClaimConfig{HeartbeatMinutes: 45}.HeartbeatInterval())
}

func TestClaimConfigValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cfg     ClaimConfig
		wantErr string
	}{
		{"zero value", ClaimConfig{}, ""},
		{"defaults", ClaimConfig{LeaseHours: 24, HeartbeatMinutes: 60}, ""},
		{"heartbeat disabled", ClaimConfig{LeaseHours: 24}, ""},
		{"lease min", ClaimConfig{LeaseHours: 1, HeartbeatMinutes: 30}, ""},
		{"lease max", ClaimConfig{LeaseHours: 168, HeartbeatMinutes: 1440}, ""},
		{"lease negative", ClaimConfig{LeaseHours: -1}, "lease_hours"},
		{"lease too long", ClaimConfig{LeaseHours: 169}, "lease_hours"},
		{"heartbeat too short", ClaimConfig{LeaseHours: 24, HeartbeatMinutes: 4}, "heartbeat_minutes"},
		{"heartbeat negative", ClaimConfig{HeartbeatMinutes: -1}, "heartbeat_minutes"},
		{"heartbeat too long", ClaimConfig{LeaseHours: 168, HeartbeatMinutes: 1441}, "heartbeat_minutes"},
		{"heartbeat over half the lease", ClaimConfig{LeaseHours: 2, HeartbeatMinutes: 61}, "half the lease"},
		{"heartbeat over half the default lease", ClaimConfig{HeartbeatMinutes: 1000}, "half the lease"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestSaveConfigRefusesInvalidClaimSettings(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	err := repo.SaveConfig(context.Background(), &TeamConfig{Claim: ClaimConfig{LeaseHours: 9999}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lease_hours")
	assert.NoFileExists(t, filepath.Join(repo.path, "config.toml"))

	ok := &TeamConfig{Claim: ClaimConfig{LeaseHours: 12, HeartbeatMinutes: 20}}
	require.NoError(t, repo.SaveConfig(context.Background(), ok))
	loaded, err := repo.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, ok.Claim.LeaseHours, loaded.Claim.LeaseHours)
	assert.Equal(t, ok.Claim.HeartbeatMinutes, loaded.Claim.HeartbeatMinutes)
}

func TestSavedHeartbeatOfZeroStaysDisabled(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	require.NoError(t, repo.SaveConfig(context.Background(), &TeamConfig{Claim: ClaimConfig{LeaseHours: 24}}))
	loaded, err := repo.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, 0, loaded.Claim.HeartbeatMinutes)
}
