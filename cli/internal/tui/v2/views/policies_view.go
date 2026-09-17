package views

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// policyTemplate holds pre-configured policy values for quick creation.
type policyTemplate struct {
	LabelKey    string // i18n key for the template label
	Name        string
	Type        string
	Target      string
	Rule        string
	Patterns    string // comma-separated
	Scope       string
	Max         string
	Enforcement string
	Message     string
	Enabled     bool
}

// builtinPolicyTemplates returns the list of available policy templates.
func builtinPolicyTemplates() []policyTemplate {
	return []policyTemplate{
		{
			LabelKey:    "tui.policies.tpl_branch_naming",
			Name:        "branch-naming",
			Type:        "regex",
			Target:      "branch_name",
			Rule:        `^(feat|fix|chore|docs|refactor|test)/`,
			Enforcement: "refuse",
			Message:     "Branches must follow type/description format (feat/*, fix/*, ...)",
		},
		{
			LabelKey:    "tui.policies.tpl_conventional_commits",
			Name:        "commit-message",
			Type:        "regex",
			Target:      "commit_message",
			Rule:        `^(feat|fix|chore|docs|refactor|perf|test|ci|build|style)(\(.+\))?!?: .+`,
			Enforcement: "warn",
			Message:     "Commits must follow Conventional Commits format",
		},
		{
			LabelKey:    "tui.policies.tpl_max_wip",
			Name:        "max-wip-tickets",
			Type:        "limit",
			Target:      "wip_tickets",
			Max:         "3",
			Enforcement: "warn",
			Message:     "Maximum 3 in-progress tickets per member",
		},
		{
			LabelKey:    "tui.policies.tpl_no_console_log",
			Name:        "no-console-log",
			Type:        "forbidden_pattern",
			Patterns:    "console.log, console.debug, debugger",
			Scope:       "diff_only",
			Enforcement: "refuse",
			Message:     "No console.log/debugger in committed code",
		},
		{
			LabelKey:    "tui.policies.tpl_no_todo",
			Name:        "no-todo",
			Type:        "forbidden_pattern",
			Patterns:    "TODO, FIXME, HACK, XXX",
			Scope:       "diff_only",
			Enforcement: "warn",
			Message:     "Resolve TODO/FIXME before merging",
		},
		{
			LabelKey:    "tui.policies.tpl_require_review",
			Name:        "require-review",
			Type:        "boolean",
			Target:      "review",
			Enabled:     true,
			Enforcement: "refuse",
			Message:     "Every PR must have an approved review",
		},
		{
			LabelKey:    "tui.policies.tpl_require_tests",
			Name:        "require-tests",
			Type:        "boolean",
			Target:      "tests",
			Enabled:     true,
			Enforcement: "warn",
			Message:     "Changes must include tests",
		},
		{
			LabelKey:    "tui.policies.tpl_no_secrets",
			Name:        "no-secrets",
			Type:        "forbidden_pattern",
			Patterns:    "password=, secret_key=, api_key=, AWS_SECRET, PRIVATE_KEY",
			Scope:       "diff_only",
			Enforcement: "refuse",
			Message:     "No plaintext secrets in code",
		},
	}
}

// PoliciesView displays and manages team policies.
type PoliciesView struct {
	app         *tview.Application
	resolveTeam ResolveTeamFunc
	slist       *widgets.SectionedList
	header      *tview.TextView
	contentFlex *tview.Flex
	shell       ShellAccess
	policies    []teamstate.Policy

	// actionIndices maps SectionedList item indices to action callbacks.
	actionIndices map[int]func()
	// policyIndices maps SectionedList item indices to policy slice indices.
	policyIndices map[int]int
}

var _ View = (*PoliciesView)(nil)
var _ CommandProvider = (*PoliciesView)(nil)

// NewPoliciesView creates a new policies view.
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
	return fmt.Sprintf("j/k %s · {/} %s · Enter %s · a %s · t %s · e %s · d %s · c %s · r %s",
		i18n.T("tui.hints.nav"), "sections",
		i18n.T("tui.hints.detail"), i18n.T("tui.hints.add"),
		"template", i18n.T("tui.hints.edit"),
		i18n.T("tui.hints.delete"), i18n.T("tui.hints.check"),
		i18n.T("tui.hints.refresh"))
}

