// Package picker provides a full-screen interactive picker component.
// It supports single and multi-select, keyboard navigation, fuzzy filtering,
// categories, and alternate screen rendering.
package picker

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/datichb/openhub/cli/internal/tui/common"
)

// Item represents a selectable item in the picker.
type Item struct {
	ID          string
	Label       string
	Description string
	Category    string
	Selected    bool
}

// Config configures the picker behavior.
type Config struct {
	Title       string
	Items       []Item
	MultiSelect bool // Allow multiple selections
	PageSize    int  // Items visible at once (default 10)
}

// Result is the output of the picker after user confirmation.
type Result struct {
	Selected []Item
	Aborted  bool
}

// Model is the Bubbletea model for the picker.
type Model struct {
	config    Config
	items     []Item // filtered items
	allItems  []Item // original items
	cursor    int    // current cursor position
	filter    string // search filter text
	filtering bool   // whether we're in filter mode
	offset    int    // scroll offset for viewport
	width     int
	height    int
	result    Result
	done      bool
}

// New creates a new picker Model.
func New(cfg Config) Model {
	if cfg.PageSize == 0 {
		cfg.PageSize = 10
	}
	items := make([]Item, len(cfg.Items))
	copy(items, cfg.Items)

	return Model{
		config:   cfg,
		items:    items,
		allItems: cfg.Items,
		width:    80,
		height:   24,
	}
}

// Result returns the picker result after the program exits.
func (m Model) Result() Result {
	return m.result
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.EnterAltScreen
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.filtering {
			return m.updateFilter(msg)
		}
		return m.updateNavigation(msg)
	}
	return m, nil
}

func (m Model) updateNavigation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.result = Result{Aborted: true}
		m.done = true
		return m, tea.Quit

	case "enter":
		m.result = m.buildResult()
		m.done = true
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.ensureVisible()
		}

	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
			m.ensureVisible()
		}

	case "home", "g":
		m.cursor = 0
		m.offset = 0

	case "end", "G":
		m.cursor = len(m.items) - 1
		m.ensureVisible()

	case "pgup":
		m.cursor -= m.config.PageSize
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.ensureVisible()

	case "pgdown":
		m.cursor += m.config.PageSize
		if m.cursor >= len(m.items) {
			m.cursor = len(m.items) - 1
		}
		m.ensureVisible()

	case " ":
		if m.config.MultiSelect && len(m.items) > 0 {
			m.items[m.cursor].Selected = !m.items[m.cursor].Selected
			// Sync back to allItems
			m.syncSelection(m.items[m.cursor])
		}

	case "*":
		if m.config.MultiSelect {
			// Toggle all
			allSelected := m.allSelected()
			for i := range m.items {
				m.items[i].Selected = !allSelected
				m.syncSelection(m.items[i])
			}
		}

	case "/":
		m.filtering = true
		m.filter = ""
	}
	return m, nil
}

