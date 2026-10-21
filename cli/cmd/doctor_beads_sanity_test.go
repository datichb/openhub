package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/domain"
)

func sanityRepo(t *testing.T, files map[string]string, commit bool) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := c.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	git("init", "-q", "-b", "main")
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	if commit {
		git("add", ".")
		git("commit", "-qm", "share beads")
	}
	return dir
}

// Q31: a .beads/ excluded without its lock fails the Beads zero-impact
// check, oh doctor --fix (FixImpact) repairs it; a shared .beads/ without
// the lock in the team's .gitignore is a warning, never repaired.
func TestBeadsSanityGateLock(t *testing.T) {
	t.Parallel()
	local := sanityRepo(t, map[string]string{".beads/config.yaml": "", ".git/info/exclude": ".beads/\n"}, false)
	res := beadsSanityResults([]domain.Project{{Name: "api", Path: local}})
	require.Len(t, res, 1)
	assert.False(t, res[0].OK)
	assert.Contains(t, res[0].Detail, "/.beads.gate.lock")

	_, err := beads.FixImpact(local, false, false)
	require.NoError(t, err)
	res = beadsSanityResults([]domain.Project{{Name: "api", Path: local}})
	require.Len(t, res, 1)
	assert.True(t, res[0].OK, res[0].Detail)
	assert.False(t, res[0].Warn)

	shared := sanityRepo(t, map[string]string{".beads/issues.jsonl": "{}\n", ".gitignore": "node_modules/\n"}, true)
	res = beadsSanityResults([]domain.Project{{Name: "api", Path: local}, {Name: "team", Path: shared}})
	require.Len(t, res, 2)
	assert.True(t, res[0].OK, res[0].Detail)
	assert.True(t, res[1].OK)
	assert.True(t, res[1].Warn)
	assert.Contains(t, res[1].Detail, "team")
	assert.Contains(t, res[1].Detail, "/.beads.gate.lock")

	r, err := beads.FixImpact(shared, false, false)
	require.NoError(t, err)
	assert.False(t, r.Changed(), "shared .beads/: nothing written")
	got, _ := os.ReadFile(filepath.Join(shared, ".gitignore"))
	assert.Equal(t, "node_modules/\n", string(got))

	require.NoError(t, os.WriteFile(filepath.Join(shared, ".gitignore"), []byte("node_modules/\n/.beads.gate.lock\n"), 0o644))
	res = beadsSanityResults([]domain.Project{{Name: "team", Path: shared}})
	require.Len(t, res, 1)
	assert.False(t, res[0].Warn)
}
