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

// The `skill` tool knows a skill by its name (the last component of its hub
// reference): `skill("planning/planner-phase-1")` fails with « Unable to load
// skill ». An instruction to load a skill names it without its category.
var (
	skillCallRe   = regexp.MustCompile(`skill\(\s*["']([^"')]+)["']\s*\)`)
	loadLineRe    = regexp.MustCompile(`(?i)\b(charg|load)`)
	categoryRefRe = regexp.MustCompile("`((?:shared|planning|quality|developer|designer|design|reviewer|auditor|orchestrator|posture|documentarian|adapters|workflow)/[a-z0-9-]+)`")
)

// C0.5: an unbalanced code fence turns the rest of a skill into code for the
// model, and flips the capability lint for everything read after it.
func TestHubMarkdownFencesBalanced(t *testing.T) {
	hub := repoHub(t)
	for _, dir := range []string{"agents", "skills", "workflows"} {
		require.NoError(t, filepath.WalkDir(filepath.Join(hub, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || (!strings.HasSuffix(path, ".md") && !strings.HasSuffix(path, ".tmpl")) {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			n := 0
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "```") {
					n++
				}
			}
			rel, _ := filepath.Rel(hub, path)
			assert.Zero(t, n%2, "%s: unbalanced code fence (%d fence lines)", rel, n)
			return nil
		}))
	}
}

// C0.6: agent names written in the content are agents of the hub (or skill
// and permission base names that share their prefix): a routing value such
// as `developer-backend` names an agent that no longer exists.
func TestNoStaleAgentNames(t *testing.T) {
	hub := repoHub(t)
	known := map[string]bool{}
	agents, err := bricks.FindAgentFiles(hub)
	require.NoError(t, err)
	for id := range agents {
		known[id] = true
	}
	for _, dir := range []string{"skills", "permissions"} {
		require.NoError(t, filepath.WalkDir(filepath.Join(hub, dir), func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				known[strings.TrimSuffix(strings.TrimSuffix(d.Name(), ".md"), ".yaml")] = true
			}
			return err
		}))
	}
	nameRe := regexp.MustCompile(`\b(?:developer|auditor|ux|ui|qa)-[a-z0-9<*]+(?:-[a-z0-9<*]+)*`)
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
				for _, m := range nameRe.FindAllString(line, -1) {
					assert.True(t, known[m], "%s:%d: %q is neither an agent, a skill nor a permission base of the hub", rel, i+1, m)
				}
			}
			return nil
		}))
	}
}

// C0.4
func TestSkillLoadInstructionsUseNames(t *testing.T) {
	hub := repoHub(t)
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
