package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
)

// fakeBd writes a bd that prints its arguments and directory, echoes stdin
// for "-" and exits with $2 for "exit".
func fakeBd(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "bd")
	script := `#!/bin/sh
ro=""; actor=""
while :; do case "$1" in --readonly) ro=1; shift;; --actor=*) actor="${1#--actor=}"; shift;; *) break;; esac; done
if [ "$1" = "where" ]; then printf '{"path":"%s/.beads"}\n' "$(pwd -P)"; exit 0; fi
echo "argv:$*"
echo "readonly:$ro"
echo "actor:$actor"
echo "beads_dir:$BEADS_DIR"
echo "pwd:$(pwd -P)"
for a in "$@"; do case "$a" in -|*=-) cat;; esac; done
for last in "$@"; do :; done
[ "$1" = "show" ] && [ "$last" = "boom" ] && { echo "bd failed" >&2; exit 4; }
[ "$1" = "show" ] && [ "$last" = "slow" ] && sleep 5
exit 0
`
	require.NoError(t, os.WriteFile(p, []byte(script), 0o755))
	return p
}

func realDir(t *testing.T) string {
	t.Helper()
	d, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	return d
}

type fixture struct {
	b       *Beads
	project string
	token   string
	grant   Grant
}

func newFixture(t *testing.T, allow []string) *fixture {
	t.Helper()
	project := realDir(t)
	require.NoError(t, os.MkdirAll(filepath.Join(project, "sub"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(project, ".beads"), 0o700))
	st, err := OpenStore("")
	require.NoError(t, err)
	dir := filepath.Join(project, ".beads")
	g := Grant{SessionID: "ses_1", GroupKey: "g1", Location: project, BeadsAllow: allow, Base: &Base{Dir: dir, Files: hashBaseFiles(dir)}}
	tok, err := st.Issue(g)
	require.NoError(t, err)
	b := &Beads{Store: st, Binary: fakeBd(t), View: func(context.Context, string) (View, error) {
		return View{Paths: ohruntime.PathMap{{Host: project, Inner: "/work/proj"}, {Host: "/bundle", Inner: "/opt/oh/bundle"}}, Locations: []string{project}}, nil
	}}
	return &fixture{b: b, project: project, token: tok, grant: g}
}

func (f *fixture) call(t *testing.T, token string, req beadswire.ExecRequest) (int, beadswire.ExecResponse) {
	t.Helper()
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, beadswire.ExecPath, bytes.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	f.b.ServeHTTP(w, r)
	var resp beadswire.ExecResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return w.Code, resp
}

func TestBeadsRunsAllowedCommandInTranslatedDir(t *testing.T) {
	f := newFixture(t, []string{"show", "dep tree"})
	code, resp := f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"--json", "show", "bd-1"}, Cwd: "/work/proj/sub"})
	require.Equal(t, http.StatusOK, code, resp.Error)
	assert.Equal(t, 0, resp.ExitCode)
	assert.Contains(t, string(resp.Stdout), "argv:show --json -- bd-1")
	assert.Contains(t, string(resp.Stdout), "beads_dir:"+filepath.Join(f.project, ".beads"), "workspace fixed by oh")
	assert.Contains(t, string(resp.Stdout), "pwd:"+filepath.Join(f.project, "sub"))

	// Outside the session locations: bd runs in the session location.
	_, resp = f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"show", "x"}, Cwd: "/tmp"})
	assert.Contains(t, string(resp.Stdout), "pwd:"+f.project+"\n")

	code, resp = f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"dep", "tree", "bd-1"}, Cwd: "/work/proj"})
	assert.Equal(t, http.StatusOK, code, resp.Error)
}

func TestBeadsRelaysExitCodeStderrAndStdin(t *testing.T) {
	f := newFixture(t, nil)
	_, resp := f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"show", "boom"}, Cwd: "/work/proj"})
	assert.Equal(t, 4, resp.ExitCode)
	assert.Equal(t, "bd failed\n", string(resp.Stderr))

	f = newFixture(t, []string{"comment"})
	_, resp = f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"comment", "bd-1", "--file", "-"}, Cwd: "/work/proj", Stdin: []byte("hello")})
	assert.True(t, strings.HasSuffix(string(resp.Stdout), "hello"), string(resp.Stdout))
}