// Mount builds the policies view with a descriptive header and SectionedList.
func (v *PoliciesView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.header = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false).
		SetWordWrap(true)
	v.header.SetBackgroundColor(theme.BgPanel)
	v.header.SetBorderPadding(1, 0, 2, 2)

	v.slist = widgets.NewSectionedList().SetApp(app)
	v.slist.SetBorderPadding(0, 0, 2, 2)

	v.contentFlex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(v.header, 5, 0, false).
		AddItem(v.slist, 0, 1, true)
	v.contentFlex.SetBackgroundColor(theme.BgPanel)

	v.refresh()
	content.AddItem(v.contentFlex, 0, 1, true)
}

// Unmount cleans up resources.
func (v *PoliciesView) Unmount() {
	v.app = nil
	v.slist = nil
	v.header = nil
	v.contentFlex = nil
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
	case 't':
		v.addFromTemplate()
		return nil
	case 'e':
		v.editSelectedPolicy()
		return nil
	case 'd':
		v.removeSelectedPolicy()
		return nil
	}
	if event.Key() == tcell.KeyEnter {
		if idx, _, ok := v.slist.CurrentItem(); ok {
			if fn, exists := v.actionIndices[idx]; exists {
				fn()
			} else if pi, exists := v.policyIndices[idx]; exists {
				v.showDetail(pi)
			}
		}
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
	v.policies = nil
	v.actionIndices = nil
	v.policyIndices = nil
	if v.slist != nil {
		v.slist.SetItems(nil)
	}

	repo := v.getRepo()
	if repo == nil {
		v.setHeaderText(i18n.T("tui.policies.team_not_configured"))
		return
	}

	syncAsync(v.app, repo, v.shell, func(_ error) {
		v.renderPolicies(repo)
	})
}

func (v *PoliciesView) setHeaderText(text string) {
	if v.header == nil {
		return
	}
	secondary := theme.ColorTag(theme.TextSecondaryHex)
	reset := theme.TagReset
	v.header.SetText(fmt.Sprintf("%s%s%s", secondary, text, reset))
}

// policySectionKey returns the i18n key for a policy type section header.
func policySectionKey(t teamstate.PolicyType) string {
	switch t {
	case teamstate.PolicyTypeRegex:
		return "tui.policies.section_regex"
	case teamstate.PolicyTypeLimit:
		return "tui.policies.section_limit"
	case teamstate.PolicyTypeForbiddenPattern:
		return "tui.policies.section_forbidden"
	case teamstate.PolicyTypeBoolean:
		return "tui.policies.section_boolean"
	default:
		return "tui.policies.section_regex"
	}
}

