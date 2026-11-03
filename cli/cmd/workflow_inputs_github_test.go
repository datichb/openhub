package cmd

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
)

func gitRepoWithOrigin(t *testing.T, origin string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", dir, "init", "-q").Run())
	if origin != "" {
		require.NoError(t, exec.Command("git", "-C", dir, "remote", "add", "origin", origin).Run())
	}
	return dir
}

func TestGithubRemote(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct{ name, origin, want string }{
		{"https", "https://github.com/acme/web.git", "github.com/acme/web"},
		{"scp-like", "git@github.com:acme/web.git", "github.com/acme/web"},
		{"enterprise", "https://github.corp.example/acme/web", "github.corp.example/acme/web"},
		{"gitlab", "https://gitlab.com/acme/web.git", ""},
		{"self-hosted gitlab", "git@git.corp.example:acme/web.git", ""},
		{"no origin", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, githubRemote(ctx, gitRepoWithOrigin(t, c.origin)))
		})
	}
	assert.Empty(t, githubRemote(ctx, t.TempDir()), "not a repository")
}

// F-07: mr.* inputs on a GitHub project say so, instead of failing on a token
// or a project error.
func TestGitlabOnlyRefusesGitHub(t *testing.T) {
	called := false
	src := func(context.Context, workflowsvc.Context, string) (string, error) {
		called = true
		return "ok", nil
	}
	c := workflowsvc.Context{ProjectID: "p1"}

	_, err := gitlabOnly(src, func(context.Context, string) string { return "github.com/acme/web" })(context.Background(), c, "12")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github.com/acme/web")
	assert.Contains(t, err.Error(), "GitLab")
	assert.False(t, called, "the GitLab source must not run")

	got, err := gitlabOnly(src, func(context.Context, string) string { return "" })(context.Background(), c, "12")
	require.NoError(t, err)
	assert.Equal(t, "ok", got)
	assert.True(t, called)
}
