package views

import (
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/datichb/openhub/cli/internal/beads"
)

// Epic filter values of the Beads picker besides an epic ID.
const (
	beadsEpicAll  = ""
	beadsEpicNone = "\x00none" // tickets without epic
)

// beadsPickerRow is a line of the picker: an epic header or a ticket.
type beadsPickerRow struct {
	epicID string // header: epic ID ("" = no epic); ticket: its group
	header bool
	ticket *beads.Ticket
}

// beadsPickerModel holds the state of the picker independently of tview.
type beadsPickerModel struct {
	tickets []beads.Ticket          // pickable tickets (no epic, no closed)
	epics   map[string]beads.Ticket // epics by ID

	query    string
	labels   []string // label filter options ("" = all labels)
	labelIdx int
	epicOpts []string // beadsEpicAll, epic IDs…, beadsEpicNone
	epicIdx  int

	rows     []beadsPickerRow
	cursor   int      // index in rows of the current ticket, -1 when none
	selected []string // selection order
}

func newBeadsPickerModel(all []beads.Ticket, filter, epic string, preselected []string) *beadsPickerModel {
	m := &beadsPickerModel{epics: map[string]beads.Ticket{}}
	labelSet := map[string]bool{}
	for _, t := range all {
		if t.Type == "epic" {
			m.epics[t.ID] = t
			continue
		}
		if !beads.IsPickableStatus(t.Status) {
			continue
		}
		m.tickets = append(m.tickets, t)
		for _, l := range t.Labels {
			labelSet[l] = true
		}
	}

	// Label filter: the configured one first, then all, then the others.
	m.labels = []string{filter}
	if filter != "" {
		m.labels = append(m.labels, "")
	}
	others := make([]string, 0, len(labelSet))
	for l := range labelSet {
		if !strings.EqualFold(l, filter) {
			others = append(others, l)
		}
	}
	sort.Strings(others)
	m.labels = append(m.labels, others...)

	// Epic filter: all, each epic having pickable tickets, no epic.
	m.epicOpts = []string{beadsEpicAll}
	hasOrphan := false
	withTickets := map[string]bool{}
	for _, t := range m.tickets {
		if _, ok := m.epics[t.Parent]; ok {
			withTickets[t.Parent] = true
		} else {
			hasOrphan = true
		}
	}
	ids := make([]string, 0, len(withTickets))
	for id := range withTickets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	m.epicOpts = append(m.epicOpts, ids...)
	if hasOrphan {
		m.epicOpts = append(m.epicOpts, beadsEpicNone)
	}
	if epic != "" {
		if i := slices.Index(m.epicOpts, epic); i >= 0 {
			m.epicIdx = i
		} else {
			m.epicOpts = append(m.epicOpts, epic) // requested epic without pickable ticket: empty list
			m.epicIdx = len(m.epicOpts) - 1
		}
	}

	known := map[string]bool{}
	for _, t := range m.tickets {
		known[t.ID] = true
	}
	for _, id := range preselected {
		if known[id] && !slices.Contains(m.selected, id) {
			m.selected = append(m.selected, id)
		}
	}
	m.rebuild()
	return m
}

func (m *beadsPickerModel) labelFilter() string { return m.labels[m.labelIdx] }
func (m *beadsPickerModel) epicFilter() string  { return m.epicOpts[m.epicIdx] }

func (m *beadsPickerModel) groupOf(t beads.Ticket) string {
	if _, ok := m.epics[t.Parent]; ok {
		return t.Parent
	}
	return ""
}

func (m *beadsPickerModel) visible(t beads.Ticket) bool {
	if l := m.labelFilter(); l != "" && !beads.HasLabelExported(t, l) {
		return false
	}
	switch e := m.epicFilter(); e {
	case beadsEpicAll:
	case beadsEpicNone:
		if m.groupOf(t) != "" {
			return false
		}
	default:
		if m.groupOf(t) != e {
			return false
		}
	}
	if q := strings.ToLower(strings.TrimSpace(m.query)); q != "" {
		hay := strings.ToLower(t.ID + " " + t.Title + " " + strings.Join(t.Labels, " "))
		for _, word := range strings.Fields(q) {
			if !strings.Contains(hay, word) {
				return false
			}
		}
	}
	return true
}

