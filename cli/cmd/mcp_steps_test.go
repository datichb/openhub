package cmd

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

// ─────────────────────────────────────────────────────────────────────────────
// buildMCPTokenStep
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildMCPTokenStep_Structure(t *testing.T) {
	token := ""
	step := buildMCPTokenStep(mcpTokenStepOpts{
		ID:           "mcp_test",
		LabelI18nKey: "cmd.init.wizard_step_mcp_figma",
		DisplayName:  "Figma",
		TokenKey:     "openhub.mcp.figma.token",
		TokenVar:     &token,
		SkipIf:       func() bool { return false },
	})

	assert.Equal(t, "mcp_test", step.ID)
	assert.NotNil(t, step.Form, "should have Form")
	assert.NotNil(t, step.OnDone, "should have OnDone")
	assert.NotNil(t, step.SkipIf, "should have SkipIf")
	assert.NotNil(t, step.InfoFields, "should have InfoFields")
}

func TestBuildMCPTokenStep_OnDone_StoresToken(t *testing.T) {
	sc := &mockSecretStore{secrets: make(map[string]string)}
	token := "my-figma-token"
	afterCalled := false

	step := buildMCPTokenStep(mcpTokenStepOpts{
		DisplayName: "Figma",
		TokenKey:    "openhub.mcp.figma.token",
		TokenVar:    &token,
		Secrets:     sc,
		AfterStore:  func() error { afterCalled = true; return nil },
	})

	err := step.OnDone()
	require.NoError(t, err)
	assert.Equal(t, "my-figma-token", sc.secrets["openhub.mcp.figma.token"])
	assert.True(t, afterCalled, "AfterStore should be called")
}

func TestBuildMCPTokenStep_OnDone_EmptyNoOp(t *testing.T) {
	sc := &mockSecretStore{secrets: make(map[string]string)}
	token := ""
	afterCalled := false

	step := buildMCPTokenStep(mcpTokenStepOpts{
		DisplayName: "Figma",
		TokenKey:    "openhub.mcp.figma.token",
		TokenVar:    &token,
		Secrets:     sc,
		AfterStore:  func() error { afterCalled = true; return nil },
	})

	err := step.OnDone()
	require.NoError(t, err)
	assert.Empty(t, sc.secrets, "no secret should be stored when token is empty")
	assert.False(t, afterCalled, "AfterStore should NOT be called when token is empty")
}

func TestBuildMCPTokenStep_InfoFields(t *testing.T) {
	token := ""
	step := buildMCPTokenStep(mcpTokenStepOpts{
		DisplayName: "Figma",
		TokenKey:    "openhub.mcp.figma.token",
		TokenVar:    &token,
	})

	// Empty token → skipped/not configured
	fields := step.InfoFields()
	require.Len(t, fields, 1)
	assert.Equal(t, "Figma", fields[0].Label)
	emptyValue := fields[0].Value

	// Non-empty token → configured (different from empty value)
	token = "some-token"
	fields = step.InfoFields()
	require.Len(t, fields, 1)
	assert.NotEqual(t, emptyValue, fields[0].Value, "configured value should differ from empty value")
}

func TestBuildMCPTokenStep_SecretsFunc(t *testing.T) {
	sc := &mockSecretStore{secrets: make(map[string]string)}
	token := "lazy-token"

	step := buildMCPTokenStep(mcpTokenStepOpts{
		DisplayName: "Test",
		TokenKey:    "openhub.mcp.test.token",
		TokenVar:    &token,
		SecretsFunc: func() domain.SecretStore { return sc },
	})

	err := step.OnDone()
	require.NoError(t, err)
	assert.Equal(t, "lazy-token", sc.secrets["openhub.mcp.test.token"])
}

func TestBuildMCPTokenStep_FormRendering(t *testing.T) {
	token := ""
	step := buildMCPTokenStep(mcpTokenStepOpts{
		LabelI18nKey: "cmd.init.wizard_step_mcp_figma",
		DisplayName:  "Figma",
		TokenKey:     "openhub.mcp.figma.token",
		TokenVar:     &token,
	})

	app := tview.NewApplication()
	form := step.Form(app, func() {})
	require.NotNil(t, form)
	assert.GreaterOrEqual(t, form.GetFormItemCount(), 1, "should have at least password field")
	assert.GreaterOrEqual(t, form.GetButtonCount(), 1, "should have submit button")
}