func (m Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filter = ""
		m.items = m.applyFilter("")
		m.cursor = 0
		m.offset = 0
		return m, nil

	case "enter":
		m.filtering = false
		return m, nil

	case "backspace":
		if m.filter != "" {
			m.filter = m.filter[:len(m.filter)-1]
			m.items = m.applyFilter(m.filter)
			m.cursor = 0
			m.offset = 0
		}

	default:
		if len(msg.String()) == 1 {
			m.filter += msg.String()
			m.items = m.applyFilter(m.filter)
			m.cursor = 0
			m.offset = 0
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if m.done {
		return ""
	}

	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(common.Primary).
		MarginBottom(1)
	b.WriteString(titleStyle.Render(m.config.Title))
	b.WriteString("\n\n")

	// Filter bar
	if m.filtering {
		filterStyle := lipgloss.NewStyle().
			Foreground(common.Highlight).
			Bold(true)
		b.WriteString(filterStyle.Render("/ " + m.filter + "█"))
		b.WriteString("\n\n")
	} else if m.filter != "" {
		dimFilter := lipgloss.NewStyle().Foreground(common.Subtle)
		b.WriteString(dimFilter.Render("filtre: " + m.filter))
		b.WriteString("\n\n")
	}

	// Items
	pageSize := m.config.PageSize
	if pageSize > len(m.items) {
		pageSize = len(m.items)
	}

	end := m.offset + pageSize
	if end > len(m.items) {
		end = len(m.items)
	}

	if len(m.items) == 0 {
		noResults := lipgloss.NewStyle().Foreground(common.Subtle).Italic(true)
		b.WriteString(noResults.Render("  Aucun résultat"))
		b.WriteString("\n")
	}

	var lastCategory string
	for i := m.offset; i < end; i++ {
		item := m.items[i]

		// Category separator
		if item.Category != "" && item.Category != lastCategory {
			catStyle := lipgloss.NewStyle().Foreground(common.Subtle).Bold(true).MarginTop(1)
			b.WriteString(catStyle.Render("  " + item.Category))
			b.WriteString("\n")
			lastCategory = item.Category
		}

		// Cursor
		cursor := "  "
		if i == m.cursor {
			cursor = common.SuccessStyle.Render(common.IconArrow + " ")
		}

		// Checkbox (multi-select)
		checkbox := ""
		if m.config.MultiSelect {
			if item.Selected {
				checkbox = common.SuccessStyle.Render("[x] ")
			} else {
				checkbox = lipgloss.NewStyle().Foreground(common.Subtle).Render("[ ] ")
			}
		}

		// Label
		label := item.Label
		if i == m.cursor {
			label = common.Bold.Render(label)
		}

		b.WriteString(cursor + checkbox + label)

		// Description
		if item.Description != "" && i == m.cursor {
			desc := lipgloss.NewStyle().Foreground(common.Subtle).Italic(true)
			b.WriteString(" " + desc.Render(item.Description))
		}
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(m.items) > pageSize {
		b.WriteString("\n")
		scrollInfo := lipgloss.NewStyle().Foreground(common.Subtle)
		b.WriteString(scrollInfo.Render(
			strings.Repeat(" ", 2) +
				"[" + strings.Repeat("·", m.offset) +
				strings.Repeat("█", pageSize) +
				strings.Repeat("·", len(m.items)-end) + "]"))
		b.WriteString("\n")
	}

	// Help
	b.WriteString("\n")
	helpStyle := lipgloss.NewStyle().Foreground(common.Subtle)
	if m.config.MultiSelect {
		b.WriteString(helpStyle.Render("  ↑/↓ naviguer • espace sélectionner • * tout • / filtrer • enter confirmer • esc annuler"))
	} else {
		b.WriteString(helpStyle.Render("  ↑/↓ naviguer • / filtrer • enter sélectionner • esc annuler"))
	}

	return b.String()
}

// --- Helpers ---

func (m *Model) ensureVisible() {
	pageSize := m.config.PageSize
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+pageSize {
		m.offset = m.cursor - pageSize + 1
	}
}

func (m Model) applyFilter(filter string) []Item {
	if filter == "" {
		items := make([]Item, len(m.allItems))
		copy(items, m.allItems)
		return items
	}
	lower := strings.ToLower(filter)
	var filtered []Item
	for _, item := range m.allItems {
		if strings.Contains(strings.ToLower(item.Label), lower) ||
			strings.Contains(strings.ToLower(item.Description), lower) ||
			strings.Contains(strings.ToLower(item.Category), lower) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (m Model) buildResult() Result {
	if m.config.MultiSelect {
		var selected []Item
		for _, item := range m.allItems {
			if item.Selected {
				selected = append(selected, item)
			}
		}
		return Result{Selected: selected}
	}
	// Single select: current item under cursor
	if len(m.items) > 0 && m.cursor < len(m.items) {
		return Result{Selected: []Item{m.items[m.cursor]}}
	}
	return Result{Aborted: true}
}

func (m *Model) syncSelection(item Item) {
	for i := range m.allItems {
		if m.allItems[i].ID == item.ID {
			m.allItems[i].Selected = item.Selected
			break
		}
	}
}

func (m Model) allSelected() bool {
	for _, item := range m.items {
		if !item.Selected {
			return false
		}
	}
	return true
}

// Run launches the picker as a standalone Bubbletea program.
// This is the simplest API for callers.
func Run(cfg Config) (Result, error) {
	model := New(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return Result{Aborted: true}, err
	}
	return finalModel.(Model).Result(), nil
}
