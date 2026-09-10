package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// PatternsView displays and manages team decomposition patterns.
type PatternsView struct {
	app         *tview.Application
	resolveTeam ResolveTeamFunc
	slist       *widgets.SectionedList
	shell       ShellAccess
	patterns    []teamstate.Pattern

	// actionIndices maps SectionedList item indices to action callbacks.
	actionIndices map[int]func()
	// patternIndices maps SectionedList item indices to pattern slice indices.
	patternIndices map[int]int
}

var _ View = (*PatternsView)(nil)
var _ CommandProvider = (*PatternsView)(nil)

// NewPatternsView creates a new patterns view.
// resolveTeam is called on every refresh to obtain the effective team config.
func NewPatternsView(resolveTeam ResolveTeamFunc) *PatternsView {
	return &PatternsView{resolveTeam: resolveTeam}
}

// SetShell provides the shell reference for modal interactions.
func (v *PatternsView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *PatternsView) ID() string { return "team.patterns" }

// Title returns the display title.
func (v *PatternsView) Title() string { return i18n.T("tui.team.patterns") }

// StatusHints returns keybinding hints.
func (v *PatternsView) StatusHints() string {
	return fmt.Sprintf("j/k %s · {/} %s · Enter %s · a %s · v %s · d %s · r %s",
		i18n.T("tui.hints.nav"), "sections",
		i18n.T("tui.hints.see"), i18n.T("tui.hints.add"),
		i18n.T("tui.hints.validate"), i18n.T("tui.hints.delete"),
		i18n.T("tui.hints.refresh"))
}

// Mount builds the patterns view with SectionedList.
func (v *PatternsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.slist = widgets.NewSectionedList().SetApp(app)
	v.slist.SetBorderPadding(1, 0, 2, 2)

	v.slist.SetItemSelectedFunc(func(idx int, item widgets.SectionItem) {
		if fn, ok := v.actionIndices[idx]; ok {
			fn()
			return
		}
		if pi, ok := v.patternIndices[idx]; ok {
			v.showPattern(pi)
		}
	})

	v.refresh()
	content.AddItem(v.slist, 0, 1, true)
}

// Unmount cleans up resources.
func (v *PatternsView) Unmount() {
	v.app = nil
	v.slist = nil
}

// HandleKey processes patterns view key events.
func (v *PatternsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'r':
		v.refresh()
		return nil
	case 'a':
		v.addPattern()
		return nil
	case 'v':
		v.validatePattern()
		return nil
	case 'd':
		v.removePattern()
		return nil
	}
	if event.Key() == tcell.KeyEnter {
		// Handled by SetItemSelectedFunc
		return event
	}
	return event
}

