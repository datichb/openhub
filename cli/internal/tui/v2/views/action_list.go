package views

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

// ActionList is the shared « list with actions per element » (modernisation
// tui-keys): wiki proposals of a checkpoint card and of the project queue,
// batch validation, children of a fan-out, bricks. Elements on one line
// each (columns aligned, optional sections), their state (pending,
// accepted, refused…), a detail pane below or beside the list (e.g. the
// before/after of a proposal), actions on the selected element bound to
// the keys of the convention (y/n/e/s, d, x, R…) and global actions (A
// accept all). Refusing asks an instruction, deleting a confirmation. No
// limit on the number of elements: the list scrolls and shows how many are
// below. The list calls back the view, which calls its service (off the
// event loop) then updates the list (SetState, SetItems).
//
// Keys reach the list through its table (InputCapture); a view that owns
// the keys of its screen may also forward them with HandleKey.
type ActionList struct {
	cfg     ActionListConfig
	root    *tview.Flex
	listBox *tview.Flex
	table   *actionTable
	footer  *tview.TextView
	detail  *tview.TextView

	items   []ActionItem
	rowItem []int    // table row → item index (-1: header, section, empty)
	rows    []string // row texts without the selection marker
	widths  []int
	marked  int // row with the marker (-1: none)
}

// ItemState is the state of an element. The list knows the usual ones
// (labels, colours); a view may use its own with StateLabels or StateText.
type ItemState string

const (
	ItemNone     ItemState = ""
	ItemPending  ItemState = "pending"
	ItemAccepted ItemState = "accepted"
	ItemRefused  ItemState = "refused"
)

// ActionItem is an element of the list.
type ActionItem struct {
	// ID identifies the element (selection kept by ID across SetItems).
	ID string
	// Section groups consecutive elements under a title (optional).
	Section string
	// Cells are the columns (plain text, escaped by the list).
	Cells []string
	State ItemState
	// StateText replaces the label of the state (plain text).
	StateText string
	// DetailTitle is the title of the detail pane (default: ID).
	DetailTitle string
	// Detail is the text of the detail pane, with tview tags: escape the
	// texts that come from agents or files (tview.Escape).
	Detail string
	// Ref is data of the view.
	Ref any
}

// ItemAction is an action on the selected element.
type ItemAction struct {
	// Key is the action of the convention (keys.Accept, keys.Refuse,
	// keys.Delete…).
	Key keys.Action
	// Label replaces the label of the key in the hints (translated, same
	// meaning: « accepter », « récupérer »).
	Label string
	// Applies tells whether the action applies to the element (nil: always).
	Applies func(ActionItem) bool
	// Confirm gives the text of the confirmation; it is asked when set, and
	// always for an action of the convention that requires one (d delete).
	Confirm func(ActionItem) (title, detail string)
	// Danger styles the confirmation as destructive (default for d).
	Danger bool
	// AskReason asks an instruction before Run (always for n refuse: the
	// instruction validated is the confirmation). ReasonOptional accepts an
	// empty one.
	AskReason      bool
	ReasonOptional bool
	// ReasonTitle is the title of the instruction prompt (optional).
	ReasonTitle func(ActionItem) string
	// Run does the action (event loop); reason is the instruction, if any.
	Run func(it ActionItem, reason string)
}

// GlobalAction is an action on the whole list (A accept all).
type GlobalAction struct {
	Key   keys.Action
	Label string
	// Confirm gives the text of the confirmation (optional).
	Confirm func(items []ActionItem) (title, detail string)
	Danger  bool
	Run     func(items []ActionItem)
}

// ActionListLayout places the detail pane.
type ActionListLayout int

const (
	// DetailBelow puts the detail under the list (default).
	DetailBelow ActionListLayout = iota
	// DetailRight puts the detail beside the list.
	DetailRight
	// DetailNone hides the detail pane.
	DetailNone
)

