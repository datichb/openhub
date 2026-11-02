package views

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/services/policy"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
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
	Unit        string // kept as is when a policy is edited
	Enforcement string
	Message     string
	Enabled     bool
}

// PolicyService is what the view needs to change the policies. Its calls
// pull and push (network): the view runs them off the event loop.
type PolicyService interface {
	Add(ctx context.Context, name string, p teamstate.Policy) error
	Update(ctx context.Context, name string, p teamstate.Policy) error
	SetEnforcement(ctx context.Context, name string, e teamstate.PolicyEnforcement) error
	Remove(ctx context.Context, name string) error
}

// PolicyServiceFunc builds the service of a team-state repository.
type PolicyServiceFunc func(repo teamstate.TeamStateWriter) PolicyService

func defaultPolicyService(repo teamstate.TeamStateWriter) PolicyService {
	return policy.New(repo)
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
			Message:     i18n.T("tui.policies.tpl_branch_naming_msg"),
		},
		{
			LabelKey:    "tui.policies.tpl_conventional_commits",
			Name:        "commit-message",
			Type:        "regex",
			Target:      "commit_message",
			Rule:        `^(feat|fix|chore|docs|refactor|perf|test|ci|build|style)(\(.+\))?!?: .+`,
			Enforcement: "warn",
			Message:     i18n.T("tui.policies.tpl_conventional_commits_msg"),
		},
		{
			LabelKey:    "tui.policies.tpl_max_wip",
			Name:        "max-wip-tickets",
			Type:        "limit",
			Target:      "wip_tickets",
			Max:         "3",
			Enforcement: "warn",
			Message:     i18n.T("tui.policies.tpl_max_wip_msg"),
		},
		{
			LabelKey:    "tui.policies.tpl_no_console_log",
			Name:        "no-console-log",
			Type:        "forbidden_pattern",
			Patterns:    "console.log, console.debug, debugger",
			Scope:       "diff_only",
			Enforcement: "refuse",
			Message:     i18n.T("tui.policies.tpl_no_console_log_msg"),
		},
		{
			LabelKey:    "tui.policies.tpl_no_todo",
			Name:        "no-todo",
			Type:        "forbidden_pattern",
			Patterns:    "TODO, FIXME, HACK, XXX",
			Scope:       "diff_only",
			Enforcement: "warn",
			Message:     i18n.T("tui.policies.tpl_no_todo_msg"),
		},
		{
			LabelKey:    "tui.policies.tpl_require_review",
			Name:        "require-review",
			Type:        "boolean",
			Target:      "review",
			Enabled:     true,
			Enforcement: "refuse",
			Message:     i18n.T("tui.policies.tpl_require_review_msg"),
		},
		{
			LabelKey:    "tui.policies.tpl_require_tests",
			Name:        "require-tests",
			Type:        "boolean",
			Target:      "tests",
			Enabled:     true,
			Enforcement: "warn",
			Message:     i18n.T("tui.policies.tpl_require_tests_msg"),
		},
		{
			LabelKey:    "tui.policies.tpl_no_secrets",
			Name:        "no-secrets",
			Type:        "forbidden_pattern",
			Patterns:    "password=, secret_key=, api_key=, AWS_SECRET, PRIVATE_KEY",
			Scope:       "diff_only",
			Enforcement: "refuse",
			Message:     i18n.T("tui.policies.tpl_no_secrets_msg"),
		},
	}
}

// PoliciesView displays and manages team policies.
type PoliciesView struct {
	app         *tview.Application
	appCtx      *app.App
	resolveTeam ResolveTeamFunc
	slist       *widgets.SectionedList
	header      *tview.TextView
	page        *widgets.ListDetailPage
	shell       ShellAccess
	policies    []teamstate.Policy
	newService  PolicyServiceFunc

	// actionIndices maps SectionedList item indices to action callbacks.
	actionIndices map[int]func()
	// policyIndices maps SectionedList item indices to policy slice indices.
	policyIndices map[int]int

	loads      loadSlot // the reads of the policies: only the latest is applied
	checkReads loadSlot // the check of the policies: only the latest is applied
}

// The keys of the view besides the convention.
var (
	policyCheckKey    = keys.Own("policies.check", 'c', "tui.hints.check")
	policyTemplateKey = keys.Own("policies.template", 't', "tui.hints.template")
)

// policiesData is what a read of the policies gives back.
type policiesData struct {
	cloned   bool
	policies []teamstate.Policy
}

