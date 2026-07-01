// Package board provides a full-screen Kanban board TUI view.
// It displays tickets in columns (todo, in_progress, done, blocked)
// with live refresh support.
package board

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/datichb/openhub/cli/internal/tui/common"
)

// Ticket represents a kanban ticket.
type Ticket struct {
	ID       string
	Title    string
	Status   string
	Priority string
	Type     string
}

// Config configures the board view.
type Config struct {
	Title        string
	Tickets      []Ticket
	RefreshFunc  func() []Ticket // called on tick for live update
	RefreshRate  time.Duration
}

// Column definitions.
var columns = []columnDef{
	{name: "TODO", status: "todo", color: lipgloss.Color("214")},
	{name: "IN PROGRESS", status: "in_progress", color: lipgloss.Color("33")},
	{name: "DONE", status: "done", color: lipgloss.Color("82")},
	{name: "BLOCKED", status: "blocked", color: lipgloss.Color("196")},
}

type columnDef struct {
	name   string
	status string
	color  lipgloss.Color
}

// Model is the Bubbletea model for the board.
type Model struct {
	config    Config
	tickets   []Ticket
	width     int
	height    int
	cursor    int // active column
	done      bool
	lastTick  time.Time
}

type tickMsg time.Time

// New creates a new board model.
func New(cfg Config) Model {
	if cfg.RefreshRate == 0 {
		cfg.RefreshRate = 5 * time.Second
	}
	return Model{
		config:   cfg,
		tickets:  cfg.Tickets,
		width:    120,
		height:   30,
		lastTick: time.Now(),
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.tickCmd(),
	)
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(m.config.RefreshRate, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.done = true
			return m, tea.Quit
		case "left", "h":
			if m.cursor > 0 {
				m.cursor--
			}
		case "right", "l":
			if m.cursor < len(columns)-1 {
				m.cursor++
			}
		case "r":
			// Manual refresh
			if m.config.RefreshFunc != nil {
				m.tickets = m.config.RefreshFunc()
			}
		}
		return m, nil

	case tickMsg:
		if m.config.RefreshFunc != nil {
			m.tickets = m.config.RefreshFunc()
		}
		m.lastTick = time.Time(msg)
		return m, m.tickCmd()
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if m.done {
		return ""
	}

	var b strings.Builder

	// Title bar
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(common.Primary).
		Background(lipgloss.Color("235")).
		Padding(0, 2).
		Width(m.width)
	b.WriteString(titleStyle.Render(m.config.Title))
	b.WriteString("\n\n")

	// Calculate column width
	colWidth := (m.width - 4) / len(columns)
	if colWidth < 20 {
		colWidth = 20
	}

	// Render columns header
	var headers []string
	for i, col := range columns {
		count := m.countByStatus(col.status)
		headerText := fmt.Sprintf(" %s (%d) ", col.name, count)

		style := lipgloss.NewStyle().
			Bold(true).
			Foreground(col.color).
			Width(colWidth).
			Align(lipgloss.Center)
		if i == m.cursor {
			style = style.Underline(true)
		}
		headers = append(headers, style.Render(headerText))
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, headers...))
	b.WriteString("\n")

	// Separator
	sep := lipgloss.NewStyle().Foreground(common.Subtle)
	b.WriteString(sep.Render(strings.Repeat("─", m.width-2)))
	b.WriteString("\n")

	// Render ticket cards per column
	maxRows := m.height - 8 // reserve space for header, footer
	if maxRows < 5 {
		maxRows = 5
	}

	var colViews []string
	for _, col := range columns {
		tickets := m.ticketsByStatus(col.status)
		colView := renderColumn(tickets, colWidth-2, maxRows, col.color)
		colViews = append(colViews, colView)
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, colViews...))

	// Footer
	b.WriteString("\n\n")
	footerStyle := lipgloss.NewStyle().Foreground(common.Subtle)
	b.WriteString(footerStyle.Render(
		fmt.Sprintf("  ←/→ colonnes • r rafraîchir • q quitter — dernière MAJ: %s — %d ticket(s) total",
			m.lastTick.Format("15:04:05"), len(m.tickets))))

	return b.String()
}

func renderColumn(tickets []Ticket, width, maxRows int, color lipgloss.Color) string {
	var cards []string
	for i, t := range tickets {
		if i >= maxRows {
			remaining := len(tickets) - maxRows
			more := lipgloss.NewStyle().Foreground(common.Subtle).Width(width).Render(
				fmt.Sprintf("  +%d autres...", remaining))
			cards = append(cards, more)
			break
		}
		cards = append(cards, renderCard(t, width, color))
	}

	if len(cards) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(common.Subtle).
			Italic(true).
			Width(width).
			Align(lipgloss.Center).
			Render("—")
		cards = append(cards, empty)
	}

	col := lipgloss.NewStyle().Width(width + 2).Padding(0, 1)
	return col.Render(strings.Join(cards, "\n"))
}

func renderCard(t Ticket, width int, color lipgloss.Color) string {
	// Priority badge
	priBadge := ""
	switch t.Priority {
	case "critical", "high":
		priBadge = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("●")
	case "medium":
		priBadge = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("●")
	case "low":
		priBadge = lipgloss.NewStyle().Foreground(common.Subtle).Render("●")
	}

	// Title (truncate if needed)
	title := t.Title
	maxTitle := width - 6
	if len(title) > maxTitle && maxTitle > 0 {
		title = title[:maxTitle-1] + "…"
	}

	// Card style
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Width(width).
		Padding(0, 1)

	content := fmt.Sprintf("%s %s\n%s",
		priBadge, title,
		lipgloss.NewStyle().Foreground(common.Subtle).Render(t.ID))

	return cardStyle.Render(content)
}

func (m Model) countByStatus(status string) int {
	count := 0
	for _, t := range m.tickets {
		if t.Status == status {
			count++
		}
	}
	return count
}

func (m Model) ticketsByStatus(status string) []Ticket {
	var result []Ticket
	for _, t := range m.tickets {
		if t.Status == status {
			result = append(result, t)
		}
	}
	return result
}

// Run launches the board as a standalone program.
func Run(cfg Config) error {
	model := New(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
