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
	return fmt.Sprintf("j/k %s · Enter %s · a %s · c %s · r %s", i18n.T("tui.hints.nav"), i18n.T("tui.hints.detail"), i18n.T("tui.hints.add"), i18n.T("tui.hints.check"), i18n.T("tui.hints.refresh"))
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

func (v *PoliciesView) getRepo() teamstate.TeamStateWriter {
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
		v.list.AddItem("  "+i18n.T("tui.policies.team_not_configured"), "", 0, nil)
		return
	}

	syncAsync(v.app, repo, v.shell, func(_ error) {
		v.renderPolicies(repo)
	})
}

func (v *PoliciesView) renderPolicies(repo teamstate.TeamStateWriter) {
	if v.list == nil {
		return
	}
	v.list.Clear()
	v.policies = nil

	policies, err := repo.LoadPolicies("")
	if err != nil {
		v.list.AddItem("  "+i18n.T("tui.settings.error")+": "+err.Error(), "", 0, nil)
		return
	}
	v.policies = policies

	if len(policies) == 0 {
		v.list.AddItem("  "+i18n.T("tui.policies.empty"), "", 0, nil)
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
	detail += fmt.Sprintf("%-14s %s\n", i18n.T("tui.policies.field_name")+":", p.Name)
	detail += fmt.Sprintf("%-14s %s\n", i18n.T("tui.policies.field_type")+":", p.Type)
	detail += fmt.Sprintf("%-14s %s\n", i18n.T("tui.policies.field_enforcement")+":", p.Enforcement)
	detail += fmt.Sprintf("%-14s %s\n", i18n.T("tui.policies.field_message")+":", p.Message)
	if p.Rule != "" {
		detail += fmt.Sprintf("%-14s %s\n", i18n.T("tui.policies.field_rule")+":", p.Rule)
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
			{Label: i18n.T("tui.policies.close"), Callback: func() {}},
		})
	}
}

func (v *PoliciesView) checkPolicies() {
	if v.shell == nil {
		return
	}

	repo := v.getRepo()
	if repo == nil {
		v.shell.ShowToastMsg(i18n.T("tui.policies.team_not_configured"), false)
		return
	}

	// Build minimal context (no git diff in TUI — just branch name check)
	ctx := teamstate.PolicyContext{
		MemberID: v.resolveTeam().MemberID,
	}

	results, err := repo.CheckAll("", ctx)
	if err != nil {
		v.shell.ShowToastMsg(i18n.T("tui.policies.check_error")+": "+err.Error(), false)
		return
	}

	if len(results) == 0 {
		v.shell.ShowToastMsg(i18n.T("tui.policies.nothing_to_check"), true)
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

	text += fmt.Sprintf("\n  %s: %d/%d", i18n.T("tui.policies.result"), passed, len(results))

	v.shell.ShowScrollableModal(i18n.T("tui.policies.check_title"), text, []ModalAction{
		{Label: i18n.T("tui.policies.close"), Callback: func() {}},
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
	{Label: "Warn", Value: "warn"},
	{Label: "Refuse", Value: "refuse"},
}

var policyScopeOptions = []SelectOption{
	{Label: "diff_only", Value: "diff_only"},
	{Label: "modified_files", Value: "modified_files"},
	{Label: "all_files", Value: "all_files"},
}

func (v *PoliciesView) addPolicy() {
	if v.shell == nil {
		return
	}
	repo := v.getRepo()
	if repo == nil {
		v.shell.ShowToastMsg(i18n.T("tui.policies.team_not_configured"), false)
		return
	}

	v.shell.ShowInlineForm(InlineFormConfig{
		Title: i18n.T("tui.policies.create_title"),
		Fields: []FormField{
			{Key: "name", Label: i18n.T("tui.policies.field_name_slug"), Type: FieldText, Required: true},
			{Key: "type", Label: i18n.T("tui.policies.field_type"), Type: FieldSelect, Options: policyTypeOptions, Default: "regex", Required: true},
			{Key: "enforcement", Label: i18n.T("tui.policies.field_enforcement"), Type: FieldSelect, Options: policyEnforcementOptions, Default: "warn"},
			{Key: "message", Label: i18n.T("tui.policies.field_message"), Type: FieldText},
			{Key: "rule", Label: i18n.T("tui.policies.field_rule"), Type: FieldText,
				Conditional: func(v map[string]string) bool { return v["type"] == "regex" }},
			{Key: "patterns", Label: i18n.T("tui.policies.field_patterns"), Type: FieldText,
				Conditional: func(v map[string]string) bool { return v["type"] == "forbidden_pattern" }},
			{Key: "scope", Label: i18n.T("tui.policies.field_scope"), Type: FieldSelect, Options: policyScopeOptions, Default: "diff_only",
				Conditional: func(v map[string]string) bool { return v["type"] == "forbidden_pattern" }},
			{Key: "max", Label: i18n.T("tui.policies.field_max"), Type: FieldText, Default: "3",
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

func (v *PoliciesView) writePolicyToml(repo teamstate.TeamStateWriter, name, pType, enforcement, message, rule string, patterns []string, scope string, max int) {
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
			v.shell.ShowToastMsg(i18n.T("tui.settings.error")+": "+err.Error(), false)
		}
		return
	}
	_, err = f.WriteString(sb.String())
	f.Close()
	if err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.policies.write_error")+": "+err.Error(), false)
		}
		return
	}

	// Commit and push
	if err := repo.CommitAndPush(context.Background(), fmt.Sprintf("policies: add %s", name), "policies.toml"); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.policies.commit_error")+": "+err.Error(), false)
		}
		return
	}

	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.Tf("tui.policies.added", name), true)
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
		{ID: "policies.add", Label: i18n.T("tui.hints.add"), Aliases: []string{"add", "new", "ajouter"}, Description: i18n.T("tui.policies.cmd_add"), Category: "Policies", Action: func() { v.addPolicy() }},
		{ID: "policies.check", Label: i18n.T("tui.hints.check"), Aliases: []string{"check", "verify", "vérifier"}, Description: i18n.T("tui.policies.cmd_check"), Category: "Policies", Action: func() { v.checkPolicies() }},
	}
}
