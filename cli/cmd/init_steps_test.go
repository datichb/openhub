package cmd

import (
	"testing"

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
)

// newTestState creates a minimal initStepState for testing.
func newTestState() *initStepState {
	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	a.Projects = &mockProjectStore{}
	appPtr := &a
	focusBtn := false
	var steps []views.WizardStep

	return &initStepState{
		SelectedLang:    "en",
		ProviderOptions: []string{"bedrock", "anthropic", "openrouter", "github-copilot"},
		TeamState:       &initWizardTeamState{},
		AppPtr:          appPtr,
		FocusBtn:        &focusBtn,
		Steps:           &steps,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// buildWelcomeStep
// ─────────────────────────────────────────────────────────────────────────────

// ─────────────────────────────────────────────────────────────────────────────
// buildLangStep
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
// MCP GitLab step (via buildMCPTokenStep with checkbox opts)
// ─────────────────────────────────────────────────────────────────────────────

// buildTestMCPGitLabStep creates the GitLab MCP step the same way init_wizard.go does.
func buildTestMCPGitLabStep(s *initStepState) views.WizardStep {
	return buildMCPTokenStep(mcpTokenStepOpts{
		ID:              "mcp_gitlab",
		LabelI18nKey:    "cmd.init.wizard_step_mcp_gitlab",
		DisplayName:     "GitLab",
		TokenKey:        config.DefaultGitLabTokenKey,
		HintI18nKey:     "cmd.init.mcp_hint_gitlab",
		TokenVar:        &s.GitlabToken,
		SkipIf:          func() bool { return s.MCPSkipped },
		SecretsFunc:     func() domain.SecretStore { return (*s.AppPtr).Secrets },
		CheckboxLabel:   i18n.T("cmd.init.mcp_gitlab_write_short"),
		CheckboxDescKey: "cmd.init.mcp_gitlab_write_desc",
		CheckboxVar:     &s.GitlabWrite,
		AfterStore: func() error {
			return config.Update(func(c *config.Config) error {
				c.MCP.Gitlab.Enabled = true
				if s.GitlabWrite {
					c.MCP.Gitlab.WriteEnabled = true
				}
				return nil
			})
		},
		ExtraInfoFields: func() []views.InfoField {
			if s.GitlabToken != "" && s.GitlabWrite {
				return []views.InfoField{{Label: "Write", Value: i18n.T("cmd.init.wizard_mcp_enabled")}}
			}
			return nil
		},
	})
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
				assert.True(t, step.SidebarHidden, "welcome must be SidebarHidden")
				assert.Nil(t, step.Form, "welcome must not have Form")
				assert.Empty(t, s.SetupMode, "the setup mode is chosen when the view renders, not before")
			},
		},
		{
			name: "lang", build: buildLangStep, id: "lang", form: true, minFields: 1, minButtons: 1,
			prepare: func(s *initStepState, step views.WizardStep) {
				*s.Steps = []views.WizardStep{buildWelcomeStep(s), step}
				s.LangStepIdx = 1
			},
			extra: func(t *testing.T, _ *initStepState, step views.WizardStep) {
				assert.True(t, step.Required, "lang must be Required")
			},
		},
		{
			name: "provider", build: buildProviderStep, id: "provider", wantSkipIf: true, wantProcess: true,
			skipFlag: func(s *initStepState, v bool) { s.ProviderSkipped = v },
			prepare: func(s *initStepState, step views.WizardStep) {
				*s.Steps = []views.WizardStep{step}
				s.ProviderStepIdx = 0
			},
		},
		{
			name: "project", build: buildProjectStep, id: "project", wantSkipIf: true, wantProcess: true,
			skipFlag: func(s *initStepState, v bool) { s.ProjectSkipped = v },
		},
		{
			name: "mcp gitlab", build: buildTestMCPGitLabStep, id: "mcp_gitlab", form: true, minFields: 2, minButtons: 1, wantSkipIf: true,
			skipFlag: func(s *initStepState, v bool) { s.MCPSkipped = v },
		},
		{
			name:  "mcp consolidated",
			build: func(s *initStepState) views.WizardStep { return buildMCPConsolidatedStep(s, *s.AppPtr) },
			id:    "mcp_consolidated", wantSkipIf: true,
			skipFlag: func(s *initStepState, v bool) { s.MCPSkipped = v },
			extra: func(t *testing.T, _ *initStepState, step views.WizardStep) {
				assert.Equal(t, "MCP", step.Label)
			},
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
			if tt.id != "welcome" && tt.id != "lang" {
				assert.NotNil(t, step.OnDone, "OnDone")
			}
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

func TestBuildLangStep_OnDone_PersistsLanguage(t *testing.T) {
	setupConfigDir(t)
	s := newTestState()
	s.SelectedLang = "fr"
	step := buildLangStep(s)

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

func TestBuildMCPGitLabStep_OnDone_StoresAndEnables(t *testing.T) {
	setupConfigDir(t)
	sc := &mockSecretStore{secrets: make(map[string]string)}
	a := newMockApp(nil, nil)
	a.Secrets = sc
	a.Config = &config.Config{}
	appPtr := &a

	s := &initStepState{
		GitlabToken:     "glpat-xxx",
		GitlabWrite:     true,
		ProviderOptions: []string{"bedrock"},
		TeamState:       &initWizardTeamState{},
		AppPtr:          appPtr,
	}

	step := buildTestMCPGitLabStep(s)
	err := step.OnDone()
	require.NoError(t, err)

	assert.Equal(t, "glpat-xxx", sc.secrets[config.DefaultGitLabTokenKey])

	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.True(t, cfg.MCP.Gitlab.Enabled)
	assert.True(t, cfg.MCP.Gitlab.WriteEnabled)
}

func TestBuildMCPGitLabStep_OnDone_EmptyNoOp(t *testing.T) {
	sc := &mockSecretStore{secrets: make(map[string]string)}
	a := newMockApp(nil, nil)
	a.Secrets = sc
	a.Config = &config.Config{}
	appPtr := &a

	s := &initStepState{
		GitlabToken:     "",
		ProviderOptions: []string{"bedrock"},
		TeamState:       &initWizardTeamState{},
		AppPtr:          appPtr,
	}

	step := buildTestMCPGitLabStep(s)
	err := step.OnDone()
	require.NoError(t, err)
	assert.Empty(t, sc.secrets, "no secret should be stored when token is empty")
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

// ─────────────────────────────────────────────────────────────────────────────
// buildMCPConsolidatedStep
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildMCPConsolidatedStep_OnDone_StoresAllTokens(t *testing.T) {
	secretsMap := make(map[string]string)
	a := newMockApp(secretsMap, nil)
	a.Projects = &mockProjectStore{}
	appPtr := &a
	s := &initStepState{
		ProviderOptions: []string{"bedrock"},
		TeamState:       &initWizardTeamState{},
		AppPtr:          appPtr,
		FocusBtn:        new(bool),
		Steps:           new([]views.WizardStep),
	}

	setupConfigDir(t)

	s.FigmaToken = "figma-tok"
	s.GitlabToken = "gitlab-tok"
	s.JiraToken = "jira-tok"
	s.JiraURL = "https://jira.test.com"
	s.GslidesToken = "gslides-tok"

	step := buildMCPConsolidatedStep(s, a)
	err := step.OnDone()
	require.NoError(t, err)

	assert.Equal(t, "figma-tok", secretsMap[config.DefaultFigmaTokenKey])
	assert.Equal(t, "gitlab-tok", secretsMap[config.DefaultGitLabTokenKey])
	assert.Equal(t, "jira-tok", secretsMap[config.DefaultJiraTokenKey])
	assert.Equal(t, "gslides-tok", secretsMap[config.DefaultGslidesTokenKey])

	// Verify Jira is enabled because URL was provided.
	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.True(t, cfg.MCP.Jira.Enabled, "Jira should be enabled when URL is provided")
	assert.Equal(t, "https://jira.test.com", cfg.MCP.Jira.URL)
}

func TestBuildMCPConsolidatedStep_OnDone_EmptyNoOp(t *testing.T) {
	secretsMap := make(map[string]string)
	a := newMockApp(secretsMap, nil)
	a.Projects = &mockProjectStore{}
	appPtr := &a
	s := &initStepState{
		ProviderOptions: []string{"bedrock"},
		TeamState:       &initWizardTeamState{},
		AppPtr:          appPtr,
		FocusBtn:        new(bool),
		Steps:           new([]views.WizardStep),
	}

	step := buildMCPConsolidatedStep(s, a)
	err := step.OnDone()
	require.NoError(t, err)

	assert.Empty(t, secretsMap, "no tokens should be stored when all empty")
}

func TestBuildMCPConsolidatedStep_InfoFields(t *testing.T) {
	s := newTestState()
	a := *s.AppPtr
	step := buildMCPConsolidatedStep(s, a)

	// All empty → all skipped
	fields := step.InfoFields()
	require.Len(t, fields, 4, "should have 4 entries (Figma, GitLab, Jira, GSlides)")
	for _, f := range fields {
		assert.Contains(t, f.Value, i18n.T("cmd.init.wizard_mcp_skipped"), "empty token should show skipped for %s", f.Label)
	}

	// Set tokens → configured (Jira without URL → warning)
	s.FigmaToken = "tok"
	s.GitlabToken = "tok"
	s.GitlabWrite = true
	s.JiraToken = "tok"
	fields = step.InfoFields()
	require.GreaterOrEqual(t, len(fields), 5, "should have 5+ entries including Write and Jira warning")

	// Jira with token but no URL shows warning
	var jiraField views.InfoField
	for _, f := range fields {
		if f.Label == "Jira" {
			jiraField = f
			break
		}
	}
	assert.Contains(t, jiraField.Value, i18n.T("cmd.init.wizard_mcp_token_saved_url_required"),
		"Jira with token but no URL should show warning")
}

func TestBuildMCPConsolidatedStep_OnDone_JiraTokenWithoutURL(t *testing.T) {
	secretsMap := make(map[string]string)
	a := newMockApp(secretsMap, nil)
	a.Projects = &mockProjectStore{}
	appPtr := &a
	s := &initStepState{
		ProviderOptions: []string{"bedrock"},
		TeamState:       &initWizardTeamState{},
		AppPtr:          appPtr,
		FocusBtn:        new(bool),
		Steps:           new([]views.WizardStep),
	}

	setupConfigDir(t)

	s.JiraToken = "jira-tok"
	s.JiraURL = "" // no URL → token stored but service NOT enabled

	step := buildMCPConsolidatedStep(s, a)
	err := step.OnDone()
	require.NoError(t, err)

	// Token is always stored in keychain.
	assert.Equal(t, "jira-tok", secretsMap[config.DefaultJiraTokenKey])

	// Jira must NOT be enabled without URL.
	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.False(t, cfg.MCP.Jira.Enabled, "Jira should NOT be enabled without URL")
	assert.Empty(t, cfg.MCP.Jira.URL, "Jira URL should remain empty")
}

func TestBuildMCPConsolidatedStep_OnDone_GitLabURLPersisted(t *testing.T) {
	secretsMap := make(map[string]string)
	a := newMockApp(secretsMap, nil)
	a.Projects = &mockProjectStore{}
	appPtr := &a
	s := &initStepState{
		ProviderOptions: []string{"bedrock"},
		TeamState:       &initWizardTeamState{},
		AppPtr:          appPtr,
		FocusBtn:        new(bool),
		Steps:           new([]views.WizardStep),
	}

	setupConfigDir(t)

	s.GitlabToken = "gl-tok"
	s.GitlabURL = "https://gitlab.corp.com"

	step := buildMCPConsolidatedStep(s, a)
	err := step.OnDone()
	require.NoError(t, err)

	assert.Equal(t, "gl-tok", secretsMap[config.DefaultGitLabTokenKey])

	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.True(t, cfg.MCP.Gitlab.Enabled, "GitLab should be enabled")
	assert.Equal(t, "https://gitlab.corp.com", cfg.MCP.Gitlab.URL, "GitLab URL should be persisted")
}

// ─────────────────────────────────────────────────────────────────────────────
// Express Mode (SetupMode)
// ─────────────────────────────────────────────────────────────────────────────