func (v *PoliciesView) renderPolicies(repo teamstate.TeamStateWriter) {
	if v.slist == nil {
		return
	}

	policies, err := repo.LoadPolicies("")
	if err != nil {
		v.slist.SetItems([]widgets.SectionItem{
			{MainText: "  " + i18n.T("tui.settings.error") + ": " + err.Error()},
		})
		return
	}
	v.policies = policies

	if len(policies) == 0 {
		v.setHeaderText(i18n.T("tui.policies.getting_started"))
	} else {
		v.setHeaderText(i18n.T("tui.policies.about"))
	}

	var items []widgets.SectionItem
	v.actionIndices = make(map[int]func())
	v.policyIndices = make(map[int]int)

	if len(policies) > 0 {
		// Group policies by type, with disabled policies in a separate section
		typeOrder := []teamstate.PolicyType{
			teamstate.PolicyTypeRegex,
			teamstate.PolicyTypeLimit,
			teamstate.PolicyTypeForbiddenPattern,
			teamstate.PolicyTypeBoolean,
		}

		grouped := make(map[teamstate.PolicyType][]int)
		var disabledIndices []int
		for i, p := range policies {
			if p.Enforcement == teamstate.EnforcementDisabled {
				disabledIndices = append(disabledIndices, i)
			} else {
				grouped[p.Type] = append(grouped[p.Type], i)
			}
		}

		for _, pType := range typeOrder {
			indices, ok := grouped[pType]
			if !ok || len(indices) == 0 {
				continue
			}
			sectionKey := policySectionKey(pType)
			items = append(items, widgets.SectionItem{
				MainText: fmt.Sprintf(i18n.T(sectionKey), len(indices)),
				IsHeader: true,
			})
			for _, pi := range indices {
				p := policies[pi]
				idx := len(items)
				items = append(items, v.buildPolicyItem(p))
				v.policyIndices[idx] = pi
			}
		}

		// Disabled policies section
		if len(disabledIndices) > 0 {
			items = append(items, widgets.SectionItem{
				MainText: fmt.Sprintf(i18n.T("tui.policies.section_disabled"), len(disabledIndices)),
				IsHeader: true,
			})
			for _, pi := range disabledIndices {
				p := policies[pi]
				idx := len(items)
				items = append(items, v.buildPolicyItem(p))
				v.policyIndices[idx] = pi
			}
		}
	}

	// ── Actions ──
	items = append(items, widgets.SectionItem{
		MainText: i18n.T("tui.policies.section_actions"),
		IsHeader: true,
	})

	templateIdx := len(items)
	items = append(items, widgets.SectionItem{
		MainText: fmt.Sprintf("  %s%s %s%s",
			theme.ColorTag(theme.ActionHex), theme.IconArrow,
			i18n.T("tui.policies.action_template"), theme.TagReset),
	})
	v.actionIndices[templateIdx] = func() { v.addFromTemplate() }

	customIdx := len(items)
	items = append(items, widgets.SectionItem{
		MainText: fmt.Sprintf("  %s%s %s%s",
			theme.ColorTag(theme.ActionHex), theme.IconArrow,
			i18n.T("tui.policies.action_custom"), theme.TagReset),
	})
	v.actionIndices[customIdx] = func() { v.addPolicy() }

	checkIdx := len(items)
	items = append(items, widgets.SectionItem{
		MainText: fmt.Sprintf("  %s%s %s%s",
			theme.ColorTag(theme.ActionHex), theme.IconArrow,
			i18n.T("tui.policies.action_check"), theme.TagReset),
	})
	v.actionIndices[checkIdx] = func() { v.checkPolicies() }

	v.slist.SetItems(items)
}

