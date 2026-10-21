package bundle

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// claimReadOnly is a body telling the agent it changes no file.
var claimReadOnly = regexp.MustCompile(`(?i)ne modifies? (?:jamais|aucun)|lecture seule|read-only|never modif`)

// C4: an agent whose body claims it never changes files is refused the
// edits and the shell commands that change files. The paths it may still
// write (its report, the wiki) are listed in testdata/capability_allow.yaml
// (claims: "<agent>: edit <pattern>"), with the reason.
func TestReadOnlyClaimsMatchPermissions(t *testing.T) {
	hub := repoHub(t)
	files, err := bricks.FindAgentFiles(hub)
	require.NoError(t, err)
	var allow capAllow
	data, err := os.ReadFile(filepath.Join("testdata", "capability_allow.yaml"))
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(data, &allow))
	allowed := map[string]bool{}
	for _, c := range allow.Claims {
		allowed[c] = true
	}
	used := map[string]bool{}
	var claimed []string
	for _, id := range sortedKeys(files) {
		src, err := os.ReadFile(files[id])
		require.NoError(t, err)
		_, body := bricks.SplitFrontmatter(src)
		if !claimReadOnly.Match(body) {
			continue
		}
		claimed = append(claimed, id)
		b := agentBuild(t, id, "bedrock")
		var own []sessionspec.PermissionRule
		for _, a := range b.Spec.Agents {
			if a.ID == id {
				own = a.Permissions
			}
		}
		rules := append(append([]sessionspec.PermissionRule(nil), own...), b.Spec.AgentGuard...)
		for _, p := range []string{"src/main.go", "README.md", "app/x.ts"} {
			assert.Equal(t, sessionspec.EffectDeny, ToolEffect(rules, sessionspec.ActionEdit, p), "%s claims read-only: edit %s", id, p)
		}
		for _, cmd := range []string{"rm x", "mv a b", "sed -i s/a/b/ x"} {
			assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rules, cmd), "%s claims read-only: %s", id, cmd)
		}
		// Every path its own rules let it write is a listed exception.
		for _, r := range own {
			if (r.Action == sessionspec.ActionEdit || r.Action == sessionspec.ActionAll) && r.Effect != sessionspec.EffectDeny {
				key := id + ": edit " + r.Resource
				if allowed[key] {
					used[key] = true
				} else {
					t.Errorf("%s claims read-only but may write %q (%s): fix the body or list %q in claims", id, r.Resource, r.Effect, key)
				}
			}
		}
	}
	sort.Strings(claimed)
	t.Logf("agents claiming read-only: %s", strings.Join(claimed, ", "))
	for _, c := range allow.Claims {
		assert.True(t, used[c], "stale claims entry of testdata/capability_allow.yaml: %s", c)
	}
}
