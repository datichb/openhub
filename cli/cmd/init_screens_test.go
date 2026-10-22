package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	providerPkg "github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// initRun is what a pass through the init wizard went through.
type initRun struct {
	screens   []string // the steps the wizard displayed, in order
	completed bool
}

// driveInitWizard mounts the real wizard engine on the real steps and
// confirms every screen it displays. The steps keep their rendering and their
// skip rules; their checks and their writes are dropped (they are tested one by
// one), so the pass counts what the engine shows for the given choices.
func driveInitWizard(t *testing.T, setupMode, teamChoice string) initRun {
	t.Helper()
	configtest.Home(t)
	i18ntest.Restore(t)

	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	a.Projects = &mockProjectStore{projects: []domain.Project{}}
	cfg, s := buildInitWizardConfig(a)

	var run initRun
	cfg.OnComplete = func(completed bool, _ error) { run.completed = completed }
	for i := range cfg.Steps {
		step := &cfg.Steps[i]
		name := step.ID
		if name == "" {
			name = step.Label
		}
		id := step.ID
		custom, form := step.CustomView, step.Form
		step.Validate, step.OnDone = nil, nil
		confirm := func(onDone func()) {
			run.screens = append(run.screens, name)
			if id == "welcome" {
				s.SetupMode = setupMode
			}
			if id == "team_mode" {
				switch teamChoice {
				case "skip":
					s.TeamState.Skipped = true
				case "init", "rejoin":
					s.TeamState.Mode = teamChoice
				}
			}
			onDone()
		}
		switch {
		case custom != nil:
			step.CustomView = func(app *tview.Application, c *tview.Flex, onDone func()) {
				custom(app, c, func() {})
				confirm(onDone)
			}
		case form != nil:
			step.Form = func(app *tview.Application, onDone func()) *tview.Form {
				f := form(app, func() {})
				confirm(onDone)
				return f
			}
		}
	}

	wiz := views.NewInlineWizardView(*cfg)
	require.NotPanics(t, func() { wiz.Mount(tview.NewFlex().SetDirection(tview.FlexRow), tview.NewApplication()) })
	wiz.Unmount()
	return run
}

// TestInitWizard_ScreenCount counts the screens the wizard displays on each
// path. Before v5.0 (U-10): solo 8, team 7 (team skipped) / 8 (new team),
// full 9 / 10 — language, provider intro and form, team gateway, project intro
// and form, MCP intro and form.
func TestInitWizard_ScreenCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping the wizard pass in short mode")
	}
	for _, tt := range []struct {
		name, mode, team string
		want             []string
	}{
		{"solo", "solo", "", []string{"welcome", "provider", "project"}},
		{"team, no team set up", "team", "skip", []string{"welcome", "team_mode", "project"}},
		{"team, new team", "team", "init", []string{"welcome", "team_mode", "team_form", "project"}},
		{"full, no team set up", "full", "skip", []string{"welcome", "provider", "team_mode", "project"}},
		{"full, new team", "full", "init", []string{"welcome", "provider", "team_mode", "team_form", "project"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			run := driveInitWizard(t, tt.mode, tt.team)
			assert.Equal(t, tt.want, run.screens)
			assert.True(t, run.completed)
		})
	}
}

