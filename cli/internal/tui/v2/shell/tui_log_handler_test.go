package shell

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTUILogHandler(t *testing.T) {
	t.Run("enabled from the configured level", func(t *testing.T) {
		h := NewTUILogHandler(nil, nil, slog.LevelWarn)
		assert.False(t, h.Enabled(context.TODO(), slog.LevelDebug), "debug")
		assert.False(t, h.Enabled(context.TODO(), slog.LevelInfo), "info")
		assert.True(t, h.Enabled(context.TODO(), slog.LevelWarn), "warn")
		assert.True(t, h.Enabled(context.TODO(), slog.LevelError), "error")
	})

	t.Run("records land in the notification store", func(t *testing.T) {
		tests := []struct {
			name      string
			log       func(*slog.Logger)
			wantCount int
			wantParts []string
		}{
			{"warning is persisted", func(l *slog.Logger) { l.Warn("test warning") }, 1, []string{"test warning"}},
			{"below the level is skipped", func(l *slog.Logger) { l.Info("should be filtered") }, 0, nil},
			{"groups prefix the message", func(l *slog.Logger) { l.WithGroup("tracker").WithGroup("sync").Warn("pull failed") }, 1, []string{"tracker.sync: pull failed"}},
			{"attributes are appended", func(l *slog.Logger) { l.With("project", "api-backend").Warn("deploy failed") }, 1, []string{"deploy failed", "project=api-backend"}},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				store := NewNotificationStore(10)
				tc.log(slog.New(NewTUILogHandler(nil, store, slog.LevelWarn)))

				items := store.All()
				require.Len(t, items, tc.wantCount, "stored notifications")
				for _, part := range tc.wantParts {
					assert.Contains(t, items[0].Message, part, "message")
				}
				if tc.wantCount > 0 {
					assert.Equal(t, ToastWarning, items[0].Level, "level")
				}
			})
		}
	})

	t.Run("levels map to toast levels and durations", func(t *testing.T) {
		assert.Equal(t, ToastError, slogToToastLevel(slog.LevelError), "error")
		assert.Equal(t, ToastWarning, slogToToastLevel(slog.LevelWarn), "warn")
		assert.Equal(t, ToastInfo, slogToToastLevel(slog.LevelInfo), "info")
		assert.Equal(t, ToastInfo, slogToToastLevel(slog.LevelDebug), "debug")

		assert.Equal(t, ToastDurationError, ToastDurationForLevel(ToastError), "error duration")
		assert.Equal(t, ToastDurationWarning, ToastDurationForLevel(ToastWarning), "warning duration")
		assert.Equal(t, ToastDurationSuccess, ToastDurationForLevel(ToastSuccess), "success duration")
		assert.Equal(t, ToastDurationInfo, ToastDurationForLevel(ToastInfo), "info duration")
	})
}
