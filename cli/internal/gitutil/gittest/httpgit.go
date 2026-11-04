package gittest

import (
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// HTTPGit is a git server over HTTP (git http-backend, the smart protocol:
// clone, fetch, push) in front of a bare repository, with or without basic
// authentication.
type HTTPGit struct {
	// URL is the URL of the repository (http://127.0.0.1:port/<name>.git).
	URL string

	anonymous atomic.Int64
	refused   atomic.Int64
	accepted  atomic.Int64
}

// Anonymous is the number of requests that came without credentials.
func (g *HTTPGit) Anonymous() int { return int(g.anonymous.Load()) }

// Refused is the number of requests that came with wrong credentials.
func (g *HTTPGit) Refused() int { return int(g.refused.Load()) }

// Accepted is the number of requests that came with the right credentials.
func (g *HTTPGit) Accepted() int { return int(g.accepted.Load()) }

// ServeHTTP serves bare as <name>.git. With a token, every request must carry
// the basic authentication user:token (401 otherwise, which makes git ask for
// credentials); with an empty token the repository is public. The test is
// skipped when git has no http-backend.
func ServeHTTP(t testing.TB, bare, name, user, token string) *HTTPGit {
	t.Helper()
	execPath, err := exec.Command("git", "--exec-path").Output()
	if err != nil {
		t.Skip("git")
	}
	backend := filepath.Join(strings.TrimSpace(string(execPath)), "git-http-backend")
	if _, err := os.Stat(backend); err != nil {
		t.Skip("git http-backend")
	}
	Run(t, bare, "config", "http.receivepack", "true")
	root := t.TempDir()
	if err := os.Symlink(bare, filepath.Join(root, name+".git")); err != nil {
		t.Skipf("symlink: %v", err)
	}
	gitHTTP := &cgi.Handler{
		Path:       backend,
		Root:       "/",
		InheritEnv: []string{"PATH", "HOME", "GIT_CONFIG_GLOBAL", "TMPDIR"},
		Env:        []string{"GIT_PROJECT_ROOT=" + root, "GIT_HTTP_EXPORT_ALL=1"},
	}
	g := &HTTPGit{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token != "" {
			u, p, ok := r.BasicAuth()
			switch {
			case !ok:
				g.anonymous.Add(1)
			case u != user || p != token:
				g.refused.Add(1)
			default:
				g.accepted.Add(1)
				gitHTTP.ServeHTTP(w, r)
				return
			}
			w.Header().Set("WWW-Authenticate", `Basic realm="git"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		g.anonymous.Add(1)
		gitHTTP.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	g.URL = srv.URL + "/" + name + ".git"
	return g
}