// ActionListConfig configures an ActionList.
type ActionListConfig struct {
	// Title is the title of the list frame (« cp-wiki · 9 propositions »).
	Title string
	// Headers are the titles of the columns (optional, fixed row).
	Headers []string
	// StateHeader is the title of the state column (with Headers).
	StateHeader string
	Layout      ActionListLayout
	Actions     []ItemAction
	Global      []GlobalAction
	// StateLabels replace the labels of states (plain text, e.g. feminine
	// forms « acceptée »), or name the states of the view.
	StateLabels map[ItemState]string
	// Empty is the text shown without elements.
	Empty string
	// OnOpen runs on Enter (optional); OpenLabel is its hint label.
	OnOpen    func(ActionItem)
	OpenLabel string
	// OnChange runs when the selection moves (optional).
	OnChange func(ActionItem)

	// Shell provides the confirmation, the instruction prompt and the
	// toasts; Confirm, AskReason and Toast override them (an overlay that
	// shows its own windows, tests).
	Shell     ShellAccess
	Confirm   func(title, detail string, danger bool, onYes func())
	AskReason func(title string, onDone func(reason string))
	Toast     func(msg string, ok bool)
}

// Validate reports a configuration that breaks the key convention: two
// actions on the same key, or an action on a key every view keeps (moves,
// / filter, r refresh, ? help, Enter, Esc).
func (c ActionListConfig) Validate() error {
	seen := map[keys.Binding]string{}
	reserved := map[keys.Binding]string{}
	for _, a := range keys.ByContext(keys.Everywhere) {
		for _, b := range a.Bindings {
			reserved[b] = a.ID
		}
	}
	check := func(a keys.Action) error {
		for _, b := range a.Bindings {
			if id, ok := reserved[b]; ok {
				return fmt.Errorf("action %s on the key of %s", a.ID, id)
			}
			if id, ok := seen[b]; ok {
				return fmt.Errorf("actions %s and %s on the same key %q", id, a.ID, a.Keys())
			}
			seen[b] = a.ID
		}
		return nil
	}
	for _, a := range c.Actions {
		if a.Run == nil {
			return fmt.Errorf("action %s without Run", a.Key.ID)
		}
		if err := check(a.Key); err != nil {
			return err
		}
	}
	for _, g := range c.Global {
		if g.Run == nil {
			return fmt.Errorf("global action %s without Run", g.Key.ID)
		}
		if err := check(g.Key); err != nil {
			return err
		}
	}
	return nil
}

// actionTable draws the table, then the footer (position, elements below).
type actionTable struct {
	*tview.Table
	after func()
}

func (t *actionTable) Draw(screen tcell.Screen) {
	t.Table.Draw(screen)
	if t.after != nil {
		t.after()
	}
}

// NewActionList builds the list (empty: SetItems fills it).
func NewActionList(cfg ActionListConfig) *ActionList {
	l := &ActionList{cfg: cfg, marked: -1}
	l.table = &actionTable{Table: tview.NewTable()}
	l.table.SetSelectable(true, false).
		SetSelectedStyle(tcell.StyleDefault.Background(theme.BgElement).Foreground(theme.FgPrimary)).
		SetBackgroundColor(theme.BgPanel)
	l.table.SetSelectionChangedFunc(func(row, _ int) { l.selectionChanged(row) })
	l.table.SetInputCapture(l.HandleKey)
	l.table.after = l.updateFooter

	l.footer = tview.NewTextView().SetDynamicColors(true)
	l.footer.SetBackgroundColor(theme.BgPanel)

	l.listBox = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(l.table, 0, 1, true).
		AddItem(l.footer, 1, 0, false)
	l.listBox.SetBackgroundColor(theme.BgPanel)
	l.listBox.SetBorder(true).SetTitleAlign(tview.AlignLeft).SetBorderColor(theme.ActiveMode.Primary)
	l.listBox.SetBorderPadding(0, 0, 1, 1)
	l.SetTitle(cfg.Title)

	l.detail = tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	l.detail.SetBackgroundColor(theme.BgPanel)
	l.detail.SetBorder(true).SetTitleAlign(tview.AlignLeft).SetBorderColor(theme.FgMuted)
	l.detail.SetBorderPadding(0, 0, 1, 1)

	switch cfg.Layout {
	case DetailRight:
		l.root = tview.NewFlex().AddItem(l.listBox, 0, 3, true).AddItem(l.detail, 0, 2, false)
	case DetailNone:
		l.root = tview.NewFlex().AddItem(l.listBox, 0, 1, true)
	default:
		l.root = tview.NewFlex().SetDirection(tview.FlexRow).AddItem(l.listBox, 0, 3, true).AddItem(l.detail, 0, 2, false)
	}
	l.root.SetBackgroundColor(theme.BgPanel)
	l.render("")
	return l
}

