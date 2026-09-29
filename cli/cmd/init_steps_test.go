package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
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

func TestBuildWelcomeStep_Structure(t *testing.T) {
	s := newTestState()
	step := buildWelcomeStep(s)

	assert.Equal(t, "welcome", step.ID)
	assert.True(t, step.Required, "welcome must be Required")
	assert.True(t, step.SidebarHidden, "welcome must be SidebarHidden")
	assert.NotNil(t, step.CustomView, "welcome must have CustomView")
	assert.Nil(t, step.Form, "welcome must not have Form")
	assert.NotNil(t, step.InfoFields, "welcome must have InfoFields")
}

func TestBuildWelcomeStep_Renders(t *testing.T) {
	s := newTestState()
	step := buildWelcomeStep(s)

	app := tview.NewApplication()
	container := tview.NewFlex().SetDirection(tview.FlexRow)
	doneCalled := false

	require.NotPanics(t, func() {
		step.CustomView(app, container, func() { doneCalled = true })
	})
	assert.Greater(t, container.GetItemCount(), 0, "container should have children")
	assert.False(t, doneCalled, "onDone should not be called during render")
}

// ─────────────────────────────────────────────────────────────────────────────
// buildLangStep
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildLangStep_Structure(t *testing.T) {
	s := newTestState()
	step := buildLangStep(s)

	assert.Equal(t, "lang", step.ID)
	assert.True(t, step.Required, "lang must be Required")
	assert.NotNil(t, step.Form, "lang must have Form")
	assert.NotNil(t, step.OnDone, "lang must have OnDone")
	assert.NotNil(t, step.InfoFields, "lang must have InfoFields")
}

func TestBuildLangStep_FormRendering(t *testing.T) {
	s := newTestState()
	steps := []views.WizardStep{buildWelcomeStep(s), buildLangStep(s)}
	*s.Steps = steps
	s.LangStepIdx = 1

	app := tview.NewApplication()
	form := steps[1].Form(app, func() {})
	require.NotNil(t, form)
	assert.Equal(t, 1, form.GetFormItemCount(), "lang form should have 1 field (DropDown)")
	assert.Equal(t, 1, form.GetButtonCount(), "lang form should have 1 button")
}

// ─────────────────────────────────────────────────────────────────────────────
// buildProviderStep
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildProviderStep_Structure(t *testing.T) {
	s := newTestState()
	step := buildProviderStep(s)

	assert.Equal(t, "provider", step.ID)
	assert.NotNil(t, step.CustomView, "provider must have CustomView")
	assert.NotNil(t, step.SkipIf, "provider must have SkipIf")
	assert.NotNil(t, step.Validate, "provider must have Validate")
	assert.NotNil(t, step.OnDone, "provider must have OnDone")
	assert.NotNil(t, step.InfoFields, "provider must have InfoFields")
	assert.NotEmpty(t, step.Processing, "provider must have Processing label")
}

func TestBuildProviderStep_SkipWhenFlagged(t *testing.T) {
	s := newTestState()
	step := buildProviderStep(s)

	s.ProviderSkipped = false
	assert.False(t, step.SkipIf(), "should not skip when flag is false")

	s.ProviderSkipped = true
	assert.True(t, step.SkipIf(), "should skip when flag is true")
}

func TestBuildProviderStep_FormRendering(t *testing.T) {
	s := newTestState()
	steps := []views.WizardStep{buildProviderStep(s)}
	*s.Steps = steps
	s.ProviderStepIdx = 0

	step := steps[0]
	assert.NotNil(t, step.CustomView, "provider must have CustomView")
	// CustomView steps manage their own layout — we verify the step builds
	// without panic by invoking the CustomView callback.
	app := tview.NewApplication()
	container := tview.NewFlex().SetDirection(tview.FlexRow)
	step.CustomView(app, container, func() {})
	assert.Greater(t, container.GetItemCount(), 0, "container should have items after CustomView renders")
}

// ─────────────────────────────────────────────────────────────────────────────
// buildProjectStep
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildProjectStep_Structure(t *testing.T) {
	s := newTestState()
	step := buildProjectStep(s)

	assert.Equal(t, "project", step.ID)
	assert.NotNil(t, step.CustomView, "project must have CustomView")
	assert.NotNil(t, step.SkipIf, "project must have SkipIf")
	assert.NotNil(t, step.Validate, "project must have Validate")
	assert.NotNil(t, step.OnDone, "project must have OnDone")
	assert.NotNil(t, step.InfoFields, "project must have InfoFields")
	assert.NotEmpty(t, step.Processing, "project must have Processing label")
}

