package docstest

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fakeChangelog = `# Changelog

## [Unreleased]

- rien

---

## [5.0.0] — 2026-11-01

### Added

- gros changement

### Fixed

- un bug

---

## [5.0.0-rc.1] — 2026-10-25

- bêta

## [4.2.0] — 2026-10-02

## [4.1.0] — 2026-09-01

- ancien
`

// releaseNotes runs .github/scripts/release-notes.sh on changelog.
func releaseNotes(t *testing.T, tag, changelog string) (string, string, error) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("bash script")
	}
	root := repoRoot(t)
	script := filepath.Join(root, ".github", "scripts", "release-notes.sh")
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}
	var stdout, stderr strings.Builder
	cmd := exec.Command("bash", script, tag, changelog)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func writeChangelog(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "CHANGELOG.md")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}

func TestReleaseNotes_Section(t *testing.T) {
	path := writeChangelog(t, fakeChangelog)

	out, _, err := releaseNotes(t, "v5.0.0", path)
	require.NoError(t, err)
	assert.Equal(t, "### Added\n\n- gros changement\n\n### Fixed\n\n- un bug\n", out,
		"heading, trailing separator and neighbours left out")

	out, _, err = releaseNotes(t, "v5.0.0-rc.1", path)
	require.NoError(t, err)
	assert.Equal(t, "- bêta\n", out, "a pre-release reads its own section first")

	out, _, err = releaseNotes(t, "v5.0.0-rc.2", path)
	require.NoError(t, err)
	assert.Contains(t, out, "gros changement", "else the section of the version it prepares")

	out, _, err = releaseNotes(t, "v4.1.0", path)
	require.NoError(t, err)
	assert.Equal(t, "- ancien\n", out, "last section of the file")
}

func TestReleaseNotes_MissingOrEmptyFails(t *testing.T) {
	path := writeChangelog(t, fakeChangelog)
	for _, tag := range []string{"v9.9.9", "v4.2.0", "v5.0", "v6.0.0-rc.1"} {
		out, stderr, err := releaseNotes(t, tag, path)
		require.Error(t, err, tag)
		assert.Empty(t, out, tag)
		assert.Contains(t, stderr, "no non-empty section", tag)
	}
}

// The real CHANGELOG has sections the script can read: the most recent
// released version (first numbered heading) gives non-empty notes.
func TestReleaseNotes_RealChangelog(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "CHANGELOG.md")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	m := regexp.MustCompile(`(?m)^## \[(\d+\.\d+\.\d+[^\]]*)\]`).FindSubmatch(data)
	require.NotNil(t, m, "no released version in CHANGELOG.md")

	out, stderr, err := releaseNotes(t, "v"+string(m[1]), path)
	require.NoError(t, err, stderr)
	assert.NotEmpty(t, strings.TrimSpace(out))
	assert.NotContains(t, out, "\n## [")
}