func (v *PoliciesView) buildPolicyItem(p teamstate.Policy) widgets.SectionItem {
	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagReset

	var enfIcon string
	switch p.Enforcement {
	case teamstate.EnforcementDisabled:
		enfIcon = fmt.Sprintf("%s○%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor)
	case teamstate.EnforcementRefuse:
		enfIcon = fmt.Sprintf("%s✗%s", theme.ColorTag(theme.ErrorHex), theme.TagColor)
	default: // warn
		enfIcon = fmt.Sprintf("%s⚠%s", theme.ColorTag(theme.WarningHex), theme.TagColor)
	}

	typeLabel := string(p.Type)
	meta := fmt.Sprintf("%s · %s", typeLabel, p.Enforcement)

	mainText := fmt.Sprintf("  %s %-30s %s%s%s", enfIcon, p.Name, muted, meta, reset)

	secondaryText := ""
	if p.Message != "" {
		secondaryText = fmt.Sprintf("    %s%s%s", theme.ColorTag(theme.TextSecondaryHex), p.Message, reset)
	}

	return widgets.SectionItem{
		MainText:      mainText,
		SecondaryText: secondaryText,
	}
}

// ── Detail modal with actions ────────────────────────────────────────────────

func (v *PoliciesView) showDetail(policyIdx int) {
	if policyIdx < 0 || policyIdx >= len(v.policies) {
		return
	}
	p := v.policies[policyIdx]

	detail := v.formatPolicyDetail(p)

	// Build action buttons: enforcement toggles | separator | edit/delete/close
	actions := []ModalAction{}

	// Enforcement toggle buttons — only show states different from current
	if p.Enforcement != teamstate.EnforcementDisabled {
		actions = append(actions, ModalAction{
			Label:    i18n.T("tui.policies.status_disabled"),
			Callback: func() { v.setEnforcement(policyIdx, teamstate.EnforcementDisabled) },
		})
	}
	if p.Enforcement != teamstate.EnforcementWarn {
		actions = append(actions, ModalAction{
			Label:    i18n.T("tui.policies.status_warn"),
			Callback: func() { v.setEnforcement(policyIdx, teamstate.EnforcementWarn) },
		})
	}
	if p.Enforcement != teamstate.EnforcementRefuse {
		actions = append(actions, ModalAction{
			Label:    i18n.T("tui.policies.status_refuse"),
			Callback: func() { v.setEnforcement(policyIdx, teamstate.EnforcementRefuse) },
		})
	}

	// Edit + Delete + Close — separated visually from enforcement toggles
	actions = append(actions,
		ModalAction{Label: i18n.T("tui.policies.edit"), Callback: func() { v.editPolicy(policyIdx) }, Separator: true},
		ModalAction{Label: i18n.T("tui.policies.delete"), Callback: func() { v.removePolicy(policyIdx) }},
		ModalAction{Label: i18n.T("tui.policies.close"), Callback: func() {}, Separator: true},
	)

	if v.shell != nil {
		v.shell.ShowScrollableModal("Policy: "+p.Name, detail, actions)
	}
}

func (v *PoliciesView) formatPolicyDetail(p teamstate.Policy) string {
	var detail string
	detail += fmt.Sprintf("%-14s %s\n", i18n.T("tui.policies.field_name")+":", p.Name)
	detail += fmt.Sprintf("%-14s %s\n", i18n.T("tui.policies.field_type")+":", p.Type)
	if p.Target != "" {
		detail += fmt.Sprintf("%-14s %s\n", i18n.T("tui.policies.field_target")+":", p.Target)
	}
	// Status with icon
	var statusLabel string
	switch p.Enforcement {
	case teamstate.EnforcementDisabled:
		statusLabel = "○ " + i18n.T("tui.policies.status_disabled")
	case teamstate.EnforcementWarn:
		statusLabel = "⚠ " + i18n.T("tui.policies.status_warn")
	case teamstate.EnforcementRefuse:
		statusLabel = "✗ " + i18n.T("tui.policies.status_refuse")
	}
	detail += fmt.Sprintf("%-14s %s\n", i18n.T("tui.policies.field_enforcement")+":", statusLabel)
	detail += fmt.Sprintf("%-14s %s\n", i18n.T("tui.policies.field_message")+":", p.Message)
	if p.Rule != "" {
		detail += fmt.Sprintf("%-14s %s\n", i18n.T("tui.policies.field_rule")+":", p.Rule)
	}
	if p.Max > 0 {
		detail += fmt.Sprintf("Max:          %d %s\n", p.Max, p.Unit)
	}
	if len(p.Patterns) > 0 {
		detail += fmt.Sprintf("Patterns:     %s\n", strings.Join(p.Patterns, ", "))
	}
	if p.Scope != "" {
		detail += fmt.Sprintf("Scope:        %s\n", p.Scope)
	}
	return detail
}

// ── Enforcement toggle ───────────────────────────────────────────────────────

func (v *PoliciesView) setEnforcement(policyIdx int, enforcement teamstate.PolicyEnforcement) {
	if policyIdx < 0 || policyIdx >= len(v.policies) {
		return
	}
	p := v.policies[policyIdx]

	repo := v.getRepo()
	if repo == nil {
		return
	}

	pMap, err := v.loadPoliciesMap(repo)
	if err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.settings.error")+": "+err.Error(), false)
		}
		return
	}

	updated := pMap[p.Name]
	updated.Enforcement = enforcement
	pMap[p.Name] = updated

	if err := repo.SavePolicies(context.Background(), pMap); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.settings.error")+": "+err.Error(), false)
		}
		return
	}

	var statusLabel string
	switch enforcement {
	case teamstate.EnforcementDisabled:
		statusLabel = i18n.T("tui.policies.status_disabled")
	case teamstate.EnforcementWarn:
		statusLabel = i18n.T("tui.policies.status_warn")
	case teamstate.EnforcementRefuse:
		statusLabel = i18n.T("tui.policies.status_refuse")
	}
	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.Tf("tui.policies.status_changed", p.Name, statusLabel), true)
	}
	v.refresh()
}

// ── Edit policy ──────────────────────────────────────────────────────────────

func (v *PoliciesView) editSelectedPolicy() {
	if v.slist == nil {
		return
	}
	idx, _, ok := v.slist.CurrentItem()
	if !ok {
		return
	}
	pi, isPolicy := v.policyIndices[idx]
	if !isPolicy {
		return
	}
	v.editPolicy(pi)
}

