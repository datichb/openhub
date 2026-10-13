package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNestedAgentsMD(t *testing.T) {
	dir := t.TempDir()
	for rel, content := range map[string]string{
		"AGENTS.md":              "root",
		"packages/web/AGENTS.md": "nested",
		"tools/AGENTS.md":        "untracked",
		"build/AGENTS.md":        "ignored",
		"docs/agents.md":         "other case",
		".gitignore":             "build/\n",
	} {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	git := exec.Command("git", "init", "-q", dir)
	require.NoError(t, git.Run())
	add := exec.Command("git", "-C", dir, "add", "AGENTS.md", "packages", ".gitignore")
	require.NoError(t, add.Run())

	assert.Equal(t, []string{"packages/web/AGENTS.md", "tools/AGENTS.md"}, nestedAgentsMD(context.Background(), dir))
	assert.Nil(t, nestedAgentsMD(context.Background(), t.TempDir()), "not a git work tree")
	assert.Nil(t, nestedAgentsMD(context.Background(), ""))
}

func TestGlobalAgentsMD(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	assert.Empty(t, globalAgentsMD())
	p := filepath.Join(cfg, "opencode", "AGENTS.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte("x"), 0o644))
	assert.Equal(t, p, globalAgentsMD())
}