// Primitive is the root of the list (to add to the content of a view or
// to a window).
func (l *ActionList) Primitive() tview.Primitive { return l.root }

// Focusable is the primitive to focus (the table).
func (l *ActionList) Focusable() tview.Primitive { return l.table }

// SetTitle sets the title of the list frame.
func (l *ActionList) SetTitle(title string) {
	if title != "" {
		title = " " + title + " "
	}
	l.listBox.SetTitle(title)
}

// SetItems replaces the elements; the selected element stays selected
// when it is still there.
func (l *ActionList) SetItems(items []ActionItem) {
	keep := ""
	if it, ok := l.Current(); ok {
		keep = it.ID
	}
	l.items = append([]ActionItem(nil), items...)
	l.render(keep)
}

// Items returns the elements.
func (l *ActionList) Items() []ActionItem { return append([]ActionItem(nil), l.items...) }

// Len is the number of elements.
func (l *ActionList) Len() int { return len(l.items) }

// Update changes an element in place (e.g. its state once the service
// recorded the decision); false when the ID is unknown.
func (l *ActionList) Update(id string, fn func(*ActionItem)) bool {
	for i := range l.items {
		if l.items[i].ID == id {
			fn(&l.items[i])
			l.render(l.currentID())
			return true
		}
	}
	return false
}

// SetState sets the state of an element.
func (l *ActionList) SetState(id string, s ItemState) bool {
	return l.Update(id, func(it *ActionItem) { it.State, it.StateText = s, "" })
}

// Current is the selected element.
func (l *ActionList) Current() (ActionItem, bool) {
	row, _ := l.table.GetSelection()
	if row < 0 || row >= len(l.rowItem) || l.rowItem[row] < 0 {
		return ActionItem{}, false
	}
	return l.items[l.rowItem[row]], true
}

func (l *ActionList) currentID() string {
	if it, ok := l.Current(); ok {
		return it.ID
	}
	return ""
}

// Select selects the element of that ID.
func (l *ActionList) Select(id string) bool {
	for row, i := range l.rowItem {
		if i >= 0 && l.items[i].ID == id {
			l.table.Select(row, 0)
			return true
		}
	}
	return false
}

// Hints is the hints line of the list: its actions, its global actions,
// then Enter.
func (l *ActionList) Hints() string {
	var parts []string
	for _, a := range l.cfg.Actions {
		parts = append(parts, hintOf(a.Key, a.Label))
	}
	for _, g := range l.cfg.Global {
		parts = append(parts, hintOf(g.Key, g.Label))
	}
	if l.cfg.OnOpen != nil {
		parts = append(parts, hintOf(keys.Open, l.cfg.OpenLabel))
	}
	return keys.Hints(parts...)
}

func hintOf(a keys.Action, label string) string {
	if label != "" {
		return a.HintAs(label)
	}
	return a.Hint()
}

// HandleKey runs the action of the key on the selected element; other keys
// (moves) are returned.
func (l *ActionList) HandleKey(ev *tcell.EventKey) *tcell.EventKey {
	for _, a := range l.cfg.Actions {
		if !a.Key.Matches(ev) {
			continue
		}
		it, ok := l.Current()
		if !ok {
			return nil
		}
		if a.Applies != nil && !a.Applies(it) {
			l.toast(i18n.Tf("tui.actionlist.not_applicable", hintOf(a.Key, a.Label), it.ID), false)
			return nil
		}
		l.runItem(a, it)
		return nil
	}
	for _, g := range l.cfg.Global {
		if g.Key.Matches(ev) {
			l.runGlobal(g)
			return nil
		}
	}
	if keys.Open.Matches(ev) && l.cfg.OnOpen != nil {
		if it, ok := l.Current(); ok {
			l.cfg.OnOpen(it)
		}
		return nil
	}
	return ev
}