func (v *PatternsView) getRepo() teamstate.TeamStateWriter {
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

func (v *PatternsView) refresh() {
	v.patterns = nil
	v.actionIndices = nil
	v.patternIndices = nil
	if v.slist != nil {
		v.slist.SetItems(nil)
	}

	repo := v.getRepo()
	if repo == nil {
		if v.slist != nil {
			v.slist.SetItems([]widgets.SectionItem{
				{MainText: "  " + i18n.T("tui.patterns.team_not_configured"), IsHeader: false},
			})
		}
		return
	}

	syncAsync(v.app, repo, v.shell, func(_ error) {
		v.renderPatterns(repo)
	})
}

func (v *PatternsView) renderPatterns(repo teamstate.TeamStateWriter) {
	if v.slist == nil {
		return
	}

	patterns, err := repo.ListPatterns(nil, 0)
	if err != nil {
		v.slist.SetItems([]widgets.SectionItem{
			{MainText: "  " + i18n.T("tui.settings.error") + ": " + err.Error()},
		})
		return
	}
	v.patterns = patterns

	var items []widgets.SectionItem
	v.actionIndices = make(map[int]func())
	v.patternIndices = make(map[int]int)

	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagReset

	// ── About section ──
	items = append(items, widgets.SectionItem{
		MainText: i18n.T("tui.patterns.section_about"),
		IsHeader: true,
	})
	items = append(items, widgets.SectionItem{
		MainText:      fmt.Sprintf("  %s%s%s", muted, i18n.T("tui.patterns.about"), reset),
		SecondaryText: "",
	})

	// Separate validated from pending
	var validated, pending []int
	for i, p := range patterns {
		if p.Validated {
			validated = append(validated, i)
		} else {
			pending = append(pending, i)
		}
	}

	if len(patterns) == 0 {
		// ── Getting started section ──
		items = append(items, widgets.SectionItem{
			MainText: i18n.T("tui.patterns.section_actions"),
			IsHeader: true,
		})
		aboutIdx := len(items)
		items = append(items, widgets.SectionItem{
			MainText:      fmt.Sprintf("  %s%s%s", muted, i18n.T("tui.patterns.getting_started"), reset),
			SecondaryText: "",
		})
		// Make this item non-interactive (info only)
		_ = aboutIdx
	} else {
		// ── Validated patterns ──
		if len(validated) > 0 {
			items = append(items, widgets.SectionItem{
				MainText: fmt.Sprintf(i18n.T("tui.patterns.section_validated"), len(validated)),
				IsHeader: true,
			})
			for _, pi := range validated {
				p := patterns[pi]
				idx := len(items)
				items = append(items, v.buildPatternItem(p))
				v.patternIndices[idx] = pi
			}
		}

		// ── Pending patterns ──
		if len(pending) > 0 {
			items = append(items, widgets.SectionItem{
				MainText: fmt.Sprintf(i18n.T("tui.patterns.section_pending"), len(pending)),
				IsHeader: true,
			})
			for _, pi := range pending {
				p := patterns[pi]
				idx := len(items)
				items = append(items, v.buildPatternItem(p))
				v.patternIndices[idx] = pi
			}
		}
	}

	// ── Actions section ──
	items = append(items, widgets.SectionItem{
		MainText: i18n.T("tui.patterns.section_actions"),
		IsHeader: true,
	})
	createIdx := len(items)
	items = append(items, widgets.SectionItem{
		MainText:      fmt.Sprintf("  %s%s%s  %s→%s", theme.ColorTag(theme.ActionHex), theme.IconArrow, theme.TagColor, theme.ColorTag(theme.AccentHex), reset),
		SecondaryText: fmt.Sprintf("    %s%s%s", muted, i18n.T("tui.patterns.action_create"), reset),
	})
	v.actionIndices[createIdx] = func() { v.addPattern() }

	v.slist.SetItems(items)
}

func (v *PatternsView) buildPatternItem(p teamstate.Pattern) widgets.SectionItem {
	icon := fmt.Sprintf("%s○%s", theme.ColorTag(theme.TextMutedHex), theme.TagColor)
	if p.Validated {
		icon = fmt.Sprintf("%s✓%s", theme.ColorTag(theme.SuccessHex), theme.TagColor)
	}

	// Build metadata: complexity + tags
	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagReset
	meta := p.Complexity
	if len(p.Tags) > 0 {
		meta += " · " + strings.Join(p.Tags, ", ")
	}

	mainText := fmt.Sprintf("  %s %-30s %s%s%s", icon, p.Name, muted, meta, reset)

	// Description as secondary text
	secondaryText := ""
	if p.Description != "" {
		secondaryText = fmt.Sprintf("    %s%s%s", muted, p.Description, reset)
	}

	return widgets.SectionItem{
		MainText:      mainText,
		SecondaryText: secondaryText,
	}
}

func (v *PatternsView) showPattern(patternIdx int) {
	if patternIdx < 0 || patternIdx >= len(v.patterns) {
		return
	}
	p := v.patterns[patternIdx]

	repo := v.getRepo()
	if repo == nil {
		return
	}

	content, err := repo.ReadPattern(p.Name)
	if err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.patterns.read_error")+": "+err.Error(), false)
		}
		return
	}

	if v.shell != nil {
		v.shell.ShowScrollableModal("Pattern: "+p.Name, content, []ModalAction{
			{Label: i18n.T("tui.policies.close"), Callback: func() {}},
		})
	}
}