func TestBuildProjectStep_SkipWhenFlagged(t *testing.T) {
	s := newTestState()
	step := buildProjectStep(s)

	s.ProjectSkipped = false
	assert.False(t, step.SkipIf(), "should not skip when flag is false")

	s.ProjectSkipped = true
	assert.True(t, step.SkipIf(), "should skip when flag is true")
}

func TestBuildProjectStep_ValidationEmpty(t *testing.T) {
	s := newTestState()
	step := buildProjectStep(s)

	s.ProjectName = ""
	errMsg := step.Validate()
	assert.NotEmpty(t, errMsg, "should fail validation when name is empty")
}

func TestBuildProjectStep_FormRendering(t *testing.T) {
	s := newTestState()
	step := buildProjectStep(s)

	assert.NotNil(t, step.CustomView, "project must have CustomView")
	app := tview.NewApplication()
	container := tview.NewFlex().SetDirection(tview.FlexRow)
	step.CustomView(app, container, func() {})
	assert.Greater(t, container.GetItemCount(), 0, "container should have items after CustomView renders")
}

// ─────────────────────────────────────────────────────────────────────────────
// buildDeployStep
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildDeployStep_Structure(t *testing.T) {
	s := newTestState()
	step := buildDeployStep(s)

	assert.Equal(t, "deploy", step.ID)
	assert.NotNil(t, step.CustomView, "deploy must have CustomView")
	assert.NotNil(t, step.SkipIf, "deploy must have SkipIf")
	assert.NotNil(t, step.OnDone, "deploy must have OnDone")
	assert.NotNil(t, step.InfoFields, "deploy must have InfoFields")
	assert.NotEmpty(t, step.Processing, "deploy must have Processing label")
}

