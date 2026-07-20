package views

import (
	"context"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// PoliciesView displays and manages team policies.
type PoliciesView struct {
	app      *tview.Application
	appCtx   *app.App
	list     *tview.List
	shell    ShellAccess
	policies []teamstate.Policy
}

var _ View = (*PoliciesView)(nil)

// NewPoliciesView creates a new policies view.
func NewPoliciesView(a *app.App) *PoliciesView {
	return &PoliciesView{appCtx: a}
}

// SetShell provides the shell reference for modal interactions.
func (v *PoliciesView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *PoliciesView) ID() string { return "policies" }

// Title returns the display title.
func (v *PoliciesView) Title() string { return "Policies" }

// StatusHints returns keybinding hints.
func (v *PoliciesView) StatusHints() string {
	return "j/k nav · Enter détail · c check · r refresh · Esc retour"
}

// Mount builds the policies list.
func (v *PoliciesView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.list = tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSecondaryTextColor(theme.FgSecondary)
	v.list.SetBackgroundColor(theme.BgPanel)
	v.list.SetBorderPadding(1, 0, 2, 2)

	v.refresh()
	content.AddItem(v.list, 0, 1, true)
}

// Unmount cleans up resources.
func (v *PoliciesView) Unmount() {
	v.app = nil
	v.list = nil
}

// HandleKey processes policies view key events.
func (v *PoliciesView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'r':
		v.refresh()
		return nil
	case 'c':
		v.checkPolicies()
		return nil
	}
	if event.Key() == tcell.KeyEnter {
		v.showDetail()
		return nil
	}
	return event
}

func (v *PoliciesView) getRepo() *teamstate.Repo {
	if v.appCtx == nil || !v.appCtx.Config.Team.Enabled {
		return nil
	}
	repo := teamstate.NewRepo(v.appCtx.Config.Team.StateRepo, v.appCtx.Config.Team.StatePath)
	if !repo.IsCloned() {
		return nil
	}
	return repo
}

func (v *PoliciesView) refresh() {
	v.list.Clear()
	v.policies = nil

	repo := v.getRepo()
	if repo == nil {
		v.list.AddItem("  Team non configurée", "", 0, nil)
		return
	}

	_ = repo.Pull(context.Background())

	policies, err := repo.LoadPolicies("")
	if err != nil {
		v.list.AddItem("  Erreur: "+err.Error(), "", 0, nil)
		return
	}
	v.policies = policies

	if len(policies) == 0 {
		v.list.AddItem("  Aucune policy définie", "", 0, nil)
		return
	}

	for _, p := range policies {
		enfIcon := "⚠"
		if p.Enforcement == teamstate.EnforcementRefuse {
			enfIcon = "✗"
		}
		v.list.AddItem(
			fmt.Sprintf("  %s %s", enfIcon, p.Name),
			fmt.Sprintf("    %s · %s", p.Type, p.Message),
			0, nil,
		)
	}
}

func (v *PoliciesView) showDetail() {
	idx := v.list.GetCurrentItem()
	if idx < 0 || idx >= len(v.policies) {
		return
	}
	p := v.policies[idx]

	var detail string
	detail += fmt.Sprintf("Nom:          %s\n", p.Name)
	detail += fmt.Sprintf("Type:         %s\n", p.Type)
	detail += fmt.Sprintf("Enforcement:  %s\n", p.Enforcement)
	detail += fmt.Sprintf("Message:      %s\n", p.Message)
	if p.Rule != "" {
		detail += fmt.Sprintf("Règle:        %s\n", p.Rule)
	}
	if p.Max > 0 {
		detail += fmt.Sprintf("Max:          %d %s\n", p.Max, p.Unit)
	}
	if len(p.Patterns) > 0 {
		detail += fmt.Sprintf("Patterns:     %v\n", p.Patterns)
	}
	if p.Scope != "" {
		detail += fmt.Sprintf("Scope:        %s\n", p.Scope)
	}

	if v.shell != nil {
		v.shell.ShowScrollableModal("Policy: "+p.Name, detail, []ModalAction{
			{Label: "Fermer", Callback: func() {}},
		})
	}
}

func (v *PoliciesView) checkPolicies() {
	if v.shell == nil {
		return
	}

	repo := v.getRepo()
	if repo == nil {
		v.shell.ShowToastMsg("Team non configurée", false)
		return
	}

	// Build minimal context (no git diff in TUI — just branch name check)
	ctx := teamstate.PolicyContext{
		MemberID: v.appCtx.Config.Team.MemberID,
	}

	results, err := repo.CheckAll("", ctx)
	if err != nil {
		v.shell.ShowToastMsg("Erreur check: "+err.Error(), false)
		return
	}

	if len(results) == 0 {
		v.shell.ShowToastMsg("Aucune policy à vérifier", true)
		return
	}

	// Format results
	var text string
	passed := 0
	for _, r := range results {
		var icon string
		if r.Passed {
			icon = "[green]✓[-]"
			passed++
		} else if r.Enforcement == teamstate.EnforcementRefuse {
			icon = "[red]✗[-]"
		} else {
			icon = "[yellow]⚠[-]"
		}
		text += fmt.Sprintf("  %s %s", icon, r.Name)
		if !r.Passed {
			text += fmt.Sprintf(" — %s", r.Message)
			if r.Details != "" {
				text += fmt.Sprintf("\n      %s", r.Details)
			}
		}
		text += "\n"
	}

	text += fmt.Sprintf("\n  Résultat: %d/%d passé(s)", passed, len(results))

	v.shell.ShowScrollableModal("Check Policies", text, []ModalAction{
		{Label: "Fermer", Callback: func() {}},
	})
}