func (v *PatternsView) addPattern() {
	if v.shell == nil {
		return
	}
	repo := v.getRepo()
	if repo == nil {
		v.shell.ShowToastMsg(i18n.T("tui.patterns.team_not_configured"), false)
		return
	}

	complexityOpts := []SelectOption{
		{Label: i18n.T("tui.patterns.complexity_low"), Value: "low"},
		{Label: i18n.T("tui.patterns.complexity_medium"), Value: "medium"},
		{Label: i18n.T("tui.patterns.complexity_high"), Value: "high"},
	}

	v.shell.ShowInlineForm(InlineFormConfig{
		Title: i18n.T("tui.patterns.create_title"),
		Fields: []FormField{
			{Key: "name", Label: i18n.T("tui.patterns.field_name"), Type: FieldText, Required: true,
				Hint: i18n.T("tui.patterns.hint_name")},
			{Key: "description", Label: i18n.T("tui.patterns.field_description"), Type: FieldText,
				Hint: i18n.T("tui.patterns.hint_description")},
			{Key: "tags", Label: i18n.T("tui.patterns.field_tags"), Type: FieldText,
				Hint: i18n.T("tui.patterns.hint_tags")},
			{Key: "complexity", Label: i18n.T("tui.patterns.field_complexity"), Type: FieldSelect,
				Options: complexityOpts, Default: "medium",
				Hint: i18n.T("tui.patterns.hint_complexity")},
		},
		OnSubmit: func(values map[string]string, _ map[string][]string) {
			name := values["name"]
			if name == "" {
				return
			}
			description := values["description"]
			p := teamstate.Pattern{
				Name:        name,
				Description: description,
				Tags:        splitTags(values["tags"]),
				Complexity:  values["complexity"],
				Source:      "manual",
				Validated:   false,
				CreatedAt:   time.Now().Format("2006-01-02"),
			}

			// Build richer markdown template
			descLine := "TODO"
			if description != "" {
				descLine = description
			}
			content := fmt.Sprintf("# %s\n\n## Description\n\n%s\n\n## Quand utiliser ce pattern\n\nDécrivez les situations dans lesquelles ce pattern s'applique.\n\n## Étapes de décomposition\n\n1. ...\n2. ...\n3. ...\n\n## Critères de validation\n\n- [ ] ...\n", name, descLine)
			if err := repo.CreatePattern(context.Background(), p, content); err != nil {
				v.shell.ShowToastMsg(i18n.T("tui.settings.error")+": "+err.Error(), false)
			} else {
				v.shell.ShowToastMsg(i18n.Tf("tui.patterns.created", name), true)
				v.refresh()
			}
		},
		OnCancel: nil,
	})
}

func (v *PatternsView) validatePattern() {
	if v.slist == nil {
		return
	}

	// Find which pattern is selected
	idx, _, ok := v.slist.CurrentItem()
	if !ok {
		return
	}
	pi, isPattern := v.patternIndices[idx]
	if !isPattern || pi < 0 || pi >= len(v.patterns) {
		return
	}
	p := v.patterns[pi]

	repo := v.getRepo()
	if repo == nil {
		return
	}

	if err := repo.ValidatePattern(context.Background(), p.Name); err != nil {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.T("tui.settings.error")+": "+err.Error(), false)
		}
	} else {
		if v.shell != nil {
			v.shell.ShowToastMsg(i18n.Tf("tui.patterns.validated", p.Name), true)
		}
		v.refresh()
	}
}

func (v *PatternsView) removePattern() {
	if v.slist == nil {
		return
	}

	idx, _, ok := v.slist.CurrentItem()
	if !ok {
		return
	}
	pi, isPattern := v.patternIndices[idx]
	if !isPattern || pi < 0 || pi >= len(v.patterns) {
		return
	}
	p := v.patterns[pi]

	repo := v.getRepo()
	if repo == nil {
		return
	}
	if v.shell == nil {
		return
	}

	v.shell.ShowSelectModal(i18n.Tf("tui.patterns.confirm_delete", p.Name), []SelectOption{
		{Label: i18n.T("tui.settings.cancel"), Value: ""},
		{Label: i18n.T("tui.patterns.yes_delete"), Value: "yes"},
	}, "", func(choice string) {
		if choice != "yes" {
			return
		}
		if err := repo.RemovePattern(context.Background(), p.Name); err != nil {
			v.shell.ShowToastMsg(i18n.T("tui.settings.error")+": "+err.Error(), false)
		} else {
			v.shell.ShowToastMsg(i18n.Tf("tui.patterns.deleted", p.Name), true)
			v.refresh()
		}
	})
}

// ContextCommands implements CommandProvider.
func (v *PatternsView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{ID: "patterns.add", Label: i18n.T("tui.hints.add"), Aliases: []string{"add", "new", "ajouter", "créer"}, Description: i18n.T("tui.patterns.cmd_add"), Category: "Patterns", Action: func() { v.addPattern() }},
		{ID: "patterns.validate", Label: i18n.T("tui.hints.validate"), Aliases: []string{"validate", "valider"}, Description: i18n.T("tui.patterns.cmd_validate"), Category: "Patterns", Action: func() { v.validatePattern() }},
		{ID: "patterns.delete", Label: i18n.T("tui.hints.delete"), Aliases: []string{"delete", "remove", "supprimer"}, Description: i18n.T("tui.patterns.cmd_delete"), Category: "Patterns", Action: func() { v.removePattern() }},
	}
}

func splitTags(s string) []string {
	var tags []string
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

func quoteSlice(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf("%q", item)
	}
	return strings.Join(quoted, ", ")
}
