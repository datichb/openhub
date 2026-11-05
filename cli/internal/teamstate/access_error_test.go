package teamstate

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// The access is a type, whatever the language of oh and the words of the
// message: a refusal of the server is an *AuthError for every operation and
// both protocols.
func TestAccessErrorIsTypedWhateverTheLanguage(t *testing.T) {
	const gitlab = "https://gitlab.example.com/org/team.git"
	const ssh = "git@gitlab.example.com:org/team.git"
	for _, tt := range []struct {
		name   string
		remote string
		msg    string
		want   bool
	}{
		{"https refused", gitlab, "git pull: fatal: Authentication failed for 'https://gitlab.example.com/org/team.git/'", true},
		{"https no credentials", gitlab, "fatal: could not read Username for 'https://gitlab.example.com': terminal prompts disabled", true},
		{"https read-only token", gitlab, "fatal: unable to access 'https://gitlab.example.com/org/team.git/': The requested URL returned error: 403", true},
		{"ssh key refused", ssh, "git@gitlab.example.com: Permission denied (publickey).\nfatal: Could not read from remote repository.", true},
		{"ssh read-only key", ssh, "remote: GitLab: You are not allowed to push code to this project.", true},
		{"gitlab does not find it", gitlab, "fatal: repository 'https://gitlab.example.com/org/team.git/' not found", true},
		{"gitlab says it in words", gitlab, "remote: The project you were looking for could not be found or you don't have permission to view it.", true},
		{"another host: a wrong URL", "https://git.example.com/org/team.git", "fatal: repository 'https://git.example.com/org/team.git/' not found", false},
		{"ssh: a wrong URL is not an access problem", ssh, "fatal: repository 'git@gitlab.example.com:org/x.git' not found", false},
		{"network", gitlab, "fatal: unable to access 'https://gitlab.example.com/': Could not resolve host: gitlab.example.com", false},
		{"conflict", gitlab, "error: could not apply abc1234... merge conflict in foo.go", false},
	} {
		for _, lang := range []string{"fr", "en"} {
			t.Run(tt.name+"/"+lang, func(t *testing.T) {
				i18ntest.Set(t, lang)
				cause := errors.New(tt.msg)
				got := classifyRemoteError(tt.remote, fmt.Errorf("pushing team-state: %w", cause))
				var authErr *AuthError
				assert.Equal(t, tt.want, errors.As(got, &authErr), "%v", got)
				if tt.want {
					assert.True(t, IsAuthError(got))
					assert.Equal(t, tt.remote, authErr.Remote)
					assert.ErrorIs(t, got, cause, "the cause of git is kept")
					assert.Same(t, authErr, AccessError(got))
					// a second classification changes nothing
					assert.Same(t, authErr, classifyRemoteError(tt.remote, got))
				}
			})
		}
	}
}

func TestClassifyRemoteErrorKeepsOfflineAndTheRest(t *testing.T) {
	net := errors.New("fatal: unable to access 'https://x/': Could not resolve host: x")
	got := classifyRemoteError("https://x/y.git", net)
	assert.ErrorIs(t, got, ErrOffline)
	assert.Equal(t, got, classifyRemoteError("https://x/y.git", got), "already offline: not wrapped twice")
	assert.False(t, IsAuthError(got))

	other := errors.New("boom")
	assert.Same(t, other, classifyRemoteError("https://x/y.git", other))
	assert.Nil(t, AccessError(other))
}

// fakeSSH puts an ssh on the PATH that refuses the key the way OpenSSH does.
func fakeSSH(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX sh")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\necho 'git@gitlab.example.com: Permission denied (publickey).' >&2\nexit 255\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GIT_ASKPASS", "")
	t.Setenv("SSH_ASKPASS", "")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// An SSH key that is not accepted is an access problem for every operation,
// not a warning of the pull: the clone, the pull, the push, the probes.
func TestSSHRefusalIsAnAuthErrorEverywhere(t *testing.T) {
	fakeSSH(t)
	const remote = "git@gitlab.example.com:org/team.git"
	ctx := t.Context()

	t.Run("probes", func(t *testing.T) {
		assert.True(t, IsAuthError(ProbeRemote(ctx, remote)))
		assert.NotNil(t, AccessError(ProbeRemote(ctx, remote)))
	})

	t.Run("clone", func(t *testing.T) {
		err := NewRepo(remote, filepath.Join(t.TempDir(), "c")).Clone(ctx)
		require.Error(t, err)
		assert.NotNil(t, AccessError(err), "%v", err)
	})

	bare, clone := gittest.BareClone(t)
	gittest.Run(t, clone, "remote", "set-url", "origin", remote)
	repo := NewRepo(remote, clone)

	t.Run("pull", func(t *testing.T) {
		err := repo.Pull(ctx)
		require.Error(t, err)
		assert.NotNil(t, AccessError(err), "%v", err)
	})

	t.Run("EnsureReady is fatal, not a warning", func(t *testing.T) {
		err := repo.EnsureReady(ctx)
		require.Error(t, err)
		assert.False(t, IsPullWarning(err))
		assert.NotNil(t, AccessError(err), "%v", err)
	})

	t.Run("push of a transaction", func(t *testing.T) {
		// the pull of the transaction is the first to be refused
		err := repo.Transact(ctx, func(_ context.Context, tx *Tx) (TxResult, error) {
			return TxResult{}, tx.WriteFile("a.txt", []byte("a"))
		})
		require.Error(t, err)
		assert.NotNil(t, AccessError(err), "%v", err)
		assert.NotContains(t, err.Error(), "push échoué", "no word of a language in the error")
	})
	_ = bare
}

// A push that the server refuses (a token without the right to write) is an
// access problem, in the words of no language.
func TestPushRefusedByTheServerIsAnAuthError(t *testing.T) {
	skipWithoutSh(t)
	emptyTmp(t)
	bare, clone := gittest.BareClone(t)
	srv := gittest.ServeHTTP(t, bare, "team-state", "oauth2", "good")
	ro := readOnlyProxy(t, srv.URL)
	restore := SetCredentialSource(nil)
	defer restore()
	gittest.Run(t, clone, "remote", "set-url", "origin", ro)
	repo := NewRepo(ro, clone)
	restore2 := SetCredentialSource(credsFor(ro, "oauth2", "good", nil))
	defer restore2()

	require.NoError(t, repo.Pull(t.Context()), "reading is allowed")
	require.NoError(t, os.WriteFile(filepath.Join(clone, "x.txt"), []byte("x"), 0o644))
	i18ntest.Set(t, "fr")
	err := repo.CommitAndPush(t.Context(), "x", "x.txt")
	require.Error(t, err)
	assert.NotNil(t, AccessError(err), "%v", err)
	assert.NotContains(t, err.Error(), "push échoué")
}

// readOnlyProxy is srv in front of which the receive-pack service is
// refused (403), as a GitLab does to a token with read_repository only.
func readOnlyProxy(t *testing.T, target string) string {
	t.Helper()
	u, err := url.Parse(target)
	require.NoError(t, err)
	rp := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: u.Scheme, Host: u.Host})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "git-receive-pack") || strings.HasSuffix(r.URL.Path, "git-receive-pack") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		rp.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + u.Path
}
