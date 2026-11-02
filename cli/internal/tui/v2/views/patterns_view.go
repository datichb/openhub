package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// PatternsView displays and manages team decomposition patterns.
type PatternsView struct {
	app         *tview.Application
	resolveTeam ResolveTeamFunc
	slist       *widgets.SectionedList
	header      *tview.TextView
	page        *widgets.ListDetailPage
	shell       ShellAccess
	patterns    []teamstate.Pattern

	// actionIndices maps SectionedList item indices to action callbacks.
	actionIndices map[int]func()
	// patternIndices maps SectionedList item indices to pattern slice indices.
	patternIndices map[int]int

	loads    loadSlot // the reads of the patterns: only the latest is applied
	readPage loadSlot // the read of the pattern being opened
}

// patternsData is what a read of the patterns gives back.
type patternsData struct {
	cloned   bool
	patterns []teamstate.Pattern
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
	return keys.Hints(keys.Navigate.Hint(), "{/} "+i18n.T("tui.hints.sections"),
		keys.Open.HintAs(i18n.T("tui.hints.see")), keys.Create.Hint(),
		keys.Accept.Hint(), keys.Delete.Hint(), keys.Refresh.Hint())
}

// Mount builds the patterns view with a descriptive header and SectionedList.
func (v *PatternsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	// Fixed-height descriptive header with word-wrap
	v.header = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false).
		SetWordWrap(true)
	v.header.SetBackgroundColor(theme.BgPanel)

	// Sectioned list for patterns + actions
	v.slist = widgets.NewSectionedList().SetApp(app)
	v.slist.SetItemChangedFunc(func(idx int, _ widgets.SectionItem) { v.showItemDetail(idx) })

	v.slist.SetItemSelectedFunc(func(idx int, item widgets.SectionItem) {
		if fn, ok := v.actionIndices[idx]; ok {
			fn()
			return
		}
		if pi, ok := v.patternIndices[idx]; ok {
			v.showPattern(pi)
		}
	})

	// The header stays above the list, the detail on the right.
	v.page = widgets.NewListDetailPage(v.Title(), i18n.T("tui.catalog.detail"))
	v.page.AddHeader(v.header, 5)
	v.page.SetMain(v.slist)

	v.refresh()
	v.page.Mount(content)
}

// Unmount cleans up resources.
func (v *PatternsView) Unmount() {
	v.loads.Invalidate()
	v.readPage.Invalidate()
	v.app = nil
	v.slist = nil
	v.header = nil
	v.page = nil
}

// HandleKey processes patterns view key events.
func (v *PatternsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch {
	case keys.Refresh.Matches(event):
		v.refresh()
		return nil
	case keys.Create.Matches(event):
		v.addPattern()
		return nil
	case keys.Accept.Matches(event):
		v.validatePattern()
		return nil
	case keys.Delete.Matches(event):
		v.removePattern()
		return nil
	}
	if keys.Open.Matches(event) {
		if v.slist == nil {
			return nil
		}
		if idx, _, ok := v.slist.CurrentItem(); ok {
			if fn, exists := v.actionIndices[idx]; exists {
				fn()
			} else if pi, exists := v.patternIndices[idx]; exists {
				v.showPattern(pi)
			}
		}
		return nil
	}
	return event
}

// newRepo is the team-state of the team, without touching the disk (nil
// without a team).
func (v *PatternsView) newRepo() teamstate.TeamStateWriter {
	tc := v.resolveTeam()
	if !tc.Enabled {
		return nil
	}
	return teamstate.NewRepo(tc.StateRepo, tc.StatePath)
}

// refresh pulls the team-state, then reads the patterns off the event loop.
func (v *PatternsView) refresh() {
	v.patterns = nil
	v.actionIndices = nil
	v.patternIndices = nil
	if v.slist != nil {
		v.slist.SetItems(nil)
	}
	setPageDetail(v.page, emptyDetail())

	repo := v.newRepo()
	if repo == nil {
		v.loads.Invalidate()
		v.setHeader(stateEmpty, i18n.T("tui.patterns.team_not_configured"))
		return
	}
	v.setHeader(stateLoading, "")

	fetch := func(context.Context) (patternsData, error) {
		if !repo.IsCloned() {
			return patternsData{}, nil
		}
		patterns, err := repo.ListPatterns(nil, 0)
		return patternsData{cloned: true, patterns: patterns}, err
	}
	syncLoad(v.app, shellContext(v.shell), &v.loads, repo, v.shell, fetch, func(d patternsData, err error) {
		v.renderPatterns(d, err)
	})
}

