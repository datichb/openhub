package fakebd

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
)

func envOf(m map[string]string) env { return func(k string) string { return m[k] } }

func TestRunRelaysGatewayAnswer(t *testing.T) {
	var got beadswire.ExecRequest
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, beadswire.ExecPath, r.URL.Path)
		auth = r.Header.Get("Authorization")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_ = json.NewEncoder(w).Encode(beadswire.ExecResponse{Stdout: []byte("out\n"), Stderr: []byte("warn\n"), ExitCode: 3})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"show", "bd-1", "--json"}, "/work/p", envOf(map[string]string{
		beadswire.EnvURL: srv.URL + "/", beadswire.EnvToken: "ohg_x",
	}), strings.NewReader("never read"), &stdout, &stderr)

	assert.Equal(t, 3, code)
	assert.Equal(t, "out\n", stdout.String())
	assert.Equal(t, "warn\n", stderr.String())
	assert.Equal(t, "Bearer ohg_x", auth)
	assert.Equal(t, []string{"show", "bd-1", "--json"}, got.Argv)
	assert.Equal(t, "/work/p", got.Cwd)
	assert.Nil(t, got.Stdin, "stdin is only forwarded when bd reads it")
}

func TestRunForwardsStdinAndRefusal(t *testing.T) {
	var got beadswire.ExecRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(beadswire.ExecResponse{Error: "create is not allowed", ExitCode: 1})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"create", "t", "--body-file", "-"}, "/work/p", envOf(map[string]string{
		beadswire.EnvURL: srv.URL, beadswire.EnvToken: "ohg_x",
	}), strings.NewReader("body"), &stdout, &stderr)
	assert.Equal(t, 1, code)
	assert.Equal(t, []byte("body"), got.Stdin)
	assert.Contains(t, stderr.String(), "create is not allowed")
	assert.Empty(t, stdout.String())
}

func TestRunWithoutGatewayOrJournal(t *testing.T) {
	var stderr bytes.Buffer
	assert.Equal(t, 1, Run([]string{"show"}, "/", envOf(nil), nil, &bytes.Buffer{}, &stderr))
	assert.Contains(t, stderr.String(), beadswire.EnvToken)

	stderr.Reset()
	assert.Equal(t, 1, Run([]string{"show"}, "/", envOf(map[string]string{beadswire.EnvMode: beadswire.ModeJournal}), nil, &bytes.Buffer{}, &stderr))
	assert.Contains(t, stderr.String(), "journal")

	stderr.Reset()
	assert.Equal(t, 1, Run(nil, "/", envOf(map[string]string{beadswire.EnvURL: "http://127.0.0.1:1", beadswire.EnvToken: "t"}), nil, &bytes.Buffer{}, &stderr))
	assert.Contains(t, stderr.String(), "not reachable")
}

func TestWantsStdin(t *testing.T) {
	assert.True(t, wantsStdin([]string{"comment", "x", "--stdin"}))
	assert.True(t, wantsStdin([]string{"create", "--body-file=-"}))
	assert.True(t, wantsStdin([]string{"import", "-"}))
	assert.False(t, wantsStdin([]string{"create", "a - b"}))
}

// A19: a command run by a Beads git hook says so to the gateway.
func TestRunTellsGitHook(t *testing.T) {
	var got beadswire.ExecRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_ = json.NewEncoder(w).Encode(beadswire.ExecResponse{})
	}))
	defer srv.Close()
	vars := map[string]string{beadswire.EnvURL: srv.URL, beadswire.EnvToken: "ohg_x"}
	assert.Equal(t, 0, Run([]string{"hooks", "run", "pre-commit"}, "/w", envOf(vars), nil, &bytes.Buffer{}, &bytes.Buffer{}))
	assert.False(t, got.GitHook)
	vars[beadswire.EnvGitHook] = "1"
	assert.Equal(t, 0, Run([]string{"hooks", "run", "pre-commit"}, "/w", envOf(vars), nil, &bytes.Buffer{}, &bytes.Buffer{}))
	assert.True(t, got.GitHook)
}

// S-06: a git hook asks the daemon socket for its hook token and sends it.
func TestRunAsksHookToken(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix socket")
	}
	dir, err := os.MkdirTemp("/tmp", "ohbd-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "ohd.sock")
	l, err := net.Listen("unix", sock)
	require.NoError(t, err)
	var asked beadswire.HookTokenRequest
	daemon := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, beadswire.HookTokenPath, r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&asked))
		_ = json.NewEncoder(w).Encode(beadswire.HookTokenResponse{HookToken: "ohh_1"})
	})}
	go func() { _ = daemon.Serve(l) }()
	t.Cleanup(func() { _ = daemon.Close() })

	var got beadswire.ExecRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_ = json.NewEncoder(w).Encode(beadswire.ExecResponse{})
	}))
	defer srv.Close()
	vars := map[string]string{beadswire.EnvURL: srv.URL, beadswire.EnvToken: "ohg_x", beadswire.EnvGitHook: "1", beadswire.EnvDaemonSocket: sock}
	assert.Equal(t, 0, Run([]string{"hooks", "run", "post-merge", "0"}, "/w", envOf(vars), nil, &bytes.Buffer{}, &bytes.Buffer{}))
	assert.Equal(t, "ohh_1", got.HookToken)
	assert.Equal(t, beadswire.HookTokenRequest{Token: "ohg_x", Argv: []string{"hooks", "run", "post-merge", "0"}}, asked)

	got = beadswire.ExecRequest{}
	delete(vars, beadswire.EnvGitHook)
	assert.Equal(t, 0, Run([]string{"hooks", "run", "post-merge", "0"}, "/w", envOf(vars), nil, &bytes.Buffer{}, &bytes.Buffer{}))
	assert.Empty(t, got.HookToken, "no hook token outside a git hook")
}
