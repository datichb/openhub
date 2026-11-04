package cmd

import (
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// buildTeamAbandonedSteps returns the steps that follow the team group of the
// init wizard when the team was given up (« Passer l'équipe », a step of the
// group skipped, « Ignorer »): they say so in the recap, ask again for what the
// team would have given, and offer the solo space.
//
//   - team_result: the recap says that the team is not configured (only when
//     the user gave it up from a step, not for the explicit « Ignorer »).
//   - provider_late: in the « team » setup type the provider is not asked, the
//     team being supposed to bring it; without a team nothing brings it, so
//     the provider step is asked here, after the group.
//   - team_solo_offer: the solo workflow space for the project of the wizard,
//     offered (not created without a yes) when the team was given up on the way.
func buildTeamAbandonedSteps(s *initStepState) []views.WizardStep {
	t := s.TeamState

	result := views.WizardStep{
		ID:            "team_result",
		Label:         i18n.T("cmd.init.wizard_step_team"),
		SidebarHidden: true,
		SkipIf:        func() bool { return !t.Abandoned },
		InfoFields: func() []views.InfoField {
			return []views.InfoField{{
				Label: i18n.T("cmd.init.wizard_team_status_label"),
				Value: i18n.T("v532w.team.not_configured"),
			}}
		},
	}

	provider := buildProviderStepAs(s, "provider_late")
	provider.SkipIf = func() bool {
		return s.SetupMode != "team" || !t.Skipped || s.ProviderSkipped
	}

	offer := views.WizardStep{
		ID:     "team_solo_offer",
		Label:  i18n.T("v532w.solo.label"),
		SkipIf: func() bool { return !t.Abandoned || t.SoloSpace },
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			idx := 1
			if t.SoloSpace {
				idx = 0
			}
			form.AddTextView("", i18n.T("v532w.solo.hint"), 70, 3, true, false)
			form.AddDropDown(i18n.T("v532w.solo.question"),
				[]string{i18n.T("v532w.solo.yes"), i18n.T("v532w.solo.no")}, idx,
				func(_ string, i int) { t.SoloSpace = i == 0 })
			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		InfoFields: func() []views.InfoField {
			value := i18n.T("v532w.solo.no")
			if t.SoloSpace {
				value = i18n.T("v532w.solo.yes")
			}
			return []views.InfoField{{Label: i18n.T("v532w.solo.label"), Value: value}}
		},
	}
	return []views.WizardStep{result, provider, offer}
}