// The groups of the sidebar start at the steps they are named after, in order,
// and the team group only shows for the "team" and "full" setup types.
func TestInitWizard_Groups(t *testing.T) {
	i18ntest.Restore(t)
	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	a.Projects = &mockProjectStore{projects: []domain.Project{}}
	cfg, s := buildInitWizardConfig(a)

	wantStart := []string{"welcome", "provider", "team_mode", "project"}
	require.Len(t, cfg.Groups, len(wantStart))
	prev := -1
	for i, g := range cfg.Groups {
		require.GreaterOrEqual(t, g.StartIdx, 0, "group %d starts at a step that exists", i)
		assert.Greater(t, g.StartIdx, prev, "groups follow the order of the steps")
		assert.Equal(t, wantStart[i], cfg.Steps[g.StartIdx].ID)
		prev = g.StartIdx
	}

	hidden := func(mode string) []bool {
		s.SetupMode = mode
		var out []bool
		for _, g := range cfg.Groups {
			out = append(out, g.Hidden != nil && g.Hidden())
		}
		return out
	}
	assert.Equal(t, []bool{false, false, true, false}, hidden("solo"), "solo: no team group")
	assert.Equal(t, []bool{false, true, false, false}, hidden("team"), "team: the provider comes from the team")
	assert.Equal(t, []bool{false, false, false, false}, hidden("full"))

	provider := views.StepByID(cfg.Steps, "provider")
	require.NotNil(t, provider)
	s.SetupMode = "team"
	assert.True(t, provider.SkipIf(), "team: the provider screen is skipped")
	s.SetupMode = "solo"
	assert.False(t, provider.SkipIf())
	s.ProviderSkipped = true
	assert.True(t, provider.SkipIf(), "skipped by the user")
}

func TestInitWizard_DefaultSetupMode(t *testing.T) {
	i18ntest.Restore(t)
	a := newMockApp(nil, nil)
	a.Projects = &mockProjectStore{}

	a.Config = &config.Config{}
	_, s := buildInitWizardConfig(a)
	assert.Equal(t, "solo", s.SetupMode, "solo by default")

	a.Config = &config.Config{Teams: []config.TeamConfig{{ID: "t", Enabled: true, StateRepo: "https://example.com/t.git", MemberID: "me"}}}
	_, s = buildInitWizardConfig(a)
	assert.Equal(t, "full", s.SetupMode, "a team already set up is not skipped silently")
}

// selectsOf are the InlineSelect items of the form of a page.
func selectsOf(t *testing.T, container *tview.Flex) []*widgets.InlineSelect {
	t.Helper()
	for i := 0; i < container.GetItemCount(); i++ {
		form, ok := container.GetItem(i).(*tview.Form)
		if !ok || form.GetFormItemCount() == 0 {
			continue
		}
		var out []*widgets.InlineSelect
		for j := 0; j < form.GetFormItemCount(); j++ {
			if sel, ok := form.GetFormItem(j).(*widgets.InlineSelect); ok {
				out = append(out, sel)
			}
		}
		return out
	}
	return nil
}

func press(sel *widgets.InlineSelect, keys ...tcell.Key) {
	handler := sel.InputHandler()
	for _, k := range keys {
		handler(tcell.NewEventKey(k, 0, tcell.ModNone), func(tview.Primitive) {})
	}
}

// The language and the setup type are on the same page; both apply at once.
func TestInitWizard_WelcomeLanguageAndSetupType(t *testing.T) {
	i18ntest.Restore(t)
	s := newTestState()
	s.SelectedLang, s.LangIdx = "en", 1
	step := buildWelcomeStep(s)
	*s.Steps = []views.WizardStep{step}

	tvApp := tview.NewApplication()
	container := tview.NewFlex().SetDirection(tview.FlexRow)
	step.CustomView(tvApp, container, func() {})

	selects := selectsOf(t, container)
	require.Len(t, selects, 2, "language and setup type")
	assert.Equal(t, "solo", s.SetupMode, "solo is preselected")
	assert.Equal(t, 0, selects[1].GetSelectedIndex())

	press(selects[1], tcell.KeyDown, tcell.KeyEnter)
	assert.Equal(t, "team", s.SetupMode)
	press(selects[1], tcell.KeyDown, tcell.KeyEnter)
	assert.Equal(t, "full", s.SetupMode)

	press(selects[0], tcell.KeyUp, tcell.KeyEnter)
	assert.Equal(t, "fr", s.SelectedLang)
	assert.Equal(t, 0, s.LangIdx)
	assert.Equal(t, "fr", i18n.Locale(), "the page is drawn again in the new language")
	assert.True(t, s.LangChanged)
	assert.Equal(t, "full", s.SetupMode, "changing the language keeps the setup type")

	// Drawn again: the focus stays on the form, on the setup type.
	container2 := tview.NewFlex().SetDirection(tview.FlexRow)
	step.CustomView(tvApp, container2, func() {})
	assert.False(t, s.LangChanged)
	selects = selectsOf(t, container2)
	require.Len(t, selects, 2)
	assert.Equal(t, 2, selects[1].GetSelectedIndex(), "the setup type chosen before is still selected")
	assert.Equal(t, setupModeLabel("full"), selects[1].GetSelectedValue())

	fields := step.InfoFields()
	require.Len(t, fields, 2)
	assert.Equal(t, "Français", fields[0].Value)
	assert.Equal(t, setupModeLabel("full"), fields[1].Value)
}

