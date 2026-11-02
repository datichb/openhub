package opencodev2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// The limits of [gc] log_max_mb and log_keep reach the start of the server:
// the log of the previous server of the data directory is rotated at the
// configured size, not at the default one.
func TestAdapterStartRotatesTheServerLogAtTheConfiguredLimit(t *testing.T) {
	start := func(t *testing.T, a *Adapter) string {
		t.Helper()
		bin, _ := dumpEnvServer(t)
		a.Binary, a.CacheDir, a.DisablePlugin = bin, t.TempDir(), true
		root := t.TempDir()
		data := filepath.Join(root, "data")
		require.NoError(t, os.MkdirAll(data, 0o700))
		logPath := filepath.Join(data, "oh-server.log")
		require.NoError(t, os.WriteFile(logPath, []byte(strings.Repeat("x", 2048)), 0o600))
		_, err := a.StartServer(t.Context(), adapters.ServerGroup{
			Key:     sessionspec.GroupKey{BundleHash: "h", ProjectID: "p1"},
			Bundle:  sessionspec.BundleSpec{Root: root, EntryAgent: "lead", Agents: []sessionspec.AgentDef{{ID: "lead", Mode: "primary"}}},
			DataDir: data, WorkDir: root,
			Provider: sessionspec.ProviderSpec{ID: "amazon-bedrock", Region: "eu-west-1"},
		})
		require.Error(t, err, "the fake server exits")
		return logPath
	}

	t.Run("configured", func(t *testing.T) {
		logPath := start(t, &Adapter{LogLimits: func() (int64, int) { return 1024, 2 }})
		assert.FileExists(t, logPath+".1", "the 2 KB log is above the 1 KB limit")
	})
	t.Run("default", func(t *testing.T) {
		logPath := start(t, &Adapter{})
		assert.NoFileExists(t, logPath+".1", "a 2 KB log is far under the default limit")
	})
}