var _ View = (*PoliciesView)(nil)
var _ CommandProvider = (*PoliciesView)(nil)

// NewPoliciesView creates a new policies view.
func NewPoliciesView(resolveTeam ResolveTeamFunc, appCtx *app.App) *PoliciesView {
	return &PoliciesView{resolveTeam: resolveTeam, appCtx: appCtx, newService: defaultPolicyService}
}

// SetPolicyService replaces the factory of the service that edits the
// policies (tests, alternative backends).
func (v *PoliciesView) SetPolicyService(f PolicyServiceFunc) {
	if f != nil {
		v.newService = f
	}
}

// SetShell provides the shell reference for modal interactions.
func (v *PoliciesView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *PoliciesView) ID() string { return "team.policies" }

// Title returns the display title.
func (v *PoliciesView) Title() string { return i18n.T("tui.team.policies") }

// StatusHints returns keybinding hints.
func (v *PoliciesView) StatusHints() string {
	return keys.Hints(keys.Navigate.Hint(), "{/} "+i18n.T("tui.hints.sections"),
		keys.Open.HintAs(i18n.T("tui.hints.detail")), keys.Create.Hint(),
		policyTemplateKey.Hint(), keys.Edit.Hint(), keys.Delete.Hint(),
		policyCheckKey.Hint(), keys.Refresh.Hint())
}

// Mount builds the policies view with a descriptive header and SectionedList.
func (v *PoliciesView) Mount(content *tview.Flex, tvApp *tview.Application) {
	v.app = tvApp

	v.header = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false).
		SetWordWrap(true)
	v.header.SetBackgroundColor(theme.BgPanel)

	v.slist = widgets.NewSectionedList().SetApp(tvApp)
	v.slist.SetItemChangedFunc(func(idx int, _ widgets.SectionItem) { v.showItemDetail(idx) })

	v.page = widgets.NewListDetailPage(v.Title(), i18n.T("tui.catalog.detail"))
	v.page.AddHeader(v.header, 5)
	v.page.SetMain(v.slist)

	v.refresh()
	v.page.Mount(content)
}

// Unmount cleans up resources.
func (v *PoliciesView) Unmount() {
	v.loads.Invalidate()
	v.checkReads.Invalidate()
	v.app = nil
	v.slist = nil
	v.header = nil
	v.page = nil
}

