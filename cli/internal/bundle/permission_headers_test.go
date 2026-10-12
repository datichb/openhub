package bundle

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bricks"
)

// C4: every permission base (permissions/<base>.yaml, plugin-tools.yaml is
// not one) starts with `# Permission base: <base>` and has one
// `# Used by: a, b` line listing, sorted, the agents whose frontmatter sets
// `permission_base: <base>`.
func TestPermissionBaseHeaders(t *testing.T) {
	hub := repoHub(t)
	files, err := bricks.FindAgentFiles(hub)
	require.NoError(t, err)
	users := map[string][]string{}
	for _, id := range sortedKeys(files) {
		fm, err := bricks.ParseAgentFrontmatter(files[id])
		require.NoError(t, err)
		if fm.PermissionBase != "" {
			users[fm.PermissionBase] = append(users[fm.PermissionBase], id)
		}
	}
	entries, err := os.ReadDir(filepath.Join(hub, "permissions"))
	require.NoError(t, err)
	bases := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") || e.Name() == bricks.PluginToolsFile {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".yaml")
		bases[base] = true
		data, err := os.ReadFile(filepath.Join(hub, "permissions", e.Name()))
		require.NoError(t, err)
		lines := strings.Split(string(data), "\n")
		assert.Equal(t, "# Permission base: "+base, lines[0], e.Name())
		var usedBy []string
		for _, l := range lines {
			if !strings.HasPrefix(l, "#") {
				break // the header ends at the first key
			}
			assert.False(t, strings.Contains(l, "Used by") && !strings.HasPrefix(l, "# Used by: "), "%s: write `# Used by: a, b` on its own line: %q", e.Name(), l)
			if rest, ok := strings.CutPrefix(l, "# Used by: "); ok {
				require.Nil(t, usedBy, "%s: one `# Used by:` line", e.Name())
				usedBy = strings.Split(rest, ", ")
			}
		}
		want := users[base]
		sort.Strings(want)
		assert.Equal(t, want, usedBy, "%s: `# Used by:` lists the agents with permission_base: %s", e.Name(), base)
	}
	for base, ids := range users {
		assert.True(t, bases[base], "permission_base %s (%v) has no permissions/%s.yaml", base, ids, base)
	}
}