func (l *ActionList) runItem(a ItemAction, it ActionItem) {
	label := a.Label
	if label == "" {
		label = a.Key.Label()
	}
	label = upperFirst(label)
	if a.AskReason || a.Key.Reason {
		title := i18n.Tf("tui.actionlist.reason_title", label, it.ID)
		if a.ReasonTitle != nil {
			title = a.ReasonTitle(it)
		}
		l.askReason(title, func(reason string) {
			reason = strings.TrimSpace(reason)
			if reason == "" && !a.ReasonOptional {
				l.toast(i18n.T("tui.actionlist.reason_required"), false)
				return
			}
			a.Run(it, reason)
		})
		return
	}
	if a.Confirm == nil && !a.Key.Confirm {
		a.Run(it, "")
		return
	}
	title, detail := i18n.Tf("tui.actionlist.confirm_title", label, it.ID), ""
	if a.Confirm != nil {
		title, detail = a.Confirm(it)
	}
	l.confirm(title, detail, a.Danger || a.Key.ID == keys.Delete.ID, func() { a.Run(it, "") })
}

func (l *ActionList) runGlobal(g GlobalAction) {
	items := l.Items()
	if g.Confirm == nil {
		g.Run(items)
		return
	}
	title, detail := g.Confirm(items)
	l.confirm(title, detail, g.Danger, func() { g.Run(items) })
}

func (l *ActionList) confirm(title, detail string, danger bool, onYes func()) {
	switch {
	case l.cfg.Confirm != nil:
		l.cfg.Confirm(title, detail, danger, onYes)
	case l.cfg.Shell != nil:
		l.cfg.Shell.Confirm(title, detail, danger, onYes)
	}
	// Without a way to confirm, nothing is done.
}

func (l *ActionList) askReason(title string, onDone func(string)) {
	switch {
	case l.cfg.AskReason != nil:
		l.cfg.AskReason(title, onDone)
	case l.cfg.Shell != nil:
		l.cfg.Shell.ShowInputModal(title, "", onDone)
	}
}

func (l *ActionList) toast(msg string, ok bool) {
	switch {
	case l.cfg.Toast != nil:
		l.cfg.Toast(msg, ok)
	case l.cfg.Shell != nil:
		l.cfg.Shell.ShowToastMsg(msg, ok)
	}
}

func upperFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}

// ── Rendering ───────────────────────────────────────────────────────────────

// stateText is the label of the state of an element (plain text).
func (l *ActionList) stateText(it ActionItem) string {
	if it.StateText != "" {
		return it.StateText
	}
	if s, ok := l.cfg.StateLabels[it.State]; ok {
		return s
	}
	switch it.State {
	case ItemPending:
		return "? " + i18n.T("tui.actionlist.state.pending")
	case ItemAccepted:
		return "✓ " + i18n.T("tui.actionlist.state.accepted")
	case ItemRefused:
		return "✗ " + i18n.T("tui.actionlist.state.refused")
	}
	return string(it.State)
}

func stateColor(s ItemState) string {
	switch s {
	case ItemAccepted:
		return theme.SuccessHex
	case ItemRefused:
		return theme.ErrorHex
	case ItemPending:
		return theme.WarningHex
	}
	return theme.TextSecondaryHex
}

// columns are the cells of an element, its state last.
func (l *ActionList) columns(it ActionItem) []string {
	cols := append([]string(nil), it.Cells...)
	if st := l.stateText(it); st != "" || l.cfg.StateHeader != "" {
		cols = append(cols, st)
	}
	return cols
}

func (l *ActionList) computeWidths() {
	l.widths = nil
	grow := func(cols []string) {
		for i, c := range cols {
			if i >= len(l.widths) {
				l.widths = append(l.widths, 0)
			}
			l.widths[i] = max(l.widths[i], runewidth.StringWidth(c))
		}
	}
	if len(l.cfg.Headers) > 0 {
		grow(l.headerColumns())
	}
	for _, it := range l.items {
		grow(l.columns(it))
	}
}

func (l *ActionList) headerColumns() []string {
	cols := append([]string(nil), l.cfg.Headers...)
	if l.cfg.StateHeader != "" {
		cols = append(cols, l.cfg.StateHeader)
	}
	return cols
}

// line aligns the columns; state is the state of the element (coloured
// last column), nil for the header.
func (l *ActionList) line(cols []string, state *ItemState) string {
	var b strings.Builder
	for i, c := range cols {
		text := tview.Escape(c)
		last := i == len(cols)-1
		if last && state != nil {
			text = theme.ColorTag(stateColor(*state)) + text + theme.TagColor
		}
		b.WriteString(text)
		if !last {
			b.WriteString(strings.Repeat(" ", l.widths[i]-runewidth.StringWidth(c)+2))
		}
	}
	return b.String()
}

