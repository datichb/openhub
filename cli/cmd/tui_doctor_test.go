package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Doctor › Corriger on a clean-up check opens its section of the clean-up
// view: « Nettoyage (oh gc) » the data of oh, « Anciens déploiements » the
// former deployments.
func TestDoctorFixOpensTheCleanupSection(t *testing.T) {
	gcFix := doctorFixFor(views.DoctorCheck{Name: i18n.T("cmd.gc.doctor.name"), OK: true, Warn: true})
	require.NotNil(t, gcFix)
	assert.NotNil(t, gcFix.Open)
	deploy := doctorFixFor(views.DoctorCheck{Name: i18n.T("cmd.migrate.cleanup.doctor_name"), OK: false, Detail: "1 projet"})
	require.NotNil(t, deploy)
	assert.NotNil(t, deploy.Open)
	assert.Nil(t, doctorFixFor(views.DoctorCheck{Name: i18n.T("cmd.migrate.cleanup.doctor_name"), OK: true}))
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