// setHeader writes a state in the header.
func (v *PatternsView) setHeader(kind stateKind, msg string) {
	if v.header != nil {
		v.header.SetText(stateText(kind, msg))
	}
}

func (v *PatternsView) setHeaderText(text string) {
	if v.header == nil {
		return
	}
	v.header.SetText(theme.Paint(theme.TextSecondaryHex, tview.Escape(text)))
}

func (v *PatternsView) renderPatterns(d patternsData, err error) {
	if v.slist == nil {
		return
	}
	switch {
	case err != nil:
		v.setHeader(stateError, err.Error())
		return
	case !d.cloned:
		v.setHeader(stateEmpty, i18n.T("tui.patterns.team_not_configured"))
		return
	}
	patterns := d.patterns
	v.patterns = patterns

	// Update header text based on state
	if len(patterns) == 0 {
		v.setHeaderText(i18n.T("tui.patterns.getting_started"))
	} else {
		v.setHeaderText(i18n.T("tui.patterns.about"))
	}

	var items []widgets.SectionItem
	v.actionIndices = make(map[int]func())
	v.patternIndices = make(map[int]int)

	// Separate validated from pending
	var validated, pending []int
	for i, p := range patterns {
		if p.Validated {
			validated = append(validated, i)
		} else {
			pending = append(pending, i)
		}
	}

	if len(patterns) > 0 {
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

	// ── Actions ──
	items = append(items, widgets.SectionItem{
		MainText: i18n.T("tui.patterns.section_actions"),
		IsHeader: true,
	})
	createIdx := len(items)
	items = append(items, widgets.SectionItem{
		MainText: "  " + theme.Paint(theme.ActiveMode.SecondaryHex, theme.SymCursor+" "+i18n.T("tui.patterns.action_create")),
	})
	v.actionIndices[createIdx] = func() { v.addPattern() }

	v.slist.SetItems(items)
	idx, _, _ := v.slist.CurrentItem()
	v.showItemDetail(idx)
}

// showItemDetail fills the detail pane with the list item at idx (pattern or
// action).
func (v *PatternsView) showItemDetail(idx int) {
	if v.page == nil {
		return
	}
	if pi, ok := v.patternIndices[idx]; ok && pi >= 0 && pi < len(v.patterns) {
		p := v.patterns[pi]
		var d detailText
		d.title(tview.Escape(p.Name))
		d.para(tview.Escape(p.Description))
		status := theme.Paint(theme.TextMutedHex, i18n.T("tui.detail.pattern.pending"))
		if p.Validated {
			status = theme.Paint(theme.SuccessHex, i18n.T("tui.detail.pattern.validated"))
		}
		d.field(i18n.T("tui.detail.status"), status)
		d.field(i18n.T("tui.detail.complexity"), tview.Escape(p.Complexity))
		d.field(i18n.T("tui.detail.tags"), tview.Escape(strings.Join(p.Tags, ", ")))
		d.field(i18n.T("tui.detail.origin"), tview.Escape(p.Source))
		d.field(i18n.T("tui.detail.project"), tview.Escape(p.Project))
		d.field(i18n.T("tui.detail.date"), tview.Escape(p.CreatedAt))
		pairs := []string{keys.Open.Keys(), i18n.T("tui.hints.see")}
		if !p.Validated {
			pairs = append(pairs, keys.Pairs(keys.Accept)...)
		}
		d.keys(append(pairs, keys.Pairs(keys.Delete, keys.Refresh)...)...)
		v.page.SetDetail(d.String())
		return
	}
	if _, ok := v.actionIndices[idx]; ok && v.slist != nil {
		var d detailText
		d.title(i18n.T("tui.patterns.action_create"))
		d.para(i18n.T("tui.patterns.hint_complexity"))
		d.keys(keys.Pairs(keys.Open, keys.Create)...)
		v.page.SetDetail(d.String())
		return
	}
	v.page.SetDetail(emptyDetail())
}

func (v *PatternsView) buildPatternItem(p teamstate.Pattern) widgets.SectionItem {
	icon := theme.Paint(theme.TextMutedHex, theme.SymInactive)
	if p.Validated {
		icon = theme.Paint(theme.SuccessHex, theme.SymSuccess)
	}

	// Build metadata: complexity + tags
	meta := p.Complexity
	if len(p.Tags) > 0 {
		meta += " · " + strings.Join(p.Tags, ", ")
	}

	mainText := fmt.Sprintf("  %s %s %s", icon, tview.Escape(fmt.Sprintf("%-30s", p.Name)), theme.Paint(theme.TextMutedHex, tview.Escape(meta)))

	// Description as secondary text in readable color
	secondaryText := ""
	if p.Description != "" {
		secondaryText = "    " + theme.Paint(theme.TextSecondaryHex, tview.Escape(p.Description))
	}

	return widgets.SectionItem{
		MainText:      mainText,
		SecondaryText: secondaryText,
	}
}

func (v *PatternsView) showPattern(patternIdx int) {
	if patternIdx < 0 || patternIdx >= len(v.patterns) || v.shell == nil {
		return
	}
	p := v.patterns[patternIdx]
	repo := v.newRepo()
	if repo == nil {
		return
	}
	sh := v.shell
	load(v.app, shellContext(sh), &v.readPage, func(context.Context) (string, error) {
		return repo.ReadPattern(p.Name)
	}, func(content string, err error) {
		if err != nil {
			sh.ShowToastMsg(i18n.T("tui.patterns.read_error")+": "+err.Error(), false)
			return
		}
		sh.ShowScrollableModal(i18n.Tf("tui.patterns.modal_title", p.Name), content, []ModalAction{
			{Label: i18n.T("tui.policies.close"), Callback: func() {}},
		})
	})
}

// act runs a write of the team-state off the event loop, then tells how it
// went and reads the patterns again. Without an event loop (tests) it runs
// at once.
func (v *PatternsView) act(work func(ctx context.Context) error, done string) {
	app, sh, ctx := v.app, v.shell, shellContext(v.shell)
	finish := func(err error) {
		if sh == nil {
			return
		}
		if err != nil {
			sh.ShowToastMsg(errorText(err), false)
			return
		}
		sh.ShowToastMsg(done, true)
		v.refresh()
	}
	if app == nil {
		finish(work(ctx))
		return
	}
	safego.Go(func() { // an action, not a read: its result is applied even if the list was refreshed
		err := work(ctx)
		app.QueueUpdateDraw(func() {
			if v.app != nil { // else unmounted meanwhile
				finish(err)
			}
		})
	})
}

func (v *PatternsView) addPattern() {
	if v.shell == nil {
		return
	}
	repo := v.newRepo()
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
				CreatedAt:   time.Now().Format(time.DateOnly),
			}

			// Build richer markdown template
			descLine := i18n.T("tui.patterns.template_description")
			if description != "" {
				descLine = description
			}
			content := i18n.Tf("tui.patterns.template", name, descLine)
			v.act(func(ctx context.Context) error { return repo.CreatePattern(ctx, p, content) }, i18n.Tf("tui.patterns.created", name))
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

	repo := v.newRepo()
	if repo == nil {
		return
	}
	v.act(func(ctx context.Context) error { return repo.ValidatePattern(ctx, p.Name) }, i18n.Tf("tui.patterns.validated", p.Name))
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

	repo := v.newRepo()
	if repo == nil || v.shell == nil {
		return
	}

	v.shell.Confirm(i18n.Tf("tui.patterns.confirm_delete", p.Name), i18n.T("tui.patterns.delete_detail"), true, func() {
		v.act(func(ctx context.Context) error { return repo.RemovePattern(ctx, p.Name) }, i18n.Tf("tui.patterns.deleted", p.Name))
	})
}

// ContextCommands implements CommandProvider.
func (v *PatternsView) ContextCommands() []ContextCommand {
	return []ContextCommand{
		{ID: "patterns.add", Label: i18n.T("tui.keys.action.create"), Aliases: []string{"add", "new", "ajouter", "créer"}, Description: i18n.T("tui.patterns.cmd_add"), Category: v.Title(), Action: func() { v.addPattern() }},
		{ID: "patterns.validate", Label: i18n.T("tui.hints.validate"), Aliases: []string{"validate", "valider"}, Description: i18n.T("tui.patterns.cmd_validate"), Category: v.Title(), Action: func() { v.validatePattern() }},
		{ID: "patterns.delete", Label: i18n.T("tui.hints.delete"), Aliases: []string{"delete", "remove", "supprimer"}, Description: i18n.T("tui.patterns.cmd_delete"), Category: v.Title(), Action: func() { v.removePattern() }},
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
