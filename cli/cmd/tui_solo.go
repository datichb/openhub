package cmd

import (
	"context"
	"log/slog"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// TUI wiring of the solo space (P2-T16): creation from the catalogue,
// attachment of a new project, « Passer en équipe » from team.detail. The
// logic is initSoloTeam / attachProjectSolo / promoteSoloTeam (CLI).

// initWizardSoloSpace attaches the first-run wizard's project to a solo
// space when the user chose one (no error blocks the wizard).
func initWizardSoloSpace(ctx context.Context, s *initStepState, project *domain.Project) {
	if s.TeamState == nil || !s.TeamState.SoloSpace || project == nil || (project.TeamID != nil && *project.TeamID != "") {
		return
	}
	if _, err := attachProjectSolo(ctx, *s.AppPtr, project); err != nil {
		slog.Warn("solo space for the first-run project", "project", project.Name, "error", err)
	}
}

// tuiAttachProjectSolo attaches a project added in the TUI to a solo space.
func tuiAttachProjectSolo(project *domain.Project) {
	a := TryApp()
	sh := tuiShell
	if a == nil || sh == nil {
		return
	}
	go func() {
		sp, err := attachProjectSolo(sh.Context(), a, project)
		sh.App().QueueUpdateDraw(func() {
			if err != nil {
				sh.ShowToast(i18n.Tf("tui.solo.project.failed", err.Error()), shell.ToastError)
				return
			}
			sh.ShowToast(i18n.Tf("tui.solo.project.done", project.Name, sp.Team.ID), shell.ToastSuccess)
		})
	}()
}

// tuiCreateSoloSpace asks for the solo space (id, member, project to
// attach) and creates it; then is called on success.
func tuiCreateSoloSpace(then func()) {
	a := TryApp()
	sh := tuiShell
	if a == nil || a.Config == nil || sh == nil {
		return
	}
	fields := []views.FormField{
		{Key: "id", Label: i18n.T("tui.solo.form.id"), Type: views.FieldText, Default: nextSoloID(a.Config), Required: true,
			Hint: i18n.T("tui.solo.form.intro")},
		{Key: "member", Label: i18n.T("tui.solo.form.member"), Type: views.FieldText, Default: defaultSoloMemberID(a.Config), Required: true},
	}
	var project *domain.Project
	if p, err := resolveActiveProject(a); err == nil && p != nil && (p.TeamID == nil || *p.TeamID == "") {
		project = p
		fields = append(fields, views.FormField{Key: "attach", Label: i18n.Tf("tui.solo.form.attach", p.Name), Type: views.FieldSelect, Default: "yes",
			Options: []views.SelectOption{{Label: i18n.T("tui.solo.form.yes"), Value: "yes"}, {Label: i18n.T("tui.solo.form.no"), Value: "no"}}})
	}
	sh.ShowInlineForm(views.InlineFormConfig{
		Title:  i18n.T("tui.solo.form.title"),
		Fields: fields,
		OnSubmit: func(vals map[string]string, _ map[string][]string) {
			p := soloTeamParams{ID: strings.TrimSpace(vals["id"]), MemberID: strings.TrimSpace(vals["member"])}
			if project != nil && vals["attach"] == "yes" {
				p.ProjectRef = project.ID
			}
			go func() {
				res, err := initSoloTeam(sh.Context(), a, p)
				sh.App().QueueUpdateDraw(func() {
					if err != nil {
						sh.ShowToast(err.Error(), shell.ToastError)
						return
					}
					msg := i18n.Tf("tui.solo.created", res.Team.ID)
					if res.Project != nil {
						msg = i18n.Tf("tui.solo.created_attached", res.Team.ID, res.Project.Name)
					}
					sh.ShowToast(msg, shell.ToastSuccess)
					if then != nil {
						then()
					}
				})
			}()
		},
	})
}

// tuiPromoteSolo turns the solo space teamID into a team (remote URL).
func tuiPromoteSolo(teamID, remote string) {
	a := TryApp()
	sh := tuiShell
	if a == nil || sh == nil {
		return
	}
	sh.ShowToast(i18n.T("tui.solo.promoting"), shell.ToastInfo)
	go func() {
		team, err := promoteSoloTeam(sh.Context(), a, teamID, remote)
		sh.App().QueueUpdateDraw(func() {
			if err != nil {
				sh.ShowToast(err.Error(), shell.ToastError)
				return
			}
			sh.ShowScrollableModal(i18n.T("tui.solo.promoted_title"), i18n.Tf("tui.solo.promoted_body", team.ID, remote), []views.ModalAction{
				{Label: i18n.T("tui.catalog.edit.close"), Callback: func() {}},
			})
			sh.RemountIf("team.detail")
		})
	}()
}
