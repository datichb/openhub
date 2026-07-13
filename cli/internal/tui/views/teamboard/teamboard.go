// Package teamboard provides a full-screen team Kanban board TUI view.
// It displays team members' claims in columns by status (IDLE, IN PROGRESS, REVIEW, BLOCKED)
// with navigation, detail toggle, and refresh support.
package teamboard

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/datichb/openhub/cli/internal/tui/common"
)

// TeamTicket represents a team member's claim for the board.
type TeamTicket struct {
	Member   string    // Display name
	TicketID string    // e.g. "bd-42"
	Project  string    // e.g. "T-SRU"
	Status   string    // idle | in_progress | review | blocked
	Since    time.Time // ClaimedAt or LastActivity
	SubBeads []SubBead // Optional sub-tickets
}

// SubBead represents a sub-ticket under a claim.
type SubBead struct {
	ID     string
	Title  string
	Status string // completed | in_progress | pending
}

// Config configures the team board view.
type Config struct {
	Title       string
	Tickets     []TeamTicket
	RefreshFunc func() []TeamTicket // called on refresh for live update
}

// Column definitions for team board.
var teamColumns = []columnDef{
	{name: "IDLE", status: "idle", color: lipgloss.Color("241")},
	{name: "IN PROGRESS", status: "in_progress", color: lipgloss.Color("33")},
	{name: "REVIEW", status: "review", color: lipgloss.Color("214")},
	{name: "BLOCKED", status: "blocked", color: lipgloss.Color("196")},
}

type columnDef struct {
	name   string
	status string
	color  lipgloss.Color
}

const (
	minWidth  = 80
	minHeight = 16
)

// Model is the BubbleTea model for the team board.
type Model struct {
	config  Config
	tickets []TeamTicket
	width   int
	height  int
	cursor  int  // active column (0-3)
	row     int  // active row within column
	detail  bool // show detail view
	done    bool
}

type tickMsg struct{}

// Init starts the model.
func (m Model) Init() tea.Cmd {
	return tea.EnterAltScreen
}

// Update handles messages.
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

		// Column navigation
		case "h", "left":
			if m.cursor > 0 {
				m.cursor--
				m.row = 0
			}
		case "l", "right":
			if m.cursor < len(teamColumns)-1 {
				m.cursor++
				m.row = 0
			}

		// Row navigation within column
		case "j", "down":
			colTickets := m.ticketsInColumn(m.cursor)
			if m.row < len(colTickets)-1 {
				m.row++
			}
		case "k", "up":
			if m.row > 0 {
				m.row--
			}

		// Detail toggle
		case "d":
			m.detail = !m.detail

		// Refresh
		case "r":
			if m.config.RefreshFunc != nil {
				m.tickets = m.config.RefreshFunc()
			}

		// Tab cycles columns
		case "tab":
			m.cursor = (m.cursor + 1) % len(teamColumns)
			m.row = 0
		}
	}
	return m, nil
}

// View renders the board.
func (m Model) View() string {
	if m.done {
		return ""
	}
	if m.width < minWidth || m.height < minHeight {
		return "\n  Terminal too small. Need at least 80x16.\n"
	}

	var b strings.Builder

	// Title bar
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("255")).
		Background(lipgloss.Color("99")).
		Padding(0, 1)
	b.WriteString(titleStyle.Render(m.config.Title))
	b.WriteString("\n\n")

	// Calculate column width
	colWidth := (m.width - 2) / len(teamColumns)
	if colWidth < 18 {
		colWidth = 18
	}

	// Column headers
	var headers []string
	for i, col := range teamColumns {
		count := len(m.ticketsInColumn(i))
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
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(strings.Repeat("─", m.width-2))
	b.WriteString(sep)
	b.WriteString("\n")

	// Column contents
	availHeight := m.height - 7 // title + headers + separator + footer
	var cols []string
	for i := range teamColumns {
		colContent := m.renderColumn(i, colWidth, availHeight)
		cols = append(cols, colContent)
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, cols...))

	// Footer
	b.WriteString("\n")
	footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	detailHint := "d:detail"
	if m.detail {
		detailHint = "d:simple"
	}
	footer := fmt.Sprintf(" h/l:columns · j/k:items · %s · r:refresh · q:quit", detailHint)
	b.WriteString(footerStyle.Render(footer))

	return b.String()
}

func (m Model) renderColumn(colIdx, width, maxHeight int) string {
	tickets := m.ticketsInColumn(colIdx)
	col := teamColumns[colIdx]

	var lines []string
	for i, t := range tickets {
		if len(lines) >= maxHeight {
			lines = append(lines, fmt.Sprintf("  +%d more...", len(tickets)-i))
			break
		}
		card := m.renderCard(t, colIdx, i, width-2, col.color)
		lines = append(lines, card)
	}

	if len(tickets) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true).
			Width(width).
			Align(lipgloss.Center)
		lines = append(lines, emptyStyle.Render("(empty)"))
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().Width(width).Render(content)
}

func (m Model) renderCard(t TeamTicket, colIdx, rowIdx, width int, color lipgloss.Color) string {
	isActive := colIdx == m.cursor && rowIdx == m.row

	borderColor := lipgloss.Color("241")
	if isActive {
		borderColor = color
	}

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(width - 2).
		Padding(0, 1)

	var content strings.Builder

	// Member name
	memberStyle := lipgloss.NewStyle().Bold(true)
	content.WriteString(memberStyle.Render(t.Member))
	content.WriteString("\n")

	// Ticket info (or idle indicator)
	if t.Status == "idle" {
		idleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)
		content.WriteString(idleStyle.Render("(no ticket)"))
	} else {
		ticketStr := fmt.Sprintf("%s/%s", t.Project, t.TicketID)
		content.WriteString(ticketStr)
		content.WriteString("\n")

		// Duration since
		since := formatDuration(time.Since(t.Since))
		sinceStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		content.WriteString(sinceStyle.Render(since))
	}

	// Detail mode: show sub-beads
	if m.detail && len(t.SubBeads) > 0 {
		content.WriteString("\n")
		completed := 0
		for i, sb := range t.SubBeads {
			icon := common.IconDot
			switch sb.Status {
			case "completed":
				icon = common.IconSuccess
				completed++
			case "in_progress":
				icon = common.IconInfo
			}
			prefix := "├"
			if i == len(t.SubBeads)-1 {
				prefix = "└"
			}
			line := fmt.Sprintf(" %s %s %s", prefix, icon, truncate(sb.ID, 10))
			content.WriteString(line)
			if i < len(t.SubBeads)-1 {
				content.WriteString("\n")
			}
		}
		content.WriteString("\n")
		progressStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		content.WriteString(progressStyle.Render(fmt.Sprintf(" Progress: %d/%d", completed, len(t.SubBeads))))
	}

	return cardStyle.Render(content.String())
}

func (m Model) ticketsInColumn(colIdx int) []TeamTicket {
	status := teamColumns[colIdx].status
	var result []TeamTicket
	for _, t := range m.tickets {
		if t.Status == status {
			result = append(result, t)
		}
	}
	return result
}

// Run launches the team board as a full-screen TUI program.
func Run(cfg Config) error {
	m := Model{
		config:  cfg,
		tickets: cfg.Tickets,
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

// --- Helpers ---

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if hours >= 24 {
		days := hours / 24
		return fmt.Sprintf("%dd%dh", days, hours%24)
	}
	return fmt.Sprintf("%dh%02dm", hours, mins)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
