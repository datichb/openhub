package cmd

import (
	"bytes"
	"testing"

	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// oh mcp setup <service>: the token goes to the keychain, the choices to
// hub.toml (hub scope) or to the project (project scope).
func TestMCPSetupWizard(t *testing.T) {
	i18ntest.Set(t, "en")
	// Steps: 0 token (password), 1 write access (check box, services that
	// have write tools only), 2 persist.
	answers := func(token string, write bool) map[int]func(f *tview.Form) {
		return map[int]func(f *tview.Form){
			0: func(f *tview.Form) { typeInto(f, 0, token) },
			1: func(f *tview.Form) { tick(f, 0, write) },
		}
	}
	tests := []struct {
		name        string
		service     string
		project     bool
		answers     map[int]func(f *tview.Form)
		abortAt     int
		wantRan     []int
		wantSecrets map[string]string
		wantHub     config.MCPServerConfig
		wantService *domain.ProjectMCPService
		wantOut     []string
	}{
		{
			name: "hub scope, token and write access", service: "gitlab", answers: answers("tok", true), abortAt: -1,
			wantRan:     []int{0, 1, 2},
			wantSecrets: map[string]string{config.MCPTokenKey("gitlab"): "tok"},
			wantHub:     config.MCPServerConfig{Enabled: true, Token: config.MCPTokenKey("gitlab"), WriteEnabled: true},
			wantOut:     []string{i18n.Tf("cmd.service.enabled", "gitlab")},
		},
		{
			name: "hub scope, read only", service: "jira", answers: answers("jt", false), abortAt: -1,
			wantRan:     []int{0, 1, 2},
			wantSecrets: map[string]string{config.MCPTokenKey("jira"): "jt"},
			wantHub:     config.MCPServerConfig{Enabled: true, Token: config.MCPTokenKey("jira")},
		},
		{
			name: "project scope, a service without write tools", service: "figma", project: true, answers: answers("ft", false), abortAt: -1,
			wantRan:     []int{0, 2},
			wantSecrets: map[string]string{config.MCPProjectTokenKey("figma", "p1"): "ft"},
			wantService: &domain.ProjectMCPService{Name: "figma", Enabled: boolPtr(true), TokenKey: config.MCPProjectTokenKey("figma", "p1")},
			wantOut:     []string{i18n.Tf("cmd.service.project_configured", "figma", "demo")},
		},
		{
			name: "project scope, no token: the hub token is inherited", service: "gitlab", project: true, answers: answers("", true), abortAt: -1,
			wantRan:     []int{0, 1, 2},
			wantSecrets: map[string]string{},
			wantService: &domain.ProjectMCPService{Name: "gitlab", Enabled: boolPtr(true), WriteEnabled: boolPtr(true)},
			wantOut:     []string{i18n.Tf("cmd.service.token_empty_warning", "GITLAB_TOKEN")},
		},
		{
			name: "the user quits at the last step: the token is stored, hub.toml is untouched", service: "gitlab", answers: answers("tok", true), abortAt: 2,
			wantRan: []int{0, 1}, wantSecrets: map[string]string{config.MCPTokenKey("gitlab"): "tok"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configtest.Home(t)
			require.NoError(t, config.Save(&config.Config{}))
			a := newMockApp(nil, nil)
			io, _, _ := testIO()
			a.IO = io
			store := &mockProjectStore{projects: []domain.Project{{ID: "p1", Name: "demo"}}}
			a.Projects = store
			application = a
			t.Cleanup(func() { application = nil })
			s := scriptWizard(t, tc.answers)
			s.abortAt = tc.abortAt

			var out bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&out)
			cmd.SetContext(t.Context())
			var project *domain.Project
			if tc.project {
				project = &store.projects[0]
			}
			require.NoError(t, runMCPSetupForService(cmd, tc.service, project))

			assert.Equal(t, "mcp setup", s.command, "command of the wizard")
			assert.Equal(t, tc.wantRan, s.ran, "steps that ran")
			assert.Equal(t, tc.wantSecrets, a.Secrets.(*mockSecretStore).secrets, "keychain")
			for _, want := range tc.wantOut {
				assert.Contains(t, out.String(), want, "output")
			}
			if tc.abortAt >= 0 {
				assert.Empty(t, out.String(), "output")
			}

			config.Reset()
			saved, err := config.Load()
			require.NoError(t, err)
			hub := saved.MCPServer(tc.service)
			require.NotNil(t, hub)
			assert.Equal(t, tc.wantHub, *hub, "hub.toml")
			if tc.wantService == nil {
				assert.Nil(t, store.projects[0].MCPConfig, "project")
				return
			}
			require.NotNil(t, store.projects[0].MCPConfig, "project")
			assert.Equal(t, []domain.ProjectMCPService{*tc.wantService}, store.projects[0].MCPConfig.Services, "project services")
		})
	}
}
