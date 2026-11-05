package cmd

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// readOnlyGit is the git server of bare behind a token that may read and not
// write: the receive-pack service answers 403, as a GitLab does to a token with
// read_repository only.
func readOnlyGit(t *testing.T, bare, user, token string) string {
	t.Helper()
	isolateGitAuth(t)
	srv := gittest.ServeHTTP(t, bare, "team-state", user, token)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	proxy := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: u.Scheme, Host: u.Host})
	ro := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "git-receive-pack") || strings.HasSuffix(r.URL.Path, "git-receive-pack") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(ro.Close)
	return ro.URL + u.Path
}

// creationApp is an application of a person who already has a team (test-team)
// and a hub.toml on disk that says so.
func creationApp(t *testing.T) (a *app.App, existing string) {
	t.Helper()
	i18ntest.Set(t, "en")
	configtest.Home(t)
	a, _, _ = setupClaimTestApp(t)
	require.NoError(t, config.Save(a.Config))
	return a, a.Config.Teams[0].StateRepo
}

// « Créer une équipe » is a new repository, always: the person has a team
// already, and gets a second one, the first left as it is.
func TestTeamInitCreatesASecondTeam(t *testing.T) {
	a, first := creationApp(t)
	second, _ := claimRepos(t)
	s := scriptWizard(t, map[int]func(f *tview.Form){
		0: func(f *tview.Form) { typeInto(f, 0, second) },
		5: func(f *tview.Form) {
			typeInto(f, 0, "other-me")
			typeInto(f, 1, "Other Me")
		},
	})

	teamInitCmd.SetContext(t.Context())
	require.NoError(t, runTeamInit(teamInitCmd, nil))

	// repository, probe, access (skipped: it answers), clone, config, identity, notifications, policies
	assert.Equal(t, []int{0, 1, 3, 4, 5, 6, 7}, s.ran, "steps that ran")
	config.Reset()
	saved, err := config.Load()
	require.NoError(t, err)
	require.Len(t, saved.Teams, 2, "the second team is added, the first kept")
	assert.Equal(t, "testuser", saved.FindTeamByRepo(first).MemberID)
	added := saved.FindTeamByRepo(second)
	require.NotNil(t, added)
	assert.Equal(t, "other-me", added.MemberID)
	assert.Equal(t, config.TeamStatePath(second), added.StatePath)
	assert.True(t, teamstate.NewRepo(second, added.StatePath).IsCloned())
	assert.Equal(t, "Other Me", cardsOf(t, second)["other-me"].DisplayName)
	assert.Equal(t, "lead", cardsOf(t, second)["other-me"].Role, "who creates the team is its lead")
	_ = a
}

// The repository of a team the hub has is not « Créer » (it is « Reconfigurer »).
func TestTeamInitRefusesTheRepositoryOfAnExistingTeam(t *testing.T) {
	_, first := creationApp(t)
	w := newTeamAssistant(t.Context(), MustApp(), nil)
	step := w.access.RepoStep()
	w.stateRepo = first
	msg := step.Validate()
	assert.Contains(t, msg, "test-team")
	assert.Contains(t, msg, "--reconfigure")

	w.stateRepo = filepath.Join(t.TempDir(), "other.git")
	assert.Empty(t, step.Validate())
}

// Nothing touches the team-state before the first screen of the assistant:
// the wizard that is run is only given the steps (it used to clone and pull
// before it drew anything).
func TestTeamInitDoesNothingBeforeTheFirstScreen(t *testing.T) {
	a, first := creationApp(t)
	clone := a.Config.Teams[0].StatePath
	headBefore := gittest.Run(t, clone, "rev-parse", "HEAD")
	remoteBefore := gittest.Run(t, first, "rev-parse", "HEAD")
	newRepo, _ := claimRepos(t)

	prev := runCLIWizard
	t.Cleanup(func() { runCLIWizard = prev })
	runCLIWizard = func(*app.App, cliWizard) cliWizardResult { return cliWizardResult{Aborted: true} }

	teamInitCmd.SetContext(t.Context())
	require.NoError(t, runTeamInit(teamInitCmd, nil))
	reconfigure(t)
	require.NoError(t, runTeamInit(teamInitCmd, nil))

	assert.Equal(t, headBefore, gittest.Run(t, clone, "rev-parse", "HEAD"), "nothing pulled or committed")
	assert.Equal(t, remoteBefore, gittest.Run(t, first, "rev-parse", "HEAD"), "nothing pushed")
	assert.NoDirExists(t, config.TeamStatePath(newRepo))
}

