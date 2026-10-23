package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

const (
	aliasRepo   = "git@mon-alias:acme/core.git"
	forgeServer = "https://gitlab.forge.example"
)

// tokenStepEnv is the step « personal token of the tracker » on a team whose
// team-state config.toml is teamTOML and whose keychain holds secrets.
func tokenStepEnv(t *testing.T, teamTOML string, secrets map[string]string) (*app.App, views.WizardStep, map[string]string) {
	t.Helper()
	configtest.Home(t)
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("OH_GITLAB_URL", "")
	t.Setenv("GITLAB_URL", "")
	t.Setenv("JIRA_TOKEN", "")
	statePath := filepath.Join(t.TempDir(), "state")
	require.NoError(t, os.MkdirAll(statePath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(statePath, "config.toml"), []byte(teamTOML), 0o644))
	cfg := &config.Config{Teams: []config.TeamConfig{{ID: "core", Enabled: true, StateRepo: aliasRepo, StatePath: statePath, MemberID: "alice"}}}
	require.NoError(t, config.Save(cfg))
	if secrets == nil {
		secrets = map[string]string{}
	}
	a := &app.App{Config: cfg, Secrets: &mockSecretStore{secrets: secrets}, Projects: &mockProjectStore{}}
	step := buildTrackerTokenStep(trackerTokenStep{
		ID: "tracker_token", Label: "Token", InfoLabel: "Tracker",
		App:       func() *app.App { return a },
		Ctx:       func() context.Context { return context.Background() },
		Repo:      func() string { return aliasRepo },
		StatePath: func() string { return statePath },
		Keep:      func() bool { return true },
	})
	return a, step, secrets
}

func gitlabTrackerTOML(extra string) string {
	return "[tracker]\ntype = \"gitlab\"\nenabled = true\ntracker_url = \"" + forgeServer + "\"\n" + extra
}

// (e) The step only appeared when no secret existed under the key: a secret
// that is there for another server, or under the default key of a Jira
// tracker, hid it, and the tracker then failed for lack of a token.
func TestTrackerTokenStepChecksTheServerNotTheSecret(t *testing.T) {
	// a secret under the default key, declared for another GitLab: no token serves the tracker
	_, step, _ := tokenStepEnv(t, gitlabTrackerTOML("tracker_token_key = \"openhub.tracker.gitlab.token\"\ntracker_token_url = \"https://gitlab.other.example\"\n"),
		map[string]string{"openhub.tracker.gitlab.token": "tok"})
	assert.False(t, step.SkipIf(), "the key is declared for gitlab.other.example, the tracker is on gitlab.forge.example")

	// the same secret, declared for the server of the tracker: nothing to ask
	_, step, _ = tokenStepEnv(t, gitlabTrackerTOML("tracker_token_key = \"openhub.tracker.gitlab.token\"\ntracker_token_url = \""+forgeServer+"\"\n"),
		map[string]string{"openhub.tracker.gitlab.token": "tok"})
	assert.True(t, step.SkipIf())

	// no declaration: the default key serves the tracker's own server (former tracker key)
	_, step, _ = tokenStepEnv(t, gitlabTrackerTOML(""), map[string]string{"openhub.tracker.gitlab.token": "tok"})
	assert.True(t, step.SkipIf())

	// nothing at all
	_, step, _ = tokenStepEnv(t, gitlabTrackerTOML(""), nil)
	assert.False(t, step.SkipIf())

	// the token of the team repository (an aliased remote: no server) does not serve the tracker
	a, step, _ := tokenStepEnv(t, gitlabTrackerTOML(""), map[string]string{config.TeamGitLabTokenKey("core"): "team"})
	assert.False(t, step.SkipIf())
	// ... until the GitLab of the repository is declared and is the tracker's
	a.Config.Teams[0].GitLabURL = forgeServer
	assert.True(t, step.SkipIf(), "the token of the team serves the GitLab declared for its repository")

	// Jira only reads a declared key
	_, step, _ = tokenStepEnv(t, "[tracker]\ntype = \"jira\"\nenabled = true\ntracker_url = \"https://jira.example\"\n",
		map[string]string{"openhub.tracker.jira.token": "tok"})
	assert.False(t, step.SkipIf(), "a Jira token under a key nobody declared is never read")
	_, step, _ = tokenStepEnv(t, "[tracker]\ntype = \"jira\"\nenabled = true\ntracker_url = \"https://jira.example\"\ntracker_token_key = \"jira.key\"\n",
		map[string]string{"jira.key": "tok"})
	assert.True(t, step.SkipIf())
}

