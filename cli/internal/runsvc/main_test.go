//go:build !container

package runsvc

import (
	"os"
	"testing"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/testenv"
)

// The tests commit in temporary repositories: no automatic git maintenance
// left running in their temporary directories (gittest.Config). They are
// also isolated from the real hub and home (testenv). With the container
// tag, TestMain is in service_gateway_container_test.go.
func TestMain(m *testing.M) {
	finish := testenv.Isolate()
	os.Exit(finish(gittest.Main(m)))
}
