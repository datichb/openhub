// Package parallel provides a full-screen TUI for monitoring parallel opencode sessions.
// It displays session status in columns, supports attach/detach to individual sessions,
// and shows conflict detection in real time.
package parallel

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/datichb/openhub/cli/internal/parallel"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

// Config configures the parallel TUI.
type Config struct {
	Title       string
	State       *parallel.ParallelState
	Servers     []*parallel.OpenCodeServer
	RefreshFunc func()          // called to refresh state (poll servers)
	AttachFunc  func(port int) error // called to attach to a session
}

// Model is the BubbleTea model for the parallel monitor.
type Model struct {
	config    Config
	cursor    int  // selected session index
	width     int
	height    int
	done      bool
	attached  bool
	lastPoll  time.Time
}

// Messages
type pollTickMsg struct{}
type attachCompleteMsg struct{}

func pollTick() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return pollTickMsg{}
	})
}

// Init starts the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(tea.EnterAltScreen, pollTick())
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case pollTickMsg:
		if m.config.RefreshFunc != nil {
			m.config.RefreshFunc()
		}
		m.lastPoll = time.Now()

		// Check if all done
		if m.config.State.AllCompleted() {
			m.done = true
			return m, tea.Quit
		}
		return m, pollTick()

	case attachCompleteMsg:
		m.attached = false
		return m, tea.EnterAltScreen

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.done = true
			return m, tea.Quit

		case "j", "down":
			snap := m.config.State.Snapshot()
			if m.cursor < len(snap.Sessions)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}

		case "enter":
			// Attach to selected session
			snap := m.config.State.Snapshot()
			if m.cursor < len(snap.Sessions) {
				sess := snap.Sessions[m.cursor]
				if sess.Status == parallel.StatusRunning && m.config.AttachFunc != nil {
					m.attached = true
					port := sess.Port
					return m, tea.Sequence(
						tea.ExitAltScreen,
						func() tea.Msg {
							_ = m.config.AttachFunc(port)
							return attachCompleteMsg{}
						},
					)
				}
			}

		case "r":
			if m.config.RefreshFunc != nil {
				m.config.RefreshFunc()
			}
			m.lastPoll = time.Now()
		}
	}
	return m, nil
}

// View renders the parallel monitor.
func (m Model) View() string {
	if m.done {
		return ""
	}
	if m.attached {
		return "\n  Attached to session. Return here when you quit opencode.\n"
	}
	if m.width < 60 || m.height < 12 {
		return "\n  Terminal too small. Need at least 60x12.\n"
	}

	var b strings.Builder

	// Title bar
	snap := m.config.State.Snapshot()
	running := 0
	completed := 0
	failed := 0
	for _, s := range snap.Sessions {
		switch s.Status {
		case parallel.StatusRunning, parallel.StatusStarting:
			running++
		case parallel.StatusCompleted:
			completed++
		case parallel.StatusFailed:
			failed++
		}
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("255")).
		Background(lipgloss.Color("99")).
		Padding(0, 1)
	statusStr := fmt.Sprintf("%d running · %d done · %d failed", running, completed, failed)
	b.WriteString(titleStyle.Render(m.config.Title))
	b.WriteString("  ")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(statusStr))
	b.WriteString("\n\n")

	// Session cards
	cardWidth := m.width - 4
	if cardWidth > 80 {
		cardWidth = 80
	}

	for i, sess := range snap.Sessions {
		isActive := i == m.cursor
		card := m.renderSessionCard(sess, isActive, cardWidth)
		b.WriteString(card)
		b.WriteString("\n")
	}

	// Conflicts section
	if len(snap.Conflicts) > 0 {
		b.WriteString("\n")
		conflictStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
		b.WriteString(conflictStyle.Render(fmt.Sprintf("  %s %d potential conflict(s):", common.IconWarning, len(snap.Conflicts))))
		b.WriteString("\n")
		for _, c := range snap.Conflicts {
			b.WriteString(fmt.Sprintf("    %s — %s [%s]\n",
				c.File, strings.Join(c.Sessions, " ↔ "), c.Severity))
		}
	}

	// Footer
	b.WriteString("\n")
	footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	footer := " j/k:navigate · enter:attach · r:refresh · q:quit"
	b.WriteString(footerStyle.Render(footer))

	return b.String()
}