func fillTokenForm(t *testing.T, step views.WizardStep, trackerType, key, server, token string) {
	t.Helper()
	require.Empty(t, step.Validate())
	form := step.Form(tview.NewApplication(), func() {})
	set := func(label, v string) {
		item := form.GetFormItemByLabel(label)
		require.NotNil(t, item, label)
		item.(*tview.InputField).SetText(v)
	}
	if key != "" {
		set(askTrackerTokenPromptKey(), key)
	}
	if server != "" {
		set(askTrackerTokenPromptServer(), server)
	}
	if token != "" {
		set(i18n.Tf("cmd.init.wizard_tracker_token_label", trackerTypeUpper(trackerType)), token)
	}
}

func trackerTypeUpper(s string) string {
	if s == "jira" {
		return "JIRA"
	}
	return "GITLAB"
}

// The step asks the key and the server, stores the token under that key and
// declares them in hub.toml (bug 3: nothing in the wizards did).
func TestTrackerTokenStepDeclaresTheKeyAndStoresTheToken(t *testing.T) {
	a, step, secrets := tokenStepEnv(t, gitlabTrackerTOML(""), nil)
	require.False(t, step.SkipIf())
	form := step.Form(tview.NewApplication(), func() {})
	key := form.GetFormItemByLabel(askTrackerTokenPromptKey())
	require.NotNil(t, key)
	assert.Equal(t, tracker.DefaultTokenKey("gitlab"), key.(*tview.InputField).GetText(), "the default key is proposed, not written")
	assert.NotNil(t, form.GetFormItemByLabel(askTrackerTokenPromptServer()))

	fillTokenForm(t, step, "gitlab", "forge.token", "https://gitlab.forge.example", "glpat-1")
	require.NoError(t, step.OnDone())
	assert.Equal(t, "glpat-1", secrets["forge.token"])
	got, err := config.Read()
	require.NoError(t, err)
	assert.Equal(t, "forge.token", got.Tracker.TrackerTokenKey)
	assert.Equal(t, "https://gitlab.forge.example", got.Tracker.TrackerTokenURL)
	assert.Equal(t, "forge.token", a.Config.Tracker.TrackerTokenKey, "the shared config follows")
	assert.True(t, step.SkipIf(), "the cascade finds the token for the server of the tracker")
	assert.Equal(t, i18n.T("cmd.init.wizard_tracker_token_stored"), step.InfoFields()[0].Value)
}

// Keeping the default key declares nothing for GitLab (it is read as the
// former key of the tracker); Jira needs the key declared.
func TestTrackerTokenStepKeepingTheDefaultKey(t *testing.T) {
	_, step, secrets := tokenStepEnv(t, gitlabTrackerTOML(""), nil)
	require.False(t, step.SkipIf())
	fillTokenForm(t, step, "gitlab", "", "", "glpat-1")
	require.NoError(t, step.OnDone())
	assert.Equal(t, "glpat-1", secrets["openhub.tracker.gitlab.token"])
	got, err := config.Read()
	require.NoError(t, err)
	assert.Empty(t, got.Tracker.TrackerTokenKey)
	assert.True(t, step.SkipIf())

	jira := "[tracker]\ntype = \"jira\"\nenabled = true\ntracker_url = \"https://jira.example\"\n"
	_, step, secrets = tokenStepEnv(t, jira, nil)
	require.False(t, step.SkipIf())
	fillTokenForm(t, step, "jira", "", "", "jira-tok")
	require.NoError(t, step.OnDone())
	assert.Equal(t, "jira-tok", secrets["openhub.tracker.jira.token"])
	got, err = config.Read()
	require.NoError(t, err)
	assert.Equal(t, "openhub.tracker.jira.token", got.Tracker.TrackerTokenKey)
	assert.True(t, step.SkipIf())
}

