package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// These tests run the team steps of oh init on the real engine, on a
// simulation screen, with real keys: the repository, the probe of its access,
// the access (a token over HTTPS, served by a local git over HTTP), the
// identity, the clone; and the buttons of the error screens.

// lockedSecrets is an in-memory keychain, safe for the goroutine of git and
// the one of the test (the real keychain is never used).
type lockedSecrets struct {
	mu sync.Mutex
	m  map[string]string
}

func newLockedSecrets() *lockedSecrets { return &lockedSecrets{m: map[string]string{}} }

func (s *lockedSecrets) Get(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[key], nil
}

func (s *lockedSecrets) Set(_ context.Context, key, val string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = val
	return nil
}

func (s *lockedSecrets) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
	return nil
}

func (s *lockedSecrets) List(context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var keys []string
	for k := range s.m {
		keys = append(keys, k)
	}
	return keys, nil
}

func (s *lockedSecrets) snapshot() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]string{}
	for k, v := range s.m {
		out[k] = v
	}
	return out
}

var _ domain.SecretStore = (*lockedSecrets)(nil)

// isolateGitAuth keeps what the machine answers to git (an editor's askpass,
// the credential helper of the keychain) out of the test, and out of the
// keychain.
func isolateGitAuth(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the askpass script needs a POSIX sh")
	}
	t.Setenv("GIT_ASKPASS", "")
	t.Setenv("SSH_ASKPASS", "")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// teamFlow is the team part of the init wizard, on the real steps.
type teamFlow struct {
	t       *testing.T
	a       *app.App
	ts      *initWizardTeamState
	s       *initStepState
	secrets *lockedSecrets
	ran     map[string]int // OnDone calls by step ID
	mu      sync.Mutex
	grouped bool // the grouped layout of oh init (sidebar), not the classic one of the palette
}

// newTeamFlow prepares a home, an application with a memory keychain and the
// state of the team wizard in the given mode.
func newTeamFlow(t *testing.T, mode string) *teamFlow {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	isolateGitAuth(t)
	i18ntest.Set(t, "en")
	configtest.Home(t)
	require.NoError(t, config.Save(&config.Config{}))
	f := &teamFlow{t: t, secrets: newLockedSecrets(), ran: map[string]int{}}
	f.a = newMockApp(nil, nil)
	f.a.Secrets = f.secrets
	f.a.Config = &config.Config{}
	f.a.Projects = &mockProjectStore{}
	io, _, _ := testIO()
	f.a.IO = io
	wireTestGitCredentials(t, f.a)
	f.ts = &initWizardTeamState{Ctx: t.Context(), Mode: mode}
	f.s = &initStepState{SetupMode: "full", TeamState: f.ts, AppPtr: &f.a, ProviderOptions: []string{"bedrock"}}
	return f
}

// steps are the steps of the team group, in the order of the wizard of oh init.
func (f *teamFlow) steps() []views.WizardStep {
	var steps []views.WizardStep
	steps = append(steps, buildInitWizardRepoSteps(&f.a, f.ts)...)
	steps = append(steps, buildInitWizardTeamSteps(&f.a, f.ts)...)
	steps = append(steps, buildInitWizardRejoinSteps(&f.a, f.ts)...)
	steps = append(steps, buildTeamAbandonedSteps(f.s)...)
	for i := range steps {
		id, onDone := steps[i].ID, steps[i].OnDone
		if onDone != nil && id != "" {
			steps[i].OnDone = func() error {
				f.mu.Lock()
				f.ran[id]++
				f.mu.Unlock()
				return onDone()
			}
		}
	}
	return steps
}

func (f *teamFlow) calls(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ran[id]
}

// run runs the wizard with the end of oh init, drives it from another goroutine
// and returns how it ended.
func (f *teamFlow) run(drive func(k *wizardKeys)) views.StandaloneResult {
	f.t.Helper()
	cfg := views.InlineWizardConfig{
		ID:         "wizard.test.team",
		Title:      "team",
		Steps:      f.steps(),
		OnComplete: buildInitOnComplete(f.s),
	}
	if f.grouped {
		cfg.Groups = []views.StepGroup{{Label: "Team", StartIdx: 0}}
	}
	return runWizardOnKeys(f.t, cfg, drive)
}

