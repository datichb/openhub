// Package dashboard provides a full-screen multi-panel dashboard TUI view.
package dashboard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/datichb/openhub/cli/internal/tui/common"
)

// Panel represents a dashboard panel with content.
type Panel struct {
	Title   string
	Content string
	Width   float64 // fraction of total width (0.0-1.0)
}

// Stats holds dashboard statistics.
type Stats struct {
	TotalProjects   int
	ActiveProjects  int
	TotalSessions   int
	TodaySessions   int
	TokensUsed      int64
	TokensSaved     int64
	TopProject      string
}

// Config configures the dashboard view.
type Config struct {
	Title string
	Stats Stats
}

// Model is the Bubbletea model for the dashboard.
type Model struct {
	config Config
	width  int
	height int
	done   bool
}

// New creates a new dashboard model.
func New(cfg Config) Model {
	return Model{
		config: cfg,
		width:  120,
		height: 30,
	}
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
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.done = true
			return m, tea.Quit
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
	s := m.config.Stats

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(common.Primary).
		Background(lipgloss.Color("235")).
		Padding(0, 2).
		Width(m.width)
	b.WriteString(titleStyle.Render(m.config.Title))
	b.WriteString("\n\n")

	// Row 1: Key metrics
	halfWidth := (m.width - 6) / 2
	if halfWidth < 30 {
		halfWidth = 30
	}

	projectsPanel := m.renderPanel("Projets", halfWidth, []string{
		fmt.Sprintf("Total:  %d", s.TotalProjects),
		fmt.Sprintf("Actifs: %d", s.ActiveProjects),
		fmt.Sprintf("Top:    %s", s.TopProject),
	})

	sessionsPanel := m.renderPanel("Sessions", halfWidth, []string{
		fmt.Sprintf("Total:      %d", s.TotalSessions),
		fmt.Sprintf("Aujourd'hui: %d", s.TodaySessions),
	})

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, projectsPanel, "  ", sessionsPanel))
	b.WriteString("\n\n")

	// Row 2: Token usage
	fullWidth := m.width - 4
	tokensPanel := m.renderPanel("Tokens", fullWidth, []string{
		fmt.Sprintf("Utilisés: %s", formatTokens(s.TokensUsed)),
		fmt.Sprintf("Économisés (RTK): %s", formatTokens(s.TokensSaved)),
		fmt.Sprintf("Ratio économie: %s", formatRatio(s.TokensSaved, s.TokensUsed)),
		"",
		renderBar(s.TokensSaved, s.TokensUsed+s.TokensSaved, fullWidth-8),
	})
	b.WriteString(tokensPanel)

	// Footer
	b.WriteString("\n\n")
	footer := lipgloss.NewStyle().Foreground(common.Subtle)
	b.WriteString(footer.Render("  q quitter"))

	return b.String()
}

func (m Model) renderPanel(title string, width int, lines []string) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(common.Primary)

	content := titleStyle.Render(title) + "\n"
	for _, line := range lines {
		content += "  " + line + "\n"
	}

	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(common.Subtle).
		Width(width).
		Padding(1, 2)

	return panelStyle.Render(content)
}

func formatTokens(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func formatRatio(saved, used int64) string {
	if used == 0 {
		return "—"
	}
	total := saved + used
	pct := float64(saved) / float64(total) * 100
	return fmt.Sprintf("%.1f%%", pct)
}

func renderBar(value, total int64, width int) string {
	if total == 0 || width <= 0 {
		return ""
	}
	filled := int(float64(value) / float64(total) * float64(width))
	if filled > width {
		filled = width
	}

	bar := lipgloss.NewStyle().Foreground(common.Success).Render(strings.Repeat("█", filled))
	empty := lipgloss.NewStyle().Foreground(common.Subtle).Render(strings.Repeat("░", width-filled))
	return bar + empty
}

// Run launches the dashboard as a standalone program.
func Run(cfg Config) error {
	model := New(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
