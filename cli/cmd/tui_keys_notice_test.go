package cmd

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

func keysNoticeApp(t *testing.T) *app.App {
	t.Helper()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	prefs := sqlite.NewPreferenceStore(st)
	return (&app.App{}).WithPreferences(prefs, prefs)
}

// The notice of the new keys is shown once, after the cleanup offer; a
// first run records it without showing it.
func TestKeysNoticeDue(t *testing.T) {
	ctx := context.Background()
	assert.False(t, keysNoticeDue(ctx, nil, false))
	assert.False(t, keysNoticeDue(ctx, &app.App{}, false), "no preferences: never")

	a := keysNoticeApp(t)
	assert.False(t, keysNoticeDue(ctx, a, false), "the cleanup screen comes first")
	markCleanupOffered(ctx, a)
	assert.True(t, keysNoticeDue(ctx, a, false))
	assert.True(t, keysNoticeDue(ctx, a, false), "not seen until shown")
	markKeysNoticeSeen(ctx, a)
	assert.False(t, keysNoticeDue(ctx, a, false), "once")

	b := keysNoticeApp(t)
	assert.False(t, keysNoticeDue(ctx, b, true), "first run: not shown")
	markCleanupOffered(ctx, b)
	assert.False(t, keysNoticeDue(ctx, b, false), "first run: recorded")
}
