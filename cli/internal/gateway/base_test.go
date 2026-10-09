package gateway

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// S-14: the grants saved by the daemon are trusted only when signed by it,
// private, and well formed.
func TestSignedStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.json")
	key := []byte("capability")
	st, err := OpenSignedStore(path, key)
	require.NoError(t, err)
	tok, err := st.Issue(Grant{SessionID: "ses_a", GroupKey: "g", Location: "/p", BeadsAllow: []string{"show"}})
	require.NoError(t, err)

	again, err := OpenSignedStore(path, key)
	require.NoError(t, err)
	_, ok := again.Lookup(tok)
	assert.True(t, ok)

	_, err = OpenSignedStore(path, []byte("other key"))
	assert.Error(t, err, "signed by another key")

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	widened := strings.Replace(string(data), `"beads_allow":["show"]`, `"beads_allow":["show","close"]`, 1)
	require.NotEqual(t, string(data), widened)
	require.NoError(t, os.WriteFile(path, []byte(widened), 0o600))
	forged, err := OpenSignedStore(path, key)
	assert.Error(t, err, "a grant changed by someone else")
	assert.Equal(t, 0, forged.Len())

	require.NoError(t, os.WriteFile(path, data, 0o600))
	require.NoError(t, os.Chmod(path, 0o644))
	if runtime.GOOS != "windows" {
		loose, err := OpenSignedStore(path, key)
		assert.Error(t, err, "readable by others")
		assert.Equal(t, 0, loose.Len())
	}

	// Malformed grants are dropped (unsigned store).
	bad := `{"grants":[{"hash":"zz","session_id":"s","group_key":"g","location":"/p"},{"hash":"` + strings.Repeat("a", 64) + `","session_id":"s","group_key":"g","location":"rel"}]}`
	require.NoError(t, os.Chmod(path, 0o600))
	require.NoError(t, os.WriteFile(path, []byte(bad), 0o600))
	plain, err := OpenStore(path)
	require.NoError(t, err)
	assert.Equal(t, 0, plain.Len())
}

// S-14: the real bd gets a minimal environment, never the daemon's secrets.
func TestBeadsEnvMinimal(t *testing.T) {
	env := beadsEnv([]string{"PATH=/bin", "HOME=/h", "LANG=fr_FR.UTF-8", "LC_ALL=C", "BEADS_ACTOR=me", "BEADS_DIR=/elsewhere",
		"BD_GIT_HOOK=1", "AWS_SECRET_ACCESS_KEY=x", "TOOL_SERVER_PASSWORD=y", "GITLAB_TOKEN=z", "OH_GATEWAY_TOKEN=ohg_x"}, "/p/.beads", false)
	assert.ElementsMatch(t, []string{"PATH=/bin", "HOME=/h", "LANG=fr_FR.UTF-8", "LC_ALL=C", "BEADS_ACTOR=me", "BEADS_DIR=/p/.beads"}, env)
	assert.Contains(t, beadsEnv(nil, "", true), beadswire.EnvGitHook+"=1")
}