func (m Model) renderSessionCard(sess parallel.SessionInfo, active bool, width int) string {
	borderColor := lipgloss.Color("241")
	if active {
		borderColor = lipgloss.Color("99")
	}

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(width).
		Padding(0, 1).
		MarginLeft(2)

	var content strings.Builder

	// Line 1: ticket + status icon
	statusIcon, statusColor := sessionStatusDisplay(sess.Status)
	statusStyle := lipgloss.NewStyle().Foreground(statusColor)

	ticketStr := fmt.Sprintf("%s/%s", sess.Project, sess.TicketID)
	if sess.Priority {
		ticketStr += " ★"
	}
	content.WriteString(fmt.Sprintf("%s  %s  %s",
		lipgloss.NewStyle().Bold(true).Render(ticketStr),
		statusStyle.Render(statusIcon),
		statusStyle.Render(string(sess.Status))))

	// Line 2: branch + duration
	content.WriteString("\n")
	branchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	content.WriteString(branchStyle.Render(sess.Branch))

	if !sess.StartedAt.IsZero() {
		elapsed := time.Since(sess.StartedAt)
		if !sess.CompletedAt.IsZero() {
			elapsed = sess.CompletedAt.Sub(sess.StartedAt)
		}
		content.WriteString(fmt.Sprintf("  %s", formatDuration(elapsed)))
	}

	// Line 3: files (if any)
	if len(sess.FilesModified) > 0 || len(sess.FilesCreated) > 0 {
		content.WriteString("\n")
		fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		totalFiles := len(sess.FilesModified) + len(sess.FilesCreated)
		content.WriteString(fileStyle.Render(fmt.Sprintf("%d file(s) touched", totalFiles)))
		// Show first 2 files
		shown := 0
		for _, f := range sess.FilesModified {
			if shown >= 2 {
				content.WriteString(fileStyle.Render(fmt.Sprintf("  +%d more", totalFiles-shown)))
				break
			}
			content.WriteString("\n")
			content.WriteString(fileStyle.Render(fmt.Sprintf("  %s %s", common.IconDot, truncatePath(f, width-10))))
			shown++
		}
	}

	// Error line
	if sess.Error != "" {
		content.WriteString("\n")
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		content.WriteString(errStyle.Render(sess.Error))
	}

	return cardStyle.Render(content.String())
}

func sessionStatusDisplay(status parallel.SessionStatus) (string, lipgloss.Color) {
	switch status {
	case parallel.StatusPending:
		return "○", lipgloss.Color("241")
	case parallel.StatusStarting:
		return "◐", lipgloss.Color("33")
	case parallel.StatusRunning:
		return "●", lipgloss.Color("33")
	case parallel.StatusCompleted:
		return "✓", lipgloss.Color("82")
	case parallel.StatusFailed:
		return "✗", lipgloss.Color("196")
	case parallel.StatusAborted:
		return "⊘", lipgloss.Color("214")
	default:
		return "?", lipgloss.Color("241")
	}
}

func formatDuration(d time.Duration) string {
	d = d.Truncate(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}

func truncatePath(path string, max int) string {
	if len(path) <= max {
		return path
	}
	// Show .../<last two segments>
	parts := strings.Split(path, "/")
	if len(parts) <= 2 {
		return path[:max-3] + "..."
	}
	short := ".../" + strings.Join(parts[len(parts)-2:], "/")
	if len(short) > max {
		return short[:max-3] + "..."
	}
	return short
}

// Run launches the parallel monitor TUI.
func Run(cfg Config) error {
	m := Model{
		config: cfg,
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
