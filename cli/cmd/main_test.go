package cmd_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/testenv"
)

// binarySetup is set by integration_test.go (build tag integration): it builds
// the oh binary of the integration tests and returns the function that removes
// it. Without the tag it is nil and the binary is not built.
var binarySetup func() (cleanup func(), err error)

// TestMain is the one of the whole test binary of ./cmd (the tests of the
// package cmd as well as the ones of cmd_test): every test is isolated from the
// real hub and home, and runs without background git maintenance.
func TestMain(m *testing.M) {
	// os.Exit skips the deferred calls: the code is returned once the
	// temporary folders are removed (runMain).
	os.Exit(runMain(m))
}

func runMain(m *testing.M) int {
	// The real hub and home are out of reach of the tests (the Go caches of
	// the user stay where they are, testenv pins them: the build below).
	finish := testenv.Isolate()

	if binarySetup != nil {
		cleanup, err := binarySetup()
		if err != nil {
			panic(err)
		}
		defer cleanup()
	}

	// No background git maintenance (a commit, a fetch or a push starts a
	// detached `git maintenance run --auto` that keeps writing to
	// .git/objects while t.TempDir is removed: flaky TestMigrateLegacyWorkflows).
	return finish(gittest.Main(m))
}

// Git runs no background maintenance in the tests (temp dir cleanup).
func TestGitMaintenanceIsOffInTests(t *testing.T) {
	t.Parallel()
	out, err := exec.Command("git", "config", "--get", "maintenance.auto").Output()
	require.NoError(t, err, "reading maintenance.auto")
	assert.Equal(t, "false", strings.TrimSpace(string(out)), "maintenance.auto")
}
