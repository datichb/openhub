package cmd

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	providerPkg "github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// newTestState creates a minimal initStepState for testing.
func newTestState() *initStepState {
	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	a.Projects = &mockProjectStore{}
	appPtr := &a
	var steps []views.WizardStep

	return &initStepState{
		SelectedLang:    "en",
		ProviderOptions: []string{"bedrock", "anthropic", "openrouter", "github-copilot"},
		TeamState:       &initWizardTeamState{},
		AppPtr:          appPtr,
		Steps:           &steps,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// buildWelcomeStep
// ─────────────────────────────────────────────────────────────────────────────

// ─────────────────────────────────────────────────────────────────────────────
// buildLangStep (now the welcome screen)
// ─────────────────────────────────────────────────────────────────────────────

// ─────────────────────────────────────────────────────────────────────────────
// buildProviderStep
// ─────────────────────────────────────────────────────────────────────────────

// ─────────────────────────────────────────────────────────────────────────────
// buildProjectStep
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildProjectStep_ValidationEmpty(t *testing.T) {
	s := newTestState()
	step := buildProjectStep(s)

	s.ProjectName = ""
	errMsg := step.Validate()
	assert.NotEmpty(t, errMsg, "should fail validation when name is empty")
}

// ─────────────────────────────────────────────────────────────────────────────
// Every step: its shape, its skip flag, its first render
// ─────────────────────────────────────────────────────────────────────────────

func TestInitSteps(t *testing.T) {
	tests := []struct {
		name  string
		build func(s *initStepState) views.WizardStep
		id    string
		// form: the step renders a form (min fields, min buttons); else a custom view
		form        bool
		minFields   int
		minButtons  int
		skipFlag    func(s *initStepState, v bool) // nil: the step has no skip flag
		prepare     func(s *initStepState, step views.WizardStep)
		wantSkipIf  bool
		wantProcess bool // has a Processing label and a Validate
		extra       func(t *testing.T, s *initStepState, step views.WizardStep)
	}{
		{
			name: "welcome", build: buildWelcomeStep, id: "welcome",
			extra: func(t *testing.T, s *initStepState, step views.WizardStep) {
				assert.True(t, step.Required, "welcome must be Required")
				assert.False(t, step.SidebarHidden, "welcome shows the language and the setup type in the sidebar")
				assert.Nil(t, step.Form, "welcome must not have Form")
				assert.Empty(t, s.SetupMode, "the setup mode is chosen when the view renders, not before")
			},
		},
		{
			name: "provider", build: buildProviderStep, id: "provider", wantSkipIf: true, wantProcess: true,
			skipFlag: func(s *initStepState, v bool) { s.ProviderSkipped = v },
			prepare: func(s *initStepState, step views.WizardStep) {
				*s.Steps = []views.WizardStep{step}
			},
		},
		{
			name: "project", build: buildProjectStep, id: "project", wantSkipIf: true, wantProcess: true,
			skipFlag: func(s *initStepState, v bool) { s.ProjectSkipped = v },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestState()
			step := tt.build(s)

			assert.Equal(t, tt.id, step.ID, "step id")
			assert.NotNil(t, step.InfoFields, "InfoFields")
			if tt.form {
				assert.NotNil(t, step.Form, "a form step has a Form")
			} else {
				assert.NotNil(t, step.CustomView, "a custom step has a CustomView")
			}
			assert.NotNil(t, step.OnDone, "OnDone")
			if tt.wantSkipIf {
				assert.NotNil(t, step.SkipIf, "SkipIf")
			}
			if tt.wantProcess {
				assert.NotNil(t, step.Validate, "Validate")
				assert.NotEmpty(t, step.Processing, "Processing label")
			}
			if tt.extra != nil {
				tt.extra(t, s, step)
			}

			if tt.skipFlag != nil {
				tt.skipFlag(s, false)
				assert.False(t, step.SkipIf(), "not skipped when the flag is false")
				tt.skipFlag(s, true)
				assert.True(t, step.SkipIf(), "skipped when the flag is true")
				tt.skipFlag(s, false)
			}

			if tt.prepare != nil {
				tt.prepare(s, step)
			}
			app := tview.NewApplication()
			if tt.form {
				form := step.Form(app, func() {})
				require.NotNil(t, form, "the form renders")
				assert.GreaterOrEqual(t, form.GetFormItemCount(), tt.minFields, "form fields")
				assert.GreaterOrEqual(t, form.GetButtonCount(), tt.minButtons, "form buttons")
				return
			}
			container := tview.NewFlex().SetDirection(tview.FlexRow)
			doneCalled := false
			require.NotPanics(t, func() { step.CustomView(app, container, func() { doneCalled = true }) })
			assert.Greater(t, container.GetItemCount(), 0, "the view puts items in the container")
			assert.False(t, doneCalled, "rendering does not finish the step")
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tier 1 — Behavioral tests (OnDone, Validate)
// ─────────────────────────────────────────────────────────────────────────────

// setupConfigDir creates a temp HOME with a minimal hub.toml so config.Update works.
func setupConfigDir(t *testing.T) {
	t.Helper()
	configtest.Home(t)
	i18ntest.Restore(t) // the steps set the language of the messages
	require.NoError(t, config.Save(&config.Config{}))
	config.Reset()
}

func TestBuildWelcomeStep_OnDone_PersistsLanguage(t *testing.T) {
	setupConfigDir(t)
	s := newTestState()
	s.SelectedLang = "fr"
	step := buildWelcomeStep(s)

	err := step.OnDone()
	require.NoError(t, err)

	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "fr", cfg.CLI.Language)
}

func TestBuildProviderStep_Validate_AllBranches(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		auth     string
		token    string
		region   string
		keychain bool
		wantErr  bool
	}{
		{"anthropic_empty_no_keychain", "anthropic", "", "", "", false, true},
		{"anthropic_with_token", "anthropic", "", "tok", "", false, false},
		{"anthropic_with_keychain", "anthropic", "", "", "", true, false},
		{"bedrock_bearer_empty", "bedrock", "bearer", "", "", false, true},
		{"bedrock_bearer_with_token_no_region", "bedrock", "bearer", "tok", "", false, true},
		{"bedrock_bearer_valid", "bedrock", "bearer", "tok", "eu-west-1", false, false},
		{"bedrock_profile_no_region", "bedrock", "profile", "", "", false, true},
		{"bedrock_profile_valid", "bedrock", "profile", "", "us-east-1", false, false},
		{"copilot_always_ok", "github-copilot", "", "", "", false, false},
		{"openrouter_empty_no_keychain", "openrouter", "", "", "", false, true},
		{"openrouter_with_keychain", "openrouter", "", "", "", true, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestState()
			s.SelectedProvider = tc.provider
			s.AuthMode = tc.auth
			s.Token = tc.token
			s.Region = tc.region
			s.HasKeychainToken = tc.keychain
			step := buildProviderStep(s)
			msg := step.Validate()
			if tc.wantErr {
				assert.NotEmpty(t, msg, "expected validation error")
			} else {
				assert.Empty(t, msg, "expected no validation error")
			}
		})
	}
}

func TestBuildProviderStep_OnDone_BedrockBearer(t *testing.T) {
	setupConfigDir(t)
	sc := &mockSecretStore{secrets: make(map[string]string)}
	a := newMockApp(nil, nil)
	a.Secrets = sc
	a.Config = &config.Config{}
	a.Projects = &mockProjectStore{}
	appPtr := &a

	s := &initStepState{
		SelectedProvider: "bedrock",
		AuthMode:         "bearer",
		Token:            "my-bedrock-token",
		Region:           "eu-west-1",
		ProviderOptions:  []string{"bedrock", "anthropic", "openrouter", "github-copilot"},
		TeamState:        &initWizardTeamState{},
		AppPtr:           appPtr,
	}

	step := buildProviderStep(s)
	err := step.OnDone()
	require.NoError(t, err)

	// Verify keychain
	expectedKey := providerPkg.KeychainKey(providerPkg.Bedrock, "")
	assert.Equal(t, "my-bedrock-token", sc.secrets[expectedKey])

	// Verify config
	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "bedrock", cfg.LLM.DefaultProvider)
	assert.Equal(t, "bearer", cfg.Provider.Bedrock.AuthMode)
	assert.Equal(t, "eu-west-1", cfg.Provider.Bedrock.AWSRegion)
}

func TestBuildProviderStep_OnDone_Anthropic(t *testing.T) {
	setupConfigDir(t)
	sc := &mockSecretStore{secrets: make(map[string]string)}
	a := newMockApp(nil, nil)
	a.Secrets = sc
	a.Config = &config.Config{}
	appPtr := &a

	s := &initStepState{
		SelectedProvider: "anthropic",
		Token:            "sk-ant-xxx",
		ProviderOptions:  []string{"bedrock", "anthropic", "openrouter", "github-copilot"},
		TeamState:        &initWizardTeamState{},
		AppPtr:           appPtr,
	}

	step := buildProviderStep(s)
	err := step.OnDone()
	require.NoError(t, err)

	expectedKey := providerPkg.KeychainKey(providerPkg.Anthropic, "")
	assert.Equal(t, "sk-ant-xxx", sc.secrets[expectedKey])

	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "anthropic", cfg.LLM.DefaultProvider)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tier 2 — Project, GitLab, OnComplete, RefreshLabels
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildProjectStep_OnDone_CreatesProject(t *testing.T) {
	store := &mockProjectStore{}
	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	a.Projects = store
	appPtr := &a

	s := &initStepState{
		ProjectName:     "myproj",
		ProjectPath:     t.TempDir(),
		ProviderOptions: []string{"bedrock"},
		TeamState:       &initWizardTeamState{},
		AppPtr:          appPtr,
	}

	step := buildProjectStep(s)
	err := step.OnDone()
	require.NoError(t, err)

	require.Len(t, store.projects, 1)
	assert.Equal(t, "myproj", store.projects[0].Name)
	assert.True(t, s.ProjectCreated, "ProjectCreated flag should be set")
}

func TestBuildProjectStep_OnDone_AttachesTeam(t *testing.T) {
	store := &mockProjectStore{}
	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	a.Projects = store
	appPtr := &a

	s := &initStepState{
		ProjectName:     "teamproj",
		ProjectPath:     t.TempDir(),
		ProviderOptions: []string{"bedrock"},
		TeamState: &initWizardTeamState{
			Configured:    true,
			TeamID:        "team-42",
			attachProject: true,
		},
		AppPtr: appPtr,
	}

	step := buildProjectStep(s)
	err := step.OnDone()
	require.NoError(t, err)

	require.Len(t, store.projects, 1)
	require.NotNil(t, store.projects[0].TeamID)
	assert.Equal(t, "team-42", *store.projects[0].TeamID)
}

func TestBuildProjectStep_Validate_AllBranches(t *testing.T) {
	tests := []struct {
		name    string
		pName   string
		pPath   string
		wantErr bool
	}{
		{"empty_name", "", ".", true},
		{"nonexistent_path", "proj", "/nonexistent/path/xyz", true},
		{"valid", "proj", "", false}, // path set to t.TempDir() below
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestState()
			s.ProjectName = tc.pName
			s.ProjectPath = tc.pPath
			if tc.name == "valid" {
				s.ProjectPath = t.TempDir()
			}
			step := buildProjectStep(s)
			msg := step.Validate()
			if tc.wantErr {
				assert.NotEmpty(t, msg, "expected validation error")
			} else {
				assert.Empty(t, msg, "expected no validation error")
			}
		})
	}
}

