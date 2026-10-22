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
	cmd.Env = append(os.Environ(), "GITHUB_REPOSITORY=datichb/openhub")
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

const linksChangelog = "# Changelog\n\n## [5.0.0] — 2026-11-01\n\n" +
	"- relatif : [guide](MIGRATION.md), [réf](docs/reference/cli.fr.md#sortie), [pointé](./SECURITY.md), [racine](/docs/a.md)\n" +
	"- ancre : [limites](#ruptures)\n" +
	"- absolus : [web](https://example.com/a.md), [http](http://example.com), [mail](mailto:a@b.c), [proto](//cdn.example.com/x), [auto](<https://example.com>)\n" +
	"- image : ![schéma](docs/img/a.svg) et [![badge](docs/img/b.svg)](docs/guide.md)\n" +
	"- deux sur une ligne : [a](A.md) puis [b](docs/B.md \"titre\")\n" +
	"- code : `[x](CODE.md)` mais [y](OUT.md)\n" +
	"- sans lien : [crochets] (parenthèses) et ](\n" +
	"\n```\n[bloc](BLOC.md)\n```\n\n" +
	"[ref]: docs/ref.md\n[web]: https://example.com/ref\n"

// The notes are read on the release page: relative links become absolute
// URLs at the tag; absolute ones, code and anchors-only are handled apart.
func TestReleaseNotes_RelativeLinksBecomeAbsolute(t *testing.T) {
	path := writeChangelog(t, linksChangelog)
	const blob = "https://github.com/datichb/openhub/blob/v5.0.0/"
	const raw = "https://github.com/datichb/openhub/raw/v5.0.0/"

	out, stderr, err := releaseNotes(t, "v5.0.0", path)
	require.NoError(t, err, stderr)

	for _, want := range []string{
		"[guide](" + blob + "MIGRATION.md)",
		"[réf](" + blob + "docs/reference/cli.fr.md#sortie)",
		"[pointé](" + blob + "SECURITY.md)",
		"[racine](" + blob + "docs/a.md)",
		"[limites](" + blob + "CHANGELOG.md#ruptures)",
		"[web](https://example.com/a.md), [http](http://example.com), [mail](mailto:a@b.c), [proto](//cdn.example.com/x), [auto](<https://example.com>)",
		"![schéma](" + raw + "docs/img/a.svg)",
		"[![badge](" + raw + "docs/img/b.svg)](" + blob + "docs/guide.md)",
		"[a](" + blob + "A.md) puis [b](" + blob + "docs/B.md \"titre\")",
		"`[x](CODE.md)` mais [y](" + blob + "OUT.md)",
		"[crochets] (parenthèses) et ](\n",
		"[bloc](BLOC.md)",
		"[ref]: " + blob + "docs/ref.md",
		"[web]: https://example.com/ref",
	} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "](MIGRATION.md)")
}

// An absolute link is left byte for byte, and the repository follows
// GITHUB_REPOSITORY (a fork publishes its own notes).
func TestReleaseNotes_RepositoryFollowsTheEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash script")
	}
	root := repoRoot(t)
	script := filepath.Join(root, ".github", "scripts", "release-notes.sh")
	path := writeChangelog(t, linksChangelog)
	var stdout strings.Builder
	cmd := exec.Command("bash", script, "v5.0.0-rc.1", path)
	cmd.Env = append(os.Environ(), "GITHUB_REPOSITORY=someone/fork")
	cmd.Stdout = &stdout
	require.NoError(t, cmd.Run())
	assert.Contains(t, stdout.String(), "[guide](https://github.com/someone/fork/blob/v5.0.0-rc.1/MIGRATION.md)")
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
	assert.Contains(t, out, "https://github.com/datichb/openhub/blob/v"+string(m[1])+"/")

	// No link left that the release page could not follow: every target is
	// absolute, or a code example.
	noCode := regexp.MustCompile("(?s)```.*?```|`[^`\n]*`").ReplaceAllString(out, "")
	for _, l := range regexp.MustCompile(`\]\(([^)\s]*)`).FindAllStringSubmatch(noCode, -1) {
		assert.Regexp(t, `^(<?[A-Za-z][A-Za-z0-9+.-]*:|//)`, l[1], "relative link left in the notes")
	}
}
