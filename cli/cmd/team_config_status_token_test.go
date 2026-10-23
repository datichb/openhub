package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// fakeGitLab accepts one token and records the tokens it was shown.
func fakeGitLab(t *testing.T, accepts string) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := r.Header.Get("PRIVATE-TOKEN")
		mu.Lock()
		seen = append(seen, tok)
		mu.Unlock()
		if tok != accepts {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"username":"alice"}`))
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), seen...)
	}
}

// statusHub is a hub whose team repository is on the GitLab "octo" and whose
// tracker is on the GitLab "forge" (two fake servers).
func statusHub(t *testing.T) (a *app.App, out *bytes.Buffer, octoSeen, forgeSeen func() []string, forgeURL string) {
	t.Helper()
	configtest.Home(t)
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("OH_GITLAB_URL", "")
	t.Setenv("GITLAB_URL", "")
	octo, octoSeen := fakeGitLab(t, "token-octo")
	forge, forgeSeen := fakeGitLab(t, "token-forge")

	statePath := filepath.Join(t.TempDir(), "state")
	_, err := teamstate.InitSolo(t.Context(), statePath, teamstate.Member{ID: "alice"})
	require.NoError(t, err)
	toml := "[tracker]\ntype = \"gitlab\"\nenabled = true\ntracker_url = \"" + forge.URL + "\"\ntracker_project = \"acme/app\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(statePath, "config.toml"), []byte(toml), 0o644))

	cfg := &config.Config{Teams: []config.TeamConfig{
		{ID: "core", Enabled: true, StateRepo: octo.URL + "/acme/core.git", StatePath: statePath, MemberID: "alice"},
	}}
	out = &bytes.Buffer{}
	io := app.DefaultIOStreams()
	io.Out, io.ErrOut = out, out
	a = &app.App{Config: cfg, IO: io, Projects: &mockProjectStore{}, Secrets: &mockSecretStore{secrets: map[string]string{
		config.TeamGitLabTokenKey("core"): "token-octo",
		"forge.token":                     "token-forge",
	}}}
	application = a
	teamConfigStatusCmd.SetContext(t.Context())
	t.Cleanup(func() { application = nil })
	return a, out, octoSeen, forgeSeen, forge.URL
}

// oh team config status reads the same cascade as the sync: the token of the
// team repository (octo) is not sent to the tracker (forge), and the
// tracker_token_key of hub.toml is read with the server declared for it.
func TestTeamConfigStatusUsesTheCascadeOfTheTracker(t *testing.T) {
	a, out, octoSeen, forgeSeen, forgeURL := statusHub(t)

	require.NoError(t, runTeamConfigStatus(teamConfigStatusCmd, nil))
	assert.NotContains(t, forgeSeen(), "token-octo", "the token of the team repository went to the tracker")
	assert.Empty(t, octoSeen(), "nothing is asked of the team repository server")
	assert.Contains(t, out.String(), "no GitLab token for the server")
	assert.Contains(t, out.String(), "tracker_token_url", "the error says where to declare the key")

	out.Reset()
	a.Config.Tracker.SetTokenDecl("forge.token", forgeURL)
	require.NoError(t, runTeamConfigStatus(teamConfigStatusCmd, nil))
	assert.Contains(t, forgeSeen(), "token-forge")
	assert.NotContains(t, forgeSeen(), "token-octo")
	assert.Contains(t, out.String(), "Connected to "+forgeURL+" as @alice", "hub.toml tracker_token_key is read by the status")
	assert.Contains(t, out.String(), "token_key:     forge.token")
	assert.Contains(t, out.String(), "token_url:     "+forgeURL)
}

// A token refused by the server is named with its source, its key and the
// server (bug 5).
func TestTeamConfigStatusNamesTheTokenTheServerRefused(t *testing.T) {
	a, out, _, _, forgeURL := statusHub(t)
	a.Secrets = &mockSecretStore{secrets: map[string]string{"forge.token": "expired"}}
	a.Config.Tracker.SetTokenDecl("forge.token", forgeURL)

	require.NoError(t, runTeamConfigStatus(teamConfigStatusCmd, nil))
	assert.Contains(t, out.String(), "refused the token (HTTP 401)")
	assert.Contains(t, out.String(), "tracker_token_key “forge.token”")
	assert.Contains(t, out.String(), "oh secrets set forge.token")
}
