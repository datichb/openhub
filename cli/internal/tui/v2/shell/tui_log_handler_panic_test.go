package shell

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/safego"
)

// A panic recovered by safego under the TUI ends as a toast (and a
// notification) that names the crash file, not the stack.
func TestTUILogHandlerShowsTheCrashFileOfARecoveredPanic(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	store := NewNotificationStore(10)
	prev := slog.Default()
	slog.SetDefault(slog.New(NewTUILogHandler(nil, store, slog.LevelWarn)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	safego.Go(func() { panic("goroutine of the view exploded") })

	require.Eventually(t, func() bool { return len(store.All()) == 1 }, 5*time.Second, 5*time.Millisecond)
	msg := store.All()[0].Message
	files, err := filepath.Glob(filepath.Join(home, "run", "crash-*.log"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Contains(t, msg, "recovered panic in goroutine")
	assert.Contains(t, msg, "goroutine of the view exploded")
	assert.Contains(t, msg, "crash_file="+files[0], "the toast names the crash file")
	assert.NotContains(t, msg, "[running]", "no stack in the toast")
	assert.NotContains(t, msg, "runtime/debug", "no stack in the toast")
	body, err := os.ReadFile(files[0])
	require.NoError(t, err)
	assert.Contains(t, string(body), "TestTUILogHandlerShowsTheCrashFileOfARecoveredPanic")
}
