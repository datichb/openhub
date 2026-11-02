package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/provider"
)

// fakeBd puts a bd on the PATH (and nothing else) that records its arguments;
// without it, no bd is found.
func fakeBd(t *testing.T, withBd bool) (log string) {
	t.Helper()
	dir := t.TempDir()
	log = filepath.Join(dir, "bd.log")
	if withBd {
		if runtime.GOOS == "windows" {
			t.Skip("shell script")
		}
		script := "#!/bin/sh\necho \"$@\" >> " + log + "\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "bd"), []byte(script), 0o755))
	}
	t.Setenv("PATH", dir)
	return log
}

// oh project add (no flag): the answers of the wizard make the project of the
// registry, with its provider, its MCP services and its team.
func TestProjectAddWizard(t *testing.T) {
	i18ntest.Set(t, "en")
	// Steps: 0 identity (name, path, language), 1 beads (check box, when bd is
	// installed), 2 custom provider? (check box), 3 provider, model and key
	// (when custom), 4 MCP services (three check boxes), 5 team (drop-down).
	identity := func(name, path string, lang int) func(f *tview.Form) {
		return func(f *tview.Form) {
			typeInto(f, 0, name)
			typeInto(f, 1, path)
			choose(f, 2, lang)
		}
	}
	tests := []struct {
		name        string
		withBd      bool
		teams       []config.TeamConfig
		answers     func(dir string) map[int]func(f *tview.Form)
		abortAt     int
		wantRan     []int
		wantErr     string
		wantProject *domain.Project // nil: none created; ID, Path and dates are not compared
		wantSecrets map[string]string
		wantOut     []string
		wantBd      string
	}{
		{
			name: "defaults: no custom provider, no MCP, no team",
			answers: func(dir string) map[int]func(f *tview.Form) {
				return map[int]func(f *tview.Form){0: identity("My App", dir, 1)}
			},
			abortAt: -1, wantRan: []int{0, 2, 4, 5},
			wantProject: &domain.Project{Name: "My App", Language: "typescript", Status: domain.ProjectStatusActive},
			wantSecrets: map[string]string{},
			wantOut:     []string{i18n.T("form.project.summary")},
		},
		{
			name:   "beads initialized with the project id as its prefix",
			withBd: true,
			answers: func(dir string) map[int]func(f *tview.Form) {
				return map[int]func(f *tview.Form){0: identity("Beady", dir, 0)}
			},
			abortAt: -1, wantRan: []int{0, 1, 2, 4, 5},
			wantProject: &domain.Project{Name: "Beady", Language: "go", Status: domain.ProjectStatusActive},
			wantSecrets: map[string]string{},
			wantOut:     []string{i18n.T("form.project.beads_initialized")},
			wantBd:      "init --prefix beady-",
		},
		{
			name: "custom provider, model trimmed, key in the keychain",
			answers: func(dir string) map[int]func(f *tview.Form) {
				return map[int]func(f *tview.Form){
					0: identity("Prov", dir, 0),
					2: func(f *tview.Form) { tick(f, 0, true) },
					3: func(f *tview.Form) {
						choose(f, 0, 2) // OpenAI
						typeInto(f, 1, "  gpt-5 ")
						typeInto(f, 2, "sk-abc")
					},
				}
			},
			abortAt: -1, wantRan: []int{0, 2, 3, 4, 5},
			wantProject: &domain.Project{Name: "Prov", Language: "go", Provider: "openai", Model: "gpt-5", Status: domain.ProjectStatusActive},
			wantSecrets: map[string]string{provider.KeychainKey(provider.Name("openai"), ""): "sk-abc"},
			wantOut:     []string{i18n.T("form.project.api_key_stored")},
		},
		{
			name: "MCP services",
			answers: func(dir string) map[int]func(f *tview.Form) {
				return map[int]func(f *tview.Form){
					0: identity("Mcp", dir, 0),
					4: func(f *tview.Form) { tick(f, 0, true); tick(f, 2, true) },
				}
			},
			abortAt: -1, wantRan: []int{0, 2, 4, 5},
			wantProject: &domain.Project{
				Name: "Mcp", Language: "go", Status: domain.ProjectStatusActive,
				MCP:       []string{"figma", "gslides"},
				MCPConfig: &domain.ProjectMCPConfig{Services: []domain.ProjectMCPService{{Name: "figma"}, {Name: "gslides"}}},
			},
			wantSecrets: map[string]string{},
		},
		{
			name:  "the team of the hub is the default",
			teams: []config.TeamConfig{{ID: "acme", Enabled: true, StateRepo: "git@x:acme/state.git", MemberID: "alice"}},
			answers: func(dir string) map[int]func(f *tview.Form) {
				return map[int]func(f *tview.Form){0: identity("Teamed", dir, 0)}
			},
			abortAt: -1, wantRan: []int{0, 2, 4, 5},
			wantProject: &domain.Project{Name: "Teamed", Language: "go", Status: domain.ProjectStatusActive, TeamID: strPtr("acme")},
			wantSecrets: map[string]string{},
		},
		{
			name:  "no team although the hub has one",
			teams: []config.TeamConfig{{ID: "acme", Enabled: true, StateRepo: "git@x:acme/state.git", MemberID: "alice"}},
			answers: func(dir string) map[int]func(f *tview.Form) {
				return map[int]func(f *tview.Form){
					0: identity("Alone", dir, 0),
					5: func(f *tview.Form) { choose(f, 1, 2) },
				}
			},
			abortAt: -1, wantRan: []int{0, 2, 4, 5},
			wantProject: &domain.Project{Name: "Alone", Language: "go", Status: domain.ProjectStatusActive},
			wantSecrets: map[string]string{},
		},
		{
			name: "no name: the command fails, nothing is created",
			answers: func(dir string) map[int]func(f *tview.Form) {
				return map[int]func(f *tview.Form){0: identity("", dir, 0)}
			},
			abortAt: -1, wantRan: []int{0}, wantErr: i18n.T("cmd.init.project_name_required"), wantSecrets: map[string]string{},
		},
		{
			name: "a path that does not exist: the command fails",
			answers: func(dir string) map[int]func(f *tview.Form) {
				return map[int]func(f *tview.Form){0: identity("Ghost", filepath.Join(dir, "missing"), 0)}
			},
			abortAt: -1, wantRan: []int{0}, wantErr: "missing", wantSecrets: map[string]string{},
		},
		{
			name: "the user quits at the team step: nothing is created",
			answers: func(dir string) map[int]func(f *tview.Form) {
				return map[int]func(f *tview.Form){0: identity("Quit", dir, 0)}
			},
			abortAt: 5, wantRan: []int{0, 2, 4}, wantSecrets: map[string]string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configtest.Home(t)
			bdLog := fakeBd(t, tc.withBd)
			dir := t.TempDir()
			a := newMockApp(nil, nil)
			a.Config.Teams = tc.teams
			io, out, _ := testIO()
			a.IO = io
			store := &mockProjectStore{}
			a.Projects = store
			s := scriptWizard(t, tc.answers(dir))
			s.abortAt = tc.abortAt

			err := runProjectAddInteractive(t.Context(), a)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, "project add", s.command, "command of the wizard")
			assert.Equal(t, tc.wantRan, s.ran, "steps that ran")
			assert.Equal(t, tc.wantSecrets, a.Secrets.(*mockSecretStore).secrets, "keychain")
			for _, want := range tc.wantOut {
				assert.Contains(t, out.String(), want, "output")
			}
			if tc.wantBd != "" {
				logged, err := os.ReadFile(bdLog)
				require.NoError(t, err)
				assert.Contains(t, string(logged), tc.wantBd, "bd calls")
			}
			if tc.wantProject == nil {
				assert.Empty(t, store.projects, "projects created")
				return
			}
			require.Len(t, store.projects, 1, "projects created")
			got := store.projects[0]
			wantID := domain.ProjectSlug(tc.wantProject.Name) + "-"
			assert.True(t, strings.HasPrefix(got.ID, wantID), "project id %q starts with %q", got.ID, wantID)
			assert.Equal(t, dir, got.Path, "path")
			want := *tc.wantProject
			want.ID, want.Path, want.CreatedAt, want.UpdatedAt = got.ID, got.Path, got.CreatedAt, got.UpdatedAt
			assert.Equal(t, want, got, "project")
		})
	}
}

func strPtr(s string) *string { return &s }
