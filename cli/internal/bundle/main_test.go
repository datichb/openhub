package bundle

import (
	"fmt"
	"os"
	"testing"

	"github.com/datichb/openhub/cli/internal/testenv"
)

// The tests are isolated from the real hub and home (testenv). The bundles of
// the shipped workflows are built once and shared (shared_test.go).
func TestMain(m *testing.M) {
	finish := testenv.Isolate()
	if err := startShared(); err != nil {
		fmt.Fprintln(os.Stderr, "bundle tests: cannot create the folder of the shared bundles:", err)
		os.Exit(finish(1))
	}
	os.Exit(finish(finishShared(m.Run())))
}
