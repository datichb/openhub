package views

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// PoliciesView displays and manages team policies.
type PoliciesView struct {
	app         *tview.Application
	resolveTeam ResolveTeamFunc
	list        *tview.List
	shell       ShellAccess
	policies    []teamstate.Policy
}

var _ View = (*PoliciesView)(nil)
var _ CommandProvider = (*PoliciesView)(nil)

// NewPoliciesView creates a new policies view.
// resolveTeam is called on every refresh to obtain the effective team config.
func NewPoliciesView(resolveTeam ResolveTeamFunc) *PoliciesView {
	return &PoliciesView{resolveTeam: resolveTeam}
}

// SetShell provides the shell reference for modal interactions.
func (v *PoliciesView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *PoliciesView) ID() string { return "team.policies" }

// Title returns the display title.
func (v *PoliciesView) Title() string { return i18n.T("tui.team.policies") }

// StatusHints returns keybinding hints.
func (v *PoliciesView) StatusHints() string {
	return "j/k nav · Enter détail · a ajouter · c check · r refresh"
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
	case 'a':
		v.addPolicy()
		return nil
	}
	if event.Key() == tcell.KeyEnter {
		v.showDetail()
		return nil
	}
	return event
}

func (v *PoliciesView) getRepo() *teamstate.Repo {
	tc := v.resolveTeam()
	if !tc.Enabled {
		return nil
	}
	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
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

	syncAsync(v.app, repo, v.shell, func(_ error) {
		v.renderPolicies(repo)
	})
}

func (v *PoliciesView) renderPolicies(repo *teamstate.Repo) {
	if v.list == nil {
		return
	}
	v.list.Clear()
	v.policies = nil

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
		MemberID: v.resolveTeam().MemberID,
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

// Policy type options for the add wizard.
var policyTypeOptions = []SelectOption{
	{Label: "Regex (branch/commit)", Value: "regex"},
	{Label: "Forbidden pattern (code)", Value: "forbidden_pattern"},
	{Label: "Limit (max WIP, etc.)", Value: "limit"},
	{Label: "Boolean (toggle)", Value: "boolean"},
}

var policyEnforcementOptions = []SelectOption{
	{Label: "Warn (avertissement)", Value: "warn"},
	{Label: "Refuse (bloquant)", Value: "refuse"},
}

var policyScopeOptions = []SelectOption{
	{Label: "Diff uniquement", Value: "diff_only"},
	{Label: "Fichiers modifiés", Value: "modified_files"},
	{Label: "Tous les fichiers", Value: "all_files"},
}

func (v *PoliciesView) addPolicy() {
	if v.shell == nil {
		return
	}
	repo := v.getRepo()
	if repo == nil {
		v.shell.ShowToastMsg("Team non configurée", false)
		return
	}

	v.shell.ShowInlineForm(InlineFormConfig{
		Title: "Créer une policy",
		Fields: []FormField{
			{Key: "name", Label: "Nom (slug)", Type: FieldText, Required: true},
			{Key: "type", Label: "Type", Type: FieldSelect, Options: policyTypeOptions, Default: "regex", Required: true},
			{Key: "enforcement", Label: "Enforcement", Type: FieldSelect, Options: policyEnforcementOptions, Default: "warn"},
			{Key: "message", Label: "Message violation", Type: FieldText},
			// regex fields
			{Key: "rule", Label: "Regex pattern", Type: FieldText,
				Conditional: func(v map[string]string) bool { return v["type"] == "regex" }},
			// forbidden_pattern fields
			{Key: "patterns", Label: "Patterns (virgule)", Type: FieldText,
				Conditional: func(v map[string]string) bool { return v["type"] == "forbidden_pattern" }},
			{Key: "scope", Label: "Scope", Type: FieldSelect, Options: policyScopeOptions, Default: "diff_only",
				Conditional: func(v map[string]string) bool { return v["type"] == "forbidden_pattern" }},
			// limit field
			{Key: "max", Label: "Maximum", Type: FieldText, Default: "3",
				Conditional: func(v map[string]string) bool { return v["type"] == "limit" }},
		},
		OnSubmit: func(values map[string]string, _ map[string][]string) {
			name := values["name"]
			if name == "" {
				return
			}
			pType := values["type"]
			enforcement := values["enforcement"]
			message := values["message"]

			var patterns []string
			maxVal := 0

			switch pType {
			case "forbidden_pattern":
				patterns = splitTags(values["patterns"])
			case "limit":
				if _, err := fmt.Sscanf(values["max"], "%d", &maxVal); err != nil {
					maxVal = 3
				}
			}

			v.writePolicyToml(repo, name, pType, enforcement, message,
				values["rule"], patterns, values["scope"], maxVal)
		},
		OnCancel: nil,
	})
}

func (v *PoliciesView) writePolicyToml(repo interface{ Path() string; CommitAndPush(ctx context.Context, msg string, files ...string) error }, name, pType, enforcement, message, rule string, patterns []string, scope string, max int) {
	// Build TOML block
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n[policies.%s]\n", name))
	sb.WriteString(fmt.Sprintf("type = %q\n", pType))
	if rule != "" {
		sb.WriteString(fmt.Sprintf("rule = %q\n", rule))
	}
	if len(patterns) > 0 {
		sb.WriteString(fmt.Sprintf("patterns = [%s]\n", quoteSlice(patterns)))
	}
	if scope != "" {
		sb.WriteString(fmt.Sprintf("scope = %q\n", scope))
	}
	if max > 0 {
		sb.WriteString(fmt.Sprintf("max = %d\n", max))
	}
	if pType == "boolean" {
		sb.WriteString("enabled = true\n")
	}
	sb.WriteString(fmt.Sprintf("enforcement = %q\n", enforcement))
	if message != "" {
		sb.WriteString(fmt.Sprintf("message = %q\n", message))
	}

	// Append to policies.toml
	policiesPath := filepath.Join(repo.Path(), "policies.toml")
	f, err := os.OpenFile(policiesPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg("Erreur: "+err.Error(), false)
		}
		return
	}
	_, err = f.WriteString(sb.String())
	f.Close()
	if err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg("Erreur écriture: "+err.Error(), false)
		}
		return
	}

	// Commit and push
	if err := repo.CommitAndPush(context.Background(), fmt.Sprintf("policies: add %s", name), "policies.toml"); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg("Commit échoué: "+err.Error(), false)
		}
		return
	}

	if v.shell != nil {
		v.shell.ShowToastMsg("Policy ajoutée: "+name, true)
	}
	v.refresh()
}

func quoteSlice(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf("%q", item)
	}
	return strings.Join(quoted, ", ")
}

// ContextCommands implements CommandProvider.
func (v *PoliciesView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{ID: "policies.add", Label: "Ajouter une policy", Aliases: []string{"add", "new"}, Description: "Créer une nouvelle policy", Category: "Policies", Action: func() { v.addPolicy() }},
		{ID: "policies.check", Label: "Vérifier", Aliases: []string{"check", "verify"}, Description: "Vérifier la conformité", Category: "Policies", Action: func() { v.checkPolicies() }},
	}
}
