package team

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// gitlabFake is a GitLab server that accepts one token and records every token
// it was shown.
type gitlabFake struct {
	*httptest.Server
	mu   sync.Mutex
	seen []string
}

func newGitLabFake(t *testing.T, accepts string) *gitlabFake {
	t.Helper()
	f := &gitlabFake{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := r.Header.Get("PRIVATE-TOKEN")
		f.mu.Lock()
		f.seen = append(f.seen, tok)
		f.mu.Unlock()
		if tok != accepts {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"username":"alice"}`))
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *gitlabFake) tokens() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.seen...)
}

// The team-state repository is on one GitLab (octo), the tracker on another
// (forge). The token of the team belongs to octo: the tracker must never get it.
func TestTeamTokenIsNotSentToTheTrackerOnAnotherServer(t *testing.T) {
	t.Setenv(tracker.GitLabTokenEnv, "")
	t.Setenv("GITLAB_URL", "")
	t.Setenv("OH_GITLAB_URL", "")
	octo := newGitLabFake(t, "token-octo")
	forge := newGitLabFake(t, "token-forge")

	cfg := &config.Config{Teams: []config.TeamConfig{
		{ID: "core", Enabled: true, StateRepo: octo.URL + "/acme/core.git", StatePath: t.TempDir(), MemberID: "alice"},
	}}
	secrets := fakeSecrets{config.TeamGitLabTokenKey("core"): "token-octo"}
	svc := New(cfg, secrets, nil)

	connect := func() error {
		creds, err := tracker.ResolveCredentials(t.Context(),
			svc.CredentialSource(tracker.CredentialSource{URL: forge.URL}, nil), tracker.TypeGitLab)
		if err != nil {
			return err
		}
		tr, err := tracker.New(creds)
		require.NoError(t, err)
		_, err = tr.TestConnection(t.Context())
		return err
	}

	_ = connect()
	assert.NotContains(t, forge.tokens(), "token-octo", "the token of the team repository went to the tracker server")
}

// The key of the tracker, declared for the server of the tracker, serves it
// whatever the token of the team (hub.toml tracker_token_key and tracker_token_url).
func TestTrackerTokenKeyDeclaredForItsServerIsUsed(t *testing.T) {
	t.Setenv(tracker.GitLabTokenEnv, "")
	t.Setenv("GITLAB_URL", "")
	t.Setenv("OH_GITLAB_URL", "")
	octo := newGitLabFake(t, "token-octo")
	forge := newGitLabFake(t, "token-forge")
	cfg := &config.Config{Teams: []config.TeamConfig{
		{ID: "core", Enabled: true, StateRepo: octo.URL + "/acme/core.git", StatePath: t.TempDir(), MemberID: "alice"},
	}}
	secrets := fakeSecrets{config.TeamGitLabTokenKey("core"): "token-octo", "forge.token": "token-forge"}
	svc := New(cfg, secrets, nil)

	var shared teamstate.TrackerConfig
	connect := func() (string, error) {
		eff := tracker.ResolveTrackerConfig(&shared, cfg.Tracker)
		creds, err := tracker.ResolveCredentials(t.Context(), svc.CredentialSource(tracker.NewCredentialSource(eff, nil), nil), tracker.TypeGitLab)
		if err != nil {
			return "", err
		}
		tr, err := tracker.New(creds)
		require.NoError(t, err)
		return tr.TestConnection(t.Context())
	}
	shared.TrackerURL = forge.URL

	_, err := connect()
	require.Error(t, err)
	assert.True(t, tracker.IsMissingToken(err), "no key is set for the tracker: the token of the team is not used")

	cfg.Tracker.SetTokenDecl("forge.token", "")
	user, err := connect()
	require.NoError(t, err, "a key without a declared server serves the tracker it is set for")
	assert.Equal(t, "alice", user)
	assert.NotContains(t, forge.tokens(), "token-octo")

	cfg.Tracker.SetTokenDecl("forge.token", forge.URL)
	user, err = connect()
	require.NoError(t, err)
	assert.Equal(t, "alice", user)
	assert.NotContains(t, forge.tokens(), "token-octo")
	assert.Empty(t, octo.tokens())

	key, server, implicit := svc.TrackerTokenDecl(nil)
	assert.Equal(t, "forge.token", key)
	assert.Equal(t, forge.URL, server)
	assert.False(t, implicit)

	cfg.Tracker.SetTokenDecl("forge.token", "")
	cfg.Tracker.TrackerURL = forge.URL
	_, server, implicit = svc.TrackerTokenDecl(nil)
	assert.Equal(t, forge.URL, server, "no declared server: the tracker's own")
	assert.True(t, implicit)
}

// A token the server refuses is named with its source, its key and the server
// (the message the sync, the board and the assistant print).
func TestRefusedTokenNamesItsSourceKeyAndServer(t *testing.T) {
	e := newTrackerEnv(t, "")
	e.writeTracker(t, "")
	secrets := fakeSecrets{config.TeamGitLabTokenKey("core"): "team-token"}
	// the fake server of newTrackerEnv refuses an empty token only: use one that refuses all
	refusing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	t.Cleanup(refusing.Close)
	e.cfg.Teams[0].StateRepo = refusing.URL + "/acme/core.git"
	e.writeTrackerAt(t, refusing.URL)

	creds, err := e.service(linkedProjects(), secrets).TrackerCredentials(t.Context(), nil)
	require.NoError(t, err)
	tr, err := tracker.New(creds) // the client the sync, the board and the assistant use
	require.NoError(t, err)
	_, err = tr.TestConnection(t.Context())
	require.ErrorIs(t, err, tracker.ErrTokenInvalid)
	var rejected *tracker.TokenRejectedError
	require.ErrorAs(t, err, &rejected)
	assert.Equal(t, tracker.TokenSourceTeam, rejected.Origin.Source)
	assert.Equal(t, config.TeamGitLabTokenKey("core"), rejected.Origin.Key)
	assert.Equal(t, tracker.ServerOf(refusing.URL), tracker.ServerOf(rejected.Origin.Server))
	assert.Equal(t, http.StatusForbidden, rejected.Status)
}

func TestServersOfTheTokenSources(t *testing.T) {
	cfg := hubConfig()
	cfg.Teams[1].StateRepo = "git@GitLab.Core.example:acme/core.git"
	svc := New(cfg, nil, nil)
	assert.Equal(t, "gitlab.core.example", svc.TeamRepoServer(nil))
	assert.Empty(t, New(&config.Config{}, nil, nil).TeamRepoServer(nil))

	assert.Empty(t, svc.GitLabMCPURL(nil), "no url set for the GitLab MCP: its tokens have no declared server")
	cfg.MCP.Gitlab.URL = "https://gitlab.mcp.example"
	assert.Equal(t, "https://gitlab.mcp.example", svc.GitLabMCPURL(nil))
}

// With no url set for the GitLab MCP its tokens are valid for gitlab.com, where
// the MCP server goes; an explicit url takes over.
func TestMCPTokenWithoutURLServesGitLabDotCom(t *testing.T) {
	t.Setenv(tracker.GitLabTokenEnv, "")
	t.Setenv(tracker.GitLabServerEnv, "")
	cfg := hubConfig()
	svc := New(cfg, fakeSecrets{config.DefaultGitLabTokenKey: "mcp"}, nil)

	res := svc.GitLabToken(t.Context(), "core", "https://gitlab.com/")
	assert.Equal(t, "mcp", res.Token)
	assert.Equal(t, tracker.TokenSourceMCP, res.Source)
	assert.True(t, res.Looks[len(res.Looks)-1].Implicit, "the server of the source is reported as implicit")

	assert.Empty(t, svc.GitLabToken(t.Context(), "core", "https://gitlab.forge.example").Token)

	cfg.MCP.Gitlab.URL = "https://gitlab.forge.example"
	assert.Equal(t, "mcp", svc.GitLabToken(t.Context(), "core", "https://gitlab.forge.example").Token)
	assert.Empty(t, svc.GitLabToken(t.Context(), "core", "https://gitlab.com").Token, "explicit url: the implicit default no longer applies")
}