// Not asked, not declared: nothing is written when the user skips.
func TestTrackerTokenStepSkippedWritesNothing(t *testing.T) {
	_, step, secrets := tokenStepEnv(t, gitlabTrackerTOML(""), nil)
	require.False(t, step.SkipIf())
	_ = step.Form(tview.NewApplication(), func() {})
	require.NoError(t, step.OnDone())
	assert.Empty(t, secrets)
	got, err := config.Read()
	require.NoError(t, err)
	assert.Empty(t, got.Tracker.TrackerTokenKey)
	assert.Equal(t, i18n.T("cmd.init.wizard_team_skipped"), step.InfoFields()[0].Value)
}

// The step and the sync read the same cascade: a token that the step finds is
// one the sync sends, and the other way round.
func TestCheckTrackerTokenAgreesWithTheCredentials(t *testing.T) {
	for _, tt := range []struct {
		name    string
		toml    string
		secrets map[string]string
		want    bool
	}{
		{"nothing", gitlabTrackerTOML(""), nil, false},
		{"former key", gitlabTrackerTOML(""), map[string]string{"openhub.tracker.gitlab.token": "t"}, true},
		{"declared elsewhere", gitlabTrackerTOML("tracker_token_key = \"k\"\ntracker_token_url = \"https://gitlab.other.example\"\n"), map[string]string{"k": "t"}, false},
		{"declared here", gitlabTrackerTOML("tracker_token_key = \"k\"\ntracker_token_url = \"" + forgeServer + "\"\n"), map[string]string{"k": "t"}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a, _, _ := tokenStepEnv(t, tt.toml, tt.secrets)
			teamCfg, err := teamstate.NewRepo(aliasRepo, a.Config.Teams[0].StatePath).LoadConfig()
			require.NoError(t, err)
			got := checkTrackerToken(t.Context(), a, "core", teamCfg)
			assert.Equal(t, tt.want, got.Found)
			_, err = tracker.ResolveCredentials(t.Context(), trackerCredentials(a, teamCfg), tracker.TypeGitLab)
			assert.Equal(t, tt.want, err == nil, "the credentials agree")
		})
	}
}

// (d) The identity step of oh team rejoin calls the GitLab the member declared
// for an aliased repository, with the token of the team, and not the alias.
func TestRejoinIdentityIsCheckedOnTheDeclaredGitLab(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("OH_GITLAB_URL", "")
	srv, seen := fakeGitLab(t, "team-token")
	a := &app.App{Config: &config.Config{}, Secrets: &mockSecretStore{secrets: map[string]string{config.TeamGitLabTokenKey("core"): "team-token"}}}
	repo := teamstate.NewRepo(aliasRepo, t.TempDir())

	require.NoError(t, validateGitLabIdentityAt(t.Context(), a, repo, &teamstate.Member{ID: "m", GitLabUsername: "alice"}, srv.URL))
	assert.Equal(t, []string{"team-token"}, seen(), "the declared GitLab was called with the token of the team")

	err := validateGitLabIdentityAt(t.Context(), a, repo, &teamstate.Member{ID: "m", GitLabUsername: "carol"}, srv.URL)
	var mismatch *identityMismatchError
	require.ErrorAs(t, err, &mismatch)
	assert.Equal(t, "alice", mismatch.AuthenticatedAs)

	// the token is looked for the declared GitLab
	assert.NotEmpty(t, gitlabTokenSourceAt(t.Context(), a, "core", aliasRepo, srv.URL))
	assert.Empty(t, gitlabTokenSourceAt(t.Context(), &app.App{Config: &config.Config{}}, "core", aliasRepo, srv.URL), "no token at all")
	assert.Equal(t, "https://gitlab.corp.example", gitlabCalledURL(aliasRepo, "https://gitlab.corp.example/"))
	assert.Equal(t, "https://gitlab.corp.example", gitlabCalledURL(aliasRepo, "gitlab.corp.example"))
	assert.Equal(t, "https://gitlab.com", gitlabCalledURL("git@gitlab.com:acme/core.git", ""))

	// registered in hub.toml: the declaration is read from there
	a.Config.Teams = []config.TeamConfig{{ID: "core", Enabled: true, StateRepo: aliasRepo, GitLabURL: srv.URL}}
	assert.Equal(t, srv.URL, registeredGitLabURL(a, aliasRepo))
	assert.NotEmpty(t, gitlabTokenSource(t.Context(), a, "core", aliasRepo))
	require.NoError(t, validateGitLabIdentity(t.Context(), a, repo, &teamstate.Member{ID: "m", GitLabUsername: "alice"}))
}
