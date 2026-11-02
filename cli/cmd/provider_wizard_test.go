package cmd

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/provider"
)

// The check box of the detected credentials is shown ticked, and tview calls the
// handler of a check box when it changes only: "used as it is" tests untick it
// and tick it again, "left as it is" tests leave it.

// providerWizardEnv isolates the hub and the credentials of the shell, and
// returns an app with a project store and the project p1.
func providerWizardEnv(t *testing.T) (*app.App, *mockProjectStore) {
	t.Helper()
	i18ntest.Set(t, "en")
	clearAWSEnv(t)
	for _, k := range []string{"AWS_REGION", "AWS_DEFAULT_REGION", "ANTHROPIC_API_KEY", "OPENROUTER_API_KEY"} {
		t.Setenv(k, "")
	}
	require.NoError(t, config.Save(&config.Config{}))
	a := newMockApp(nil, nil)
	io, _, _ := testIO()
	a.IO = io
	store := &mockProjectStore{projects: []domain.Project{{ID: "p1", Name: "demo"}}}
	a.Projects = store
	return a, store
}

func savedHub(t *testing.T) *config.Config {
	t.Helper()
	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	return cfg
}

// oh provider setup anthropic|openrouter: the key goes to the keychain and the
// provider is recorded for the hub or for the project.
func TestProviderAPIKeyWizard(t *testing.T) {
	// Steps: 0 detected key (check box, only when the shell has one),
	// 1 the key (password, hint), 2 persist.
	key := func(k string) map[int]func(f *tview.Form) {
		return map[int]func(f *tview.Form){1: func(f *tview.Form) { typeInto(f, 0, k) }}
	}
	hubKey := provider.KeychainKey(provider.Anthropic, "")
	projectKey := provider.KeychainKey(provider.Anthropic, "p1")
	tests := []struct {
		name         string
		env          string
		project      bool
		answers      map[int]func(f *tview.Form)
		abortAt      int
		wantRan      []int
		wantSecrets  map[string]string
		wantTokenKey string
		wantOut      string
	}{
		{
			name: "hub scope: the key is stored", answers: key("sk-123"), abortAt: -1, wantRan: []int{1, 2},
			wantSecrets: map[string]string{hubKey: "sk-123"}, wantOut: i18n.Tf("cmd.provider.hub_configured", "anthropic"),
		},
		{
			name: "project scope: the key is stored for the project", project: true, answers: key("sk-456"), abortAt: -1, wantRan: []int{1, 2},
			wantSecrets: map[string]string{projectKey: "sk-456"}, wantTokenKey: projectKey, wantOut: i18n.Tf("cmd.provider.project_configured", "anthropic", "demo"),
		},
		{
			name: "no key: a warning, nothing is stored", answers: key(""), abortAt: -1, wantRan: []int{1, 2},
			wantSecrets: map[string]string{}, wantOut: i18n.T("cmd.provider.no_key"),
		},
		{
			name: "the key of the shell is used as it is", env: "sk-shell", answers: map[int]func(f *tview.Form){0: func(f *tview.Form) { tick(f, 0, false); tick(f, 0, true) }}, abortAt: -1, wantRan: []int{0},
			wantSecrets: map[string]string{}, wantOut: i18n.Tf("cmd.provider.hub_configured", "anthropic"),
		},
		{
			name: "the key of the shell is used when the ticked box is left as it is", env: "sk-shell", abortAt: -1, wantRan: []int{0},
			wantSecrets: map[string]string{}, wantOut: i18n.Tf("cmd.provider.hub_configured", "anthropic"),
		},
		{
			name: "the key of the shell is declined: a key is asked", env: "sk-shell",
			answers: map[int]func(f *tview.Form){
				0: func(f *tview.Form) { tick(f, 0, false) },
				1: func(f *tview.Form) { typeInto(f, 0, "sk-own") },
			},
			abortAt: -1, wantRan: []int{0, 1, 2}, wantSecrets: map[string]string{hubKey: "sk-own"}, wantOut: i18n.Tf("cmd.provider.hub_configured", "anthropic"),
		},
		{
			name: "the user quits: nothing is stored", answers: key("sk-123"), abortAt: 2, wantRan: []int{1}, wantSecrets: map[string]string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a, store := providerWizardEnv(t)
			t.Setenv("ANTHROPIC_API_KEY", tc.env)
			s := scriptWizard(t, tc.answers)
			s.abortAt = tc.abortAt
			var project *domain.Project
			if tc.project {
				project = &store.projects[0]
			}

			require.NoError(t, setupAPIKey(t.Context(), a, project, provider.Anthropic))

			assert.Equal(t, "provider setup", s.command, "command of the wizard")
			assert.Equal(t, tc.wantRan, s.ran, "steps that ran")
			assert.Equal(t, tc.wantSecrets, a.Secrets.(*mockSecretStore).secrets, "keychain")
			out := a.IO.Out.(interface{ String() string }).String()
			if tc.wantOut == "" {
				assert.Empty(t, out, "output")
			} else {
				assert.Contains(t, out, tc.wantOut, "output")
			}
			if tc.wantTokenKey != "" {
				require.NotNil(t, store.projects[0].ProviderConfig, "project provider config")
				assert.Equal(t, tc.wantTokenKey, store.projects[0].ProviderConfig.TokenKey, "token key of the project")
			} else {
				assert.Nil(t, store.projects[0].ProviderConfig, "project provider config")
			}
		})
	}
}

