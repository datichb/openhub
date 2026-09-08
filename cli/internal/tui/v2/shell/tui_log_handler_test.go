package shell

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTUILogHandler_Enabled(t *testing.T) {
	h := NewTUILogHandler(nil, nil, slog.LevelWarn)

	assert.False(t, h.Enabled(nil, slog.LevelDebug))
	assert.False(t, h.Enabled(nil, slog.LevelInfo))
	assert.True(t, h.Enabled(nil, slog.LevelWarn))
	assert.True(t, h.Enabled(nil, slog.LevelError))
}

func TestTUILogHandler_Handle_PersistsToStore(t *testing.T) {
	store := NewNotificationStore(10)
	h := NewTUILogHandler(nil, store, slog.LevelWarn)

	logger := slog.New(h)
	logger.Warn("test warning")

	items := store.All()
	assert.Equal(t, 1, len(items))
	assert.Equal(t, ToastWarning, items[0].Level)
	assert.Contains(t, items[0].Message, "test warning")
}

func TestTUILogHandler_Handle_SkipsBelowLevel(t *testing.T) {
	store := NewNotificationStore(10)
	h := NewTUILogHandler(nil, store, slog.LevelWarn)

	logger := slog.New(h)
	logger.Info("should be filtered")

	items := store.All()
	assert.Equal(t, 0, len(items))
}

func TestTUILogHandler_WithGroup(t *testing.T) {
	store := NewNotificationStore(10)
	h := NewTUILogHandler(nil, store, slog.LevelWarn)

	logger := slog.New(h).WithGroup("tracker").WithGroup("sync")
	logger.Warn("pull failed")

	items := store.All()
	assert.Equal(t, 1, len(items))
	assert.Contains(t, items[0].Message, "tracker.sync: pull failed")
}

func TestTUILogHandler_WithAttrs(t *testing.T) {
	store := NewNotificationStore(10)
	h := NewTUILogHandler(nil, store, slog.LevelWarn)

	logger := slog.New(h).With("project", "api-backend")
	logger.Warn("deploy failed")

	items := store.All()
	assert.Equal(t, 1, len(items))
	assert.Contains(t, items[0].Message, "deploy failed")
	assert.Contains(t, items[0].Message, "project=api-backend")
}

func TestSlogToToastLevel(t *testing.T) {
	assert.Equal(t, ToastError, slogToToastLevel(slog.LevelError))
	assert.Equal(t, ToastWarning, slogToToastLevel(slog.LevelWarn))
	assert.Equal(t, ToastInfo, slogToToastLevel(slog.LevelInfo))
	assert.Equal(t, ToastInfo, slogToToastLevel(slog.LevelDebug))
}

func TestToastDurationForLevel(t *testing.T) {
	assert.Equal(t, ToastDurationError, ToastDurationForLevel(ToastError))
	assert.Equal(t, ToastDurationWarning, ToastDurationForLevel(ToastWarning))
	assert.Equal(t, ToastDurationSuccess, ToastDurationForLevel(ToastSuccess))
	assert.Equal(t, ToastDurationInfo, ToastDurationForLevel(ToastInfo))
}
