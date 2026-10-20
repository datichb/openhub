package runner

import (
	"os"
	"testing"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/testenv"
)

// The tests commit and push to bare repositories: no automatic git
// maintenance left running in their temporary directories (gittest.Config).
// They are also isolated from the real hub and home (testenv).
func TestMain(m *testing.M) {
	finish := testenv.Isolate()
	os.Exit(finish(gittest.Main(m)))
}
