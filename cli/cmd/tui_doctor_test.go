package cmd

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/services/doctor"
)

// Doctor › Corriger on a clean-up check opens its section of the clean-up
// view: « Nettoyage (oh gc) » the data of oh, « Anciens déploiements » the
// former deployments.
func TestDoctorFixOpensTheCleanupSection(t *testing.T) {
	gcFix := runtimeFix(doctor.Result{ID: doctorIDGC, OK: true, Warn: true})
	require.NotNil(t, gcFix)
	assert.NotNil(t, gcFix.Open)
	assert.Nil(t, runtimeFix(doctor.Result{ID: doctorIDGC, OK: true}), "nothing to reclaim")
	deploy := runtimeFix(doctor.Result{ID: doctorIDDeployCleanup, OK: false, Detail: "1 projet"})
	require.NotNil(t, deploy)
	assert.NotNil(t, deploy.Open)
	assert.Nil(t, runtimeFix(doctor.Result{ID: doctorIDDeployCleanup, OK: true}))
	assert.Nil(t, runtimeFix(doctor.Result{ID: "runtime", OK: false}), "other results of the runtime check have no repair")
}

// The registry is valid (unique identifiers) and the checks that can be
// repaired carry their repair: the TUI needs no command of the CLI to run it.
func TestDoctorRegistryIDsAndFixes(t *testing.T) {
	reg, err := doctor.NewRegistry(doctorRegistry()...)
	require.NoError(t, err, "identifiers are unique and every check runs")
	for _, id := range []string{"os", "go", "git", "bd", "config", "setup", "tracker_tokens", "tracker_connection", "tracker_username", "tracker_identity", "database", "beads", "runtime"} {
		_, ok := reg.Get(id)
		assert.True(t, ok, id)
	}
	for _, id := range []string{"config", "beads", "runtime"} {
		c, _ := reg.Get(id)
		assert.NotNil(t, c.Fix, "%s can be repaired", id)
	}
	for _, id := range []string{"os", "go", "git", "bd"} {
		c, _ := reg.Get(id)
		assert.Nil(t, c.Fix, "%s has no repair", id)
	}
}

// The repairs write their report to the writer they are given: nothing
// needs a cobra command to run.
func TestDoctorRepairsRunWithoutACommand(t *testing.T) {
	origReport := startupReport
	t.Cleanup(func() { startupReport = origReport })
	startupReport = nil
	var out bytes.Buffer
	require.NoError(t, repairConfig(&out, alwaysConfirm), "a hub.toml that loads has nothing to restore")
	assert.Empty(t, out.String())

	fix := beadsFix(doctor.Result{OK: false})
	require.NotNil(t, fix)
	assert.NotNil(t, fix.Apply)
	assert.Nil(t, beadsFix(doctor.Result{OK: true}))
	report, err := doctorFixRun(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, "\x1b[1mdone\x1b[0m")
		return err
	})(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "done", report, "the report is shown in a window: no terminal styles")
}

// One command « Nettoyage » for both clean-ups.
func TestCleanupIsOneCommand(t *testing.T) {
	found := false
	for _, c := range buildCommands(&app.App{Config: &config.Config{}}) {
		assert.NotEqual(t, "gc", c.ID, "gc is an alias of cleanup")
		if c.ID == "cleanup" {
			found = true
			assert.Contains(t, c.Aliases, "gc")
			assert.Contains(t, c.Aliases, "deploy-cleanup")
		}
	}
	assert.True(t, found)
}
