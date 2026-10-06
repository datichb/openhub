package gitlab_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/remote/gitlab"
	"github.com/datichb/openhub/cli/internal/remote/gitlab/gitlabtest"
)

func TestClientProjectFilesVariables(t *testing.T) {
	ctx := context.Background()
	srv := gitlabtest.New(t, "acme/dev")
	srv.Token = "glpat-ok"
	c := gitlab.New(srv.URL, "glpat-ok")

	u, err := c.CurrentUser(ctx)
	require.NoError(t, err)
	assert.Equal(t, "alice", u.Username)

	g, err := c.Group(ctx, "acme/dev")
	require.NoError(t, err)

	_, err = c.Project(ctx, "acme/dev/oh-runner")
	assert.True(t, gitlab.IsNotFound(err))

	p, err := c.CreateProject(ctx, gitlab.CreateProjectRequest{Name: "oh-runner", Path: "oh-runner", NamespaceID: g.ID})
	require.NoError(t, err)
	assert.Equal(t, "acme/dev/oh-runner", p.PathWithNamespace)
	assert.True(t, p.RegistryEnabled())

	// Lookup by full path (encoded %2F) and by ID.
	byPath, err := c.Project(ctx, "acme/dev/oh-runner")
	require.NoError(t, err)
	assert.Equal(t, p.ID, byPath.ID)

	_, err = c.File(ctx, "acme/dev/oh-runner", ".gitlab-ci.yml", "main")
	assert.True(t, gitlab.IsNotFound(err))
	require.NoError(t, c.CommitFile(ctx, "acme/dev/oh-runner", "main", ".gitlab-ci.yml", []byte("stages: [run]\n"), "init", true))
	data, err := c.File(ctx, "acme/dev/oh-runner", ".gitlab-ci.yml", "main")
	require.NoError(t, err)
	assert.Equal(t, "stages: [run]\n", string(data))
	require.NoError(t, c.CommitFile(ctx, "acme/dev/oh-runner", "main", ".gitlab-ci.yml", []byte("x"), "update", false))

	require.NoError(t, c.SetVariable(ctx, "acme/dev/oh-runner", gitlab.Variable{Key: "OH_LLM_KEY", Value: "secret-value-1", Masked: true, Protected: true, Raw: true}))
	require.NoError(t, c.SetVariable(ctx, "acme/dev/oh-runner", gitlab.Variable{Key: "OH_LLM_KEY", Value: "secret-value-2", Masked: true, Protected: true, Raw: true}))
	vs, err := c.Variables(ctx, "acme/dev/oh-runner")
	require.NoError(t, err)
	require.Len(t, vs, 1)
	assert.Equal(t, "OH_LLM_KEY", vs[0].Key)
	assert.Empty(t, vs[0].Value, "values are never kept by the client")
	assert.Equal(t, "secret-value-2", srv.Project("acme/dev/oh-runner").Variables["OH_LLM_KEY"].Value)

	tr, err := c.CreateTrigger(ctx, "acme/dev/oh-runner", "oh")
	require.NoError(t, err)
	assert.NotEmpty(t, tr.Token)
}

func TestClientErrors(t *testing.T) {
	ctx := context.Background()
	srv := gitlabtest.New(t, "acme")
	srv.Token = "right"
	_, err := gitlab.New(srv.URL, "wrong").CurrentUser(ctx)
	assert.True(t, gitlab.IsUnauthorized(err))
	var apiErr *gitlab.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 401, apiErr.Status)
	assert.Contains(t, err.Error(), "401 Unauthorized")
}

func TestClientGenericPackages(t *testing.T) {
	ctx := context.Background()
	srv := gitlabtest.New(t, "acme")
	srv.AddProject("acme/oh-runner")
	c := gitlab.New(srv.URL, "tok")

	ok, err := c.GenericPackageExists(ctx, "acme/oh-runner", "oh-cli", "abc123", "oh-linux-amd64")
	require.NoError(t, err)
	assert.False(t, ok)

	body := []byte("binary")
	_, err = c.UploadGenericPackage(ctx, "acme/oh-runner", "oh-cli", "abc123", "oh-linux-amd64", bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err)
	ok, err = c.GenericPackageExists(ctx, "acme/oh-runner", "oh-cli", "abc123", "oh-linux-amd64")
	require.NoError(t, err)
	assert.True(t, ok)

	rc, err := c.DownloadGenericPackage(ctx, "acme/oh-runner", "oh-cli", "abc123", "oh-linux-amd64")
	require.NoError(t, err)
	got, _ := io.ReadAll(rc)
	rc.Close()
	assert.Equal(t, body, got)

	_, err = c.GenericPackageURL("acme/oh-runner", "oh cli", "1", "f")
	assert.Error(t, err)
	u, err := c.GenericPackageURL("acme/oh-runner", "oh-cli", "abc123", "oh-linux-amd64")
	require.NoError(t, err)
	assert.Equal(t, srv.URL+"/api/v4/projects/acme%2Foh-runner/packages/generic/oh-cli/abc123/oh-linux-amd64", u)
}