// runWizardOnKeys runs a wizard on a simulation screen and drives it from
// another goroutine: the wizard ends by itself once the last answer is given.
func runWizardOnKeys(t *testing.T, cfg views.InlineWizardConfig, drive func(k *wizardKeys)) views.StandaloneResult {
	t.Helper()
	k := &wizardKeys{t: t, sc: tcell.NewSimulationScreen("UTF-8"), appc: make(chan *tview.Application, 1)}
	require.NoError(t, k.sc.Init())
	for i := range cfg.Steps {
		if form := cfg.Steps[i].Form; form != nil {
			cfg.Steps[i].Form = func(app *tview.Application, done func()) *tview.Form {
				k.started.Do(func() { k.appc <- app })
				return form(app, done)
			}
			break
		}
	}
	finished, ended := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(finished)
		select {
		case k.tvApp = <-k.appc:
		case <-time.After(5 * time.Second):
			t.Errorf("the wizard did not start")
			return
		}
		// the application starts the screen again when it takes it: the size
		// is set once it runs
		k.sc.SetSize(140, 60)
		// SetSize posts no resize event: without one, a page drawn before the
		// new size stays half drawn until the next key
		_ = k.sc.PostEvent(tcell.NewEventResize(140, 60))
		k.tvApp.QueueUpdateDraw(func() {})
		drive(k)
		select {
		case <-ended:
		case <-time.After(10 * time.Second):
			if !k.failed {
				t.Errorf("the wizard still waits for an answer:\n%s", screenContents(k.sc))
			}
			k.tvApp.Stop()
		}
	}()
	res := views.RunInlineWizardStandaloneOn(views.NewInlineWizardView(cfg), k.sc)
	close(ended)
	<-finished
	return res
}

// waitFor is wizardKeys.waitFor with a longer wait: git runs behind it.
func waitForText(k *wizardKeys, text string) {
	k.t.Helper()
	if k.failed {
		return
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		read := make(chan string, 1)
		k.tvApp.QueueUpdate(func() { read <- screenContents(k.sc) })
		select {
		case txt := <-read:
			if strings.Contains(txt, text) {
				// a page that has just been drawn takes its keys a moment later
				time.Sleep(150 * time.Millisecond)
				return
			}
		case <-time.After(200 * time.Millisecond):
		}
		time.Sleep(10 * time.Millisecond)
	}
	k.failed = true
	k.t.Errorf("the screen does not show %q:\n%s", text, screenContents(k.sc))
}

// seeTeamStateOver returns the HTTPS server of the claim repositories, with a
// token, and the URL of the repository.
func httpTeamState(t *testing.T, token string) (*gittest.HTTPGit, string) {
	t.Helper()
	bare, _ := claimRepos(t)
	srv := gittest.ServeHTTP(t, bare, "team-state", "oauth2", token)
	return srv, bare
}

// laterTracker answers the tracker step that follows a joined team.
func laterTracker(k *wizardKeys) {
	waitForText(k, i18n.T("cmd.init.wizard_tracker_not_configured"))
	k.key(tcell.KeyCtrlS)
}

func retryLabel() string    { return i18n.T("v532w.error.retry") }
func skipTeamLabel() string { return i18n.T("v532w.action.skip_team") }

