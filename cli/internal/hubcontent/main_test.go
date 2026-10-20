package hubcontent

import (
	"os"
	"testing"

	"github.com/datichb/openhub/cli/internal/testenv"
)

// The tests are isolated from the real hub and home (testenv).
func TestMain(m *testing.M) {
	finish := testenv.Isolate()
	os.Exit(finish(m.Run()))
}
