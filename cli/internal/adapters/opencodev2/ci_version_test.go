package opencodev2

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ciOpencodeVersion = regexp.MustCompile(`(?m)^\s*OPENCODE_VERSION:\s*"?([0-9]+\.[0-9]+\.[0-9]+)"?\s*$`)

// The opencode the CI installs (and tests oh against) is the one oh recommends
// and pins: moving one without the others is caught here (audit v5, S-38, N-1).
func TestCIVersionIsRecommended(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	require.NoError(t, err)

	found := ciOpencodeVersion.FindAllSubmatch(data, -1)
	require.Len(t, found, 1, "OPENCODE_VERSION must be set once in .github/workflows/ci.yml")
	assert.Equal(t, RecommendedVersion(), string(found[0][1]),
		"OPENCODE_VERSION of ci.yml differs from opencode_recommended in compatibility.json")
}
