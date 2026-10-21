package bdschema

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/beads/bdtest"
)

func TestEmbeddedHasWhatOhReads(t *testing.T) {
	s := Embedded()
	assert.Equal(t, 1, s.Version)
	assert.Empty(t, Missing(s))
	for _, f := range ViewFields {
		_, inRecord := s.Types["issue"].Properties[f]
		assert.False(t, inRecord, "%s is a record field now: remove it from ViewFields", f)
	}
}

func TestMissing(t *testing.T) {
	s, err := Parse([]byte(`{"schema_version":2,"types":{"issue":{"properties":{"id":{"type":"string"},"priority":{"type":"boolean"}}}}}`))
	require.NoError(t, err)
	got := Missing(s)
	assert.Contains(t, got, "issue.title")
	assert.Contains(t, got, "issue.priority (boolean)")
	assert.Contains(t, got, "dependency.depends_on_id")
	assert.NotContains(t, got, "issue.id")

	_, err = Parse([]byte(`{}`))
	assert.Error(t, err)
}

// Contract with the installed bd: its record schema still has every field
// oh reads (a different schema only needs schema_bd.json regenerated).
//
// bd schema answers at once (no bd init, no Dolt): the test stays in -short.
func TestInstalledBdSchema(t *testing.T) {
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd is not installed")
	}
	cmd := exec.Command("bd", "schema")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "BEADS_DIR=")
	out, err := cmd.Output()
	require.NoError(t, err)
	live, err := Parse(out)
	require.NoError(t, err)
	assert.Empty(t, Missing(live), "bd output no longer has fields oh reads")
	if string(out) != string(embedded) {
		t.Log("the installed bd has another record schema: regenerate schema_bd.json (bd schema > internal/beads/bdschema/schema_bd.json)")
	}
}

// Contract with the installed bd: `bd show --json` adds the view fields oh
// reads (parent, revision, dependency_type of the dependencies).
func TestInstalledBdViewFields(t *testing.T) {
	bdtest.RequireRealBd(t)
	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "BEADS_DIR=")
		out, err := cmd.Output()
		require.NoError(t, err, "%v", args)
		return strings.TrimSpace(string(out))
	}
	run("git", "init", "-q")
	run("bd", "init", "--quiet", "--prefix", "vf", "--skip-hooks", "--skip-agents")
	epic := run("bd", "q", "epic", "-t", "epic")
	child := run("bd", "q", "child", "--parent", epic)
	var recs []map[string]any
	require.NoError(t, json.Unmarshal([]byte(run("bd", "show", child, "--json")), &recs))
	require.Len(t, recs, 1)
	assert.Equal(t, epic, recs[0]["parent"])
	assert.NotEmpty(t, recs[0]["revision"])
	deps, _ := recs[0]["dependencies"].([]any)
	require.NotEmpty(t, deps)
	assert.Equal(t, "parent-child", deps[0].(map[string]any)["dependency_type"])
}
