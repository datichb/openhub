package cmd

import (
	"fmt"
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
	"github.com/datichb/openhub/cli/internal/testutil/golden"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The keys of the editor, in one place: a change of key is one line here.
var columnEditorTestKeys = struct {
	add, del, role, mappings, up, down, back, moveTo rune
}{add: 'n', del: 'd', role: 'o', mappings: 'm', up: 'K', down: 'J', back: 'q', moveTo: 'v'}

var tviewTags = regexp.MustCompile(`\[[^\[\]]*\]`)

type modalCall struct {
	title, current string
	confirm        func(string)
}

type selectCall struct {
	title   string
	options []views.SelectOption
	pick    func(string)
}

type confirmCall struct {
	title, detail string
	danger        bool
	yes           func()
}

// fakeColumnUI records what the editor asks of the shell.
type fakeColumnUI struct {
	modals   []modalCall
	selects  []selectCall
	confirms []confirmCall
	toasts   []string
	levels   []shell.ToastLevel
}

func (f *fakeColumnUI) Confirm(title, detail string, danger bool, onYes func()) {
	f.confirms = append(f.confirms, confirmCall{title, detail, danger, onYes})
}

// lastConfirm is the last confirmation asked, nil when none was.
func (f *fakeColumnUI) lastConfirm() *confirmCall {
	if len(f.confirms) == 0 {
		return nil
	}
	return &f.confirms[len(f.confirms)-1]
}

func (f *fakeColumnUI) ShowInputModal(title, current string, confirm func(string)) {
	f.modals = append(f.modals, modalCall{title, current, confirm})
}

func (f *fakeColumnUI) ShowSelectModal(title string, options []views.SelectOption, _ string, pick func(string)) {
	f.selects = append(f.selects, selectCall{title, options, pick})
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
	for _, want := range []string{"↑↓/jk move", "K move up", "J move down", "n add", "e rename", "d delete", "o role", "Ctrl+S save"} {
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
	c := e.ui.lastConfirm()
	require.NotNil(t, c, "d asks before it deletes")
	assert.Equal(t, i18n.Tf("a12.columns.confirm_column_title", "IN PROGRESS"), c.title)
	assert.Equal(t, i18n.Tf("a12.columns.confirm_column_detail_mappings", 2), c.detail)
	assert.True(t, c.danger, "a dangerous confirmation: the focus starts on Cancel")
	assert.Len(t, e.names(), 6, "nothing is deleted before the answer")
	assert.Empty(t, e.ui.toasts)
	c.yes()
	assert.Equal(t, []string{"todo", "review", "validation", "done", "blocked"}, e.names())
	assert.Equal(t, map[string]string{"later": "todo"}, *e.labels, "the labels of the column go with it")
	assert.Empty(t, *e.status)
	assert.Equal(t, i18n.Tf("tui.team.columns.column_deleted", "IN PROGRESS")+i18n.Tf("tui.team.columns.mappings_cleaned", 2), e.ui.lastToast())

	e.typ(columnEditorTestKeys.del) // REVIEW, no mapping
	c = e.ui.lastConfirm()
	assert.Equal(t, i18n.T("a12.columns.confirm_column_detail"), c.detail, "no mapping to mention")
	c.yes()
	assert.Equal(t, i18n.Tf("tui.team.columns.column_deleted", "REVIEW"), e.ui.lastToast())
}

// Answering « Annuler » (or closing the box) is not calling yes: the column
// and its mappings are still there.
func TestColumnEditorKeepsWhatIsNotConfirmed(t *testing.T) {
	e := newColumnEditor(t, defaultColumns(), map[string]string{"wip": "in_progress"}, map[string]string{}, "gitlab")
	e.typ('j')
	e.typ(columnEditorTestKeys.del)
	require.Len(t, e.ui.confirms, 1)
	assert.Len(t, e.names(), 6)
	assert.Equal(t, map[string]string{"wip": "in_progress"}, *e.labels)
	assert.Empty(t, e.ui.toasts)

	e.typ(columnEditorTestKeys.mappings)
	e.typ(columnEditorTestKeys.del)
	require.Len(t, e.ui.confirms, 2)
	assert.Len(t, *e.labels, 1, "the mapping stays too")
}

// A column that went away between the question and the answer is not
// removed twice, nor another one in its place.
func TestColumnEditorDeletesTheColumnAskedAbout(t *testing.T) {
	e := newColumnEditor(t, defaultColumns(), nil, nil, "")
	e.typ('j') // IN PROGRESS
	e.typ(columnEditorTestKeys.del)
	yes := e.ui.lastConfirm().yes
	e.typ('j') // the cursor moves on while the question is open
	yes()
	assert.Equal(t, []string{"todo", "review", "validation", "done", "blocked"}, e.names())
	yes()
	assert.Len(t, e.names(), 5, "asked twice: removed once")
}

func TestColumnEditorKeepsTwoColumns(t *testing.T) {
	cols := []teamstate.BoardColumnConfig{{ID: "a", Name: "A", Role: "initial"}, {ID: "b", Name: "B", Role: "terminal"}}
	e := newColumnEditor(t, cols, nil, nil, "")
	e.typ(columnEditorTestKeys.del)
	assert.Equal(t, []string{"a", "b"}, e.names())
	assert.Nil(t, e.ui.lastConfirm(), "nothing to confirm: the refusal comes first")
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
	assert.Equal(t, []string{"active", "review", "terminal", "blocked", "initial"}, roles)

	// a column with no role counts as active
	cols := []teamstate.BoardColumnConfig{{ID: "a", Name: "A"}, {ID: "b", Name: "B", Role: "terminal"}}
	e = newColumnEditor(t, cols, nil, nil, "")
	e.typ(columnEditorTestKeys.role)
	assert.Equal(t, "review", (*e.columns)[0].Role)
}

func TestColumnEditorMappings(t *testing.T) {
	labels := map[string]string{"wip": "in_progress", "doing": "in_progress", "later": "todo"}
	statuses := map[string]string{"In Dev": "in_progress"}
	e := newColumnEditor(t, defaultColumns(), labels, statuses, "jira")
	_, secondary := e.list.GetItemText(2)
	assert.Contains(t, secondary, "labels: doing, wip, In Dev", "the labels then the statuses of the column")
	assert.Contains(t, e.hintsText(), "m mappings")

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
	assert.Contains(t, e.hintsText(), "q "+i18n.T("tui.team.columns.hint_back_columns"))
	assert.Contains(t, e.hintsText(), "n "+i18n.T("tui.team.columns.hint_add_label"))
	assert.Contains(t, e.hintsText(), "e rename")
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

// The label and the status under the cursor of the mappings mode are renamed
// and deleted (the cursor mark in front of the name is not part of the name).
func TestColumnEditorRenamesAndDeletesAMapping(t *testing.T) {
	labels := map[string]string{"wip": "in_progress", "doing": "in_progress"}
	statuses := map[string]string{"In Dev": "in_progress"}
	e := newColumnEditor(t, defaultColumns(), labels, statuses, "jira")
	e.typ('j')
	e.typ(columnEditorTestKeys.mappings)

	// rename a label
	e.press(tcell.KeyEnter, 0)
	require.Len(t, e.ui.modals, 1)
	assert.Equal(t, i18n.T("tui.team.columns.rename_prompt"), e.ui.modals[0].title)
	assert.Equal(t, "doing", e.ui.modals[0].current)
	e.ui.modals[0].confirm("doing")
	e.ui.modals[0].confirm("")
	assert.Contains(t, *e.labels, "doing", "same name or empty: nothing changes")
	e.ui.modals[0].confirm(" stuck ")
	assert.Equal(t, "in_progress", (*e.labels)["stuck"])
	assert.NotContains(t, *e.labels, "doing")

	// rename a status
	e.typ('j')
	e.typ('j')
	e.press(tcell.KeyEnter, 0)
	require.Len(t, e.ui.modals, 2)
	assert.Equal(t, "In Dev", e.ui.modals[1].current)
	e.ui.modals[1].confirm("Coding")
	assert.Equal(t, map[string]string{"Coding": "in_progress"}, *e.status)

	// delete a label, then a status
	e.list.SetCurrentItem(2)
	e.typ(columnEditorTestKeys.del)
	c := e.ui.lastConfirm()
	require.NotNil(t, c, "d asks before it deletes a label")
	assert.Equal(t, i18n.Tf("a12.columns.confirm_label_title", "stuck"), c.title)
	assert.Equal(t, i18n.Tf("a12.columns.confirm_mapping_detail", "IN PROGRESS"), c.detail)
	assert.Len(t, *e.labels, 2, "nothing is deleted before the answer")
	c.yes()
	assert.Len(t, *e.labels, 1)
	assert.Equal(t, i18n.Tf("tui.team.columns.mapping_deleted", "stuck"), e.ui.lastToast())
	for i := 0; i < e.list.GetItemCount(); i++ {
		if strings.Contains(e.mainText(i), "Coding") {
			e.list.SetCurrentItem(i)
		}
	}
	e.typ(columnEditorTestKeys.del)
	c = e.ui.lastConfirm()
	assert.Equal(t, i18n.Tf("a12.columns.confirm_status_title", "Coding"), c.title)
	assert.Len(t, *e.status, 1)
	c.yes()
	assert.Empty(t, *e.status)
	assert.Equal(t, i18n.Tf("tui.team.columns.mapping_deleted", "Coding"), e.ui.lastToast())

	// on a header there is nothing to rename or delete
	e.list.SetCurrentItem(0)
	before, asked := len(e.ui.modals), len(e.ui.confirms)
	e.press(tcell.KeyEnter, 0)
	e.typ(columnEditorTestKeys.del)
	assert.Len(t, e.ui.modals, before)
	assert.Len(t, e.ui.confirms, asked)
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
	e.typ(columnEditorTestKeys.role) // active → review
	e.typ(columnEditorTestKeys.role) // review → terminal
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
	assert.Equal(t, theme.TextSecondaryHex, columnRoleHex(teamstate.ColumnRoleReview))
	assert.Equal(t, "red", columnColorName(teamstate.BoardColumnConfig{Color: "red"}, tcell.ColorRed))
	assert.Equal(t, "auto (#ff0000)", columnColorName(teamstate.BoardColumnConfig{}, tcell.NewRGBColor(255, 0, 0)))
}

// The keys of the editor are those of the convention (n add, e rename, d
// delete, Ctrl+S save) and o for the role: a and r, which it used before, do
// nothing, r being the refresh of the other views.
func TestColumnEditorKeysFollowTheConvention(t *testing.T) {
	e := newColumnEditor(t, defaultColumns(), map[string]string{"wip": "in_progress"}, map[string]string{}, "gitlab")
	role := (*e.columns)[0].Role
	e.typ('a')
	e.typ('r')
	assert.Empty(t, e.ui.modals, "a does not add any more")
	assert.Equal(t, role, (*e.columns)[0].Role, "r does not change the role any more")

	e.typ('e')
	require.Len(t, e.ui.modals, 1, "e renames, like Enter")
	assert.Equal(t, i18n.T("cmd.discovery.field.column_rename"), e.ui.modals[0].title)
	e.press(tcell.KeyEnter, 0)
	require.Len(t, e.ui.modals, 2)
	assert.Equal(t, e.ui.modals[0].title, e.ui.modals[1].title)

	// Alt+letter is not the letter
	e.list.InputHandler()(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModAlt), func(tview.Primitive) {})
	assert.Equal(t, role, (*e.columns)[0].Role)

	// the mappings mode: n adds a label, e renames, d deletes, q goes back
	e.typ('j')
	e.typ(columnEditorTestKeys.mappings)
	e.typ('a')
	assert.Len(t, e.ui.modals, 2, "a adds no label")
	e.typ('n')
	require.Len(t, e.ui.modals, 3)
	assert.Equal(t, i18n.T("tui.team.columns.label_name_prompt"), e.ui.modals[2].title)
	e.typ('e')
	assert.Len(t, e.ui.modals, 4, "e renames the label under the cursor")
	e.typ(columnEditorTestKeys.back)
	assert.Contains(t, e.mainText(0), "Columns")
}

// The empty list of a section tells which key adds to it.
func TestColumnEditorMappingsSayWhichKeyAdds(t *testing.T) {
	e := newColumnEditor(t, defaultColumns(), map[string]string{}, map[string]string{}, "")
	e.typ(columnEditorTestKeys.mappings)
	_, secondary := e.list.GetItemText(2)
	assert.Contains(t, tviewTags.ReplaceAllString(secondary, ""), "Press n to add")
}

// The words of the two confirmations, in both languages (golden).
func TestColumnEditorConfirmationTexts(t *testing.T) {
	var b strings.Builder
	for _, lang := range []string{"fr", "en"} {
		i18ntest.Set(t, lang)
		fmt.Fprintf(&b, "[%s]\n", lang)
		for _, c := range []struct {
			name string
			n    int
		}{{"IN PROGRESS", 2}, {"REVIEW", 0}} {
			title, detail := columnDeleteTexts(c.name, c.n)
			fmt.Fprintf(&b, "column %d: %s | %s\n", c.n, title, detail)
		}
		for _, kind := range []string{"label", "status"} {
			title, detail := mappingDeleteTexts(kind, "wip", "IN PROGRESS")
			fmt.Fprintf(&b, "%s: %s | %s\n", kind, title, detail)
		}
	}
	golden.Assert(t, "tui_team_columns_confirm", b.String())
}

// v moves the label or the status under the cursor to another column: the
// other columns are offered, the mapping leaves the list of the column being
// edited, the cursor stays in the list and the move is told.
func TestColumnEditorMovesAMappingToAnotherColumn(t *testing.T) {
	labels := map[string]string{"wip": "in_progress", "doing": "in_progress", "later": "todo"}
	statuses := map[string]string{"In Dev": "in_progress"}
	e := newColumnEditor(t, defaultColumns(), labels, statuses, "jira")
	e.typ('j')
	e.typ(columnEditorTestKeys.mappings)
	assert.Contains(t, e.hintsText(), "v "+i18n.T("tui.team.columns.hint_move_to"))

	// doing (first) goes to REVIEW
	assert.Contains(t, e.mainText(e.list.GetCurrentItem()), "doing")
	e.typ(columnEditorTestKeys.moveTo)
	require.Len(t, e.ui.selects, 1)
	call := e.ui.selects[0]
	assert.Equal(t, i18n.Tf("tui.team.columns.move_title", "doing"), call.title)
	var ids []string
	for _, o := range call.options {
		ids = append(ids, o.Value)
	}
	assert.Equal(t, []string{"todo", "review", "validation", "done", "blocked"}, ids, "every column but the one being edited")
	assert.Equal(t, "REVIEW", call.options[1].Label)
	assert.Equal(t, "in_progress", (*e.labels)["doing"], "nothing moves before the answer")

	call.pick("review")
	assert.Equal(t, "review", (*e.labels)["doing"])
	assert.Equal(t, "in_progress", (*e.labels)["wip"], "the others stay")
	assert.Equal(t, i18n.Tf("tui.team.columns.mapping_moved", "doing", "REVIEW"), e.ui.lastToast())
	var shown []string
	for i := 0; i < e.list.GetItemCount(); i++ {
		shown = append(shown, e.mainText(i))
	}
	assert.NotContains(t, strings.Join(shown, "|"), "doing", "it left the column being edited")
	assert.Contains(t, shown[e.list.GetCurrentItem()], "wip", "the cursor is on an item of the list, not on a header")

	// the status of a Jira board goes the same way
	for i := range shown {
		if strings.Contains(shown[i], "In Dev") {
			e.list.SetCurrentItem(i)
		}
	}
	e.typ(columnEditorTestKeys.moveTo)
	require.Len(t, e.ui.selects, 2)
	e.ui.selects[1].pick("blocked")
	assert.Equal(t, map[string]string{"In Dev": "blocked"}, *e.status)
	assert.Contains(t, shown[e.list.GetCurrentItem()], "wip")

	// the new column shows it
	e.typ(columnEditorTestKeys.back)
	_, secondary := e.list.GetItemText(3)
	assert.Contains(t, secondary, "labels: doing", "REVIEW now holds doing")
}

// Nothing to move on a header; an answer that is not another column of the
// editor changes nothing; with a single column there is nowhere to go.
func TestColumnEditorMoveKeepsWhatItCannotMove(t *testing.T) {
	labels := map[string]string{"wip": "in_progress"}
	e := newColumnEditor(t, defaultColumns(), labels, map[string]string{}, "gitlab")
	e.typ('j')
	e.typ(columnEditorTestKeys.mappings)

	e.list.SetCurrentItem(0)
	e.typ(columnEditorTestKeys.moveTo)
	assert.Empty(t, e.ui.selects, "a header is not a label")

	e.list.SetCurrentItem(2)
	e.typ(columnEditorTestKeys.moveTo)
	require.Len(t, e.ui.selects, 1)
	for _, answer := range []string{"", "in_progress", "nowhere"} {
		e.ui.selects[0].pick(answer)
		assert.Equal(t, map[string]string{"wip": "in_progress"}, *e.labels, "answer %q", answer)
	}

	// the column keys do not move anything in the columns mode
	e.typ(columnEditorTestKeys.back)
	e.typ(columnEditorTestKeys.moveTo)
	assert.Len(t, e.ui.selects, 1)

	one := newColumnEditor(t, []teamstate.BoardColumnConfig{{ID: "only", Name: "ONLY", Role: teamstate.ColumnRoleTerminal}},
		map[string]string{"wip": "only"}, map[string]string{}, "gitlab")
	one.typ(columnEditorTestKeys.mappings)
	assert.NotContains(t, one.hintsText(), "v "+i18n.T("tui.team.columns.hint_move_to"), "nowhere to move to: no hint")
	one.typ(columnEditorTestKeys.moveTo)
	assert.Empty(t, one.ui.selects)
	assert.Equal(t, i18n.T("tui.team.columns.move_no_other"), one.ui.lastToast())
}

// The words of the move, and the keys line of the mappings, in both languages
// (golden).
func TestColumnEditorMoveTexts(t *testing.T) {
	var b strings.Builder
	for _, lang := range []string{"fr", "en"} {
		e := newColumnEditor(t, defaultColumns(), map[string]string{"wip": "in_progress"}, map[string]string{}, "gitlab")
		i18ntest.Set(t, lang)
		e.typ('j')
		e.typ(columnEditorTestKeys.mappings)
		e.typ(columnEditorTestKeys.moveTo)
		fmt.Fprintf(&b, "[%s]\n", lang)
		fmt.Fprintf(&b, "keys: %s\n", tviewTags.ReplaceAllString(e.hintsText(), ""))
		fmt.Fprintf(&b, "title: %s\n", e.ui.selects[0].title)
		e.ui.selects[0].pick("review")
		fmt.Fprintf(&b, "toast: %s\n", e.ui.lastToast())
		fmt.Fprintf(&b, "no other column: %s\n", i18n.T("tui.team.columns.move_no_other"))
	}
	golden.Assert(t, "tui_team_columns_move", b.String())
}