// A clone that fails, then « Passer l'équipe »: the wizard ends without error,
// setup_done is written, no team is added, and the whole group is skipped.
func TestTeamGroupGivenUpAfterAFailureEndsCleanly(t *testing.T) {
	f := newTeamFlow(t, "init")
	f.ts.Repo = filepath.Join(t.TempDir(), "absent.git")
	f.ts.MemberID = "alice"

	res := f.run(func(k *wizardKeys) {
		waitForText(k, i18n.T("cmd.team.init.repo_url_title"))
		k.key(tcell.KeyCtrlS)
		waitForText(k, "Cannot read the repository")
		waitForText(k, skipTeamLabel())
		k.key(tcell.KeyTab, tcell.KeyTab, tcell.KeyEnter) // retry → change the URL → skip the team
		waitForText(k, i18n.T("v532w.team.not_configured"))
		waitForText(k, i18n.T("v532w.solo.question"))
		k.key(tcell.KeyCtrlS) // no solo space
	})

	require.NoError(t, res.Err, "the error of the step the user left is not the error of the wizard")
	assert.True(t, res.Completed)
	assert.True(t, f.ts.Abandoned)
	assert.True(t, f.ts.Skipped)
	assert.Empty(t, f.ts.Mode, "no mode: the group is skipped whole")
	assert.False(t, f.ts.Configured)
	for _, id := range []string{"team_sync", "rejoin_clone", "rejoin_validate"} {
		assert.Zero(t, f.calls(id), "%s did not run", id)
	}

	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.True(t, cfg.CLI.SetupDone, "setup_done is written (the early return on the error is gone)")
	assert.Empty(t, cfg.Teams, "no [[teams]]")
	assert.False(t, f.ts.SoloSpace, "the solo space is offered, not created without a yes")
}

// The access is asked before anything is cloned or asked about the member: a
// repository that wants a token is known by the probe; the token is checked,
// kept in the keychain of oh, and given to git by the askpass for the clone,
// the pull and the push.
func TestTeamCreateOverHTTPSAsksTheTokenBeforeTheClone(t *testing.T) {
	f := newTeamFlow(t, "init")
	srv, bare := httpTeamState(t, "glpat-s3cret")
	url := srv.URL
	f.ts.Repo = url
	f.ts.MemberID = "alice"
	f.ts.DisplayName = "Alice"
	state := teamstate.NewRepo(url, config.TeamStatePath(url))

	res := f.run(func(k *wizardKeys) {
		waitForText(k, i18n.T("cmd.team.init.repo_url_title"))
		k.key(tcell.KeyCtrlS)
		// the probe was refused: the access step, nothing cloned yet
		waitForText(k, i18n.T("cmd.init.wizard_team_cred_token"))
		waitForText(k, "Git answered")
		assert.False(t, state.IsCloned(), "nothing is cloned before the access")
		assert.Zero(t, f.calls("team_sync"))
		k.key(tcell.KeyTab) // user name → token
		k.typ("glpat-s3cret")
		k.key(tcell.KeyCtrlS)
		// the identity of the member: still no clone
		waitForText(k, i18n.T("v532m.identity.id"))
		assert.False(t, state.IsCloned(), "the identity is asked before the clone")
		k.key(tcell.KeyCtrlS)
		laterTracker(k)
	})

	require.NoError(t, res.Err)
	assert.True(t, res.Completed)
	assert.True(t, f.ts.Configured, "the team is joined")
	assert.True(t, f.ts.Access.TokenStored)

	keys := f.secrets.snapshot()
	assert.Equal(t, "glpat-s3cret", keys[config.TeamGitTokenKey(url)], "the token is in the keychain of oh")
	assert.Equal(t, "oauth2", keys[config.TeamGitUserKey(url)])
	assert.True(t, state.IsCloned(), "cloned in the folder of every team assistant")
	assert.Equal(t, config.TeamStatePath(url), state.Path())
	assert.NoDirExists(t, config.DefaultTeamStatePath())
	assert.Positive(t, srv.Accepted(), "git sent the token")
	assert.Zero(t, srv.Refused())
	assert.True(t, state.HasMember(t.Context(), "alice"), "the member is registered")

	// pull and push go on with the token, without anything typed again
	ctx := t.Context()
	require.NoError(t, state.Pull(ctx))
	require.NoError(t, os.WriteFile(filepath.Join(state.Path(), "projects", "note.txt"), []byte("hi\n"), 0o644))
	require.NoError(t, state.CommitAndPush(ctx, "note", "projects/note.txt"))
	assert.Contains(t, gittest.Run(t, bare, "ls-tree", "-r", "--name-only", "main"), "projects/note.txt")

	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	require.Len(t, cfg.Teams, 1)
	assert.Equal(t, state.Path(), cfg.Teams[0].StatePath, "hub.toml points at the one path")
}