func TestBeadsRefusals(t *testing.T) {
	f := newFixture(t, nil) // default read-only list
	for _, tc := range []struct {
		argv []string
		want string
	}{
		{[]string{"create", "title"}, "create"},
		{[]string{"dep", "add", "a", "b"}, "dep add"},
		{[]string{"show", "x", "--db", "/other.db"}, "--db"},
		{[]string{"--db=/x", "show"}, "--db"},
		{[]string{"-C/elsewhere", "show"}, "--directory"},
		{[]string{"-qC/elsewhere", "list"}, "--directory"}, // S-08: combined short flags
		{[]string{"list", "--DB=/x"}, "--DB"},
		{[]string{"show", "x", "--database=y"}, "--database"},
		{[]string{"--global", "list"}, "--global"},
		{[]string{"--weird", "show"}, "--weird"},
		{[]string{"find-duplicates", "--method", "ai"}, "--method=ai"},
	} {
		code, resp := f.call(t, f.token, beadswire.ExecRequest{Argv: tc.argv, Cwd: "/work/proj"})
		assert.Equal(t, http.StatusForbidden, code, tc.argv)
		assert.Equal(t, 1, resp.ExitCode)
		assert.Contains(t, resp.Error, tc.want, tc.argv)
		assert.Empty(t, resp.Stdout)
	}

	f = newFixture(t, []string{})
	code, resp := f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"show", "x"}})
	assert.Equal(t, http.StatusForbidden, code)
	assert.NotEmpty(t, resp.Error)

	// Help and version are always harmless.
	code, _ = f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"--version"}})
	assert.Equal(t, http.StatusOK, code)
	code, _ = f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"help", "create"}})
	assert.Equal(t, http.StatusOK, code)
}

func TestBeadsTokenAndSession(t *testing.T) {
	f := newFixture(t, nil)
	code, resp := f.call(t, "ohg_unknown", beadswire.ExecRequest{Argv: []string{"show"}})
	assert.Equal(t, http.StatusUnauthorized, code)
	assert.NotEmpty(t, resp.Error)

	f.b.Alive = func(context.Context, string) bool { return false }
	code, _ = f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"show"}})
	assert.Equal(t, http.StatusUnauthorized, code)

	f.b.Alive = nil
	f.b.Store.RevokeOwner("g1")
	code, _ = f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"show"}})
	assert.Equal(t, http.StatusUnauthorized, code)
}

// Reads run with --readonly (bd refuses a write behind the allow-list);
// writes do not, and carry the actor set by oh.
func TestBeadsReadOnlyFlagAndActor(t *testing.T) {
	f := newFixture(t, []string{"show", "query", "update", "comments", "ready"})
	agent := "developer"
	f.b.Agent = func(context.Context, Grant) string { return agent }
	f.grant.Human = "Ada Lovelace"
	tok, err := f.b.Store.Issue(f.grant)
	require.NoError(t, err)
	run := func(argv ...string) string {
		code, resp := f.call(t, tok, beadswire.ExecRequest{Argv: argv, Cwd: "/work/proj"})
		require.Equal(t, http.StatusOK, code, resp.Error)
		return string(resp.Stdout)
	}
	for _, argv := range [][]string{{"show", "bd-1"}, {"query", "status=open"}, {"comments", "bd-1"}, {"ready"}} {
		out := run(argv...)
		assert.Contains(t, out, "readonly:1\n", argv)
		assert.Contains(t, out, "actor:\n", argv)
	}
	out := run("update", "bd-1", "--title", "x")
	assert.Contains(t, out, "readonly:\n")
	assert.Contains(t, out, "actor:Ada Lovelace via oh/developer\n")
	out = run("comments", "add", "bd-1", "hi")
	assert.Contains(t, out, "readonly:\n")
	assert.Contains(t, out, "actor:Ada Lovelace via oh/developer\n")

	// bd makes the actor of a claim the assignee: the person alone.
	for _, argv := range [][]string{{"update", "bd-1", "--claim"}, {"update", "bd-1", "-s", "in_progress"}, {"ready", "--claim"}} {
		out = run(argv...)
		assert.Contains(t, out, "readonly:\n", argv)
		assert.Contains(t, out, "actor:Ada Lovelace\n", argv)
	}

	// A subagent not named yet.
	agent = ""
	f.grant.Holder = "ses_child"
	tok, err = f.b.Store.Issue(f.grant)
	require.NoError(t, err)
	assert.Contains(t, run("update", "bd-1", "--title", "y"), "actor:Ada Lovelace via oh/subagent\n")

	// Agents cannot choose the actor.
	for _, argv := range [][]string{{"--actor", "me", "update", "bd-1"}, {"update", "bd-1", "--actor=me"}} {
		code, resp := f.call(t, tok, beadswire.ExecRequest{Argv: argv, Cwd: "/work/proj"})
		assert.Equal(t, http.StatusForbidden, code, argv)
		assert.Contains(t, resp.Error, "--actor")
	}
}

// A session whose allow-list names only read commands is read-only: the
// write forms of those commands are refused.
func TestBeadsReadOnlySession(t *testing.T) {
	f := newFixture(t, nil) // default list: read commands only
	for _, argv := range [][]string{{"comments", "add", "bd-1", "hi"}, {"ready", "--claim"}} {
		code, resp := f.call(t, f.token, beadswire.ExecRequest{Argv: argv, Cwd: "/work/proj"})
		assert.Equal(t, http.StatusForbidden, code, argv)
		assert.Contains(t, resp.Error, "read-only", argv)
	}
	for _, argv := range [][]string{{"comments", "bd-1"}, {"ready"}, {"query", "status=open"}} {
		code, resp := f.call(t, f.token, beadswire.ExecRequest{Argv: argv, Cwd: "/work/proj"})
		assert.Equal(t, http.StatusOK, code, argv, resp.Error)
	}
}