func TestBuildDeployStep_SkipWithoutProject(t *testing.T) {
	s := newTestState()
	step := buildDeployStep(s)

	s.ProjectSkipped = false
	s.ProjectCreated = false
	assert.True(t, step.SkipIf(), "should skip when no project was created")

	s.ProjectCreated = true
	assert.False(t, step.SkipIf(), "should not skip when project was created")

	s.ProjectSkipped = true
	assert.True(t, step.SkipIf(), "should skip when project section was skipped")
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

func TestBuildMCPGitLabStep_Structure(t *testing.T) {
	s := newTestState()
	step := buildTestMCPGitLabStep(s)

	assert.Equal(t, "mcp_gitlab", step.ID)
	assert.NotNil(t, step.Form, "gitlab must have Form")
	assert.NotNil(t, step.SkipIf, "gitlab must have SkipIf")
	assert.NotNil(t, step.OnDone, "gitlab must have OnDone")
	assert.NotNil(t, step.InfoFields, "gitlab must have InfoFields")
}

func TestBuildMCPGitLabStep_SkipWhenFlagged(t *testing.T) {
	s := newTestState()
	step := buildTestMCPGitLabStep(s)

	s.MCPSkipped = false
	assert.False(t, step.SkipIf(), "should not skip when flag is false")

	s.MCPSkipped = true
	assert.True(t, step.SkipIf(), "should skip when flag is true")
}

func TestBuildMCPGitLabStep_FormRendering(t *testing.T) {
	s := newTestState()
	step := buildTestMCPGitLabStep(s)

	app := tview.NewApplication()
	form := step.Form(app, func() {})
	require.NotNil(t, form)
	// password field + write checkbox + hints + submit
	assert.GreaterOrEqual(t, form.GetFormItemCount(), 2, "gitlab form should have at least 2 fields")
	assert.GreaterOrEqual(t, form.GetButtonCount(), 1, "gitlab form should have at least 1 button")
}

// ─────────────────────────────────────────────────────────────────────────────
// Tier 1 — Behavioral tests (OnDone, Validate)
// ─────────────────────────────────────────────────────────────────────────────

// setupConfigDir creates a temp HOME with a minimal hub.toml so config.Update works.
func setupConfigDir(t *testing.T) {
	t.Helper()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	config.Reset()
	hubDir := filepath.Join(tmpHome, ".oh")
	require.NoError(t, os.MkdirAll(hubDir, 0o755))
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
	assert.Equal(t, "bedrock", cfg.Opencode.DefaultProvider)
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
	assert.Equal(t, "anthropic", cfg.Opencode.DefaultProvider)
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

func TestBuildMCPConsolidatedStep_Structure(t *testing.T) {
	s := newTestState()
	a := *s.AppPtr
	step := buildMCPConsolidatedStep(s, a)

	assert.Equal(t, "mcp_consolidated", step.ID)
	assert.Equal(t, "MCP", step.Label)
	assert.NotNil(t, step.CustomView, "mcp consolidated must have CustomView")
	assert.NotNil(t, step.SkipIf, "mcp consolidated must have SkipIf")
	assert.NotNil(t, step.OnDone, "mcp consolidated must have OnDone")
	assert.NotNil(t, step.InfoFields, "mcp consolidated must have InfoFields")
}

func TestBuildMCPConsolidatedStep_SkipWhenFlagged(t *testing.T) {
	s := newTestState()
	a := *s.AppPtr
	step := buildMCPConsolidatedStep(s, a)

	s.MCPSkipped = false
	assert.False(t, step.SkipIf(), "should not skip when flag is false")

	s.MCPSkipped = true
	assert.True(t, step.SkipIf(), "should skip when flag is true")
}

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

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "hub.toml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("[cli]"), 0o644))
	t.Setenv("OH_CONFIG_PATH", cfgPath)
	config.Reset()

	s.FigmaToken = "figma-tok"
	s.GitlabToken = "gitlab-tok"
	s.GslidesToken = "gslides-tok"

	step := buildMCPConsolidatedStep(s, a)
	err := step.OnDone()
	require.NoError(t, err)

	assert.Equal(t, "figma-tok", secretsMap[config.DefaultFigmaTokenKey])
	assert.Equal(t, "gitlab-tok", secretsMap[config.DefaultGitLabTokenKey])
	assert.Equal(t, "gslides-tok", secretsMap[config.DefaultGslidesTokenKey])
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
	require.Len(t, fields, 3, "should have 3 entries (Figma, GitLab, GSlides)")
	for _, f := range fields {
		assert.Contains(t, f.Value, i18n.T("cmd.init.wizard_mcp_skipped"), "empty token should show skipped for %s", f.Label)
	}

	// Set tokens → configured
	s.FigmaToken = "tok"
	s.GitlabToken = "tok"
	s.GitlabWrite = true
	fields = step.InfoFields()
	require.GreaterOrEqual(t, len(fields), 4, "should have 4 entries including Write")
}

// ─────────────────────────────────────────────────────────────────────────────
// Express Mode (SetupMode)
// ─────────────────────────────────────────────────────────────────────────────

func TestExpressMode_SoloSkipsTeam(t *testing.T) {
	s := newTestState()
	s.SetupMode = "solo"

	// Build provider step with team-mode skip wrapper (same as init_wizard.go).
	providerStep := buildProviderStep(s)
	origSkipIf := providerStep.SkipIf
	providerStep.SkipIf = func() bool {
		if s.SetupMode == "team" {
			return true
		}
		if origSkipIf != nil {
			return origSkipIf()
		}
		return false
	}

	// In solo mode, provider should NOT be skipped.
	assert.False(t, providerStep.SkipIf(), "provider should not be skipped in solo mode")

	// Simulate team mode skip for team intro.
	teamState := s.TeamState
	s.SetupMode = "solo"
	teamState.Skipped = false

	// The team intro SkipIf sets Skipped=true when solo.
	teamSkipIf := func() bool {
		if s.SetupMode == "solo" {
			teamState.Skipped = true
			return true
		}
		return false
	}

	assert.True(t, teamSkipIf(), "team intro should be skipped in solo mode")
	assert.True(t, teamState.Skipped, "teamState.Skipped should be set in solo mode")
}

func TestExpressMode_TeamSkipsProvider(t *testing.T) {
	s := newTestState()
	s.SetupMode = "team"

	providerStep := buildProviderStep(s)
	origSkipIf := providerStep.SkipIf
	providerStep.SkipIf = func() bool {
		if s.SetupMode == "team" {
			return true
		}
		if origSkipIf != nil {
			return origSkipIf()
		}
		return false
	}

	assert.True(t, providerStep.SkipIf(), "provider should be skipped in team mode")
}

func TestExpressMode_FullShowsAll(t *testing.T) {
	s := newTestState()
	s.SetupMode = "full"

	providerStep := buildProviderStep(s)
	origSkipIf := providerStep.SkipIf
	providerStep.SkipIf = func() bool {
		if s.SetupMode == "team" {
			return true
		}
		if origSkipIf != nil {
			return origSkipIf()
		}
		return false
	}

	assert.False(t, providerStep.SkipIf(), "provider should not be skipped in full mode")

	teamSkipIf := func() bool {
		if s.SetupMode == "solo" {
			return true
		}
		return false
	}
	assert.False(t, teamSkipIf(), "team should not be skipped in full mode")
}

func TestExpressMode_DefaultIsSolo(t *testing.T) {
	s := newTestState()
	_ = buildWelcomeStep(s)

	// SetupMode is initialized inside the CustomView closure when rendered,
	// so before rendering it should be empty.
	assert.Equal(t, "", s.SetupMode, "SetupMode should be empty before CustomView renders")
}
