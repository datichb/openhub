package bundle

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bricks"
)

// handoffHeading matches a handoff heading, at the start of a line or quoted
// in a code span: `## Retour vers X`, `## Retour intermédiaire vers X`,
// `## Question pour (l')X`.
var handoffHeading = regexp.MustCompile("(?:^\\s*|`)#+ *(?:Retour (?:intermédiaire )?vers|Question pour) +(?:l'|l’|le )?(<[^<>`\n]+>|[A-Za-z0-9…-]+)")

// handoffAllow are files whose handoff headings name no agent of the hub, on
// purpose or until the lightening phase reworks their handoffs.
var handoffAllow = map[string]string{}

// C4: a handoff heading names its recipient by its agent id (or the
// placeholder `<appelant>`): the coordinator looks for that exact heading.
func TestHandoffHeadingsNameAgents(t *testing.T) {
	hub := repoHub(t)
	files, err := bricks.FindAgentFiles(hub)
	require.NoError(t, err)
	known := map[string]bool{"<appelant>": true, "appelant": true, "…": true}
	for id := range files {
		known[id] = true
	}
	used := map[string]bool{}
	for _, dir := range []string{"agents", "skills", "workflows"} {
		require.NoError(t, filepath.WalkDir(filepath.Join(hub, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || (!strings.HasSuffix(path, ".md") && !strings.HasSuffix(path, ".tmpl")) {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(hub, path)
			for i, line := range strings.Split(string(data), "\n") {
				for _, m := range handoffHeading.FindAllStringSubmatch(line, -1) {
					if known[m[1]] {
						continue
					}
					if _, ok := handoffAllow[filepath.ToSlash(rel)]; ok && strings.HasPrefix(rel, "skills") {
						used[filepath.ToSlash(rel)] = true
						continue
					}
					t.Errorf("%s:%d: handoff heading for %q: name an agent of the hub or <appelant>", rel, i+1, m[1])
				}
			}
			return nil
		}))
	}
	for f := range handoffAllow {
		assert.True(t, used[f], "handoffAllow: %s names only known recipients now, remove it", f)
	}
}

func TestHandoffHeadingPattern(t *testing.T) {
	var got []string
	for _, l := range []string{"## Retour vers orchestrator-dev", "## Question pour l'orchestrator", "le bloc `## Retour vers <appelant>`", "### Retour intermédiaire vers Planner", "Retour vers x"} {
		for _, m := range handoffHeading.FindAllStringSubmatch(l, -1) {
			got = append(got, m[1])
		}
	}
	assert.Equal(t, []string{"orchestrator-dev", "orchestrator", "<appelant>", "Planner"}, got)
}
