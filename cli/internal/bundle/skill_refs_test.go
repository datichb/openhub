package bundle

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The `skill` tool knows a skill by its name (the last component of its hub
// reference): `skill("planning/planner-phase-1")` fails with « Unable to load
// skill ». An instruction to load a skill names it without its category.
var (
	skillCallRe   = regexp.MustCompile(`skill\(\s*["']([^"')]+)["']\s*\)`)
	loadLineRe    = regexp.MustCompile(`(?i)\b(charg|load)`)
	categoryRefRe = regexp.MustCompile("`((?:shared|planning|quality|developer|designer|design|reviewer|auditor|orchestrator|posture|documentarian|adapters|workflow)/[a-z0-9-]+)`")
)

// C0.4
func TestSkillLoadInstructionsUseNames(t *testing.T) {
	hub := repoHub(t)
	for _, dir := range []string{"agents", "skills", "workflows"} {
		require.NoError(t, filepath.WalkDir(filepath.Join(hub, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !(strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".tmpl")) {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(hub, path)
			for i, line := range strings.Split(string(data), "\n") {
				for _, m := range skillCallRe.FindAllStringSubmatch(line, -1) {
					assert.NotContains(t, m[1], "/", "%s:%d: skill(%q): the tool takes the name, without category", rel, i+1, m[1])
				}
				if loadLineRe.MatchString(line) {
					for _, m := range categoryRefRe.FindAllStringSubmatch(line, -1) {
						assert.Failf(t, "load instruction with a category", "%s:%d: `%s`: name the skill `%s`", rel, i+1, m[1], skillID(m[1]))
					}
				}
			}
			return nil
		}))
	}
}
