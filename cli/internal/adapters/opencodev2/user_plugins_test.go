package opencodev2

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// S-11: the plugins of the user's own tool configuration are listed without a
// server (oh doctor).
func TestUserPlugins(t *testing.T) {
	write := func(t *testing.T, root, rel, content string) {
		t.Helper()
		p := filepath.Join(root, "opencode", rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	a := New("", t.TempDir())

	t.Run("no configuration", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		got, err := a.UserPlugins()
		require.NoError(t, err)
		assert.Empty(t, got)
	})
	t.Run("config file and plugin directories", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", root)
		write(t, root, "opencode.json", `{"model":"a/b","plugin":["some-pkg@1.2.0",["tuple-pkg",{"k":1}]]}`)
		write(t, root, "plugin/local.ts", "export default {}")
		write(t, root, "plugins/other.js", "")
		write(t, root, "plugin/.hidden", "")
		got, err := a.UserPlugins()
		require.NoError(t, err)
		assert.Equal(t, []string{"plugin/local.ts", "plugins/other.js", "some-pkg@1.2.0", "tuple-pkg"}, got)
	})
	t.Run("jsonc with comments and trailing commas", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", root)
		write(t, root, "opencode.jsonc", "{\n // a comment\n \"plugin\": [\n  \"a-pkg\", /* inline */\n  \"b//pkg\",\n ],\n}\n")
		got, err := a.UserPlugins()
		require.NoError(t, err)
		assert.Equal(t, []string{"a-pkg", "b//pkg"}, got)
	})
	t.Run("unreadable config: nothing listed", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", root)
		write(t, root, "opencode.json", `{not json`)
		got, err := a.UserPlugins()
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}
