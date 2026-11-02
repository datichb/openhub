package cmd

import (
	"regexp"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
)

// The keys of the editor, in one place: a change of key is one line here.
var columnEditorTestKeys = struct {
	add, del, role, mappings, up, down, back rune
}{add: 'a', del: 'd', role: 'r', mappings: 'm', up: 'K', down: 'J', back: 'q'}

var tviewTags = regexp.MustCompile(`\[[^\[\]]*\]`)

type modalCall struct {
	title, current string
	confirm        func(string)
}

// fakeColumnUI records what the editor asks of the shell.
type fakeColumnUI struct {
	modals []modalCall
	toasts []string
	levels []shell.ToastLevel
}

func (f *fakeColumnUI) ShowInputModal(title, current string, confirm func(string)) {
	f.modals = append(f.modals, modalCall{title, current, confirm})
}

func (f *fakeColumnUI) ShowToast(msg string, level shell.ToastLevel) {
	f.toasts = append(f.toasts, msg)
	f.levels = append(f.levels, level)
}

func (f *fakeColumnUI) lastToast() string {
	if len(f.toasts) == 0 {
		return ""
	}
	return f.toasts[len(f.toasts)-1]
}

// editorHarness drives the step of the column editor like the wizard does.
type editorHarness struct {
	t        *testing.T
	list     *tview.List
	hints    *tview.TextView
	ui       *fakeColumnUI
	columns  *[]teamstate.BoardColumnConfig
	result   *[]teamstate.BoardColumnConfig
	labels   *map[string]string
	status   *map[string]string
	validate func() string
	done     int
}

func newColumnEditor(t *testing.T, cols []teamstate.BoardColumnConfig, labels, statuses map[string]string, trackerType string) *editorHarness {
	t.Helper()
	i18ntest.Set(t, "en")
	e := &editorHarness{t: t, ui: &fakeColumnUI{}}
	prev := columnUI
	columnUI = func() columnEditorUI { return e.ui }
	t.Cleanup(func() { columnUI = prev })

	e.columns = &cols
	e.result = &[]teamstate.BoardColumnConfig{}
	var lm, sm *map[string]string
	if labels != nil {
		lm = &labels
		e.labels = lm
	}
	if statuses != nil {
		sm = &statuses
		e.status = sm
	}
	step := buildColumnEditorStep(e.columns, e.result, lm, sm, trackerType)
	container := tview.NewFlex()
	step.CustomView(tview.NewApplication(), container, func() { e.done++ })
	var ok bool
	e.list, ok = container.GetItem(0).(*tview.List)
	require.True(t, ok)
	e.hints, ok = container.GetItem(1).(*tview.TextView)
	require.True(t, ok)
	e.validate = step.Validate
	return e
}

func (e *editorHarness) press(key tcell.Key, r rune) {
	e.t.Helper()
	e.list.InputHandler()(tcell.NewEventKey(key, r, tcell.ModNone), func(tview.Primitive) {})
}

func (e *editorHarness) typ(r rune) { e.t.Helper(); e.press(tcell.KeyRune, r) }

func (e *editorHarness) names() []string {
	var out []string
	for _, c := range *e.columns {
		out = append(out, c.ID)
	}
	return out
}

func (e *editorHarness) hintsText() string { return e.hints.GetText(true) }

// text of the main line of item i, without tags.
func (e *editorHarness) mainText(i int) string {
	main, _ := e.list.GetItemText(i)
	return tviewTags.ReplaceAllString(main, "")
}

func defaultColumns() []teamstate.BoardColumnConfig {
	return append([]teamstate.BoardColumnConfig(nil), teamstate.DefaultBoardConfig().Columns...)
}

func TestColumnEditorRendersTheColumns(t *testing.T) {
	e := newColumnEditor(t, defaultColumns(), nil, nil, "")
	// header, 6 columns, blank, legend title, legend
	assert.Equal(t, 1+6+1+1+1, e.list.GetItemCount())
	assert.Equal(t, 1, e.list.GetCurrentItem(), "the cursor starts on the first column")
	assert.Contains(t, e.mainText(0), "Columns (6)")
	assert.Contains(t, e.mainText(1), "TODO")
	assert.Contains(t, e.mainText(1), "initial")
	assert.Contains(t, e.mainText(1), "1/6")
	_, secondary := e.list.GetItemText(1)
	assert.Contains(t, secondary, "id: todo")
	assert.NotContains(t, secondary, "labels:", "no mappings without mappings maps")
	assert.Contains(t, e.mainText(6), "blocked")

	hints := e.hintsText()
	for _, want := range []string{"nav", i18n.T("tui.team.columns.hint_move"), i18n.T("tui.team.columns.hint_add"),
		i18n.T("tui.team.columns.hint_rename"), i18n.T("tui.team.columns.hint_delete"), i18n.T("tui.team.columns.hint_role"),
		"Ctrl+S", i18n.T("tui.team.columns.hint_save")} {
		assert.Contains(t, hints, want)
	}
	assert.NotContains(t, hints, "mappings", "the mappings key only shows with mappings")
}

