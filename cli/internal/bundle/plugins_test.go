package bundle

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

func TestNpmPackageName(t *testing.T) {
	for spec, want := range map[string]string{
		"context-mode":        "context-mode",
		"context-mode@latest": "context-mode",
		"@scope/pkg":          "@scope/pkg",
		"@scope/pkg@^1.2.0":   "@scope/pkg",
	} {
		assert.Equal(t, want, npmPackageName(spec), spec)
	}
}

// P1-T15: `plugins:` and `code_mode:` of the workflow reach the bundle.
func TestBuildAppliesWorkflowPluginsAndCodeMode(t *testing.T) {
	src := `apiVersion: oh/v1
kind: Workflow
id: quick
risk: write
code_mode: true
entry: { agent: developer }
agents:
  developer: { role: workflow, mode: primary }
plugins:
  - context-mode@latest
  - { id: "@acme/probe", options: { verbose: true } }
`
	b, err := Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), Spec: parseSpec(t, src)})
	require.NoError(t, err)
	assert.True(t, b.Spec.CodeMode)
	assert.Equal(t, []sessionspec.PluginDef{
		{ID: "context-mode", Dir: "context-mode@latest"},
		{ID: "@acme/probe", Dir: "@acme/probe", Options: map[string]any{"verbose": true}},
	}, b.Spec.Plugins)

	// Without plugins and code_mode: none shipped, code mode off, other hash.
	bare := strings.Replace(src[:strings.Index(src, "plugins:")], "code_mode: true\n", "", 1)
	off, err := Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), Spec: parseSpec(t, bare)})
	require.NoError(t, err)
	assert.False(t, off.Spec.CodeMode)
	assert.Empty(t, off.Spec.Plugins)
	assert.NotEqual(t, b.Spec.Hash, off.Spec.Hash)

	// The caller's plugin list wins over the workflow's.
	own := []sessionspec.PluginDef{{ID: "x", Dir: "x"}}
	c, err := Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), Spec: parseSpec(t, src), Plugins: own})
	require.NoError(t, err)
	assert.Equal(t, own, c.Spec.Plugins)
}
