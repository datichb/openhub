// Package board provides a full-screen Kanban board TUI view.
// It displays tickets in columns (todo, in_progress, done, blocked)
// with live refresh support, mouse navigation, and vertical scrolling.
package board

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/datichb/openhub/cli/internal/i18n"
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
	Title       string
	Tickets     []Ticket
	RefreshFunc func() []Ticket // called on tick for live update
	RefreshRate time.Duration
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

const (
	minWidth  = 80
	minHeight = 20
)

// Model is the Bubbletea model for the board.
type Model struct {
	config   Config
	tickets  []Ticket
	width    int
	height   int
	cursor   int   // active column
	offsets  []int // scroll offset per column
	done     bool
	lastTick time.Time
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
		offsets:  make([]int, len(columns)),
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
		case "down", "j":
			maxOffset := m.maxOffsetForColumn(m.cursor)
			if m.offsets[m.cursor] < maxOffset {
				m.offsets[m.cursor]++
			}
		case "up", "k":
			if m.offsets[m.cursor] > 0 {
				m.offsets[m.cursor]--
			}
		case "r":
			// Manual refresh
			if m.config.RefreshFunc != nil {
				m.tickets = m.config.RefreshFunc()
			}
		}
		return m, nil

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if m.offsets[m.cursor] > 0 {
				m.offsets[m.cursor]--
			}
		case tea.MouseButtonWheelDown:
			maxOffset := m.maxOffsetForColumn(m.cursor)
			if m.offsets[m.cursor] < maxOffset {
				m.offsets[m.cursor]++
			}
		case tea.MouseButtonLeft:
			// Click on column header area → switch column
			colWidth := m.colWidth()
			if colWidth > 0 {
				clickedCol := msg.X / colWidth
				if clickedCol >= 0 && clickedCol < len(columns) {
					m.cursor = clickedCol
				}
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

	// Guard: terminal too small
	if m.width < minWidth || m.height < minHeight {
		msg := i18n.Tf("tui.terminal_too_small", m.width, m.height, minWidth, minHeight)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg)
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
	colWidth := m.colWidth()

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
	maxRows := m.maxRows()

	var colViews []string
	for i, col := range columns {
		tickets := m.ticketsByStatus(col.status)
		offset := m.offsets[i]

		// Clamp offset
		if offset > len(tickets) {
			offset = len(tickets)
		}

		colView := m.renderColumn(tickets, offset, colWidth-2, maxRows, col.color, i == m.cursor)
		colViews = append(colViews, colView)
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, colViews...))

	// Footer
	b.WriteString("\n\n")
	footerStyle := lipgloss.NewStyle().Foreground(common.Subtle)
	b.WriteString(footerStyle.Render(
		fmt.Sprintf("  %s", i18n.Tf("tui.board.footer", m.lastTick.Format("15:04:05"), len(m.tickets)))))

	return b.String()
}

func (m Model) renderColumn(tickets []Ticket, offset, width, maxRows int, color lipgloss.Color, active bool) string {
	var cards []string

	// Scroll-up indicator
	if offset > 0 {
		indicator := lipgloss.NewStyle().Foreground(common.Subtle).Width(width).Align(lipgloss.Center)
		cards = append(cards, indicator.Render("▲"))
	}

	// Visible tickets
	end := offset + maxRows
	if end > len(tickets) {
		end = len(tickets)
	}

	for i := offset; i < end; i++ {
		cards = append(cards, renderCard(tickets[i], width, color))
	}

	// Scroll-down indicator or overflow count
	if end < len(tickets) {
		remaining := len(tickets) - end
		indicator := lipgloss.NewStyle().Foreground(common.Subtle).Width(width).Align(lipgloss.Center)
		cards = append(cards, indicator.Render(
			fmt.Sprintf("▼ %s", i18n.Tf("tui.board.overflow", remaining))))
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

	col := lipgloss.NewStyle().Width(width+2).Padding(0, 1)
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

func (m Model) colWidth() int {
	w := (m.width - 4) / len(columns)
	if w < 20 {
		w = 20
	}
	return w
}

func (m Model) maxRows() int {
	r := m.height - 8
	if r < 5 {
		r = 5
	}
	return r
}

func (m Model) maxOffsetForColumn(col int) int {
	if col < 0 || col >= len(columns) {
		return 0
	}
	tickets := m.ticketsByStatus(columns[col].status)
	maxRows := m.maxRows()
	maxOff := len(tickets) - maxRows
	if maxOff < 0 {
		return 0
	}
	return maxOff
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
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