func TestColumnEditorMovesTheCursor(t *testing.T) {
	e := newColumnEditor(t, defaultColumns(), nil, nil, "")
	e.press(tcell.KeyDown, 0)
	assert.Equal(t, 2, e.list.GetCurrentItem())
	e.typ('j')
	assert.Equal(t, 3, e.list.GetCurrentItem())
	e.press(tcell.KeyUp, 0)
	e.typ('k')
	assert.Equal(t, 1, e.list.GetCurrentItem())
	e.typ('k')
	assert.Equal(t, 1, e.list.GetCurrentItem(), "never on the header")
	for i := 0; i < 10; i++ {
		e.typ('j')
	}
	assert.Equal(t, 6, e.list.GetCurrentItem(), "never past the last column")
}

func TestColumnEditorMovesAColumn(t *testing.T) {
	e := newColumnEditor(t, defaultColumns(), nil, nil, "")
	e.typ('j') // IN PROGRESS
	e.typ(columnEditorTestKeys.up)
	assert.Equal(t, []string{"in_progress", "todo", "review", "validation", "done", "blocked"}, e.names())
	assert.Equal(t, 1, e.list.GetCurrentItem(), "the cursor follows the column")
	assert.Equal(t, i18n.T("tui.team.columns.moved_up"), e.ui.lastToast())

	e.typ(columnEditorTestKeys.up)
	assert.Equal(t, "in_progress", e.names()[0], "the first column stays first")

	e.typ(columnEditorTestKeys.down)
	assert.Equal(t, []string{"todo", "in_progress", "review", "validation", "done", "blocked"}, e.names())
	assert.Equal(t, 2, e.list.GetCurrentItem())
	assert.Equal(t, i18n.T("tui.team.columns.moved_down"), e.ui.lastToast())
}

func TestColumnEditorAddsAColumnAfterTheCurrentOne(t *testing.T) {
	e := newColumnEditor(t, defaultColumns(), nil, nil, "")
	e.typ(columnEditorTestKeys.add)
	require.Len(t, e.ui.modals, 1)
	assert.Equal(t, i18n.T("cmd.discovery.field.column_name"), e.ui.modals[0].title)
	assert.Empty(t, e.ui.modals[0].current)

	e.ui.modals[0].confirm("")
	assert.Len(t, e.names(), 6, "an empty name adds nothing")

	e.ui.modals[0].confirm(" Code Review ")
	assert.Equal(t, []string{"todo", "code_review", "in_progress", "review", "validation", "done", "blocked"}, e.names())
	added := (*e.columns)[1]
	assert.Equal(t, "Code Review", added.Name)
	assert.Equal(t, "active", added.Role, "the role is suggested from the name (active when nothing matches)")
	assert.Equal(t, 2, e.list.GetCurrentItem(), "the cursor is on the new column")
	assert.Equal(t, i18n.Tf("tui.team.columns.column_added", "Code Review"), e.ui.lastToast())
}

func TestColumnEditorRenames(t *testing.T) {
	e := newColumnEditor(t, defaultColumns(), nil, nil, "")
	e.press(tcell.KeyEnter, 0)
	require.Len(t, e.ui.modals, 1)
	assert.Equal(t, i18n.T("cmd.discovery.field.column_rename"), e.ui.modals[0].title)
	assert.Equal(t, "TODO", e.ui.modals[0].current)
	e.ui.modals[0].confirm("")
	assert.Equal(t, "TODO", (*e.columns)[0].Name)
	e.ui.modals[0].confirm(" Backlog ")
	assert.Equal(t, "Backlog", (*e.columns)[0].Name)
	assert.Equal(t, "todo", (*e.columns)[0].ID, "the id does not change")
	assert.Contains(t, e.mainText(1), "Backlog")
}

