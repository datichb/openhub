package cmd

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// initRun is what a pass through the init wizard went through.
type initRun struct {
	screens   []string // the steps the wizard displayed, in order
	completed bool
}

// driveInitWizard mounts the real wizard engine on the real steps and
// confirms every screen it displays. The steps keep their rendering and their
// skip rules; their checks and their writes are dropped (they are tested one by
// one), so the pass counts what the engine shows for the given choices.
func driveInitWizard(t *testing.T, setupMode, teamChoice string) initRun {
	t.Helper()
	configtest.Home(t)
	i18ntest.Restore(t)

	a := newMockApp(nil, nil)
	a.Config = &config.Config{}
	a.Projects = &mockProjectStore{projects: []domain.Project{}}
	cfg, s := buildInitWizardConfig(a)

	var run initRun
	cfg.OnComplete = func(completed bool, _ error) { run.completed = completed }
	for i := range cfg.Steps {
		step := &cfg.Steps[i]
		name := step.ID
		if name == "" {
			name = step.Label
		}
		id := step.ID
		custom, form := step.CustomView, step.Form
		step.Validate, step.OnDone = nil, nil
		confirm := func(onDone func()) {
			run.screens = append(run.screens, name)
			if id == "welcome" {
				s.SetupMode = setupMode
			}
			if id == "team_mode" {
				switch teamChoice {
				case "skip":
					s.TeamState.Skipped = true
				case "init", "rejoin":
					s.TeamState.Mode = teamChoice
				}
			}
			onDone()
		}
		switch {
		case custom != nil:
			step.CustomView = func(app *tview.Application, c *tview.Flex, onDone func()) {
				custom(app, c, func() {})
				confirm(onDone)
			}
		case form != nil:
			step.Form = func(app *tview.Application, onDone func()) *tview.Form {
				f := form(app, func() {})
				confirm(onDone)
				return f
			}
		}
	}

	wiz := views.NewInlineWizardView(*cfg)
	require.NotPanics(t, func() { wiz.Mount(tview.NewFlex().SetDirection(tview.FlexRow), tview.NewApplication()) })
	wiz.Unmount()
	return run
}

func TestInitWizard_ScreenCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping the wizard pass in short mode")
	}
	for _, tt := range []struct {
		name, mode, team string
	}{
		{"solo", "solo", ""},
		{"team skip", "team", "skip"},
		{"team init", "team", "init"},
		{"full skip", "full", "skip"},
		{"full init", "full", "init"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			run := driveInitWizard(t, tt.mode, tt.team)
			t.Logf("%d screens: %v", len(run.screens), run.screens)
			assert.True(t, run.completed)
		})
	}
}
