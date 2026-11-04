package bdargs

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ciBdVersion = regexp.MustCompile(`(?m)^\s*BD_VERSION:\s*"?([0-9]+\.[0-9]+\.[0-9]+)"?\s*$`)

// The bd the CI installs is the one the argument table was generated from:
// `oh doctor` warns on any other version, and the gateway refuses the options
// of another one. Moving one without the other is caught here.
func TestCIVersionIsGeneratedFor(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	require.NoError(t, err)

	found := ciBdVersion.FindAllSubmatch(data, -1)
	require.Len(t, found, 1, "BD_VERSION must be set once in .github/workflows/ci.yml")
	assert.Equal(t, VersionNumber(GeneratedFor), string(found[0][1]),
		"BD_VERSION of ci.yml differs from the version spec_bd.go was generated from (go run ./gen/main.go)")
}