func TestColumnEditorDeletes(t *testing.T) {
	e := newColumnEditor(t, defaultColumns(), map[string]string{"wip": "in_progress", "later": "todo"}, map[string]string{"Doing": "in_progress"}, "jira")
	e.typ('j') // IN PROGRESS
	e.typ(columnEditorTestKeys.del)
	assert.Equal(t, []string{"todo", "review", "validation", "done", "blocked"}, e.names())
	assert.Equal(t, map[string]string{"later": "todo"}, *e.labels, "the labels of the column go with it")
	assert.Empty(t, *e.status)
	assert.Equal(t, i18n.Tf("tui.team.columns.column_deleted", "IN PROGRESS")+i18n.Tf("tui.team.columns.mappings_cleaned", 2), e.ui.lastToast())

	e.typ(columnEditorTestKeys.del) // REVIEW, no mapping
	assert.Equal(t, i18n.Tf("tui.team.columns.column_deleted", "REVIEW"), e.ui.lastToast())
}

func TestColumnEditorKeepsTwoColumns(t *testing.T) {
	cols := []teamstate.BoardColumnConfig{{ID: "a", Name: "A", Role: "initial"}, {ID: "b", Name: "B", Role: "terminal"}}
	e := newColumnEditor(t, cols, nil, nil, "")
	e.typ(columnEditorTestKeys.del)
	assert.Equal(t, []string{"a", "b"}, e.names())
	assert.Equal(t, i18n.T("cmd.discovery.validate.min_columns"), e.ui.lastToast())
	assert.Equal(t, shell.ToastError, e.ui.levels[len(e.ui.levels)-1])
}

func TestColumnEditorCyclesTheRole(t *testing.T) {
	e := newColumnEditor(t, defaultColumns(), nil, nil, "")
	var roles []string
	for i := 0; i < 5; i++ {
		e.typ(columnEditorTestKeys.role)
		roles = append(roles, (*e.columns)[0].Role)
	}
	assert.Equal(t, []string{"active", "terminal", "blocked", "initial", "active"}, roles)

	// a column with no role counts as active
	cols := []teamstate.BoardColumnConfig{{ID: "a", Name: "A"}, {ID: "b", Name: "B", Role: "terminal"}}
	e = newColumnEditor(t, cols, nil, nil, "")
	e.typ(columnEditorTestKeys.role)
	assert.Equal(t, "terminal", (*e.columns)[0].Role)
}

func TestColumnEditorMappings(t *testing.T) {
	labels := map[string]string{"wip": "in_progress", "doing": "in_progress", "later": "todo"}
	statuses := map[string]string{"In Dev": "in_progress"}
	e := newColumnEditor(t, defaultColumns(), labels, statuses, "jira")
	_, secondary := e.list.GetItemText(2)
	assert.Contains(t, secondary, "labels: doing, wip, In Dev", "the labels then the statuses of the column")
	assert.Contains(t, e.hintsText(), "mappings")

	// m opens the mappings of the column under the cursor
	e.typ('j')
	e.typ(columnEditorTestKeys.mappings)
	assert.Contains(t, e.mainText(0), "Mappings → IN PROGRESS")
	var shown []string
	for i := 0; i < e.list.GetItemCount(); i++ {
		shown = append(shown, e.mainText(i))
	}
	text := strings.Join(shown, "|")
	assert.Contains(t, text, "doing")
	assert.Contains(t, text, "wip")
	assert.Contains(t, text, "Statuses (Jira)")
	assert.Contains(t, text, "In Dev")
	assert.NotContains(t, text, "later")
	assert.Contains(t, e.hintsText(), i18n.T("tui.team.columns.hint_back_columns"))
	assert.Contains(t, e.hintsText(), i18n.T("tui.team.columns.hint_add_label"))
	assert.Contains(t, shown[e.list.GetCurrentItem()], "doing", "the cursor starts on the first mapping")

	// the cursor skips the headers
	e.typ('j')
	assert.Contains(t, shown[e.list.GetCurrentItem()], "wip")
	e.typ('j')
	assert.Contains(t, shown[e.list.GetCurrentItem()], "In Dev")
	e.typ('k')
	assert.Contains(t, shown[e.list.GetCurrentItem()], "wip")

	// add a label
	e.typ(columnEditorTestKeys.add)
	require.Len(t, e.ui.modals, 1)
	assert.Equal(t, i18n.T("tui.team.columns.label_name_prompt"), e.ui.modals[0].title)
	e.ui.modals[0].confirm("")
	assert.Len(t, *e.labels, 3)
	e.ui.modals[0].confirm(" blocked-by ")
	assert.Equal(t, "in_progress", (*e.labels)["blocked-by"])
	assert.Equal(t, i18n.Tf("tui.team.columns.label_added", "blocked-by", "in_progress"), e.ui.lastToast())

	// back to the columns
	e.typ(columnEditorTestKeys.back)
	assert.Contains(t, e.mainText(0), "Columns (6)")
	assert.Equal(t, 6+1+1+1+1, e.list.GetItemCount())
}

