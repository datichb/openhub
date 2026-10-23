package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// answerPrompts plays the answers of the user to the questions asked with
// fmt.Scanln (askInput, askYN, askSelect).
func answerPrompts(t *testing.T, answers ...string) {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	_, err = w.WriteString(strings.Join(answers, "\n") + "\n")
	require.NoError(t, err)
	require.NoError(t, w.Close())
	prev := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = prev; _ = r.Close() })
}

// trackerWizardEnv is a team whose team-state is a local repository without
// tracker, and a GitLab fake server that accepts one token.
func trackerWizardEnv(t *testing.T, accepts string) (a *app.App, repo *teamstate.Repo, out *bytes.Buffer, serverURL string, secrets map[string]string) {
	t.Helper()
	configtest.Home(t)
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("OH_GITLAB_URL", "")
	t.Setenv("GITLAB_URL", "")
	srv, _ := fakeGitLab(t, accepts)
	statePath := filepath.Join(t.TempDir(), "state")
	repo, err := teamstate.InitSolo(t.Context(), statePath, teamstate.Member{ID: "alice"})
	require.NoError(t, err)
	cfg := &config.Config{Teams: []config.TeamConfig{
		{ID: "core", Enabled: true, StateRepo: "git@mon-alias:acme/core.git", StatePath: statePath, MemberID: "alice"},
	}}
	require.NoError(t, config.Save(cfg))
	out = &bytes.Buffer{}
	io := app.DefaultIOStreams()
	io.Out, io.ErrOut = out, out
	secrets = map[string]string{}
	a = &app.App{Config: cfg, IO: io, Projects: &mockProjectStore{}, Secrets: &mockSecretStore{secrets: secrets}}
	return a, repo, out, srv.URL, secrets
}

// Bug 3: oh team config → Tracker sync wrote tracker_token_key to the team
// config with the default value (openhub.tracker.<type>.token), whatever the
// user answered and without saying so.
func TestTeamConfigTrackerSyncDoesNotFillTheTokenKeyOnItsOwn(t *testing.T) {
	a, repo, out, serverURL, _ := trackerWizardEnv(t, "never")
	teamCfg, err := repo.LoadConfig()
	require.NoError(t, err)

	// GitLab, the URL, no key, no token, then the yes/no questions and no project
	answerPrompts(t, "1", serverURL, "", "n", "n", "n", "n", "n", "")
	require.NoError(t, configureTrackerSync(t.Context(), a, repo, teamCfg, true, false))

	saved, err := repo.LoadConfig()
	require.NoError(t, err)
	assert.Empty(t, saved.Tracker.TrackerTokenKey, "the key is declared by the user or not at all")
	assert.Empty(t, saved.Tracker.TrackerTokenURL)
	assert.Contains(t, out.String(), "openhub.tracker.gitlab.token", "what is read without a declared key is said")
}

// The user declares the key and the server: both are saved with the team
// config and the token is stored under that key.
func TestTeamConfigTrackerSyncDeclaresTheKeyAndTheServer(t *testing.T) {
	a, repo, _, serverURL, secrets := trackerWizardEnv(t, "glpat-1")
	teamCfg, err := repo.LoadConfig()
	require.NoError(t, err)

	// GitLab, the URL, the key, its server, the token, then the rest
	answerPrompts(t, "1", serverURL, "forge.token", serverURL, "y", "glpat-1", "n", "n", "n", "n", "")
	require.NoError(t, configureTrackerSync(t.Context(), a, repo, teamCfg, true, false))

	saved, err := repo.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "forge.token", saved.Tracker.TrackerTokenKey)
	assert.Equal(t, serverURL, saved.Tracker.TrackerTokenURL)
	assert.Equal(t, "glpat-1", secrets["forge.token"])
	assert.Empty(t, secrets[tracker.DefaultTokenKey("gitlab")])
}

// A server that designates nothing is refused: the previous value stays.
func TestAskTrackerTokenDecl(t *testing.T) {
	answers := map[string]string{}
	ask := func(prompt, def string) string {
		if v, ok := answers[prompt]; ok {
			return v
		}
		return def
	}
	var out bytes.Buffer
	keyPrompt := askTrackerTokenPromptKey()
	serverPrompt := askTrackerTokenPromptServer()

	answers[keyPrompt], answers[serverPrompt] = "forge.token", "not a server"
	key, server := askTrackerTokenDecl(&out, ask, declTeam, "gitlab", "", "https://old.example")
	assert.Equal(t, "forge.token", key)
	assert.Equal(t, "https://old.example", server, "an invalid server leaves the previous one")
	assert.Contains(t, out.String(), "not a server")

	answers[keyPrompt] = ""
	key, server = askTrackerTokenDecl(&out, ask, declTeam, "gitlab", "forge.token", "https://old.example")
	assert.Empty(t, key)
	assert.Empty(t, server, "no key: no server either")

	delete(answers, serverPrompt)
	answers[keyPrompt] = "k"
	key, server = askTrackerTokenDecl(&out, ask, declLocal, "jira", "k", "ignored")
	assert.Equal(t, "k", key)
	assert.Empty(t, server, "Jira has no server rule")

	answers[keyPrompt] = "-"
	key, server = askTrackerTokenDecl(&out, ask, declLocal, "gitlab", "forge.token", "https://old.example")
	assert.Empty(t, key, "- clears the key (an empty answer keeps the proposal in the terminal)")
	assert.Empty(t, server)
}