func (l *ActionList) render(keepID string) {
	t := l.table
	t.Clear()
	l.rowItem, l.rows, l.marked = nil, nil, -1
	l.computeWidths()
	add := func(text string, item int, selectable bool, color string) {
		row := len(l.rowItem)
		cell := tview.NewTableCell(text).SetSelectable(selectable).SetExpansion(1)
		if color != "" {
			cell.SetTextColor(tcell.GetColor(color))
		}
		t.SetCell(row, 0, cell)
		l.rowItem = append(l.rowItem, item)
		l.rows = append(l.rows, text)
	}
	fixed := 0
	if len(l.cfg.Headers) > 0 {
		add("  "+l.line(l.headerColumns(), nil), -1, false, theme.TextMutedHex)
		fixed = 1
	}
	t.SetFixed(fixed, 0)
	if len(l.items) == 0 {
		add("  "+tview.Escape(l.cfg.Empty), -1, false, theme.TextMutedHex)
		l.detail.SetTitle("")
		l.detail.SetText("")
		return
	}
	section := ""
	for i, it := range l.items {
		if it.Section != "" && it.Section != section {
			add(theme.ColorTag(theme.ActiveMode.PrimaryHex)+tview.Escape(it.Section)+theme.TagColor, -1, false, "")
		}
		section = it.Section
		st := it.State
		add("  "+l.line(l.columns(it), &st), i, true, "")
	}
	row := l.firstRow(keepID)
	t.Select(row, 0)
	t.ScrollToBeginning()
	if row >= 0 {
		l.selectionChanged(row)
	}
}

// firstRow is the row of keepID, else the first element.
func (l *ActionList) firstRow(keepID string) int {
	first := -1
	for row, i := range l.rowItem {
		if i < 0 {
			continue
		}
		if first < 0 {
			first = row
		}
		if keepID != "" && l.items[i].ID == keepID {
			return row
		}
	}
	return first
}

// marker puts ▸ before the selected row.
func (l *ActionList) setMarker(row int) {
	if l.marked >= 0 && l.marked < len(l.rows) {
		l.table.GetCell(l.marked, 0).SetText(l.rows[l.marked])
	}
	l.marked = -1
	if row >= 0 && row < len(l.rows) && l.rowItem[row] >= 0 {
		l.table.GetCell(row, 0).SetText("▸" + strings.TrimPrefix(l.rows[row], " "))
		l.marked = row
	}
}

func (l *ActionList) selectionChanged(row int) {
	l.setMarker(row)
	it, ok := l.Current()
	if !ok {
		return
	}
	title := it.DetailTitle
	if title == "" {
		title = it.ID
	}
	l.detail.SetTitle(" " + tview.Escape(title) + " ")
	l.detail.SetText(it.Detail)
	l.detail.ScrollToBeginning()
	if l.cfg.OnChange != nil {
		l.cfg.OnChange(it)
	}
}

// updateFooter shows the position of the selection and how many elements
// are below the visible rows.
func (l *ActionList) updateFooter() {
	if len(l.items) == 0 {
		l.footer.SetText("")
		return
	}
	pos := 0
	row, _ := l.table.GetSelection()
	for r := 0; r <= row && r < len(l.rowItem); r++ {
		if l.rowItem[r] >= 0 {
			pos++
		}
	}
	text := i18n.Tf("tui.actionlist.position", pos, len(l.items))
	_, _, _, h := l.table.GetInnerRect()
	offset, _ := l.table.GetOffset()
	fixed := 0
	if len(l.cfg.Headers) > 0 {
		fixed = 1
	}
	lastVisible := offset + h - 1 // rows after the fixed ones start at offset+fixed
	below := 0
	for r := max(lastVisible+1, fixed); r < len(l.rowItem); r++ {
		if l.rowItem[r] >= 0 {
			below++
		}
	}
	if below > 0 {
		text += "  ·  " + i18n.Tf("tui.actionlist.more", below)
	}
	l.footer.SetText(theme.ColorTag(theme.TextMutedHex) + text + theme.TagColor)
}