// U-09: the description of the setup type is visible whatever the height:
// listed under each option when there is room, else one line under the selector.
func TestInitWizard_WelcomeDescriptionAlwaysVisible(t *testing.T) {
	i18ntest.Restore(t)
	orig := availableWizardHeight
	t.Cleanup(func() { availableWizardHeight = orig })

	height := func(rows int) (listed, withBanner bool, h int) {
		availableWizardHeight = func() int { return rows }
		s := newTestState()
		step := buildWelcomeStep(s)
		container := tview.NewFlex().SetDirection(tview.FlexRow)
		step.CustomView(tview.NewApplication(), container, func() {})
		sels := selectsOf(t, container)
		require.Len(t, sels, 2)
		h = sels[1].GetFieldHeight()
		n := len(initSetupModes)
		return h > n+1+1, welcomeIntroText(rows >= welcomeBannerMinHeight) != welcomeIntroText(false), h
	}

	n := len(initSetupModes)
	for _, rows := range []int{12, 20, welcomeDescMinHeight - 1} {
		listed, _, h := height(rows)
		assert.False(t, listed, "%d rows: no room for the list of descriptions", rows)
		assert.Equal(t, 1+n+1, h, "%d rows: label + options + the description of the highlighted option", rows)
	}
	for _, rows := range []int{welcomeDescMinHeight, welcomeBannerMinHeight, 60} {
		listed, withBanner, _ := height(rows)
		assert.True(t, listed, "%d rows: every option shows its description", rows)
		assert.Equal(t, rows >= welcomeBannerMinHeight, withBanner, "%d rows: banner", rows)
	}
}

func TestPrefillProjectFromCwd(t *testing.T) {
	i18ntest.Restore(t)
	i18n.SetLocale("en")

	t.Run("folder name without git remote", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "my-app")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		t.Chdir(dir)
		t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
		s := newTestState()
		source := prefillProjectFromCwd(s)
		resolved, err := filepath.EvalSymlinks(dir)
		require.NoError(t, err)
		gotPath, err := filepath.EvalSymlinks(s.ProjectPath)
		require.NoError(t, err)
		assert.Equal(t, resolved, gotPath, "the path is the current folder")
		assert.Equal(t, "my-app", s.ProjectName)
		assert.Equal(t, i18n.T("cmd.init.wizard_detected_cwd"), source)
	})

	t.Run("git remote name wins", func(t *testing.T) {
		dir := setupGitRepo(t)
		t.Chdir(dir)
		out, err := exec.Command("git", "-C", dir, "remote", "add", "origin", "https://example.com/team/shiny-repo.git").CombinedOutput()
		require.NoError(t, err, string(out))
		s := newTestState()
		source := prefillProjectFromCwd(s)
		assert.Equal(t, "shiny-repo", s.ProjectName)
		assert.Equal(t, "git remote", source)
		assert.True(t, filepath.IsAbs(s.ProjectPath))
	})

	t.Run("what is already set is kept", func(t *testing.T) {
		t.Chdir(t.TempDir())
		s := newTestState()
		s.ProjectName, s.ProjectPath = "kept", "/somewhere"
		assert.Empty(t, prefillProjectFromCwd(s))
		assert.Equal(t, "kept", s.ProjectName)
		assert.Equal(t, "/somewhere", s.ProjectPath)
	})
}