// A wrong token is refused with the words of the server, and not kept; the
// button « Réessayer » shows the form again.
func TestTeamAccessRefusesAWrongTokenAndKeepsNothing(t *testing.T) {
	f := newTeamFlow(t, "init")
	srv, _ := httpTeamState(t, "glpat-good")
	f.ts.Repo = srv.URL
	f.ts.MemberID = "alice"

	res := f.run(func(k *wizardKeys) {
		waitForText(k, i18n.T("cmd.team.init.repo_url_title"))
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.T("cmd.init.wizard_team_cred_token"))
		k.key(tcell.KeyTab)
		k.typ("glpat-bad")
		k.key(tcell.KeyCtrlS)
		waitForText(k, "refuses this token")
		assert.Empty(t, f.secrets.snapshot(), "a token the server refuses is not kept")
		k.key(tcell.KeyEnter) // retry: the form again, with what was typed
		waitForText(k, "Git answered")
		k.key(tcell.KeyTab)
		k.key(tcell.KeyCtrlA)
		for range "glpat-bad" {
			k.key(tcell.KeyDelete)
		}
		k.typ("glpat-good")
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.T("v532m.identity.id"))
		k.key(tcell.KeyCtrlS)
		laterTracker(k)
	})

	require.NoError(t, res.Err)
	assert.True(t, f.ts.Configured)
	assert.Equal(t, "glpat-good", f.secrets.snapshot()[config.TeamGitTokenKey(srv.URL)])
}

// « Réessayer » probes again: a repository that did not exist a moment ago.
func TestTeamProbeRetryAndChangeTheURL(t *testing.T) {
	t.Run("retry", func(t *testing.T) {
		f := newTeamFlow(t, "init")
		bare, _ := claimRepos(t)
		later := filepath.Join(t.TempDir(), "later.git")
		f.ts.Repo = later
		f.ts.MemberID = "alice"

		res := f.run(func(k *wizardKeys) {
			waitForText(k, i18n.T("cmd.team.init.repo_url_title"))
			k.key(tcell.KeyCtrlS)
			waitForText(k, "Cannot read the repository")
			copyTree(t, bare, later) // the repository is there now
			k.key(tcell.KeyEnter)    // the focus starts on « Retry »
			waitForText(k, i18n.T("v532m.identity.id"))
			k.key(tcell.KeyCtrlS)
			laterTracker(k)
		})

		require.NoError(t, res.Err)
		assert.True(t, f.ts.Configured)
		assert.False(t, f.ts.Abandoned)
		assert.Equal(t, 2, f.calls("team_probe"), "probed twice")
	})

	t.Run("change the URL", func(t *testing.T) {
		f := newTeamFlow(t, "init")
		bare, _ := claimRepos(t)
		bad := filepath.Join(t.TempDir(), "typo.git")
		f.ts.Repo = bad
		f.ts.MemberID = "alice"

		res := f.run(func(k *wizardKeys) {
			waitForText(k, i18n.T("cmd.team.init.repo_url_title"))
			k.key(tcell.KeyCtrlS)
			waitForText(k, "Cannot read the repository")
			k.key(tcell.KeyTab, tcell.KeyEnter) // retry → change the URL
			waitForText(k, i18n.T("cmd.team.init.repo_url_title"))
			k.key(tcell.KeyCtrlE) // end of the field
			for range bad {
				k.key(tcell.KeyBackspace2)
			}
			k.typ(bare)
			k.key(tcell.KeyCtrlS)
			waitForText(k, i18n.T("v532m.identity.id"))
			k.key(tcell.KeyCtrlS)
			laterTracker(k)
		})

		require.NoError(t, res.Err)
		assert.True(t, f.ts.Configured)
		assert.Equal(t, bare, f.ts.Repo)
	})
}