// HandleKey processes policies view key events.
func (v *PoliciesView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch {
	case keys.Refresh.Matches(event):
		v.refresh()
		return nil
	case policyCheckKey.Matches(event):
		v.checkPolicies()
		return nil
	case keys.Create.Matches(event):
		v.addPolicy()
		return nil
	case policyTemplateKey.Matches(event):
		v.addFromTemplate()
		return nil
	case keys.Edit.Matches(event):
		v.editSelectedPolicy()
		return nil
	case keys.Delete.Matches(event):
		v.removeSelectedPolicy()
		return nil
	}
	if keys.Open.Matches(event) {
		if v.slist == nil {
			return nil
		}
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

// newRepo is the team-state of the team, without touching the disk (nil
// without a team).
func (v *PoliciesView) newRepo() teamstate.TeamStateWriter {
	tc := v.resolveTeam()
	if !tc.Enabled {
		return nil
	}
	return teamstate.NewRepo(tc.StateRepo, tc.StatePath)
}

// refresh pulls the team-state, then reads the policies off the event loop.
func (v *PoliciesView) refresh() {
	v.policies = nil
	v.actionIndices = nil
	v.policyIndices = nil
	if v.slist != nil {
		v.slist.SetItems(nil)
	}
	setPageDetail(v.page, emptyDetail())

	repo := v.newRepo()
	if repo == nil {
		v.loads.Invalidate()
		v.setHeader(stateEmpty, i18n.T("tui.policies.team_not_configured"))
		return
	}
	v.setHeader(stateLoading, "")

	fetch := func(context.Context) (policiesData, error) {
		if !repo.IsCloned() {
			return policiesData{}, nil
		}
		policies, err := repo.LoadPolicies("")
		return policiesData{cloned: true, policies: policies}, err
	}
	syncLoad(v.app, shellContext(v.shell), &v.loads, repo, v.shell, fetch, func(d policiesData, err error) {
		v.renderPolicies(d, err)
	})
}

// setHeader writes a state in the header.
func (v *PoliciesView) setHeader(kind stateKind, msg string) {
	if v.header != nil {
		v.header.SetText(stateText(kind, msg))
	}
}

func (v *PoliciesView) setHeaderText(text string) {
	if v.header == nil {
		return
	}
	v.header.SetText(theme.Paint(theme.TextSecondaryHex, tview.Escape(text)))
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

func (v *PoliciesView) renderPolicies(d policiesData, err error) {
	if v.slist == nil {
		return
	}
	switch {
	case err != nil:
		v.setHeader(stateError, err.Error())
		return
	case !d.cloned:
		v.setHeader(stateEmpty, i18n.T("tui.policies.team_not_configured"))
		return
	}
	policies := d.policies
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
		MainText: "  " + theme.Paint(theme.ActiveMode.SecondaryHex, theme.SymCursor+" "+i18n.T("tui.policies.action_template")),
	})
	v.actionIndices[templateIdx] = func() { v.addFromTemplate() }

	customIdx := len(items)
	items = append(items, widgets.SectionItem{
		MainText: "  " + theme.Paint(theme.ActiveMode.SecondaryHex, theme.SymCursor+" "+i18n.T("tui.policies.action_custom")),
	})
	v.actionIndices[customIdx] = func() { v.addPolicy() }

	checkIdx := len(items)
	items = append(items, widgets.SectionItem{
		MainText: "  " + theme.Paint(theme.ActiveMode.SecondaryHex, theme.SymCursor+" "+i18n.T("tui.policies.action_check")),
	})
	v.actionIndices[checkIdx] = func() { v.checkPolicies() }

	v.slist.SetItems(items)
	idx, _, _ := v.slist.CurrentItem()
	v.showItemDetail(idx)
}

// showItemDetail fills the detail pane with the list item at idx (policy or
// action).
func (v *PoliciesView) showItemDetail(idx int) {
	if v.page == nil {
		return
	}
	if pi, ok := v.policyIndices[idx]; ok && pi >= 0 && pi < len(v.policies) {
		p := v.policies[pi]
		var d detailText
		d.title(tview.Escape(p.Name))
		d.b.WriteString(tview.Escape(v.formatPolicyDetail(p)))
		pairs := append([]string{keys.Open.Keys(), i18n.T("tui.hints.actions")}, keys.Pairs(keys.Edit, keys.Delete)...)
		d.keys(append(pairs, policyCheckKey.Keys(), policyCheckKey.Label(), keys.Refresh.Keys(), keys.Refresh.Label())...)
		v.page.SetDetail(d.String())
		return
	}
	if _, ok := v.actionIndices[idx]; ok && v.slist != nil {
		items := v.slist.GetItems()
		var d detailText
		if idx >= 0 && idx < len(items) {
			d.title(strings.TrimSpace(items[idx].MainText))
		}
		d.para(i18n.T("tui.detail.policy.action"))
		d.keys(append(keys.Pairs(keys.Open, keys.Create), policyTemplateKey.Keys(), policyTemplateKey.Label())...)
		v.page.SetDetail(d.String())
		return
	}
	v.page.SetDetail(emptyDetail())
}

func (v *PoliciesView) buildPolicyItem(p teamstate.Policy) widgets.SectionItem {
	enfIcon := policyIcon(p.Enforcement)
	meta := fmt.Sprintf("%s · %s", p.Type, p.Enforcement)

	mainText := fmt.Sprintf("  %s %s %s", enfIcon, tview.Escape(fmt.Sprintf("%-30s", p.Name)), theme.Paint(theme.TextMutedHex, tview.Escape(meta)))

	secondaryText := ""
	if p.Message != "" {
		secondaryText = "    " + theme.Paint(theme.TextSecondaryHex, tview.Escape(p.Message))
	}

	return widgets.SectionItem{
		MainText:      mainText,
		SecondaryText: secondaryText,
	}
}

// policyIcon is the symbol of an enforcement level: disabled is the grey
// « off » circle, refuse the red cross, warn the yellow warning.
func policyIcon(e teamstate.PolicyEnforcement) string {
	switch e {
	case teamstate.EnforcementDisabled:
		return theme.Paint(theme.TextMutedHex, theme.SymInactive)
	case teamstate.EnforcementRefuse:
		return theme.Paint(theme.ErrorHex, theme.SymFailure)
	default: // warn
		return theme.Paint(theme.WarningHex, theme.SymWarning)
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
		v.shell.ShowScrollableModal(i18n.Tf("tui.policies.modal_title", p.Name), detail, actions)
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
		statusLabel = theme.SymInactive + " " + i18n.T("tui.policies.status_disabled")
	case teamstate.EnforcementWarn:
		statusLabel = theme.SymWarning + " " + i18n.T("tui.policies.status_warn")
	case teamstate.EnforcementRefuse:
		statusLabel = theme.SymFailure + " " + i18n.T("tui.policies.status_refuse")
	}
	detail += fmt.Sprintf("%-14s %s\n", i18n.T("tui.policies.field_enforcement")+":", statusLabel)
	detail += fmt.Sprintf("%-14s %s\n", i18n.T("tui.policies.field_message")+":", p.Message)
	if p.Rule != "" {
		detail += fmt.Sprintf("%-14s %s\n", i18n.T("tui.policies.field_rule")+":", p.Rule)
	}
	if p.Max > 0 {
		detail += fmt.Sprintf("%-14s %d %s\n", i18n.T("tui.policies.field_max")+":", p.Max, p.Unit)
	}
	if len(p.Patterns) > 0 {
		detail += fmt.Sprintf("%-14s %s\n", i18n.T("tui.policies.field_patterns_label")+":", strings.Join(p.Patterns, ", "))
	}
	if p.Scope != "" {
		detail += fmt.Sprintf("%-14s %s\n", i18n.T("tui.policies.field_scope")+":", p.Scope)
	}
	return detail
}

// ── Enforcement toggle ───────────────────────────────────────────────────────

func (v *PoliciesView) setEnforcement(policyIdx int, enforcement teamstate.PolicyEnforcement) {
	if policyIdx < 0 || policyIdx >= len(v.policies) {
		return
	}
	p := v.policies[policyIdx]

	repo := v.newRepo()
	if repo == nil {
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
	v.mutate(repo, func(ctx context.Context, svc PolicyService) error {
		return svc.SetEnforcement(ctx, p.Name, enforcement)
	}, i18n.Tf("tui.policies.status_changed", p.Name, statusLabel))
}

// mutate runs op on the policy service off the event loop (it checks that the
// clone is there, pulls and pushes the team-state), then, on the loop, shows the error or the success
// toast and reloads the list. Without a running application (tests) it runs
// inline.
func (v *PoliciesView) mutate(repo teamstate.TeamStateWriter, op func(ctx context.Context, svc PolicyService) error, success string) {
	svc := v.newService(repo)
	ctx := context.Background()
	if v.shell != nil {
		ctx = v.shell.Context()
	}
	tvApp := v.app
	run := func() error {
		if !repo.IsCloned() { // a stat of the disk: here, off the event loop
			return errors.New(i18n.T("tui.policies.team_not_configured"))
		}
		return op(ctx, svc)
	}
	done := func(err error) {
		if v.shell != nil {
			if err != nil {
				v.shell.ShowToastMsg(errorText(err), false)
			} else {
				v.shell.ShowToastMsg(success, true)
			}
		}
		if err == nil && v.app == tvApp {
			v.refresh()
		}
	}
	if tvApp == nil {
		done(run())
		return
	}
	safego.Go(func() {
		err := run()
		tvApp.QueueUpdateDraw(func() { done(err) })
	})
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
	repo := v.newRepo()
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
		Unit:        p.Unit,
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
	repo := v.newRepo()
	if repo == nil {
		return
	}

	v.shell.Confirm(i18n.Tf("tui.policies.confirm_delete", p.Name), i18n.T("tui.policies.delete_detail"), true, func() {
		v.mutate(repo, func(ctx context.Context, svc PolicyService) error {
			return svc.Remove(ctx, p.Name)
		}, i18n.Tf("tui.policies.deleted", p.Name))
	})
}

// ── Check policies ───────────────────────────────────────────────────────────

func (v *PoliciesView) checkPolicies() {
	if v.shell == nil {
		return
	}
	repo := v.newRepo()
	if repo == nil {
		v.shell.ShowToastMsg(i18n.T("tui.policies.team_not_configured"), false)
		return
	}
	v.shell.ShowToastMsg(i18n.T("tui.policies.checking"), true)

	// What the check needs from the view, read on the loop; the rest (git, the
	// files, the policies) is read off it.
	member, appCtx, baseBranch := v.resolveTeam().MemberID, v.appCtx, ""
	if appCtx != nil && appCtx.Config != nil {
		baseBranch = appCtx.Config.Worktree.BaseBranch
	}
	sh := v.shell
	load(v.app, shellContext(sh), &v.checkReads, func(context.Context) ([]teamstate.PolicyResult, error) {
		if !repo.IsCloned() {
			return nil, errors.New(i18n.T("tui.policies.team_not_configured"))
		}
		return repo.CheckAll("", policyCheckContext(repo, appCtx, member, baseBranch))
	}, func(results []teamstate.PolicyResult, err error) {
		if err != nil {
			sh.ShowToastMsg(i18n.T("tui.policies.check_error")+": "+err.Error(), false)
			return
		}
		if len(results) == 0 {
			sh.ShowToastMsg(i18n.T("tui.policies.nothing_to_check"), true)
			return
		}

		var b strings.Builder
		passed := 0
		for _, r := range results {
			var icon string
			switch {
			case r.Passed:
				icon = tagged(theme.SuccessHex, theme.SymSuccess)
				passed++
			case r.Enforcement == teamstate.EnforcementRefuse:
				icon = tagged(theme.ErrorHex, theme.SymFailure)
			default:
				icon = tagged(theme.WarningHex, theme.SymWarning)
			}
			fmt.Fprintf(&b, "  %s %s", icon, tview.Escape(r.Name))
			if !r.Passed {
				fmt.Fprintf(&b, " — %s", tview.Escape(r.Message))
				if r.Details != "" {
					fmt.Fprintf(&b, "\n      %s", tview.Escape(r.Details))
				}
			}
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "\n  %s: %d/%d", i18n.T("tui.policies.result"), passed, len(results))

		sh.ShowScrollableModal(i18n.T("tui.policies.check_title"), b.String(), []ModalAction{
			{Label: i18n.T("tui.policies.close"), Callback: func() {}},
		})
	})
}

// policyCheckContext is what the policies are checked against: the member,
// and, from the first project of the hub, its branch, its diff and its files.
// It runs git and reads files: call it off the event loop.
func policyCheckContext(repo teamstate.TeamStateReader, appCtx *app.App, member, configuredBase string) teamstate.PolicyContext {
	ctx := teamstate.PolicyContext{MemberID: member}
	if appCtx == nil || appCtx.Projects == nil {
		return ctx
	}
	p, err := resolveFirstProject(appCtx)
	if err != nil || p.Path == "" {
		return ctx
	}
	baseBranch := gitutil.DetectBaseBranch(p.Path, configuredBase)

	if b := gitCurrentBranch(p.Path); b != "" {
		ctx.BranchName = b
	}

	if diffLines, err := gitutil.DiffAddedLines(p.Path, baseBranch); err == nil {
		ctx.DiffLines = diffLines
	} else {
		slog.Debug("tui policies: failed to get diff lines", "error", err)
	}
	if modFiles, err := gitutil.ModifiedFiles(p.Path, baseBranch); err == nil {
		ctx.ModifiedFiles = modFiles
		ctx.HasTests = gitutil.HasTestFiles(modFiles)
	} else {
		slog.Debug("tui policies: failed to get modified files", "error", err)
	}

	// Lazy population of file content for all_files and modified_files scopes.
	policies, _ := repo.LoadPolicies("")
	needsAllFiles, needsModifiedContent := false, false
	for _, pol := range policies {
		if pol.Type == teamstate.PolicyTypeForbiddenPattern && pol.Enforcement != teamstate.EnforcementDisabled {
			switch pol.Scope {
			case "all_files":
				needsAllFiles = true
			case "modified_files":
				needsModifiedContent = true
			}
		}
	}
	if needsAllFiles {
		if tracked, err := gitutil.TrackedFiles(p.Path); err == nil {
			if lines, err := gitutil.ReadFileLines(p.Path, tracked); err == nil {
				ctx.AllFileLines = lines
			} else {
				slog.Debug("tui policies: failed to read tracked files", "error", err)
			}
		}
	}
	if needsModifiedContent && len(ctx.ModifiedFiles) > 0 {
		if lines, err := gitutil.ReadFileLines(p.Path, ctx.ModifiedFiles); err == nil {
			ctx.ModifiedFileLines = lines
		} else {
			slog.Debug("tui policies: failed to read modified file contents", "error", err)
		}
	}
	return ctx
}

// ── Add from template ────────────────────────────────────────────────────────

func (v *PoliciesView) addFromTemplate() {
	if v.shell == nil {
		return
	}
	repo := v.newRepo()
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
	repo := v.newRepo()
	if repo == nil {
		v.shell.ShowToastMsg(i18n.T("tui.policies.team_not_configured"), false)
		return
	}
	v.showPolicyForm(repo, nil, "")
}

// ── Policy form (create + edit) ──────────────────────────────────────────────

// showPolicyForm displays the policy creation/edit form.
// If editName is non-empty, this is an edit (Update); otherwise a creation
// (Add). Both go through the policy service, off the event loop.
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
				Options: policyTypeOptions(), Default: defaultType, Required: true,
				Hint: i18n.T("tui.policies.hint_type")},
			{Key: "target", Label: i18n.T("tui.policies.field_target"), Type: FieldText,
				Default: defaultTarget, Hint: i18n.T("tui.policies.hint_target"),
				Conditional: func(v map[string]string) bool { return v["type"] != "forbidden_pattern" }},
			{Key: "enforcement", Label: i18n.T("tui.policies.field_enforcement"), Type: FieldSelect,
				Options: policyEnforcementOptions(), Default: defaultEnforcement,
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
			scope := ""
			unit := ""
			if tpl != nil {
				scope, unit = tpl.Scope, tpl.Unit
			}

			switch pType {
			case "forbidden_pattern":
				patterns = splitTags(values["patterns"])
				scope = values["scope"]
			case "limit":
				if _, err := fmt.Sscanf(values["max"], "%d", &maxVal); err != nil {
					maxVal = 3
				}
			}

			p := teamstate.Policy{
				Type:        teamstate.PolicyType(pType),
				Target:      target,
				Patterns:    patterns,
				Scope:       scope,
				Unit:        unit,
				Max:         maxVal,
				Enabled:     pType == "boolean",
				Enforcement: teamstate.PolicyEnforcement(enforcement),
				Message:     message,
			}
			if pType == "regex" {
				p.Rule = values["rule"]
			}

			if editName != "" {
				p.Name = name
				v.mutate(repo, func(ctx context.Context, svc PolicyService) error {
					return svc.Update(ctx, editName, p)
				}, i18n.Tf("tui.policies.updated", name))
			} else {
				v.mutate(repo, func(ctx context.Context, svc PolicyService) error {
					return svc.Add(ctx, name, p)
				}, i18n.Tf("tui.policies.added", name))
			}
		},
		OnCancel: nil,
	})
}

