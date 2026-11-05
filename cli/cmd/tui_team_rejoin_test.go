package cmd

import (
	"context"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// These tests drive the steps of the TUI rejoin wizard one by one, like the
// engine does, and pin what they ask, skip, validate and do. They keep the
// wizard the same while its builder is split.

func rejoinStepIDs(steps []views.WizardStep) []string {
	ids := make([]string, len(steps))
	for i, s := range steps {
		ids[i] = s.ID
	}
	return ids
}

func inputOf(t *testing.T, form *tview.Form, label string) *tview.InputField {
	t.Helper()
	for i := 0; i < form.GetFormItemCount(); i++ {
		if in, ok := form.GetFormItem(i).(*tview.InputField); ok && in.GetLabel() == label {
			return in
		}
	}
	t.Fatalf("no input %q in the form", label)
	return nil
}

func dropDownOf(t *testing.T, form *tview.Form) *tview.DropDown {
	t.Helper()
	for i := 0; i < form.GetFormItemCount(); i++ {
		if dd, ok := form.GetFormItem(i).(*tview.DropDown); ok {
			return dd
		}
	}
	t.Fatal("no drop-down in the form")
	return nil
}

// pickMember drives the member step like the engine does: the list opens on
// the entry that creates one's own card, each Down goes to the next member,
// Enter picks.
func pickMember(t *testing.T, tvApp *tview.Application, step views.WizardStep, downs int) (picked int) {
	t.Helper()
	container := tview.NewFlex()
	step.CustomView(tvApp, container, func() { picked++ })
	press := func(key tcell.Key) {
		h := container.InputHandler()
		require.NotNil(t, h)
		h(tcell.NewEventKey(key, 0, tcell.ModNone), func(p tview.Primitive) { tvApp.SetFocus(p) })
	}
	for range downs {
		press(tcell.KeyDown)
	}
	press(tcell.KeyEnter)
	return picked
}

func TestRejoinWizardStepsAndInitialSkips(t *testing.T) {
	i18ntest.Set(t, "en")
	cfg := buildRejoinWizard(context.Background(), &app.App{Config: &config.Config{}})
	assert.Equal(t, "wizard.team.rejoin", cfg.ID)
	assert.Equal(t, i18n.T("tui.team.rejoin"), cfg.Title)
	assert.Equal(t, []string{
		"team_repo", "team_host_scan", "team_host", "team_probe", "team_access", "rejoin_clone", "rejoin_member", "rejoin_new_member", "rejoin_handle",
		"rejoin_tracker_username", "rejoin_gitlab_token", "", "rejoin_identity_mismatch", "rejoin_tracker_setup", "rejoin_tracker_token",
	}, rejoinStepIDs(cfg.Steps))

	steps := cfg.Steps
	for i := 0; i < 6; i++ {
		assert.True(t, steps[i].Required, "step %d (repository, SSH server, probe, access, clone) cannot be left out: the wizard is only the team", i)
	}
	assert.True(t, steps[6].Required, "the member is required")
	assert.Equal(t, i18n.T("cmd.team.rejoin.validating"), steps[11].Processing)

	// nothing typed yet: the conditional steps stay out
	for _, i := range []int{1, 2, 4, 7, 8, 9, 10, 12, 13, 14} {
		require.NotNil(t, steps[i].SkipIf, "step %d is conditional", i)
		assert.True(t, steps[i].SkipIf(), "step %d is skipped at the start", i)
	}
	assert.False(t, steps[0].SkipIf(), "the repository is always asked")
	assert.False(t, steps[3].SkipIf(), "and probed")
	assert.Nil(t, steps[5].SkipIf)
	assert.False(t, steps[6].SkipIf(), "the member is always picked")
	assert.Nil(t, steps[11].SkipIf)
}

func TestRejoinWizardRepositoryStep(t *testing.T) {
	i18ntest.Set(t, "en")
	cfg := buildRejoinWizard(context.Background(), &app.App{Config: &config.Config{}})
	step := cfg.Steps[0]

	assert.Equal(t, i18n.T("cmd.team.init.validate.repo_required"), step.Validate())
	assert.Empty(t, step.InfoFields()[0].Value)

	form := step.Form(tview.NewApplication(), func() {})
	inputOf(t, form, i18n.T("cmd.team.init.repo_url_title")).SetText("  git@host:team/state.git ")
	assert.Empty(t, step.Validate(), "a repository is enough")
	assert.Equal(t, "  git@host:team/state.git ", step.InfoFields()[0].Value, "the repository is kept as typed")

	inputOf(t, form, i18n.T("cmd.init.wizard_team_gitlab_url")).SetText(" not a url ")
	assert.Equal(t, teamGitLabURLProblem("not a url"), step.Validate(), "the GitLab address is checked and trimmed")
	assert.NotEmpty(t, step.Validate())

	done := 0
	form = step.Form(tview.NewApplication(), func() { done++ })
	form.GetButton(0).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
	assert.Equal(t, 1, done)
	assert.Equal(t, i18n.T("tui.team.rejoin.next"), form.GetButton(0).GetLabel())
}

// The whole wizard on a real team-state: repository, member, rejoin, tracker.
func TestRejoinWizardRunsEveryStep(t *testing.T) {
	a, repo, _ := setupClaimTestApp(t)
	configtest.Home(t)
	i18ntest.Set(t, "en")
	require.NoError(t, config.Save(&config.Config{}))
	a.Config.Teams = nil

	cfg := buildRejoinWizard(t.Context(), a)
	steps := cfg.Steps
	tvApp := tview.NewApplication()

	// 0. repository
	form := steps[0].Form(tvApp, func() {})
	inputOf(t, form, i18n.T("cmd.team.init.repo_url_title")).SetText(repo.Remote())
	require.Empty(t, steps[0].Validate())
	require.NoError(t, steps[3].OnDone(), "a local path answers the probe")
	assert.True(t, steps[4].SkipIf(), "no access to set up")
	require.NoError(t, steps[5].OnDone(), "the team-state is cloned and its members listed")
	assert.Equal(t, i18n.T("cmd.team.init.processing_repo"), steps[5].Processing)

	// 1. member: the entry that creates one's card, then one line per member
	assert.Empty(t, steps[6].InfoFields(), "no member chosen yet")
	assert.Equal(t, 1, pickMember(t, tvApp, steps[6], 1))
	assert.Equal(t, []views.InfoField{{Label: i18n.T("cmd.init.wizard_rejoin_member_label"), Value: "Test User (testuser)"}}, steps[6].InfoFields())

	// 1a, 1b: the member picked has no card to create and nothing more to ask
	assert.True(t, steps[7].SkipIf(), "the card exists: none to create")
	assert.False(t, steps[8].SkipIf(), "the card has no GitLab handle: asked, optional")
	require.NoError(t, steps[8].OnDone(), "left empty, nothing changes")
	assert.True(t, steps[9].SkipIf(), "no tracker on another server")
	assert.True(t, steps[10].SkipIf(), "the member has no GitLab name")

	// 2. rejoin
	require.NoError(t, steps[11].OnDone())
	require.Len(t, a.Config.Teams, 1)
	assert.Equal(t, "testuser", a.Config.Teams[0].MemberID)
	assert.Equal(t, i18n.T("tui.team.rejoin.status_label"), steps[11].InfoFields()[0].Label)

	// 2b. no identity to recover
	assert.True(t, steps[12].SkipIf())

	// 3. tracker: none configured in this team-state, configure now
	assert.False(t, steps[13].SkipIf())
	trackerForm := steps[13].Form(tvApp, func() {})
	dd := dropDownOf(t, trackerForm)
	assert.Equal(t, 3, dd.GetOptionCount(), "placeholder, configure now, configure later")
	assert.Equal(t, i18n.T("cmd.init.wizard_tracker_configure_later"), steps[13].InfoFields()[0].Value)
	dd.SetCurrentOption(1)
	assert.Equal(t, i18n.T("cmd.init.wizard_tracker_configure_now"), steps[13].InfoFields()[0].Value)
	dd.SetCurrentOption(2)
	assert.Equal(t, i18n.T("cmd.init.wizard_tracker_configure_later"), steps[13].InfoFields()[0].Value)
	dd.SetCurrentOption(0)
	assert.Equal(t, i18n.T("cmd.init.wizard_tracker_configure_later"), steps[13].InfoFields()[0].Value)

	// 3b. only after « keep »
	assert.True(t, steps[14].SkipIf())

	// the end: nothing more to launch without « configure now »
	cfg.OnComplete(true, nil)
	cfg.OnComplete(false, nil)
}

// A member with a GitLab name and a tracker on another server are asked more.
func TestRejoinWizardAsksForTheTokenAndTheTrackerName(t *testing.T) {
	a, repo, _ := setupClaimTestApp(t)
	configtest.Home(t)
	i18ntest.Set(t, "en")
	require.NoError(t, config.Save(&config.Config{}))
	a.Config.Teams = nil
	gittest.Commit(t, repo.Path(), "members.toml", "[members.testuser]\ndisplay_name = \"Test User\"\nrole = \"dev\"\ngitlab_username = \"tu\"\n", "gitlab name")
	toml := "[tracker]\ntype = \"gitlab\"\nenabled = true\ntracker_url = \"https://tracker.example.com\"\ntracker_project = \"acme/app\"\n"
	gittest.Commit(t, repo.Path(), "config.toml", toml, "tracker")
	gittest.Run(t, repo.Path(), "push", "-q")

	cfg := buildRejoinWizard(t.Context(), a)
	steps := cfg.Steps
	tvApp := tview.NewApplication()
	form := steps[0].Form(tvApp, func() {})
	inputOf(t, form, i18n.T("cmd.team.init.repo_url_title")).SetText(repo.Remote())
	require.NoError(t, steps[3].OnDone())
	require.NoError(t, steps[5].OnDone())
	pickMember(t, tvApp, steps[6], 1)
	assert.True(t, steps[8].SkipIf(), "the card has a GitLab handle: none to ask")

	// the tracker is on another server than the team repository, the member has no name there
	assert.False(t, steps[9].SkipIf())
	nameForm := steps[9].Form(tvApp, func() {})
	inputOf(t, nameForm, i18n.T("tui.team.rejoin.tracker_label")).SetText(" tu-tracker ")
	assert.Equal(t, "tu-tracker", steps[9].InfoFields()[0].Value)

	// a member with a GitLab name chooses what to do with the token
	assert.False(t, steps[10].SkipIf())
	assert.Equal(t, i18n.T("cmd.init.wizard_rejoin_token_choice_required"), steps[10].Validate())
	tokenForm := steps[10].Form(tvApp, func() {})
	dd := dropDownOf(t, tokenForm)
	options := dd.GetOptionCount()
	require.GreaterOrEqual(t, options, 3, "placeholder, new, skip (and reuse when a token is known)")
	dd.SetCurrentOption(options - 2) // new token
	assert.Equal(t, i18n.T("cmd.init.wizard_rejoin_token_new_required"), steps[10].Validate())
	inputOf(t, tokenForm, i18n.T("cmd.init.wizard_rejoin_gitlab_token_label")).SetText("glpat-x")
	assert.Empty(t, steps[10].Validate())
	assert.Equal(t, i18n.T("tui.team.wizard.token_stored"), steps[10].InfoFields()[0].Value)
	dd.SetCurrentOption(options - 1) // skip
	assert.Equal(t, i18n.T("cmd.init.wizard_team_skipped"), steps[10].InfoFields()[0].Value)
}

// The identity step validates its choice before it asks for anything.
func TestRejoinWizardIdentityStepValidation(t *testing.T) {
	i18ntest.Set(t, "en")
	cfg := buildRejoinWizard(context.Background(), &app.App{Config: &config.Config{}})
	step := cfg.Steps[12]
	assert.Equal(t, i18n.T("cmd.init.wizard_rejoin_token_choice_required"), step.Validate(), "no choice yet")
	assert.Equal(t, i18n.T("tui.team.wizard.identity"), step.InfoFields()[0].Label)
	assert.Equal(t, i18n.T("tui.team.wizard.identity_verified"), step.InfoFields()[0].Value)
}

func TestMemberTrackerUsername(t *testing.T) {
	assert.Empty(t, memberTrackerUsername(nil, "x"))
}

// A team-state whose tracker is set offers to keep it, reconfigure it or leave
// it for later; « keep » opens the step of the token.
func TestRejoinWizardTrackerChoices(t *testing.T) {
	a, repo, _ := setupClaimTestApp(t)
	configtest.Home(t)
	i18ntest.Set(t, "en")
	require.NoError(t, config.Save(&config.Config{}))
	a.Config.Teams = nil
	toml := "[tracker]\ntype = \"gitlab\"\nenabled = true\ntracker_url = \"https://gitlab.com\"\ntracker_project = \"acme/app\"\n"
	gittest.Commit(t, repo.Path(), "config.toml", toml, "tracker")
	gittest.Run(t, repo.Path(), "push", "-q")

	cfg := buildRejoinWizard(t.Context(), a)
	steps := cfg.Steps
	tvApp := tview.NewApplication()
	form := steps[0].Form(tvApp, func() {})
	inputOf(t, form, i18n.T("cmd.team.init.repo_url_title")).SetText(repo.Remote())
	require.NoError(t, steps[3].OnDone())
	require.NoError(t, steps[5].OnDone())

	tracker := steps[13]
	assert.False(t, tracker.SkipIf())
	trackerForm := tracker.Form(tvApp, func() {})
	dd := dropDownOf(t, trackerForm)
	assert.Equal(t, 4, dd.GetOptionCount(), "placeholder, keep, reconfigure, later")

	dd.SetCurrentOption(1)
	assert.Equal(t, i18n.T("cmd.init.wizard_tracker_keep_existing"), tracker.InfoFields()[0].Value)
	dd.SetCurrentOption(2)
	assert.Equal(t, i18n.T("cmd.init.wizard_tracker_configure_now"), tracker.InfoFields()[0].Value)
	assert.True(t, steps[14].SkipIf(), "no token step unless the tracker is kept")
	dd.SetCurrentOption(3)
	assert.Equal(t, i18n.T("cmd.init.wizard_tracker_configure_later"), tracker.InfoFields()[0].Value)
	assert.NoError(t, tracker.OnDone())

	// « reconfigure » then the end: the discovery starts later, without a shell here
	dd.SetCurrentOption(2)
	cfg.OnComplete(true, nil)
}

func newRejoinWizard(secrets *mockSecretStore) *rejoinWizard {
	a := &app.App{Config: &config.Config{}}
	if secrets != nil {
		a.Secrets = secrets
	}
	w := makeRejoinWizard(context.Background(), a)
	w.stateRepo = "git@host:acme/state.git"
	return w
}

// A new GitLab token is stored under the key of the team; any other choice
// stores nothing.
func TestRejoinWizardStoresTheNewToken(t *testing.T) {
	i18ntest.Set(t, "en")
	secrets := &mockSecretStore{secrets: map[string]string{}}
	w := newRejoinWizard(secrets)
	step := w.gitlabTokenStep()

	w.tokenChoice, w.gitlabToken = "skip", "glpat-x"
	require.NoError(t, step.OnDone())
	assert.Empty(t, secrets.secrets)

	w.tokenChoice, w.gitlabToken = "new", ""
	require.NoError(t, step.OnDone())
	assert.Empty(t, secrets.secrets, "no token typed")

	w.tokenChoice, w.gitlabToken = "new", "glpat-x"
	require.NoError(t, step.OnDone())
	assert.Equal(t, map[string]string{config.TeamGitLabTokenKey(config.RepoNameFromRemote(w.stateRepo)): "glpat-x"}, secrets.secrets)

	w.a.Secrets = nil
	require.NoError(t, step.OnDone(), "no keychain: nothing to store")

	w.tokenChoice = "reuse"
	assert.Equal(t, i18n.T("cmd.init.wizard_rejoin_token_reuse_short"), step.InfoFields()[0].Value)
}

// The step of the identity shows why the identity does not match, and the
// choice « skip » lets the rejoin go on.
func TestRejoinWizardIdentityMismatch(t *testing.T) {
	i18ntest.Set(t, "en")
	w := newRejoinWizard(&mockSecretStore{secrets: map[string]string{}})
	step := w.identityMismatchStep()
	assert.True(t, step.SkipIf(), "no mismatch: no step")

	w.identityMismatch = &identityMismatchError{AuthenticatedAs: "project_1_bot", ExpectedUser: "tu", MemberID: "testuser", IsBot: true}
	assert.False(t, step.SkipIf())
	form := step.Form(tview.NewApplication(), func() {})
	text := form.GetFormItem(0).(*tview.TextView).GetText(true)
	assert.Equal(t, i18n.Tf("cmd.init.wizard_identity_bot_detected", "project_1_bot"), text)
	w.identityMismatch.IsBot = false
	form = step.Form(tview.NewApplication(), func() {})
	assert.Equal(t, i18n.Tf("cmd.init.wizard_identity_user_mismatch", "project_1_bot", "testuser", "tu"),
		form.GetFormItem(0).(*tview.TextView).GetText(true))

	dd := dropDownOf(t, form)
	require.Equal(t, 3, dd.GetOptionCount())
	assert.Equal(t, i18n.T("cmd.init.wizard_rejoin_token_choice_required"), step.Validate(), "no choice")
	dd.SetCurrentOption(1)
	assert.Equal(t, "verify", w.mismatchChoice)
	assert.Equal(t, i18n.T("cmd.init.wizard_rejoin_token_new_required"), step.Validate(), "verify needs a token")
	inputPassword := form.GetFormItem(3).(*tview.InputField)
	inputPassword.SetText("glpat-y")
	assert.Equal(t, "glpat-y", w.mismatchPAT)
	assert.Empty(t, step.Validate())
	assert.Equal(t, i18n.T("tui.team.wizard.identity_verified"), step.InfoFields()[0].Value)

	dd.SetCurrentOption(2)
	assert.Equal(t, "skip", w.mismatchChoice)
	require.NoError(t, step.OnDone())
	assert.Nil(t, w.identityMismatch, "skipped: the mismatch is dropped")
	assert.True(t, step.SkipIf())
	assert.Equal(t, i18n.T("cmd.init.wizard_identity_skip"), step.InfoFields()[0].Value)
}