// rebuild recomputes the rows, keeping the cursor on the same ticket when
// it is still visible.
func (m *beadsPickerModel) rebuild() {
	current := ""
	if t := m.current(); t != nil {
		current = t.ID
	}
	groups := map[string][]beads.Ticket{}
	for _, t := range m.tickets {
		if m.visible(t) {
			g := m.groupOf(t)
			groups[g] = append(groups[g], t)
		}
	}
	order := make([]string, 0, len(groups))
	for g := range groups {
		if g != "" {
			order = append(order, g)
		}
	}
	sort.Strings(order)
	if _, ok := groups[""]; ok {
		order = append(order, "")
	}

	m.rows = m.rows[:0]
	m.cursor = -1
	for _, g := range order {
		ts := groups[g]
		sort.SliceStable(ts, func(i, j int) bool {
			pi, pj := priorityRank(ts[i].Priority), priorityRank(ts[j].Priority)
			if pi != pj {
				return pi < pj
			}
			return ts[i].ID < ts[j].ID
		})
		m.rows = append(m.rows, beadsPickerRow{epicID: g, header: true})
		for i := range ts {
			t := ts[i]
			m.rows = append(m.rows, beadsPickerRow{epicID: g, ticket: &t})
			if t.ID == current {
				m.cursor = len(m.rows) - 1
			}
		}
	}
	if m.cursor < 0 {
		m.move(1)
	}
}

// priorityRank orders P0 < P1 < … < unknown.
func priorityRank(p string) int {
	if n, err := strconv.Atoi(strings.TrimPrefix(strings.ToUpper(p), "P")); err == nil {
		return n
	}
	return 99
}

func (m *beadsPickerModel) current() *beads.Ticket {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	return m.rows[m.cursor].ticket
}

// move moves the cursor by delta tickets (headers are skipped); with no
// current ticket it goes to the first one.
func (m *beadsPickerModel) move(delta int) {
	idx := make([]int, 0, len(m.rows))
	pos := -1
	for i, r := range m.rows {
		if r.ticket != nil {
			if i == m.cursor {
				pos = len(idx)
			}
			idx = append(idx, i)
		}
	}
	if len(idx) == 0 {
		m.cursor = -1
		return
	}
	if pos < 0 {
		m.cursor = idx[0]
		return
	}
	pos = min(max(pos+delta, 0), len(idx)-1)
	m.cursor = idx[pos]
}

func (m *beadsPickerModel) moveToEdge(last bool) {
	m.cursor = -1
	m.move(1)
	if last {
		m.move(len(m.rows))
	}
}

func (m *beadsPickerModel) isSelected(id string) bool { return slices.Contains(m.selected, id) }

func (m *beadsPickerModel) toggle() {
	t := m.current()
	if t == nil {
		return
	}
	if i := slices.Index(m.selected, t.ID); i >= 0 {
		m.selected = slices.Delete(m.selected, i, i+1)
	} else {
		m.selected = append(m.selected, t.ID)
	}
}

// result returns the chosen tickets: the selection in multi mode (the
// current ticket when nothing is selected), the current ticket otherwise.
func (m *beadsPickerModel) result(multi bool) []string {
	if multi && len(m.selected) > 0 {
		return append([]string(nil), m.selected...)
	}
	if t := m.current(); t != nil {
		return []string{t.ID}
	}
	return nil
}

func (m *beadsPickerModel) cycleLabel() {
	m.labelIdx = (m.labelIdx + 1) % len(m.labels)
	m.rebuild()
}

func (m *beadsPickerModel) cycleEpic() {
	m.epicIdx = (m.epicIdx + 1) % len(m.epicOpts)
	m.rebuild()
}

func (m *beadsPickerModel) setQuery(q string) {
	m.query = q
	m.rebuild()
}

// ticketCount returns the number of visible tickets.
func (m *beadsPickerModel) ticketCount() int {
	n := 0
	for _, r := range m.rows {
		if r.ticket != nil {
			n++
		}
	}
	return n
}