func (v *PoliciesView) editPolicy(policyIdx int) {
	if v.shell == nil || policyIdx < 0 || policyIdx >= len(v.policies) {
		return
	}
	repo := v.getRepo()
	if repo == nil {
		return
	}

	p := v.policies[policyIdx]

	// Build a template from the existing policy to pre-fill the form
	tpl := &policyTemplate{
		Name:        p.Name,
		Type:        string(p.Type),
		Target:      p.Target,
		Rule:        p.Rule,
		Patterns:    strings.Join(p.Patterns, ", "),
		Scope:       p.Scope,
		Max:         "",
		Enforcement: string(p.Enforcement),
		Message:     p.Message,
		Enabled:     p.Enabled,
	}
	if p.Max > 0 {
		tpl.Max = fmt.Sprintf("%d", p.Max)
	}

	v.showPolicyForm(repo, tpl, p.Name)
}

// ── Remove policy ────────────────────────────────────────────────────────────

func (v *PoliciesView) removeSelectedPolicy() {
	if v.slist == nil {
		return
	}
	idx, _, ok := v.slist.CurrentItem()
	if !ok {
		return
	}
	pi, isPolicy := v.policyIndices[idx]
	if !isPolicy {
		return
	}
	v.removePolicy(pi)
}

func (v *PoliciesView) removePolicy(policyIdx int) {
	if v.shell == nil || policyIdx < 0 || policyIdx >= len(v.policies) {
		return
	}
	p := v.policies[policyIdx]
	repo := v.getRepo()
	if repo == nil {
		return
	}

	v.shell.ShowSelectModal(i18n.Tf("tui.policies.confirm_delete", p.Name), []SelectOption{
		{Label: i18n.T("tui.settings.cancel"), Value: ""},
		{Label: i18n.T("tui.policies.yes_delete"), Value: "yes"},
	}, "", func(choice string) {
		if choice != "yes" {
			return
		}
		pMap, err := v.loadPoliciesMap(repo)
		if err != nil {
			v.shell.ShowToastMsg(i18n.T("tui.settings.error")+": "+err.Error(), false)
			return
		}
		delete(pMap, p.Name)
		if err := repo.SavePolicies(context.Background(), pMap); err != nil {
			v.shell.ShowToastMsg(i18n.T("tui.settings.error")+": "+err.Error(), false)
			return
		}
		v.shell.ShowToastMsg(i18n.Tf("tui.policies.deleted", p.Name), true)
		v.refresh()
	})
}

// ── Helper ───────────────────────────────────────────────────────────────────

func (v *PoliciesView) loadPoliciesMap(repo teamstate.TeamStateWriter) (map[string]teamstate.Policy, error) {
	policies, err := repo.LoadPolicies("")
	if err != nil {
		return nil, err
	}
	m := make(map[string]teamstate.Policy, len(policies))
	for _, p := range policies {
		m[p.Name] = p
	}
	return m, nil
}

// ── Check policies ───────────────────────────────────────────────────────────

