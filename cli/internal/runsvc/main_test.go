//go:build !container

package runsvc

import (
	"os"
	"testing"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

// The tests commit in temporary repositories: no automatic git maintenance
// left running in their temporary directories (gittest.Config). With the
// container tag, TestMain is in service_gateway_container_test.go.
func TestMain(m *testing.M) { os.Exit(gittest.Main(m)) }
