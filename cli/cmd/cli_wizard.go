package cmd

import (
	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// cliWizard is the wizard of a command (oh provider setup, oh project add…):
// the command it belongs to and its steps.
type cliWizard struct {
	Command string
	Steps   []views.WizardStep
}

// cliWizardResult is how a CLI wizard ended: every step done, the user
// quitting, or a step failing.
type cliWizardResult struct {
	Completed bool
	Aborted   bool
	Err       error
}

// runCLIWizard runs the wizard of a command and waits for its end. It is a
// variable so that the tests answer the steps without a terminal.
var runCLIWizard = func(a *app.App, w cliWizard) cliWizardResult {
	res := views.RunWizard(views.WizardConfig{
		Layout: layout.Config{
			ProjectName: a.Config.Name,
			Command:     w.Command,
			StatusHints: i18n.T("wizard.hints.default"),
		},
		Steps: w.Steps,
	})
	return cliWizardResult{Completed: res.Completed, Aborted: res.Aborted, Err: res.Err}
}
