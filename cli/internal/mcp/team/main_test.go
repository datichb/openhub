package team

import (
	"os"
	"testing"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

// The tests commit and push to bare repositories: no automatic git
// maintenance left running in their temporary directories (gittest.Config).
func TestMain(m *testing.M) { os.Exit(gittest.Main(m)) }
