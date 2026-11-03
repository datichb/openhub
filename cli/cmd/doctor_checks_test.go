package cmd

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The TUI Doctor view runs the same registry as `oh doctor`.
func TestDoctorViewUsesDoctorRegistry(t *testing.T) {
	if views.DoctorChecks == nil {
		t.Fatal("views.DoctorChecks not set")
	}
	assert.Equal(t, reflect.ValueOf(collectDoctorChecks).Pointer(), reflect.ValueOf(views.DoctorChecks).Pointer())
	i18ntest.Set(t, "en")
	var names []string
	for _, c := range doctorRegistry() {
		names = append(names, c.Name)
	}
	for _, want := range []string{"oh version", "Provider credentials", "API keys (keychain)", "Beads zero-impact (hooks and gitignore)", "v5 runtime"} {
		assert.Contains(t, names, want)
	}
}
