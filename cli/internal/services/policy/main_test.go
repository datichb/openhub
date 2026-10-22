package policy

import (
	"os"
	"testing"

	"github.com/datichb/openhub/cli/internal/testenv"
)

// The tests are isolated from the real hub and home, and commit with the git
// configuration of the tests (testenv): a CI runner has no git identity.
func TestMain(m *testing.M) {
	finish := testenv.Isolate()
	os.Exit(finish(m.Run()))
}