// The token is revoked between the probe and the clone: the clone fails as an
// access problem, and « Configurer l'accès » goes back to the access step,
// first of the buttons; the steps after it are asked again.
func TestTeamCloneRefusedOffersToSetTheAccessUp(t *testing.T) {
	f := newTeamFlow(t, "init")
	srv, _ := httpTeamState(t, "glpat-good")
	url := srv.URL
	require.NoError(t, f.secrets.Set(t.Context(), config.TeamGitTokenKey(url), "glpat-good")) // known from an earlier run
	f.ts.Repo = url
	f.ts.MemberID = "alice"
	f.ts.DisplayName = "Alice"

	res := f.run(func(k *wizardKeys) {
		waitForText(k, i18n.T("cmd.team.init.repo_url_title"))
		k.key(tcell.KeyCtrlS)
		// the probe passes with the token of the keychain: the identity comes next
		waitForText(k, i18n.T("v532m.identity.id"))
		assert.Equal(t, 1, f.calls("team_probe"))
		require.NoError(t, f.secrets.Set(t.Context(), config.TeamGitTokenKey(url), "glpat-revoked"))
		k.key(tcell.KeyCtrlS)
		// the clone is refused
		waitForText(k, i18n.T("v532w.action.access"))
		waitForText(k, skipTeamLabel())
		k.key(tcell.KeyLeft, tcell.KeyEnter) // retry ← : « Configurer l'accès », first button
		waitForText(k, "Git answered")
		k.key(tcell.KeyTab)
		k.typ("glpat-good")
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.T("v532m.identity.id")) // asked again
		k.key(tcell.KeyCtrlS)
		laterTracker(k)
	})

	require.NoError(t, res.Err)
	assert.True(t, f.ts.Configured)
	assert.Equal(t, "glpat-good", f.secrets.snapshot()[config.TeamGitTokenKey(url)], "the new token replaced the old one")
	assert.True(t, teamstate.NewRepo(url, config.TeamStatePath(url)).IsCloned())
}

// The buttons of an error: « Configurer l'accès » first only for an access
// problem; retry is always there; the palette leaves instead of skipping.
func TestTeamAccessErrorActions(t *testing.T) {
	i18ntest.Set(t, "en")
	repo := "https://gitlab.example.com/org/team.git"
	a := &teamAccess{Repo: &repo}
	labels := func(actions []views.ErrorAction) []string {
		var out []string
		for _, x := range actions {
			out = append(out, x.Label)
		}
		return out
	}

	auth := a.ErrorActions(errors.New("fatal: could not read Username for 'https://gitlab.example.com': terminal prompts disabled"))
	assert.Equal(t, []string{i18n.T("v532w.action.access"), retryLabel(), i18n.T("v532w.action.change_url"), skipTeamLabel()}, labels(auth))
	assert.Equal(t, stepTeamAccess, auth[0].GoToStep)
	assert.False(t, a.ForceAccess)
	auth[0].Before()
	assert.True(t, a.ForceAccess, "the access step shows although the probe passed")
	assert.True(t, auth[1].Retry)
	assert.Equal(t, stepTeamRepo, auth[2].GoToStep)
	assert.True(t, auth[3].Skip)

	other := a.ErrorActions(errors.New("could not resolve host"))
	assert.Equal(t, []string{retryLabel(), i18n.T("v532w.action.change_url"), skipTeamLabel()}, labels(other))

	gaveUp := false
	a.Abandon = func() { gaveUp = true }
	skip := a.ErrorActions(errors.New("boom"))[2]
	skip.Before()
	assert.True(t, gaveUp, "« Passer l'équipe » gives the team up before it skips")

	a.Quit = true
	palette := a.ErrorActions(errors.New("boom"))
	assert.Equal(t, i18n.T("v532w.action.quit"), palette[len(palette)-1].Label)
	assert.True(t, palette[len(palette)-1].Quit)

	onAccess := a.errorActions(errors.New("fatal: Authentication failed"), true)
	assert.NotContains(t, labels(onAccess), i18n.T("v532w.action.access"), "the access step does not offer to go to itself")
}