func (v *PoliciesView) checkPolicies() {
	if v.shell == nil {
		return
	}

	repo := v.getRepo()
	if repo == nil {
		v.shell.ShowToastMsg(i18n.T("tui.policies.team_not_configured"), false)
		return
	}

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

	var text string
	passed := 0
	for _, r := range results {
		var icon string
		switch {
		case r.Passed:
			icon = "[green]✓[-]"
			passed++
		case r.Enforcement == teamstate.EnforcementRefuse:
			icon = "[red]✗[-]"
		default:
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

// ── Add from template ────────────────────────────────────────────────────────

func (v *PoliciesView) addFromTemplate() {
	if v.shell == nil {
		return
	}
	repo := v.getRepo()
	if repo == nil {
		v.shell.ShowToastMsg(i18n.T("tui.policies.team_not_configured"), false)
		return
	}

	templates := builtinPolicyTemplates()
	opts := make([]SelectOption, len(templates))
	for i, t := range templates {
		opts[i] = SelectOption{
			Label: i18n.T(t.LabelKey),
			Value: t.Name,
		}
	}

	v.shell.ShowSelectModal(i18n.T("tui.policies.template_title"), opts, "", func(choice string) {
		if choice == "" {
			return
		}
		var tpl policyTemplate
		for _, t := range templates {
			if t.Name == choice {
				tpl = t
				break
			}
		}
		v.showPolicyForm(repo, &tpl, "")
	})
}

// addPolicy opens the custom policy creation form (no template pre-fill).
func (v *PoliciesView) addPolicy() {
	if v.shell == nil {
		return
	}
	repo := v.getRepo()
	if repo == nil {
		v.shell.ShowToastMsg(i18n.T("tui.policies.team_not_configured"), false)
		return
	}
	v.showPolicyForm(repo, nil, "")
}

// ── Policy form (create + edit) ──────────────────────────────────────────────

// showPolicyForm displays the policy creation/edit form.
// If editName is non-empty, this is an edit: load-modify-save via SavePolicies.
// If editName is empty, this is a creation: append to policies.toml.
func (v *PoliciesView) showPolicyForm(repo teamstate.TeamStateWriter, tpl *policyTemplate, editName string) {
	defaultName := ""
	defaultType := "regex"
	defaultTarget := ""
	defaultEnforcement := "warn"
	defaultMessage := ""
	defaultRule := ""
	defaultPatterns := ""
	defaultScope := "diff_only"
	defaultMax := "3"

	if tpl != nil {
		defaultName = tpl.Name
		defaultType = tpl.Type
		defaultTarget = tpl.Target
		defaultEnforcement = tpl.Enforcement
		defaultMessage = tpl.Message
		defaultRule = tpl.Rule
		defaultPatterns = tpl.Patterns
		if tpl.Scope != "" {
			defaultScope = tpl.Scope
		}
		if tpl.Max != "" {
			defaultMax = tpl.Max
		}
	}

	title := i18n.T("tui.policies.create_title")
	if editName != "" {
		title = i18n.T("tui.policies.edit_title")
	}

	v.shell.ShowInlineForm(InlineFormConfig{
		Title: title,
		Fields: []FormField{
			{Key: "name", Label: i18n.T("tui.policies.field_name_slug"), Type: FieldText, Required: true,
				Default: defaultName, Hint: i18n.T("tui.policies.hint_name")},
			{Key: "type", Label: i18n.T("tui.policies.field_type"), Type: FieldSelect,
				Options: policyTypeOptions, Default: defaultType, Required: true,
				Hint: i18n.T("tui.policies.hint_type")},
			{Key: "target", Label: i18n.T("tui.policies.field_target"), Type: FieldText,
				Default: defaultTarget, Hint: i18n.T("tui.policies.hint_target"),
				Conditional: func(v map[string]string) bool { return v["type"] != "forbidden_pattern" }},
			{Key: "enforcement", Label: i18n.T("tui.policies.field_enforcement"), Type: FieldSelect,
				Options: policyEnforcementOptions, Default: defaultEnforcement,
				Hint: i18n.T("tui.policies.hint_enforcement")},
			{Key: "message", Label: i18n.T("tui.policies.field_message"), Type: FieldText,
				Default: defaultMessage, Hint: i18n.T("tui.policies.hint_message")},
			{Key: "rule", Label: i18n.T("tui.policies.field_rule"), Type: FieldText,
				Default: defaultRule, Hint: i18n.T("tui.policies.hint_rule"),
				Conditional: func(v map[string]string) bool { return v["type"] == "regex" }},
			{Key: "patterns", Label: i18n.T("tui.policies.field_patterns"), Type: FieldText,
				Default: defaultPatterns, Hint: i18n.T("tui.policies.hint_patterns"),
				Conditional: func(v map[string]string) bool { return v["type"] == "forbidden_pattern" }},
			{Key: "scope", Label: i18n.T("tui.policies.field_scope"), Type: FieldSelect,
				Options: policyScopeOptions, Default: defaultScope,
				Hint:        i18n.T("tui.policies.hint_scope"),
				Conditional: func(v map[string]string) bool { return v["type"] == "forbidden_pattern" }},
			{Key: "max", Label: i18n.T("tui.policies.field_max"), Type: FieldText,
				Default: defaultMax, Hint: i18n.T("tui.policies.hint_max"),
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
			target := values["target"]

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

			if editName != "" {
				// Edit mode: load → modify → save
				v.savePolicyEdit(repo, editName, name, pType, target, enforcement, message,
					values["rule"], patterns, values["scope"], maxVal,
					tpl != nil && tpl.Enabled)
			} else {
				// Create mode: append to file
				v.writePolicyToml(repo, name, pType, target, enforcement, message,
					values["rule"], patterns, values["scope"], maxVal,
					tpl != nil && tpl.Type == "boolean" && tpl.Enabled)
			}
		},
		OnCancel: nil,
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
	{Label: "Disabled", Value: "disabled"},
	{Label: "Warn", Value: "warn"},
	{Label: "Refuse", Value: "refuse"},
}

var policyScopeOptions = []SelectOption{
	{Label: "diff_only", Value: "diff_only"},
	{Label: "modified_files", Value: "modified_files"},
	{Label: "all_files", Value: "all_files"},
}

// savePolicyEdit replaces an existing policy via load-modify-save.
func (v *PoliciesView) savePolicyEdit(repo teamstate.TeamStateWriter, oldName, newName, pType, target, enforcement, message, rule string, patterns []string, scope string, maxVal int, enabledBool bool) {
	pMap, err := v.loadPoliciesMap(repo)
	if err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.settings.error")+": "+err.Error(), false)
		}
		return
	}

	// Remove old entry (handles rename)
	delete(pMap, oldName)

	// Build new policy
	p := teamstate.Policy{
		Type:        teamstate.PolicyType(pType),
		Target:      target,
		Rule:        rule,
		Patterns:    patterns,
		Scope:       scope,
		Enforcement: teamstate.PolicyEnforcement(enforcement),
		Message:     message,
	}
	if maxVal > 0 {
		p.Max = maxVal
	}
	if pType == "boolean" || enabledBool {
		p.Enabled = true
	}

	pMap[newName] = p

	if err := repo.SavePolicies(context.Background(), pMap); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.settings.error")+": "+err.Error(), false)
		}
		return
	}

	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.Tf("tui.policies.updated", newName), true)
	}
	v.refresh()
}

