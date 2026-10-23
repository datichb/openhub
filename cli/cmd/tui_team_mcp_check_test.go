package cmd

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tracker"
)

func noGitLabEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("OH_GITLAB_URL", "")
	t.Setenv("GITLAB_URL", "")
}

// Bug 8: t in the team MCP view calls the GitLab of the MCP with the token of
// the cascade, and tells who the token belongs to and where it comes from.
func TestMCPTestGitLabCallsTheServer(t *testing.T) {
	noGitLabEnv(t)
	srv, seen := fakeGitLab(t, "good")
	a := gitlabHub(map[string]string{"my.key": "good"})
	a.Config.MCP.Gitlab.Token = "my.key"
	a.Config.MCP.Gitlab.URL = srv.URL

	run := prepareMCPTest(a, "gitlab")
	require.NotNil(t, run)
	a.Config.MCP.Gitlab.URL = "http://127.0.0.1:1" // the loop goes on: the snapshot was taken

	detail, err := run(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []string{"good"}, seen(), "one call, with the token of the cascade")
	assert.Contains(t, detail, "alice")
	assert.Contains(t, detail, "MCP token (key my.key)")
	assert.Contains(t, detail, tracker.ServerOf(srv.URL))
}

func TestMCPTestGitLabRefusedTokenNamesServerSourceAndKey(t *testing.T) {
	noGitLabEnv(t)
	srv, _ := fakeGitLab(t, "other")
	a := gitlabHub(map[string]string{"my.key": "expired"})
	a.Config.MCP.Gitlab.Token = "my.key"
	a.Config.MCP.Gitlab.URL = srv.URL

	_, err := prepareMCPTest(a, "gitlab")(t.Context())

	require.Error(t, err)
	assert.Contains(t, err.Error(), tracker.ServerOf(srv.URL))
	assert.Contains(t, err.Error(), "refused the token (HTTP 401)")
	assert.Contains(t, err.Error(), "MCP token (key my.key)")
	assert.Contains(t, err.Error(), "oh secrets set my.key")
}

// No token for the server of the MCP (the team token is for another GitLab):
// no call is made and the error lists the sources read.
func TestMCPTestGitLabWithoutTokenForTheServer(t *testing.T) {
	noGitLabEnv(t)
	srv, seen := fakeGitLab(t, "core")
	a := gitlabHub(map[string]string{"openhub.team.core.gitlab.token": "core"})
	a.Config.MCP.Gitlab.URL = srv.URL

	_, err := prepareMCPTest(a, "gitlab")(t.Context())

	var missing *tracker.MissingTokenError
	require.ErrorAs(t, err, &missing)
	assert.Equal(t, srv.URL, missing.Server)
	assert.Empty(t, seen(), "a token is not sent to a server it is not declared for")
	assert.Contains(t, err.Error(), "openhub.team.core.gitlab.token")
}

func TestMCPTestGitLabNetworkErrorNamesServerAndToken(t *testing.T) {
	noGitLabEnv(t)
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nobody listens any more
	a := gitlabHub(map[string]string{"my.key": "good"})
	a.Config.MCP.Gitlab.Token = "my.key"
	a.Config.MCP.Gitlab.URL = url

	_, err := prepareMCPTest(a, "gitlab")(t.Context())

	require.Error(t, err)
	assert.Contains(t, err.Error(), url)
	assert.Contains(t, err.Error(), "MCP token (key my.key)")
}

func fakeJira(t *testing.T, accepts string) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.URL.Path+" "+r.Header.Get("Authorization"))
		mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+accepts {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"name":"bob","displayName":"Bob"}`))
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), seen...)
	}
}

func TestMCPTestJira(t *testing.T) {
	t.Setenv("JIRA_TOKEN", "")
	t.Setenv("JIRA_URL", "")
	srv, seen := fakeJira(t, "good")

	t.Run("calls the Jira of the MCP with its token", func(t *testing.T) {
		a := gitlabHub(map[string]string{"jira.key": "good"})
		a.Config.MCP.Jira.URL = srv.URL
		a.Config.MCP.Jira.Token = "jira.key"

		detail, err := prepareMCPTest(a, "jira")(t.Context())

		require.NoError(t, err)
		assert.Equal(t, []string{"/rest/api/2/myself Bearer good"}, seen())
		assert.Contains(t, detail, "bob")
		assert.Contains(t, detail, "MCP token (key jira.key)")
	})
	t.Run("a refused token names the server, the source and the key", func(t *testing.T) {
		a := gitlabHub(map[string]string{"jira.key": "expired"})
		a.Config.MCP.Jira.URL = srv.URL
		a.Config.MCP.Jira.Token = "jira.key"

		_, err := prepareMCPTest(a, "jira")(t.Context())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "refused the token (HTTP 401)")
		assert.Contains(t, err.Error(), "MCP token (key jira.key)")
		assert.Contains(t, err.Error(), "oh secrets set jira.key")
	})
	t.Run("the environment variable comes first", func(t *testing.T) {
		t.Setenv("JIRA_TOKEN", "good")
		a := gitlabHub(map[string]string{"jira.key": "expired"})
		a.Config.MCP.Jira.URL = srv.URL
		a.Config.MCP.Jira.Token = "jira.key"

		detail, err := prepareMCPTest(a, "jira")(t.Context())

		require.NoError(t, err)
		assert.Contains(t, detail, "environment variable JIRA_TOKEN")
	})
	t.Run("no token: the sources read", func(t *testing.T) {
		a := gitlabHub(map[string]string{})
		a.Config.MCP.Jira.URL = srv.URL
		a.Config.MCP.Jira.Token = "jira.key"

		_, err := prepareMCPTest(a, "jira")(t.Context())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "JIRA_TOKEN")
		assert.Contains(t, err.Error(), "jira.key")
		assert.Contains(t, err.Error(), srv.URL)
	})
	t.Run("no URL", func(t *testing.T) {
		a := gitlabHub(map[string]string{"jira.key": "good"})
		a.Config.MCP.Jira.Token = "jira.key"

		_, err := prepareMCPTest(a, "jira")(t.Context())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "mcp.jira.url")
	})
}

// Figma and Google Slides have no call that checks their token.
func TestMCPTestHasNoTestForFigmaAndGslides(t *testing.T) {
	a := gitlabHub(nil)
	assert.Nil(t, prepareMCPTest(a, "figma"))
	assert.Nil(t, prepareMCPTest(a, "gslides"))
}
