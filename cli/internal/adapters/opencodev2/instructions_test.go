package opencodev2

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

func TestProjectAmbientInstructions(t *testing.T) {
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
	gittest.Run(t, dir, "init", "-q")
	gittest.Run(t, dir, "add", "AGENTS.md", "packages", ".gitignore")

	a := &Adapter{}
	ctx := context.Background()
	assert.Equal(t, []string{"packages/web/AGENTS.md", "tools/AGENTS.md"}, a.ProjectAmbientInstructions(ctx, dir))
	assert.Nil(t, a.ProjectAmbientInstructions(ctx, t.TempDir()), "not a git work tree")
	assert.Nil(t, a.ProjectAmbientInstructions(ctx, ""))
}

func TestUserAmbientInstructions(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	a := &Adapter{}
	assert.Empty(t, a.UserAmbientInstructions())
	p := filepath.Join(cfg, "opencode", "AGENTS.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte("x"), 0o644))
	assert.Equal(t, []string{p}, a.UserAmbientInstructions())
}
