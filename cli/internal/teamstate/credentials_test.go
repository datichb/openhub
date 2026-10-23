package teamstate

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

func TestIsAuthError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"terminal prompts disabled", errors.New("git pull --rebase: fatal: could not read Username for 'https://gitlab.example.com': terminal prompts disabled"), true},
		{"access denied", errors.New("git pull: remote: HTTP Basic: Access denied. The provided password or token is incorrect"), true},
		{"could not read the username", errors.New("fatal: could not read Username for 'https://github.com': No such device or address"), true},
		{"network error", errors.New("git pull: fatal: unable to connect to github.com: connection refused"), false},
		{"rebase conflict", errors.New("git pull --rebase: error: could not apply abc1234... fix: merge conflict in foo.go"), false},
		{"no error", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isAuthError(tt.err))
		})
	}
}

func TestIsHTTPS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		remote string
		want   bool
	}{
		{"https://gitlab.octo.tools/org/team-state.git", true},
		{"http://gitlab.internal/org/repo.git", true},
		{"git@gitlab.octo.tools:org/team-state.git", false},
		{"ssh://git@gitlab.octo.tools/org/team-state.git", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.remote, func(t *testing.T) {
			assert.Equal(t, tt.want, IsHTTPS(tt.remote))
		})
	}
}

func TestAuthErrorMessage(t *testing.T) {
	t.Parallel()
	remote := "https://gitlab.octo.tools/org/team-state.git"
	msg := authErrorMessage(remote)
	assert.Contains(t, msg, remote, "names the remote")
	assert.Contains(t, msg, "oauth2", "mentions the GitLab oauth2 username")
	assert.Contains(t, msg, "token", "mentions the token")
	assert.Contains(t, msg, "team configure", "tells how to reconfigure")
}

// Bug 13: the HTTPS steps of the wizards asked a token for a repository that
// answers without one.
func TestRemoteAccessible(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	bare, _ := gittest.BareClone(t)
	assert.True(t, RemoteAccessible(ctx, bare), "a repository git can read")

	assert.False(t, RemoteAccessible(ctx, filepath.Join(t.TempDir(), "absent.git")), "a path that is no repository")
	assert.False(t, RemoteAccessible(ctx, ""), "no remote")

	t.Run("http server that wants a token", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("WWW-Authenticate", `Basic realm="git"`)
			w.WriteHeader(http.StatusUnauthorized)
		}))
		t.Cleanup(srv.Close)
		assert.False(t, RemoteAccessible(ctx, srv.URL+"/org/team.git"))
	})
}
