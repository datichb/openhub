package bundle

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Piste T, T5: the shipped agents, skills and prompts only set the built-in
// statuses of Beads (bd 1.3: `bd update -s review|cancelled` is refused
// without a project configuration; review and cancelled are labels).
func TestShippedContentUsesBuiltinBeadsStatuses(t *testing.T) {
	builtin := map[string]bool{"open": true, "in_progress": true, "blocked": true, "deferred": true, "closed": true, "pinned": true, "hooked": true}
	re := regexp.MustCompile(`bd update [^\n|]*?(?:-s|--status)[ =]([A-Za-z_<>$-]+)`)
	hub := repoHub(t)
	for _, dir := range []string{"agents", "skills", "workflows"} {
		require.NoError(t, filepath.WalkDir(filepath.Join(hub, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			for _, m := range re.FindAllStringSubmatch(string(data), -1) {
				st := m[1]
				if strings.ContainsAny(st, "<$") {
					continue // placeholder
				}
				assert.True(t, builtin[st], "%s: `%s` (status %q is not a Beads status)", strings.TrimPrefix(p, hub+"/"), m[0], st)
			}
			return nil
		}))
	}
}