func TestBeadsFilePaths(t *testing.T) {
	f := newFixture(t, []string{"create", "export", "import"})
	require.NoError(t, os.WriteFile(filepath.Join(f.project, "d.md"), []byte("x"), 0o644))

	_, resp := f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"create", "t", "--body-file", "/work/proj/d.md"}, Cwd: "/work/proj"})
	assert.Contains(t, string(resp.Stdout), "argv:create --body-file="+filepath.Join(f.project, "d.md")+" -- t")

	_, resp = f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"create", "t", "--body-file=sub/../d.md", "--metadata", "@/work/proj/d.md"}, Cwd: "/work/proj"})
	assert.Contains(t, string(resp.Stdout), "--body-file=sub/../d.md --metadata=@"+filepath.Join(f.project, "d.md"))

	_, resp = f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"import", "/work/proj/i.jsonl"}, Cwd: "/work/proj"})
	assert.Contains(t, string(resp.Stdout), "argv:import -- "+filepath.Join(f.project, "i.jsonl"))

	for _, argv := range [][]string{
		{"create", "t", "--body-file", "/etc/passwd"},
		{"create", "t", "--body-file", "../../../etc/passwd"},
		{"create", "t", "--body-file", "/opt/oh/bundle/agents/a.md"},
		{"export", "-o", "/tmp/out.jsonl"},
		{"export", "--output=/home/x"},
		{"export", "-o/etc/x"},       // S-08: attached value
		{"create", "-f/etc/hosts"},   // S-08: -f is --file for create
		{"import", "-i/etc/x.jsonl"}, // attached input
		{"create", "--graph=/etc/x"},
		{"import", "../outside.jsonl"},
	} {
		code, resp := f.call(t, f.token, beadswire.ExecRequest{Argv: argv, Cwd: "/work/proj"})
		assert.Equal(t, http.StatusForbidden, code, argv)
		assert.NotEmpty(t, resp.Error, argv)
	}

	// A symbolic link inside the location pointing outside is refused.
	require.NoError(t, os.Symlink("/etc/hosts", filepath.Join(f.project, "link")))
	code, _ := f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"create", "t", "--body-file", "link"}, Cwd: "/work/proj"})
	assert.Equal(t, http.StatusForbidden, code)
}

func TestBeadsTimeoutAndOutputCap(t *testing.T) {
	f := newFixture(t, nil)
	f.b.Timeout = 200 * time.Millisecond
	_, resp := f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"show", "slow"}, Cwd: "/work/proj"})
	assert.Equal(t, 1, resp.ExitCode)
	assert.NotEmpty(t, resp.Error)

	f.b.Timeout, f.b.MaxOutput = 0, 8
	_, resp = f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"show", "long-argument"}, Cwd: "/work/proj"})
	assert.Len(t, resp.Stdout, 8)
	assert.Contains(t, string(resp.Stderr), "8")
}

func TestBeadsMissingBinary(t *testing.T) {
	f := newFixture(t, nil)
	f.b.Binary = ""
	t.Setenv("PATH", t.TempDir())
	code, resp := f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"show"}, Cwd: "/work/proj"})
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.NotEmpty(t, resp.Error)
}

func TestStorePersistsOnlyHashes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.json")
	st, err := OpenStore(path)
	require.NoError(t, err)
	tok, err := st.Issue(Grant{SessionID: "ses_a", GroupKey: "g", Location: "/p", BeadsAllow: []string{}})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(tok, beadswire.TokenPrefix))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), tok)
	assert.NotContains(t, string(data), strings.TrimPrefix(tok, beadswire.TokenPrefix))
	fi, _ := os.Stat(path)
	assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())

	again, err := OpenStore(path)
	require.NoError(t, err)
	g, ok := again.Lookup(tok)
	require.True(t, ok)
	assert.NotNil(t, g.BeadsAllow, "an explicit empty allow-list stays empty (not the default)")
	assert.Empty(t, g.BeadsAllow)

	// A new token for the same session replaces the previous one.
	tok2, err := again.Issue(Grant{SessionID: "ses_a", GroupKey: "g", Location: "/p"})
	require.NoError(t, err)
	_, ok = again.Lookup(tok)
	assert.False(t, ok)
	g, ok = again.Lookup(tok2)
	require.True(t, ok)
	assert.Nil(t, g.BeadsAllow)

	again.RevokeSession("ses_a")
	assert.Equal(t, 0, again.Len())
	reread, _ := OpenStore(path)
	assert.Equal(t, 0, reread.Len())
}