// The access step tells the SSH user what to do with the agent; the HTTPS one
// asks for a token and needs one.
func TestTeamAccessStepForSSHAndHTTPS(t *testing.T) {
	i18ntest.Set(t, "en")
	repo := "git@gitlab.example.com:org/team.git"
	a := &teamAccess{Repo: &repo, NeedAccess: true}
	step := a.AccessStep()
	assert.False(t, step.SkipIf(), "the probe was refused")
	assert.Empty(t, step.Validate(), "no token for SSH")
	form := step.Form(tview.NewApplication(), func() {})
	text := form.GetFormItem(0).(*tview.TextView).GetText(true)
	assert.Contains(t, text, "ssh-add")
	assert.Contains(t, text, repo)
	assert.Equal(t, i18n.T("v532w.error.retry"), form.GetButton(0).GetLabel())

	repo = "https://gitlab.example.com/org/team.git"
	assert.NotEmpty(t, step.Validate(), "a token is needed over HTTPS")
	a.Token = "  glpat-x "
	assert.Empty(t, step.Validate())
	form = step.Form(tview.NewApplication(), func() {})
	assert.Contains(t, form.GetFormItem(0).(*tview.TextView).GetText(true), "asks for access")
	assert.Equal(t, defaultGitUser, a.Username, "oauth2 for GitLab by default")

	a.NeedAccess = false
	assert.True(t, step.SkipIf(), "no access problem: no step")
	a.ForceAccess = true
	assert.False(t, step.SkipIf(), "asked for after a failure")
	a.Active = func() bool { return false }
	assert.True(t, step.SkipIf(), "the group is given up")
}

// A step of the team group skipped gives the group up: nothing of it is asked
// again, whichever step it was.
func TestTeamGroupStepsAllGiveTheTeamUpWhenSkipped(t *testing.T) {
	f := newTeamFlow(t, "rejoin")
	steps := f.steps()
	byID := map[string]views.WizardStep{}
	for _, s := range steps {
		byID[s.ID] = s
	}
	for _, id := range []string{"team_repo", "team_probe", "team_access", "team_form", "team_sync", "rejoin_clone", "rejoin_member", "rejoin_new_member", "rejoin_validate"} {
		s, ok := byID[id]
		require.True(t, ok, id)
		require.NotNil(t, s.OnSkip, "%s: skipping it gives the team up", id)
		f.ts.Skipped, f.ts.Mode, f.ts.Abandoned = false, "rejoin", false
		s.OnSkip()
		assert.True(t, f.ts.Skipped, id)
		assert.Empty(t, f.ts.Mode, id)
		assert.True(t, f.ts.Abandoned, id)
		for _, other := range []string{"team_repo", "team_probe", "team_form", "team_sync", "rejoin_clone", "rejoin_member", "rejoin_validate"} {
			assert.True(t, byID[other].SkipIf(), "after %s the group's %s is skipped", id, other)
		}
	}
	// optional steps of the group are only skipped themselves
	for _, id := range []string{"rejoin_tracker_username", "rejoin_gitlab_token", "rejoin_identity_mismatch"} {
		assert.Nil(t, byID[id].OnSkip, "%s is optional", id)
	}
}