// hub.toml is written as soon as the card is in the team-state: a wizard that
// stops at the notifications leaves the team configured, not a card in a team
// the hub does not know.
func TestTeamInitWritesHubTomlRightAfterTheCard(t *testing.T) {
	a, _ := creationApp(t)
	second, _ := claimRepos(t)
	s := scriptWizard(t, map[int]func(f *tview.Form){
		0: func(f *tview.Form) { typeInto(f, 0, second) },
		5: func(f *tview.Form) { typeInto(f, 0, "other-me") },
	})
	s.abortAt = 6 // the person quits at the notifications

	teamInitCmd.SetContext(t.Context())
	require.NoError(t, runTeamInit(teamInitCmd, nil))

	assert.Equal(t, []int{0, 1, 3, 4, 5}, s.ran)
	config.Reset()
	saved, err := config.Load()
	require.NoError(t, err)
	team := saved.FindTeamByRepo(second)
	require.NotNil(t, team, "the team is in hub.toml")
	assert.Equal(t, "other-me", team.MemberID)
	assert.Contains(t, cardsOf(t, second), "other-me")
	assert.Equal(t, "other-me", a.Config.FindTeamByRepo(second).MemberID, "and in memory")
}

// A repository whose team has a card with the ID the person types: the step
// refuses it, with « C'est moi »; the card of a colleague is never taken for
// the person's own because they have the same ID in another team.
func TestTeamInitCreationRecoversWithThatsMe(t *testing.T) {
	creationApp(t)
	colleagues := teamWithColleagues(t)
	w := newTeamAssistant(t.Context(), MustApp(), nil)
	w.stateRepo = colleagues
	steps := w.steps()
	clone := views.StepByID(steps, "team_clone")
	require.NotNil(t, clone)
	require.NoError(t, clone.OnDone())
	identity := views.StepByID(steps, "team_identity")
	require.NotNil(t, identity)

	// the default ID is who the person is in their other team: testuser, a
	// colleague's card in this one
	assert.Equal(t, "testuser", w.tss.MemberID)
	assert.False(t, w.tss.HasMember, "not the card of the person: it is only a suggestion")
	w.tss.MemberID, w.tss.DisplayName = "ben", "Benjamin B."
	err := identity.OnDone()
	require.ErrorIs(t, err, teamstate.ErrMemberExists)
	assert.Equal(t, "Benjamin", cardsOf(t, colleagues)["ben"].DisplayName, "the card of the colleague is untouched")

	var claim *views.ErrorAction
	for _, action := range identity.ErrorActions(err) {
		if action.Label == i18n.T("v532m.join.action_claim") {
			claim = &action
		}
	}
	require.NotNil(t, claim, "« C'est moi » is offered by the assistant")
	claim.Before()
	require.NoError(t, identity.OnDone())
	assert.Equal(t, "Benjamin", cardsOf(t, colleagues)["ben"].DisplayName, "attached as it is")
	team := MustApp().Config.FindTeamByRepo(colleagues)
	require.NotNil(t, team, "hub.toml is written")
	assert.Equal(t, "ben", team.MemberID)
}

