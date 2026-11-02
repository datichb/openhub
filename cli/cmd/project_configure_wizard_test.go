package cmd

import (
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// oh project configure: the answers of the form are written to the project,
// and only when one of them changed it.
func TestProjectConfigureWizard(t *testing.T) {
	i18ntest.Set(t, "en")
	tests := []struct {
		name        string
		answers     map[int]func(f *tview.Form)
		abortAt     int
		want        domain.Project
		wantUpdated bool
		wantOut     string
	}{
		{
			name: "new language, provider and model are written",
			answers: map[int]func(f *tview.Form){0: func(f *tview.Form) {
				choose(f, 0, 3) // Rust
				typeInto(f, 1, "openai")
				typeInto(f, 2, "gpt-5")
			}},
			abortAt:     -1,
			want:        domain.Project{ID: "p1", Name: "demo", Language: "rust", Provider: "openai", Model: "gpt-5"},
			wantUpdated: true,
			wantOut:     i18n.T("cmd.project.config_updated"),
		},
		{
			name:    "nothing changed: nothing is written",
			answers: map[int]func(f *tview.Form){},
			abortAt: -1,
			want:    domain.Project{ID: "p1", Name: "demo", Language: "go", Provider: "bedrock", Model: "m"},
			wantOut: i18n.T("cmd.project.no_changes"),
		},
		{
			name:    "the user quits: nothing is written",
			abortAt: 0,
			want:    domain.Project{ID: "p1", Name: "demo", Language: "go", Provider: "bedrock", Model: "m"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &mockProjectStore{projects: []domain.Project{{ID: "p1", Name: "demo", Language: "go", Provider: "bedrock", Model: "m"}}}
			io, out, _ := testIO()
			a := &app.App{Config: &config.Config{}, Projects: store, IO: io}
			s := scriptWizard(t, tc.answers)
			s.abortAt = tc.abortAt

			project, err := store.Get(t.Context(), "p1")
			require.NoError(t, err)
			require.NoError(t, runProjectConfigureInteractive(t.Context(), a, project))

			assert.Equal(t, "project configure", s.command, "command of the wizard")
			got := store.projects[0]
			assert.Equal(t, tc.wantUpdated, !got.UpdatedAt.IsZero(), "the project was updated")
			got.UpdatedAt = time.Time{}
			assert.Equal(t, tc.want, got, "project in the store")
			if tc.wantOut == "" {
				assert.Empty(t, out.String(), "output")
			} else {
				assert.Contains(t, out.String(), tc.wantOut, "output")
			}
		})
	}
}