func (v *PoliciesView) writePolicyToml(repo teamstate.TeamStateWriter, name, pType, target, enforcement, message, rule string, patterns []string, scope string, maxVal int, enabledBool bool) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "\n[policies.%s]\n", name)
	fmt.Fprintf(&sb, "type = %q\n", pType)
	if target != "" {
		fmt.Fprintf(&sb, "target = %q\n", target)
	}
	if rule != "" {
		fmt.Fprintf(&sb, "rule = %q\n", rule)
	}
	if len(patterns) > 0 {
		fmt.Fprintf(&sb, "patterns = [%s]\n", quoteSlice(patterns))
	}
	if scope != "" {
		fmt.Fprintf(&sb, "scope = %q\n", scope)
	}
	if maxVal > 0 {
		fmt.Fprintf(&sb, "max = %d\n", maxVal)
	}
	if pType == "boolean" || enabledBool {
		sb.WriteString("enabled = true\n")
	}
	fmt.Fprintf(&sb, "enforcement = %q\n", enforcement)
	if message != "" {
		fmt.Fprintf(&sb, "message = %q\n", message)
	}

	policiesPath := filepath.Join(repo.Path(), "policies.toml")
	f, err := os.OpenFile(policiesPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
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

// ContextCommands implements CommandProvider.
func (v *PoliciesView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{ID: "policies.add", Label: i18n.T("tui.hints.add"), Aliases: []string{"add", "template", "ajouter"}, Description: i18n.T("tui.policies.cmd_add"), Category: "Policies", Action: func() { v.addFromTemplate() }},
		{ID: "policies.add.custom", Label: i18n.T("tui.hints.add"), Aliases: []string{"custom", "personnalisé"}, Description: i18n.T("tui.policies.cmd_add_custom"), Category: "Policies", Action: func() { v.addPolicy() }},
		{ID: "policies.edit", Label: i18n.T("tui.hints.edit"), Aliases: []string{"edit", "éditer", "modifier"}, Description: i18n.T("tui.policies.cmd_edit"), Category: "Policies", Action: func() { v.editSelectedPolicy() }},
		{ID: "policies.delete", Label: i18n.T("tui.hints.delete"), Aliases: []string{"delete", "remove", "supprimer"}, Description: i18n.T("tui.policies.cmd_delete"), Category: "Policies", Action: func() { v.removeSelectedPolicy() }},
		{ID: "policies.check", Label: i18n.T("tui.hints.check"), Aliases: []string{"check", "verify", "vérifier"}, Description: i18n.T("tui.policies.cmd_check"), Category: "Policies", Action: func() { v.checkPolicies() }},
	}
}