// policyTypeOptions are the policy types of the form.
func policyTypeOptions() []SelectOption {
	return []SelectOption{
		{Label: i18n.T("tui.policies.type_regex"), Value: "regex"},
		{Label: i18n.T("tui.policies.type_forbidden"), Value: "forbidden_pattern"},
		{Label: i18n.T("tui.policies.type_limit"), Value: "limit"},
		{Label: i18n.T("tui.policies.type_boolean"), Value: "boolean"},
	}
}

// policyEnforcementOptions are the enforcement levels of the form.
func policyEnforcementOptions() []SelectOption {
	return []SelectOption{
		{Label: i18n.T("tui.policies.status_disabled"), Value: "disabled"},
		{Label: i18n.T("tui.policies.status_warn"), Value: "warn"},
		{Label: i18n.T("tui.policies.status_refuse"), Value: "refuse"},
	}
}

var policyScopeOptions = []SelectOption{
	{Label: "diff_only", Value: "diff_only"},
	{Label: "modified_files", Value: "modified_files"},
	{Label: "all_files", Value: "all_files"},
}

// ContextCommands implements CommandProvider.
func (v *PoliciesView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{ID: "policies.add", Label: i18n.T("tui.policies.action_template"), Aliases: []string{"add", "template", "ajouter"}, Description: i18n.T("tui.policies.cmd_add"), Category: v.Title(), Action: func() { v.addFromTemplate() }},
		{ID: "policies.add.custom", Label: i18n.T("tui.policies.action_custom"), Aliases: []string{"custom", "personnalisé"}, Description: i18n.T("tui.policies.cmd_add_custom"), Category: v.Title(), Action: func() { v.addPolicy() }},
		{ID: "policies.edit", Label: i18n.T("tui.hints.edit"), Aliases: []string{"edit", "éditer", "modifier"}, Description: i18n.T("tui.policies.cmd_edit"), Category: v.Title(), Action: func() { v.editSelectedPolicy() }},
		{ID: "policies.delete", Label: i18n.T("tui.hints.delete"), Aliases: []string{"delete", "remove", "supprimer"}, Description: i18n.T("tui.policies.cmd_delete"), Category: v.Title(), Action: func() { v.removeSelectedPolicy() }},
		{ID: "policies.check", Label: i18n.T("tui.hints.check"), Aliases: []string{"check", "verify", "vérifier"}, Description: i18n.T("tui.policies.cmd_check"), Category: v.Title(), Action: func() { v.checkPolicies() }},
	}
}