func TestColumnEditorMappingsOnlyWhereThereAreMappings(t *testing.T) {
	e := newColumnEditor(t, defaultColumns(), nil, nil, "")
	e.typ(columnEditorTestKeys.mappings)
	assert.Contains(t, e.mainText(0), "Columns (6)", "no mappings mode without mappings")
}

func TestColumnEditorSavesOnCtrlS(t *testing.T) {
	cols := []teamstate.BoardColumnConfig{{ID: "a", Name: "A", Role: "initial"}, {ID: "b", Name: "B", Role: "active"}}
	e := newColumnEditor(t, cols, nil, nil, "")
	e.press(tcell.KeyCtrlS, 0)
	assert.Equal(t, i18n.T("cmd.discovery.validate.need_terminal"), e.ui.lastToast(), "a terminal column is needed")
	assert.Zero(t, e.done)
	assert.Equal(t, i18n.T("cmd.discovery.validate.min_columns"), e.validate(), "nothing saved yet")

	e.typ('j')
	e.typ(columnEditorTestKeys.role) // active → terminal
	e.press(tcell.KeyCtrlS, 0)
	assert.Equal(t, 1, e.done)
	assert.Equal(t, "terminal", (*e.result)[1].Role)
	assert.Empty(t, e.validate())

	// the result is a copy
	(*e.columns)[0].Name = "changed"
	assert.Equal(t, "A", (*e.result)[0].Name)

	one := []teamstate.BoardColumnConfig{{ID: "a", Name: "A", Role: "terminal"}}
	e = newColumnEditor(t, one, nil, nil, "")
	e.press(tcell.KeyCtrlS, 0)
	assert.Equal(t, i18n.T("cmd.discovery.validate.min_columns"), e.ui.lastToast())
}

func TestColumnEditorInfoFields(t *testing.T) {
	e := newColumnEditor(t, defaultColumns(), nil, nil, "")
	e.press(tcell.KeyCtrlS, 0)
	step := buildColumnEditorStep(e.columns, e.result, nil, nil, "")
	fields := step.InfoFields()
	require.Len(t, fields, 2)
	assert.Equal(t, i18n.Tf("cmd.discovery.info.columns_selected", 6), fields[0].Value)
	assert.Equal(t, "TODO → IN PROGRESS → REVIEW → VALIDATION → DONE → BLOCKED", fields[1].Value)
	assert.NoError(t, step.OnDone())
}

func TestColumnEditorIgnoresOtherKeys(t *testing.T) {
	e := newColumnEditor(t, defaultColumns(), nil, nil, "")
	before := e.names()
	for _, r := range "xyzw" {
		e.typ(r)
	}
	assert.Equal(t, before, e.names())
	assert.Empty(t, e.ui.modals)
	assert.Empty(t, e.ui.toasts)
}

func TestColumnRoleHexAndColorName(t *testing.T) {
	assert.Equal(t, theme.WarningHex, columnRoleHex(teamstate.ColumnRoleInitial))
	assert.Equal(t, theme.SuccessHex, columnRoleHex(teamstate.ColumnRoleTerminal))
	assert.Equal(t, theme.ErrorHex, columnRoleHex(teamstate.ColumnRoleBlocked))
	assert.Equal(t, theme.AccentHex, columnRoleHex(teamstate.ColumnRoleActive))
	assert.Equal(t, "red", columnColorName(teamstate.BoardColumnConfig{Color: "red"}, tcell.ColorRed))
	assert.Equal(t, "auto (#ff0000)", columnColorName(teamstate.BoardColumnConfig{}, tcell.NewRGBColor(255, 0, 0)))
}
