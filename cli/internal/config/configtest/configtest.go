// Package configtest isolates the tests from the hub of the user.
package configtest

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/config"
)

// Home points HOME (and USERPROFILE, read on Windows) at a new temporary directory and returns it; the hub is
// then <home>/.oh (OH_HOME, which would relocate it, is cleared for the
// test). The config cache and the notices (former keys already announced)
// are reset now and when the test ends, so a Load reads this hub and the
// next test does not see its configuration.
//
//	home := configtest.Home(t)
//	require.NoError(t, config.Save(cfg)) // writes <home>/.oh/hub.toml
func Home(t testing.TB) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir on Windows
	t.Setenv("OH_HOME", "")
	reset := func() {
		config.Reset()
		config.ResetNotices()
	}
	reset()
	t.Cleanup(reset)
	return home
}
