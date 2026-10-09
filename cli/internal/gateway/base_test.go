package gateway

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
)

// S-08: the workspace of the session is fixed when the grant is issued; a
// redirect or a configuration written afterwards, or a `.beads` of a
// sub-directory, does not change the database bd uses.
func TestBeadsBaseFixed(t *testing.T) {
	f := newFixture(t, []string{"list"})
	f.b.View = nil
	beadsDir := filepath.Join(f.project, ".beads")

	// A .beads of a sub-directory is ignored: BEADS_DIR is the session's.
	require.NoError(t, os.MkdirAll(filepath.Join(f.project, "sub", ".beads"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(f.project, "sub", ".beads", "redirect"), []byte("/other/.beads"), 0o600))
	code, resp := f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"list"}, Cwd: filepath.Join(f.project, "sub")})
	require.Equal(t, http.StatusOK, code, resp.Error)
	assert.Contains(t, string(resp.Stdout), "beads_dir:"+beadsDir+"\n")

	for _, name := range []string{"redirect", "metadata.json", "config.yaml"} {
		p := filepath.Join(beadsDir, name)
		require.NoError(t, os.WriteFile(p, []byte("/other/.beads"), 0o600))
		code, resp = f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"list"}, Cwd: f.project})
		assert.Equal(t, http.StatusForbidden, code, name)
		assert.Contains(t, resp.Error, p, name)
		assert.NotContains(t, string(resp.Stdout), "argv:", "bd did not run")
		require.NoError(t, os.Remove(p))
	}
	code, _ = f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"list"}, Cwd: f.project})
	assert.Equal(t, http.StatusOK, code, "file restored")

	// Help and version need no workspace.
	f.grant.Base = &Base{}
	tok, err := f.b.Store.Issue(f.grant)
	require.NoError(t, err)
	code, resp = f.call(t, tok, beadswire.ExecRequest{Argv: []string{"list"}, Cwd: f.project})
	assert.Equal(t, http.StatusForbidden, code, "no workspace when the session started")
	assert.NotEmpty(t, resp.Error)
	code, _ = f.call(t, tok, beadswire.ExecRequest{Argv: []string{"--version"}, Cwd: f.project})
	assert.Equal(t, http.StatusOK, code)
}

func TestResolveBase(t *testing.T) {
	f := newFixture(t, nil)
	base := f.b.ResolveBase(context.Background(), f.project)
	require.NotNil(t, base)
	assert.Equal(t, filepath.Join(f.project, ".beads"), base.Dir)
	assert.Equal(t, map[string]string{"redirect": "", "metadata.json": "", "config.yaml": ""}, base.Files)

	// A grant without a workspace (older daemon) resolves it at its first command.
	f.grant.Base = nil
	f.grant.Holder = "child"
	tok, err := f.b.Store.Issue(f.grant)
	require.NoError(t, err)
	code, resp := f.call(t, tok, beadswire.ExecRequest{Argv: []string{"show", "x"}, Cwd: "/work/proj"})
	require.Equal(t, http.StatusOK, code, resp.Error)
	g, _ := f.b.Store.Lookup(tok)
	require.NotNil(t, g.Base)
	assert.Equal(t, base.Dir, g.Base.Dir)

	f.b.Binary = filepath.Join(t.TempDir(), "missing")
	assert.Nil(t, f.b.ResolveBase(context.Background(), f.project), "bd not runnable: resolved later")
}
