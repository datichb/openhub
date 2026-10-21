package views

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunInlineWizardStandalone(t *testing.T) {
	tests := []struct {
		name       string
		withHook   bool
		wantResult StandaloneResult
	}{
		{"all steps skipped: the OnComplete hook is forwarded", true, StandaloneResult{Completed: true}},
		{"no OnComplete hook", false, StandaloneResult{Completed: true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var completed []bool
			cfg := InlineWizardConfig{
				ID:    "test.standalone",
				Title: "Standalone",
				Steps: []WizardStep{{Label: "A", Skip: true}, {Label: "B", SkipIf: func() bool { return true }}},
			}
			if tc.withHook {
				cfg.OnComplete = func(ok bool, err error) {
					assert.NoError(t, err, "OnComplete error")
					completed = append(completed, ok)
				}
			}

			var result StandaloneResult
			require.NotPanics(t, func() { result = RunInlineWizardStandalone(NewInlineWizardView(cfg)) }, "run")
			assert.Equal(t, tc.wantResult, result, "result")
			if tc.withHook {
				assert.Equal(t, []bool{true}, completed, "the original OnComplete fires once, completed")
			}
		})
	}
}

func TestStandaloneShellStub(t *testing.T) {
	t.Parallel()
	stub := &standaloneShellStub{app: tview.NewApplication()}

	t.Run("the methods the wizard uses are no-ops", func(t *testing.T) {
		require.NotPanics(t, func() {
			stub.SetOmnibarVisible(false)
			stub.SetOmnibarVisible(true)
			stub.PushView(nil)
			_ = stub.Context()
		})
	})

	t.Run("the other methods panic", func(t *testing.T) {
		calls := map[string]func(){
			"ShowInputModal":       func() { stub.ShowInputModal("", "", nil) },
			"ShowPasswordModal":    func() { stub.ShowPasswordModal("", nil) },
			"ShowSelectModal":      func() { stub.ShowSelectModal("", nil, "", nil) },
			"ShowMultiSelectModal": func() { stub.ShowMultiSelectModal("", nil, nil, nil) },
			"ShowScrollableModal":  func() { stub.ShowScrollableModal("", "", nil) },
			"ShowToastMsg":         func() { stub.ShowToastMsg("", false) },
			"ShowInlineForm":       func() { stub.ShowInlineForm(InlineFormConfig{}) },
			"SetProjectMode":       func() { stub.SetProjectMode(nil) },
			"SetActiveProject":     func() { stub.SetActiveProject(nil) },
			"ActiveProject":        func() { _ = stub.ActiveProject() },
			"SetMode":              func() { stub.SetMode("") },
			"Mode":                 func() { _ = stub.Mode() },
			"SetActiveTeam":        func() { stub.SetActiveTeam(nil) },
			"ActiveTeam":           func() { _ = stub.ActiveTeam() },
		}
		for name, call := range calls {
			assert.Panics(t, call, name)
		}
	})
}