// Nothing brings the provider in the « team » setup type when the team is
// given up: it is asked after the group, and the recap says the team is not
// configured; the solo space is offered once, not created without a yes.
func TestTeamGivenUpAsksTheProviderAgainAndOffersTheSoloSpace(t *testing.T) {
	f := newTeamFlow(t, "")
	steps := buildTeamAbandonedSteps(f.s)
	byID := map[string]views.WizardStep{}
	for _, s := range steps {
		byID[s.ID] = s
	}
	result, provider, offer := byID["team_result"], byID["provider_late"], byID["team_solo_offer"]

	f.s.SetupMode = "team"
	f.ts.Skipped = true
	assert.False(t, provider.SkipIf(), "team setup, no team: the provider is asked")
	assert.True(t, result.SkipIf(), "an explicit « Ignorer » is not a failure to report")
	assert.True(t, offer.SkipIf())

	f.ts.abandon()
	assert.False(t, result.SkipIf())
	assert.Equal(t, i18n.T("v532w.team.not_configured"), result.InfoFields()[0].Value)
	assert.False(t, offer.SkipIf())
	assert.False(t, provider.SkipIf())

	form := offer.Form(tview.NewApplication(), func() {})
	dd := form.GetFormItem(1).(*tview.DropDown)
	_, text := dd.GetCurrentOption()
	assert.Equal(t, i18n.T("v532w.solo.no"), text, "no solo space unless the user says yes")
	dd.SetCurrentOption(0)
	assert.True(t, f.ts.SoloSpace)
	assert.Equal(t, i18n.T("v532w.solo.yes"), offer.InfoFields()[0].Value)
	assert.True(t, offer.SkipIf(), "already a solo space")

	f.ts.SoloSpace = false
	f.s.SetupMode = "full"
	assert.True(t, provider.SkipIf(), "full: the provider step before the group was asked already")
	f.s.SetupMode = "team"
	f.s.ProviderSkipped = true
	assert.True(t, provider.SkipIf(), "the user skipped the provider")
	f.s.ProviderSkipped = false
	f.ts.Skipped = false
	assert.True(t, provider.SkipIf(), "a team is configured: it brings the provider")
}

// The clone of another repository in the folder is not reused by the rejoin.
func TestRejoinCloneOfAnotherRemoteIsRefused(t *testing.T) {
	configtest.Home(t)
	isolateGitAuth(t)
	i18ntest.Set(t, "en")
	bare, clone := claimRepos(t)
	other, _ := claimRepos(t)
	// the folder the rejoin uses for `other` holds the clone of `bare`
	want := config.TeamStatePath(other)
	require.NoError(t, os.MkdirAll(filepath.Dir(want), 0o755))
	require.NoError(t, os.Rename(clone, want))

	_, _, _, err := listTeamMembers(t.Context(), other, want)

	var mismatch *teamstate.RemoteMismatchError
	require.ErrorAs(t, err, &mismatch)
	assert.Equal(t, want, mismatch.Path)
	assert.Equal(t, other, mismatch.Want)
	assert.NotEqual(t, bare, other)
}

// The palette's « Rejoindre une équipe » has the same order: the access
// before the clone; and it leaves instead of skipping.
func TestRejoinWizardOfThePaletteAsksTheAccessBeforeTheClone(t *testing.T) {
	f := newTeamFlow(t, "")
	srv, _ := httpTeamState(t, "glpat-s3cret")
	url := srv.URL
	w := makeRejoinWizard(t.Context(), f.a)
	w.stateRepo = url
	cfg := buildPaletteRejoin(w)
	local := teamstate.NewRepo(url, config.TeamStatePath(url))

	res := runWizardOnKeys(t, cfg, func(k *wizardKeys) {
		waitForText(k, i18n.T("cmd.team.init.repo_url_title"))
		k.key(tcell.KeyCtrlS)
		waitForText(k, "Git answered")
		assert.False(t, local.IsCloned(), "nothing is cloned before the access")
		k.key(tcell.KeyTab)
		k.typ("glpat-s3cret")
		k.key(tcell.KeyCtrlS)
		waitForText(k, "Test User") // the members of the cloned team-state
		assert.True(t, local.IsCloned())
		k.key(tcell.KeyDown, tcell.KeyEnter)
		waitForText(k, i18n.T("v532m.handle.label"))
		k.key(tcell.KeyCtrlS) // the card has no GitLab handle: optional, left empty
		waitForText(k, i18n.T("cmd.init.wizard_tracker_not_configured"))
		k.key(tcell.KeyCtrlS)
	})

	require.NoError(t, res.Err)
	assert.True(t, res.Completed)
	assert.Equal(t, "glpat-s3cret", f.secrets.snapshot()[config.TeamGitTokenKey(url)])
	assert.True(t, w.access.TokenStored)
	assert.Positive(t, srv.Accepted())
	config.Reset()
	cfg2, err := config.Load()
	require.NoError(t, err)
	require.Len(t, cfg2.Teams, 1)
	assert.Equal(t, local.Path(), cfg2.Teams[0].StatePath, "the one path")
}

