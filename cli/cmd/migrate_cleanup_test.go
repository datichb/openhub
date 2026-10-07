package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
)

// fakeCleaner is a tool adapter that knows a former deployment: a
// « legacy » folder and a « tool.json » file with an « old » key.
type fakeCleaner struct{ adapters.ToolAdapter }

func (fakeCleaner) ScanLegacy(dir string, _ adapters.LegacyOptions) (*adapters.LegacyPlan, error) {
	p := &adapters.LegacyPlan{Dir: dir, ConfigFile: "tool.json"}
	if _, err := os.Stat(filepath.Join(dir, "legacy")); err == nil {
		p.Items = []adapters.LegacyItem{{Rel: "legacy", Dir: true, Count: 1}}
		p.Removed = []string{"old"}
		p.DiffFunc = func() string { return "-  \"old\": 3" }
		p.ApplyFunc = func() error { return os.RemoveAll(filepath.Join(dir, "legacy")) }
	}
	return p, nil
}

// The cleanup of former deployments goes through the adapter (D19): the
// file formats are tested in its own package.
func TestMigrateCleanupScanAndApply(t *testing.T) {
	prev := v5Adapter
	v5Adapter = fakeCleaner{}
	t.Cleanup(func() { v5Adapter = prev })
	deployed := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(deployed, "legacy"), 0o755))
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

	var out bytes.Buffer
	printCleanupPlan(&out, list[0], true)
	assert.Contains(t, out.String(), "legacy")
	assert.Contains(t, out.String(), "tool.json")
	assert.Contains(t, out.String(), `-  "old": 3`)

	require.NoError(t, applyCleanups(&out, list))
	assert.NoDirExists(t, filepath.Join(deployed, "legacy"))
	list, err = scanDeployLeftovers(context.Background(), a, "")
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestOfferCleanupFor(t *testing.T) {
	assert.False(t, offerCleanupFor(rootCmd), "TUI shows its own screen")
	assert.True(t, offerCleanupFor(runCmd))
	assert.False(t, offerCleanupFor(migrateCleanupCmd))
}