// « Créer » asks the right to write: a token that reads is caught at the probe,
// before the clone and before any question.
func TestTeamInitProbeCatchesAReadOnlyToken(t *testing.T) {
	creationApp(t)
	bare, _ := claimRepos(t)
	wireTestGitCredentials(t, MustApp())
	ro := readOnlyGit(t, bare, "oauth2", "reader")
	MustApp().Secrets = &mockSecretStore{secrets: map[string]string{
		config.TeamGitTokenKey(ro): "reader", config.TeamGitUserKey(ro): "oauth2",
	}}

	w := newTeamAssistant(t.Context(), MustApp(), nil)
	w.stateRepo = ro
	probe := w.access.ProbeStep()
	require.NoError(t, probe.OnDone())
	assert.True(t, w.access.NeedAccess, "read-only: the access step is shown")
	assert.False(t, w.access.Open)
	assert.NoDirExists(t, config.TeamStatePath(ro), "nothing is cloned")

	// the same token is fine to rejoin: reading is enough
	rejoin := &teamAccess{Ctx: w.access.Ctx, App: w.access.App, Repo: &w.stateRepo}
	require.NoError(t, rejoin.ProbeStep().OnDone())
	assert.True(t, rejoin.Open)
}

// « Reconfigurer » on a team that was only added to hub.toml (never cloned):
// the steps used to fail with ErrNotCloned; the synchronisation clones.
func TestTeamReconfigureClonesATeamThatWasNeverCloned(t *testing.T) {
	i18ntest.Set(t, "en")
	configtest.Home(t)
	bare, _ := claimRepos(t)
	statePath := filepath.Join(t.TempDir(), "never-cloned")
	io, _, _ := testIO()
	a := &app.App{Config: &config.Config{Teams: []config.TeamConfig{{
		ID: "added", Enabled: true, StateRepo: bare, StatePath: statePath, MemberID: "testuser",
	}}}, Projects: &mockProjectStore{}, IO: io}
	application = a
	t.Cleanup(func() { application = nil })
	team := a.Config.Teams[0]

	w := newTeamAssistant(t.Context(), a, &team)
	steps := w.steps()
	require.NoError(t, views.StepByID(steps, "team_sync_state").OnDone())

	assert.True(t, teamstate.NewRepo(bare, statePath).IsCloned())
	assert.True(t, w.tss.HasMember, "the card of hub.toml is the person's own")
	assert.Equal(t, "Test User", w.tss.DisplayName)
}

// A team that cannot be reconfigured says so (archived, solo, none).
func TestReconfigurable(t *testing.T) {
	assert.True(t, reconfigurable(config.TeamConfig{Enabled: true, StateRepo: "git@x:o/t.git"}))
	assert.False(t, reconfigurable(config.TeamConfig{Enabled: false, StateRepo: "git@x:o/t.git"}), "archived")
	assert.False(t, reconfigurable(config.TeamConfig{Enabled: true, Solo: true}), "solo")
	assert.False(t, reconfigurable(config.TeamConfig{Enabled: true}), "none")

	creationApp(t)
	MustApp().Config.Teams = nil
	teamInitCmd.SetContext(t.Context())
	reconfigure(t)
	err := runTeamInit(teamInitCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oh team rejoin")
}

func TestFindExistingCloneForRemote(t *testing.T) {
	i18ntest.Set(t, "en")
	dir := t.TempDir()
	clone := filepath.Join(dir, "clone")
	gittest.Run(t, dir, "init", "-q", "-b", "main", clone)
	a := &app.App{Config: &config.Config{Teams: []config.TeamConfig{
		{ID: "acme", Enabled: true, StateRepo: "https://gitlab.example.com/org/team.git", StatePath: clone},
	}}}

	for _, remote := range []string{
		"https://gitlab.example.com/org/team.git",
		"git@gitlab.example.com:org/team.git/",
		"https://user@GitLab.example.com/org/team",
	} {
		found, ok := findExistingCloneForRemote(t.Context(), a, remote)
		require.True(t, ok, remote)
		assert.Equal(t, clone, found.Path)
		assert.Equal(t, "team acme", found.Source)
	}
	_, ok := findExistingCloneForRemote(t.Context(), a, "https://gitlab.example.com/org/other.git")
	assert.False(t, ok)

	require.NoError(t, os.RemoveAll(clone))
	_, ok = findExistingCloneForRemote(t.Context(), a, "https://gitlab.example.com/org/team.git")
	assert.False(t, ok, "a folder that is not a clone any more is no clone")
}