func TestRejoinWizardOfThePaletteLeavesOnTheLastButton(t *testing.T) {
	f := newTeamFlow(t, "")
	w := makeRejoinWizard(t.Context(), f.a)
	w.stateRepo = filepath.Join(t.TempDir(), "absent.git")
	cfg := views.InlineWizardConfig{ID: "wizard.team.rejoin", Title: "rejoin", Steps: []views.WizardStep{
		w.access.RepoStep(), w.access.ProbeStep(), w.access.AccessStep(), w.cloneStep(),
	}}

	res := runWizardOnKeys(t, cfg, func(k *wizardKeys) {
		waitForText(k, i18n.T("cmd.team.init.repo_url_title"))
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.T("v532w.action.quit"))
		k.key(tcell.KeyTab, tcell.KeyTab, tcell.KeyEnter) // retry → change the URL → quit
	})

	assert.False(t, res.Completed)
	assert.Error(t, res.Err, "the user left on the error of the probe")
	config.Reset()
	cfg2, err := config.Load()
	require.NoError(t, err)
	assert.Empty(t, cfg2.Teams)
}

// Ctrl+B from the identity goes back to the repository (the probe has nothing
// to edit), and going forward probes again.
func TestTeamBackFromTheIdentityLandsOnTheRepository(t *testing.T) {
	f := newTeamFlow(t, "init")
	bare, _ := claimRepos(t)
	f.ts.Repo = bare
	f.ts.MemberID = "alice"

	res := f.run(func(k *wizardKeys) {
		waitForText(k, i18n.T("cmd.team.init.repo_url_title"))
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.T("v532m.identity.id"))
		k.key(tcell.KeyCtrlB)
		waitForText(k, i18n.T("cmd.team.init.repo_url_title"))
		k.key(tcell.KeyCtrlS)
		waitForText(k, i18n.T("v532m.identity.id"))
		k.key(tcell.KeyCtrlS)
		laterTracker(k)
	})

	require.NoError(t, res.Err)
	assert.True(t, f.ts.Configured)
	assert.Equal(t, 2, f.calls("team_probe"), "probed again after the way back")
}

// The same in the grouped layout of oh init (the pages have a centered
// column and a sidebar): the error screen shows its buttons, and giving the
// team up ends the wizard cleanly.
func TestTeamGroupGivenUpInTheGroupedLayout(t *testing.T) {
	f := newTeamFlow(t, "init")
	f.grouped = true
	f.ts.Repo = filepath.Join(t.TempDir(), "absent.git")

	res := f.run(func(k *wizardKeys) {
		waitForText(k, i18n.T("cmd.team.init.repo_url_title"))
		k.key(tcell.KeyCtrlS)
		waitForText(k, "Cannot read the repository")
		waitForText(k, skipTeamLabel())
		waitForText(k, i18n.T("v532w.action.change_url"))
		k.key(tcell.KeyTab, tcell.KeyTab, tcell.KeyEnter)
		waitForText(k, i18n.T("v532w.solo.question"))
		k.key(tcell.KeyCtrlS)
	})

	require.NoError(t, res.Err)
	assert.True(t, res.Completed)
	assert.True(t, f.ts.Abandoned)
	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.True(t, cfg.CLI.SetupDone)
	assert.Empty(t, cfg.Teams)
}

// buildPaletteRejoin is the wizard of the palette's « Rejoindre une équipe »
// around the state w (the test sets the repository).
func buildPaletteRejoin(w *rejoinWizard) views.InlineWizardConfig {
	steps := []views.WizardStep{w.access.RepoStep(), w.access.ProbeStep(), w.access.AccessStep(), w.cloneStep()}
	steps = append(steps, w.join.Steps()...)
	steps = append(steps, w.trackerNameStep(), w.gitlabTokenStep(), w.validateStep(), w.identityMismatchStep(),
		w.trackerSetupStep(), w.trackerTokenStep())
	return views.InlineWizardConfig{ID: "wizard.team.rejoin", Title: "rejoin", OnComplete: w.onComplete, Steps: steps}
}
