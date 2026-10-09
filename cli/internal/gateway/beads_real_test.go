//go:build integration

package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
)

// Contract with the real bd (1.3.1 when the table was generated): the
// proofs of the v5 audit (S-07, S-08) run through the gateway and change
// nothing; the canonical arguments are read by bd as the gateway read them.
func TestRealBeadsArgsContract(t *testing.T) {
	bd, err := exec.LookPath("bd")
	if err != nil {
		t.Skip("bd is not installed")
	}
	root := realDir(t)
	run := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "BEADS_DIR=")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "%v: %s", args, out)
		return strings.TrimSpace(string(out))
	}
	initProject := func(name, prefix string) string {
		p := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(p, 0o755))
		run(p, "git", "init", "-q")
		run(p, "bd", "init", "--quiet", "--prefix", prefix)
		return p
	}
	project, other := initProject("proj", "ct"), initProject("other", "ot")
	create := func(dir, title string) string { return run(dir, "bd", "q", title) }
	a, b, c := create(project, "a"), create(project, "b"), create(project, "c")
	secret := create(other, "secret")
	status := func(id string) string {
		var issues []struct {
			Status string `json:"status"`
		}
		require.NoError(t, json.Unmarshal([]byte(run(project, "bd", "show", "--json", id)), &issues))
		require.Len(t, issues, 1)
		return issues[0].Status
	}

	st, err := OpenStore("")
	require.NoError(t, err)
	gw := &Beads{Store: st, Binary: bd}
	st.ResolveBase = gw.ResolveBase
	var closing []BeadsOp
	gw.Guard = func(_ context.Context, _ Grant, op BeadsOp) error {
		if op.Kind == OpClose {
			closing = append(closing, op)
			return errors.New("closing refused by oh")
		}
		return nil
	}
	tok, err := st.Issue(Grant{SessionID: "ses_c", GroupKey: "g", Location: project,
		BeadsAllow: []string{"show", "list", "update", "close", "duplicate", "supersede", "import", "dep"}})
	require.NoError(t, err)
	g, _ := st.Lookup(tok)
	require.NotNil(t, g.Base)
	require.Equal(t, filepath.Join(project, ".beads"), g.Base.Dir)
	exec := func(cwd string, argv ...string) (int, beadswire.ExecResponse) {
		resp, code := gw.Exec(context.Background(), g, beadswire.ExecRequest{Argv: argv, Cwd: cwd})
		return code, resp
	}

	// S-07: every way of closing reaches the workflow guard.
	for _, argv := range [][]string{
		{"update", a, "-sclosed"},
		{"update", a, "-s=closed"},
		{"update", "-qsclosed", a},
		{"-s", "closed", "update", a},
		{"update", a, "--status=closed"},
		{"close", a},
		{"done", a},
		{"duplicate", a, "--of", b},
		{"supersede", a, "--with=" + b},
	} {
		code, resp := exec(project, argv...)
		assert.Equal(t, http.StatusForbidden, code, argv)
		assert.Contains(t, resp.Error, "closing refused by oh", argv)
	}
	assert.Len(t, closing, 9)
	assert.Equal(t, "open", status(a))

	// S-08: other databases and files out of the project.
	for _, argv := range [][]string{
		{"list", "-qC" + other},
		{"-C" + other, "list"},
		{"list", "--db=" + filepath.Join(other, ".beads", "embeddeddolt")},
		{"list", "--DB=/x"},
		{"export", "-o/etc/oh-test"},
		{"show", a, "--repo=" + other},
	} {
		code, resp := exec(project, argv...)
		assert.Equal(t, http.StatusForbidden, code, argv)
		assert.NotContains(t, string(resp.Stdout), secret, argv)
	}

	// The canonical arguments are what bd runs.
	code, resp := exec(project, "update", b, "-sin_progress", "-q")
	require.Equal(t, http.StatusOK, code, resp.Error)
	require.Equal(t, 0, resp.ExitCode, string(resp.Stderr))
	assert.Equal(t, "in_progress", status(b))

	// A redirect or a sub-directory workspace does not change the database.
	sub := filepath.Join(project, "sub")
	require.NoError(t, os.MkdirAll(filepath.Join(sub, ".beads"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sub, ".beads", "redirect"), []byte(filepath.Join(other, ".beads")+"\n"), 0o644))
	code, resp = exec(sub, "list", "--json")
	require.Equal(t, http.StatusOK, code, resp.Error)
	assert.Contains(t, string(resp.Stdout), c)
	assert.NotContains(t, string(resp.Stdout), secret)

	redirect := filepath.Join(project, ".beads", "redirect")
	require.NoError(t, os.WriteFile(redirect, []byte(filepath.Join(other, ".beads")+"\n"), 0o644))
	code, resp = exec(project, "list", "--json")
	assert.Equal(t, http.StatusForbidden, code)
	assert.NotContains(t, string(resp.Stdout), secret)
	require.NoError(t, os.Remove(redirect))
}