// The provider detection (environment, AWS credentials…) still preselects
// the provider on the provider screen.
func TestInitWizard_ProviderAutoDetection(t *testing.T) {
	i18ntest.Restore(t)
	for _, k := range []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_BEARER_TOKEN_BEDROCK", "AWS_PROFILE",
		"AWS_REGION", "AWS_DEFAULT_REGION", "ANTHROPIC_API_KEY", "OPENROUTER_API_KEY", "GITHUB_TOKEN"} {
		t.Setenv(k, "")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir()) // no gh: Copilot is not detected

	for _, tc := range []struct {
		name string
		env  map[string]string
		want string
		idx  int
	}{
		{"nothing detected: the first provider", nil, "bedrock", 0},
		{"bedrock keys", map[string]string{"AWS_ACCESS_KEY_ID": "a", "AWS_SECRET_ACCESS_KEY": "b"}, "bedrock", 0},
		{"bedrock API key", map[string]string{"AWS_BEARER_TOKEN_BEDROCK": "tok"}, "bedrock", 0},
		{"anthropic key", map[string]string{"ANTHROPIC_API_KEY": "sk-ant-123456789"}, "anthropic", 1},
		{"openrouter key", map[string]string{"OPENROUTER_API_KEY": "sk-or-123456789"}, "openrouter", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			s := newTestState()
			step := buildProviderStep(s)
			*s.Steps = []views.WizardStep{step}
			container := tview.NewFlex().SetDirection(tview.FlexRow)
			step.CustomView(tview.NewApplication(), container, func() {})
			assert.Equal(t, tc.want, s.SelectedProvider)
			assert.Equal(t, tc.idx, s.ProviderIdx)
		})
	}
}

// What the wizard writes: language, provider and project through the steps,
// then the setup flag. The hub.toml and the secrets are those of the former
// wizard for the same answers.
func TestInitWizard_WritesAtTheEnd(t *testing.T) {
	configtest.Home(t)
	i18ntest.Restore(t)
	require.NoError(t, config.Save(&config.Config{}))
	config.Reset()

	secrets := map[string]string{}
	store := &mockProjectStore{}
	a := newMockApp(secrets, nil)
	a.Config = &config.Config{}
	a.Projects = store
	cfg, s := buildInitWizardConfig(a)

	projectDir := t.TempDir()
	s.SelectedLang = "fr"
	s.SelectedProvider = "bedrock"
	s.AuthMode = "bearer"
	s.Token = "bedrock-bearer"
	s.Region = "eu-west-1"
	s.ProjectName = "demo"
	s.ProjectPath = projectDir

	for _, id := range []string{"welcome", "provider", "project"} {
		step := views.StepByID(cfg.Steps, id)
		require.NotNil(t, step, id)
		require.NotNil(t, step.OnDone, id)
		require.NoError(t, step.OnDone(), id)
	}
	cfg.OnComplete(true, nil)

	config.Reset()
	got, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "fr", got.CLI.Language)
	assert.True(t, got.CLI.SetupDone)
	assert.Equal(t, "bedrock", got.LLM.DefaultProvider)
	assert.Equal(t, "bearer", got.Provider.Bedrock.AuthMode)
	assert.Equal(t, "eu-west-1", got.Provider.Bedrock.AWSRegion)
	assert.Equal(t, "bedrock-bearer", secrets[providerPkg.KeychainKey(providerPkg.Bedrock, "")])
	require.Len(t, store.projects, 1)
	assert.Equal(t, "demo", store.projects[0].Name)
	assert.Equal(t, "bedrock", store.projects[0].Provider)
	assert.True(t, s.ProjectCreated)

	data, err := os.ReadFile(config.ConfigPath())
	require.NoError(t, err)
	t.Logf("hub.toml:\n%s", data)
}

// MCP is no longer a screen: the summary points to oh mcp setup.
func TestInitWizard_SummarySuggestsMCPSetup(t *testing.T) {
	i18ntest.Restore(t)
	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	a.Projects = &mockProjectStore{}
	cfg, _ := buildInitWizardConfig(a)
	require.NotNil(t, cfg.SummaryNote)
	assert.Contains(t, cfg.SummaryNote(), "oh mcp setup")
	for _, step := range cfg.Steps {
		assert.NotContains(t, step.ID, "mcp", "no MCP step in the wizard")
	}
}
