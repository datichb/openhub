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

	"github.com/datichb/openhub/cli/internal/beads/bdtest"
	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

// Contract with the real bd (1.3.1 when the table was generated): the
// proofs of the v5 audit (S-07, S-08) run through the gateway and change
// nothing; the canonical arguments are read by bd as the gateway read them.
func TestRealBeadsArgsContract(t *testing.T) {
	bd := bdtest.RequireRealBd(t)
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

	// oh sets the actor of a write: bd records it in the history.
	gw.Agent = func(context.Context, Grant) string { return "developer" }
	g.Human = "Ada"
	code, resp = exec(project, "update", c, "--title", "c renamed")
	require.Equal(t, http.StatusOK, code, resp.Error)
	require.Equal(t, 0, resp.ExitCode, string(resp.Stderr))
	assert.Contains(t, run(project, "bd", "history", c, "--events"), "Ada via oh/developer")
	gw.Agent, g.Human = nil, ""

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

// Contract with the real bd: --readonly, added by the gateway to every read
// command, makes bd refuse the writes (second line behind the allow-list),
// and every read command of the table still runs with it.
func TestRealBeadsReadOnlyContract(t *testing.T) {
	bdtest.RequireRealBd(t)
	p := realDir(t)
	run := func(args ...string) (string, error) {
		cmd := exec.Command("bd", args...)
		cmd.Dir = p
		cmd.Env = append(os.Environ(), "BEADS_DIR=")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	gittest.Run(t, p, "init", "-q")
	out, err := run("init", "--quiet", "--prefix", "ro", "--skip-hooks", "--skip-agents")
	require.NoError(t, err, out)
	id, err := run("q", "a")
	require.NoError(t, err, id)
	id = strings.TrimSpace(id)
	for _, argv := range [][]string{
		{"create", "x"}, {"q", "x"}, {"update", id, "--title", "y"}, {"update", id, "--claim"}, {"close", id},
		{"comments", "add", id, "hi"}, {"label", "add", id, "l"}, {"dep", "add", id, id}, {"note", id, "n"},
		{"assign", id, "bob"}, {"defer", id}, {"remember", "m"}, {"heartbeat", id}, {"ready", "--claim"},
	} {
		out, err := run(append([]string{"--readonly"}, argv...)...)
		assert.Error(t, err, "%v: %s", argv, out)
		assert.Contains(t, out, "read-only mode", argv)
	}
	for _, argv := range [][]string{
		{"show", id}, {"list"}, {"ready"}, {"query", "status=open"}, {"count"}, {"comments", id}, {"history", id},
		{"dep", "tree", id}, {"label", "list-all"}, {"statuses", "--json"}, {"where"},
	} {
		out, err := run(append([]string{"--readonly"}, argv...)...)
		assert.NoError(t, err, "%v: %s", argv, out)
	}
	show, err := run("show", id, "--json")
	require.NoError(t, err)
	assert.Contains(t, show, `"title": "a"`, "nothing was written")
}

// Contract with the real bd: the provenance oh records is kept once, even
// recorded twice, and read back by `bd provenance log`.
func TestRealBeadsProvenance(t *testing.T) {
	bd := bdtest.RequireRealBd(t)
	p := realDir(t)
	gittest.Run(t, p, "init", "-q")
	run := func(args ...string) string {
		cmd := exec.Command("bd", args...)
		cmd.Dir = p
		cmd.Env = append(os.Environ(), "BEADS_DIR=")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "%v: %s", args, out)
		return strings.TrimSpace(string(out))
	}
	run("init", "--quiet", "--prefix", "pv", "--skip-hooks", "--skip-agents")
	id := run("q", "a")
	st, err := OpenStore("")
	require.NoError(t, err)
	gw := &Beads{Store: st, Binary: bd}
	st.ResolveBase = gw.ResolveBase
	tok, err := st.Issue(Grant{SessionID: "ses_p", GroupKey: "g", Location: p})
	require.NoError(t, err)
	g, _ := st.Lookup(tok)
	ev := Provenance{Issue: id, Kind: ProvCommit, RefKind: RefGitSHA, Ref: strings.Repeat("0123abcd", 5), Actor: Actor("Ada", "developer"),
		Payload: map[string]string{"session": "ses_p"}}
	require.NoError(t, gw.RecordProvenance(context.Background(), g, ev))
	require.NoError(t, gw.RecordProvenance(context.Background(), g, ev))
	log := run("provenance", "log", id, "--json")
	assert.Equal(t, 1, strings.Count(log, strings.Repeat("0123abcd", 5)), log)
	assert.Contains(t, log, "Ada via oh/developer")
	for _, ev := range []Provenance{
		{Issue: id, Kind: ProvClaim, RefKind: RefWorkID, Ref: "ses_p", Payload: map[string]string{"workflow": "ticket"}},
		{Issue: id, Kind: ProvHandoff, RefKind: RefPR, Ref: "https://gitlab.example.com/g/p/-/merge_requests/7"},
	} {
		require.NoError(t, gw.RecordProvenance(context.Background(), g, ev), ev.Kind)
	}
}
