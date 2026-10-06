package workflow

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const editDoc = `apiVersion: oh/v1
kind: Workflow
id: ticket-hotfix
# Patch of the hub ticket.
extends: hub:ticket
risk: write # kept
checkpoints:
  cp-2: { label: Revue, mandatory: true }
`

func TestDocEdit(t *testing.T) {
	e, err := ParseDocEdit([]byte(editDoc))
	require.NoError(t, err)
	v, ok := e.Scalar("risk")
	assert.True(t, ok)
	assert.Equal(t, "write", v)
	assert.True(t, e.Has("checkpoints", "cp-2", "mandatory"))
	assert.Equal(t, []string{"cp-2"}, e.Keys("checkpoints"))

	require.NoError(t, e.Set("read", "risk"))
	require.NoError(t, e.Set("auto", "checkpoints", "cp-2", "mode", "auto"))
	require.NoError(t, e.Set([]string{"local", "container"}, "runtime", "allowed"))
	require.NoError(t, e.Set("developer", "entry", "agent"))
	e.Unset("checkpoints", "cp-2", "mandatory")
	e.Unset("nope", "x")
	out, err := e.Bytes()
	require.NoError(t, err)
	s := string(out)
	assert.Contains(t, s, "# Patch of the hub ticket.")
	assert.Contains(t, s, "risk: read # kept")
	assert.Contains(t, s, "cp-2: {label: Revue, mode: {auto: auto}}", "flow mapping kept in flow style")
	assert.Contains(t, s, "entry:\n  agent: developer")
	assert.Contains(t, s, "- container")

	doc, diags := Parse(out, Source{Layer: LayerTeam})
	require.False(t, diags.HasErrors(), "%v", diags)
	assert.False(t, doc.Has("checkpoints.cp-2.mandatory"))
	assert.Equal(t, RiskRead, doc.Spec.Risk)

	// Removing the last key of a mapping removes the mapping.
	e.Unset("entry", "agent")
	assert.False(t, e.Has("entry"))
	require.NoError(t, e.Rename("cp-hotfix", "checkpoints", "cp-2"))
	assert.Equal(t, []string{"cp-hotfix"}, e.Keys("checkpoints"))
	assert.Error(t, e.Rename("cp-hotfix", "checkpoints", "nope"))

	empty, err := ParseDocEdit(nil)
	require.NoError(t, err)
	require.NoError(t, empty.Set("x", "id"))
	out, _ = empty.Bytes()
	assert.Equal(t, "id: x\n", string(out))
	_, err = ParseDocEdit([]byte("- a\n"))
	assert.Error(t, err)
	strs, ok := e.Strings("runtime", "allowed")
	assert.True(t, ok)
	assert.Equal(t, "local,container", strings.Join(strs, ","))
}
