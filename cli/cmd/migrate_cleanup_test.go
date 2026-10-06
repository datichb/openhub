package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
)

func TestMigrateCleanupScanAndApply(t *testing.T) {
	deployed := t.TempDir()
	for _, f := range []string{".opencode/agents/developer.md", ".opencode/team.json"} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(deployed, f)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(deployed, f), []byte("x"), 0o644))
	}
	cfg := "{\n  \"provider\": {\n    \"litellm\": {}\n  },\n  \"subagent_depth\": 3\n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(deployed, "opencode.json"), []byte(cfg), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(deployed, ".opencode", ".deploy-state"),
		[]byte(`{"provider": "bedrock", "config_snapshot": `+cfg+`}`), 0o644))
	clean := t.TempDir()

	a := &app.App{Config: &config.Config{}, Projects: &mockProjectStore{projects: []domain.Project{
		{ID: "old", Name: "old", Path: deployed, Status: domain.ProjectStatusActive},
		{ID: "new", Name: "new", Path: clean, Status: domain.ProjectStatusActive},
		{ID: "gone", Name: "gone", Path: filepath.Join(clean, "missing"), Status: domain.ProjectStatusActive},
	}}}
	list, err := scanDeployLeftovers(context.Background(), a, "")
	require.NoError(t, err)
	require.Len(t, list, 1, "only the project with leftovers")
	assert.Equal(t, "old", list[0].Name)
	assert.Equal(t, []string{"subagent_depth"}, list[0].Plan.Removed, "provider.litellm is not the deployed provider")

	var out bytes.Buffer
	printCleanupPlan(&out, list[0], true)
	assert.Contains(t, out.String(), ".opencode/agents")
	assert.Contains(t, out.String(), `-  "subagent_depth": 3`)

	require.NoError(t, applyCleanups(&out, list))
	assert.NoDirExists(t, filepath.Join(deployed, ".opencode"))
	data, err := os.ReadFile(filepath.Join(deployed, "opencode.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "litellm", "user key kept")
	assert.NotContains(t, string(data), "subagent_depth")

	list, err = scanDeployLeftovers(context.Background(), a, "")
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestOfferCleanupFor(t *testing.T) {
	assert.False(t, offerCleanupFor(rootCmd), "TUI shows its own screen")
	assert.True(t, offerCleanupFor(runCmd))
	assert.False(t, offerCleanupFor(migrateCleanupCmd))
}