// oh provider setup bedrock: the credentials go to the keychain, the profile,
// the region and the auth mode to hub.toml or to the project.
func TestProviderBedrockWizard(t *testing.T) {
	// Steps: 0 detected credentials, 1 auth mode, 2 bearer token + region,
	// 3 profile + region, 4 persist.
	hubKey := provider.KeychainKey(provider.Bedrock, "")
	projectKey := provider.KeychainKey(provider.Bedrock, "p1")
	region := func(n int) string { return provider.BedrockRegions[n-1].Code }
	tests := []struct {
		name        string
		env         map[string]string
		project     bool
		answers     map[int]func(f *tview.Form)
		abortAt     int
		wantRan     []int
		wantRefused map[int]string
		wantSecrets map[string]string
		wantHub     config.ProviderConfig
		wantProject *domain.ProjectProviderConfig
		wantOut     string
	}{
		{
			name: "bearer token, hub scope",
			answers: map[int]func(f *tview.Form){
				1: func(f *tview.Form) { choose(f, 0, 0) },
				2: func(f *tview.Form) { typeInto(f, 0, "ABSK-1"); choose(f, 1, 4) },
			},
			abortAt: -1, wantRan: []int{1, 2, 4},
			wantSecrets: map[string]string{hubKey: "ABSK-1"},
			wantHub:     config.ProviderConfig{AWSRegion: region(4), AuthMode: "bearer"},
			wantOut:     i18n.Tf("cmd.provider.hub_configured", "bedrock"),
		},
		{
			name: "AWS profile, hub scope",
			answers: map[int]func(f *tview.Form){
				1: func(f *tview.Form) { choose(f, 0, 1) },
				3: func(f *tview.Form) { typeInto(f, 0, "dev"); choose(f, 1, 2) },
			},
			abortAt: -1, wantRan: []int{1, 3, 4}, wantSecrets: map[string]string{},
			wantHub: config.ProviderConfig{AWSProfile: "dev", AWSRegion: region(2), AuthMode: "profile"},
			wantOut: i18n.Tf("cmd.provider.hub_configured", "bedrock"),
		},
		{
			name: "AWS profile left to its default",
			answers: map[int]func(f *tview.Form){
				1: func(f *tview.Form) { choose(f, 0, 1) },
				3: func(f *tview.Form) { choose(f, 1, 1) },
			},
			abortAt: -1, wantRan: []int{1, 3, 4}, wantSecrets: map[string]string{},
			wantHub: config.ProviderConfig{AWSProfile: "default", AWSRegion: region(1), AuthMode: "profile"},
			wantOut: i18n.Tf("cmd.provider.hub_configured", "bedrock"),
		},
		{
			name:    "bearer token, project scope",
			project: true,
			answers: map[int]func(f *tview.Form){
				1: func(f *tview.Form) { choose(f, 0, 0) },
				2: func(f *tview.Form) { typeInto(f, 0, "ABSK-2"); choose(f, 1, 4) },
			},
			abortAt: -1, wantRan: []int{1, 2, 4},
			wantSecrets: map[string]string{projectKey: "ABSK-2"},
			wantProject: &domain.ProjectProviderConfig{AWSRegion: region(4), AuthMode: "bearer", TokenKey: projectKey},
			wantOut:     i18n.Tf("cmd.provider.project_configured", "bedrock", "demo"),
		},
		{
			name: "no region chosen: the step is refused, nothing is written",
			answers: map[int]func(f *tview.Form){
				1: func(f *tview.Form) { choose(f, 0, 0) },
				2: func(f *tview.Form) { typeInto(f, 0, "ABSK-3") },
			},
			abortAt: -1, wantRan: []int{1, 2}, wantSecrets: map[string]string{},
			wantRefused: map[int]string{2: i18n.T("cmd.init.wizard_region_required")},
		},
		{
			name:    "API key of the shell, used as it is",
			env:     map[string]string{"AWS_BEARER_TOKEN_BEDROCK": "ABSK-shell"},
			answers: map[int]func(f *tview.Form){0: func(f *tview.Form) { tick(f, 0, false); tick(f, 0, true) }},
			abortAt: -1, wantRan: []int{0},
			wantSecrets: map[string]string{hubKey: "ABSK-shell"},
			wantHub:     config.ProviderConfig{AuthMode: "bearer"},
			wantOut:     i18n.Tf("cmd.provider.from_env.done", hubKey),
		},
		{
			name:    "API key of the shell, ticked box left as it is",
			env:     map[string]string{"AWS_BEARER_TOKEN_BEDROCK": "ABSK-shell"},
			abortAt: -1, wantRan: []int{0},
			wantSecrets: map[string]string{hubKey: "ABSK-shell"},
			wantHub:     config.ProviderConfig{AuthMode: "bearer"},
			wantOut:     i18n.Tf("cmd.provider.from_env.done", hubKey),
		},
		{
			name:    "AWS keys of the shell, used as they are",
			env:     map[string]string{"AWS_ACCESS_KEY_ID": "AKIA", "AWS_SECRET_ACCESS_KEY": "secret"},
			answers: map[int]func(f *tview.Form){0: func(f *tview.Form) { tick(f, 0, false); tick(f, 0, true) }},
			abortAt: -1, wantRan: []int{0}, wantSecrets: map[string]string{},
			wantHub: config.ProviderConfig{AuthMode: "env"},
			wantOut: i18n.Tf("cmd.provider.hub_configured", "bedrock"),
		},
		{
			name: "the user quits at the last step: the token is not stored",
			answers: map[int]func(f *tview.Form){
				1: func(f *tview.Form) { choose(f, 0, 0) },
				2: func(f *tview.Form) { typeInto(f, 0, "ABSK-4"); choose(f, 1, 4) },
			},
			abortAt: 4, wantRan: []int{1, 2}, wantSecrets: map[string]string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a, store := providerWizardEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			s := scriptWizard(t, tc.answers)
			s.abortAt = tc.abortAt
			var project *domain.Project
			if tc.project {
				project = &store.projects[0]
			}

			require.NoError(t, setupBedrock(t.Context(), a, project))

			assert.Equal(t, "provider setup", s.command, "command of the wizard")
			assert.Equal(t, tc.wantRan, s.ran, "steps that ran")
			if tc.wantRefused != nil {
				assert.Equal(t, tc.wantRefused, s.refused, "steps refused")
			} else {
				assert.Empty(t, s.refused, "steps refused")
			}
			assert.Equal(t, tc.wantSecrets, a.Secrets.(*mockSecretStore).secrets, "keychain")
			assert.Equal(t, tc.wantHub, savedHub(t).Provider.Bedrock, "hub.toml")
			out := a.IO.Out.(interface{ String() string }).String()
			if tc.wantOut == "" {
				assert.Empty(t, out, "output")
			} else {
				assert.Contains(t, out, tc.wantOut, "output")
			}
			assert.Equal(t, tc.wantProject, store.projects[0].ProviderConfig, "project provider config")
		})
	}
}
