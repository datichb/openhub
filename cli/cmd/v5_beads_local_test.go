package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// QB1: the bd of local sessions calls `oh __bd` with every argument, and is
// rewritten when the oh executable moved.
func TestEnsureBeadsShim(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell shim")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "oh it's")
	require.NoError(t, os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s|' \"$@\"\n"), 0o755))
	shims := filepath.Join(dir, "bin")
	require.NoError(t, ensureBeadsShim(shims, fake))

	out, err := exec.Command(filepath.Join(shims, "bd"), "close", "bd-1", "--reason", "a b").Output()
	require.NoError(t, err)
	assert.Equal(t, BeadsShimArg+"|close|bd-1|--reason|a b|", string(out))

	require.NoError(t, ensureBeadsShim(shims, "/elsewhere/oh"))
	data, err := os.ReadFile(filepath.Join(shims, "bd"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "exec /elsewhere/oh "+BeadsShimArg)
}

// QB1: the gateway runs the real bd, never the fake one of local sessions.
func TestRealBeadsBinarySkipsShim(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell shim")
	}
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	require.NoError(t, ensureBeadsShim(ohShimsDir(), "/usr/bin/true"))
	real := filepath.Join(t.TempDir(), "real")
	require.NoError(t, os.MkdirAll(real, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(real, "bd"), []byte("#!/bin/sh\n"), 0o755))
	t.Setenv("PATH", strings.Join([]string{ohShimsDir(), real}, string(os.PathListSeparator)))
	assert.Equal(t, filepath.Join(real, "bd"), realBeadsBinary())
	t.Setenv("PATH", ohShimsDir())
	assert.Empty(t, realBeadsBinary())
}