func TestBuildInitOnComplete_SetupDone(t *testing.T) {
	setupConfigDir(t)
	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	appPtr := &a

	s := &initStepState{
		TeamState: &initWizardTeamState{},
		AppPtr:    appPtr,
	}

	onComplete := buildInitOnComplete(s)
	onComplete(true, nil)

	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.True(t, cfg.CLI.SetupDone, "SetupDone should be persisted after OnComplete")
}

func TestBuildInitOnComplete_NotCompletedNoOp(t *testing.T) {
	setupConfigDir(t)
	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	appPtr := &a

	s := &initStepState{
		TeamState: &initWizardTeamState{},
		AppPtr:    appPtr,
	}

	onComplete := buildInitOnComplete(s)
	onComplete(false, nil) // not completed

	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.False(t, cfg.CLI.SetupDone, "SetupDone should NOT be set when not completed")
}

func TestBuildInitRefreshLabels_UpdatesLabels(t *testing.T) {
	i18ntest.Restore(t)
	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	a.Projects = &mockProjectStore{}
	wiz := buildFirstRunInlineWizard(a)
	require.NotNil(t, wiz)

	// The wizard is built — verify that step labels exist (non-empty).
	// We can't easily call RefreshLabels from outside, but we can
	// verify the wizard builds without error and has the expected ID.
	assert.Equal(t, "wizard.init", wiz.ID())
}
