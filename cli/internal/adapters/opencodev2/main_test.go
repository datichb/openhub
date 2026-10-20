//go:build !integration || windows

package opencodev2

import (
	"os"
	"testing"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/testenv"
)

// The tests are isolated from the real hub and home (testenv) and commit in
// temporary repositories: no automatic git maintenance left running in their
// temporary directories (gittest.Config). With the integration tag, TestMain
// is in testmain_integration_test.go.
func TestMain(m *testing.M) {
	finish := testenv.Isolate()
	os.Exit(finish(gittest.Main(m)))
}
