package cmd

import (
	"context"
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// deletingStore remembers the projects that were deleted.
type deletingStore struct {
	*mockProjectStore
	deleted []string
}

func (s *deletingStore) Delete(_ context.Context, id string) error {
	s.deleted = append(s.deleted, id)
	return nil
}

// oh project remove, without an id: the user picks a project and confirms.
func TestProjectRemoveWizard(t *testing.T) {
	i18ntest.Set(t, "en")
	pick := func(opt int, confirm bool) map[int]func(f *tview.Form) {
		return map[int]func(f *tview.Form){
			0: func(f *tview.Form) { choose(f, 0, opt) },
			1: func(f *tview.Form) { tick(f, 0, confirm) },
		}
	}
	tests := []struct {
		name        string
		answers     map[int]func(f *tview.Form)
		abortAt     int
		wantRan     []int
		wantDeleted []string
		wantOut     string
	}{
		{"the second project is picked and confirmed", pick(1, true), -1, []int{0, 1}, []string{"p2"}, "removed"},
		{"the first project is the default", pick(0, true), -1, []int{0, 1}, []string{"p1"}, "removed"},
		{"not confirmed: nothing is deleted, no error", pick(1, false), -1, []int{0, 1}, nil, i18n.T("form.project.cancelled")},
		{"the user quits at the confirmation", pick(1, true), 1, []int{0}, nil, ""},
		{"the user quits at once", pick(1, true), 0, nil, nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &deletingStore{mockProjectStore: &mockProjectStore{projects: []domain.Project{
				{ID: "p1", Name: "alpha", Path: "/a", Status: domain.ProjectStatusActive},
				{ID: "p2", Name: "beta", Path: "/b", Status: domain.ProjectStatusActive},
			}}}
			io, out, _ := testIO()
			application = &app.App{Config: &config.Config{}, Projects: store, IO: io}
			t.Cleanup(func() { application = nil })
			s := scriptWizard(t, tc.answers)
			s.abortAt = tc.abortAt

			cmd := projectRemoveCmd()
			cmd.SetContext(t.Context())
			require.NoError(t, cmd.RunE(cmd, nil), "the command")

			assert.Equal(t, "project remove", s.command, "command of the wizard")
			assert.Equal(t, tc.wantRan, s.ran, "steps that ran")
			assert.Equal(t, tc.wantDeleted, store.deleted, "projects deleted")
			if tc.wantOut == "" {
				assert.Empty(t, out.String(), "output")
			} else {
				assert.Contains(t, out.String(), tc.wantOut, "output")
			}
		})
	}
}
