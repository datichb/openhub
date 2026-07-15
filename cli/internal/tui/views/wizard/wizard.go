// Package wizard provides a reusable BubbleTea model for multi-step wizards
// with a side-by-side layout: sidebar (prerequisites + step list) on the left,
// and an embedded huh form on the right.
package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/datichb/openhub/cli/internal/tui/common"
)

// ─────────────────────────────────────────────────────────────────────────────
// Messages
// ─────────────────────────────────────────────────────────────────────────────

// StepDoneMsg is sent when a step's OnDone callback completes.
type StepDoneMsg struct {
	Err error
}

// stepDoneCmd wraps an OnDone callback into a tea.Cmd.
func stepDoneCmd(fn func() error) tea.Cmd {
	return func() tea.Msg {
		var err error
		if fn != nil {
			err = fn()
		}
		return StepDoneMsg{Err: err}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Step configuration
// ─────────────────────────────────────────────────────────────────────────────

// StepConfig defines a single wizard step.
type StepConfig struct {
	Label  string     // Display name in the sidebar
	Form   *huh.Form  // The huh form for this step (nil if skip)
	Skip   bool       // If true, step is pre-skipped (already satisfied)
	OnDone func() error // Callback executed when form completes (commit, write, etc.)
}

// ─────────────────────────────────────────────────────────────────────────────
// Model
// ─────────────────────────────────────────────────────────────────────────────

// Model is the BubbleTea model for a multi-step wizard with sidebar.
type Model struct {
	title   string
	prereqs []string
	steps   []StepConfig
	status  []common.StepStatus

	current int
	width   int
	height  int
	err     error
	done    bool
	aborted bool
}

// New creates a wizard model from the given configuration.
// Steps marked with Skip=true will appear as StepDone in the sidebar.
func New(title string, prereqs []string, steps []StepConfig) Model {
	statuses := make([]common.StepStatus, len(steps))
	firstActive := -1

	for i, s := range steps {
		if s.Skip {
			statuses[i] = common.StepDone
		} else {
			statuses[i] = common.StepPending
			if firstActive == -1 {
				firstActive = i
				statuses[i] = common.StepActive
			}
		}
	}

	current := firstActive
	if current == -1 {
		current = len(steps) // all skipped → done immediately
	}

	return Model{
		title:   title,
		prereqs: prereqs,
		steps:   steps,
		status:  statuses,
		current: current,
	}
}

// Done returns true when the wizard finished (all steps done/skipped).
func (m Model) Done() bool {
	return m.done
}

// Aborted returns true if the user cancelled (esc/ctrl+c).
func (m Model) Aborted() bool {
	return m.aborted
}

// Err returns any error that occurred during step execution.
func (m Model) Err() error {
	return m.err
}

// ─────────────────────────────────────────────────────────────────────────────
// BubbleTea interface
// ─────────────────────────────────────────────────────────────────────────────

// Init initializes the wizard, starting the first active step's form.
func (m Model) Init() tea.Cmd {
	// If all steps are skipped, quit immediately
	if m.current >= len(m.steps) {
		m.done = true
		return tea.Quit
	}
	form := m.steps[m.current].Form
	if form != nil {
		return form.Init()
	}
	// Step has no form (skip) → trigger done
	return stepDoneCmd(m.steps[m.current].OnDone)
}

// Update handles messages for the wizard model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Resize current form to fit the right panel
		if m.current < len(m.steps) && m.steps[m.current].Form != nil {
			formWidth := m.formWidth()
			m.steps[m.current].Form = m.steps[m.current].Form.
				WithWidth(formWidth).
				WithHeight(m.height - 4) // -4 for frame borders + footer
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.aborted = true
			return m, tea.Quit
		}

	case StepDoneMsg:
		if msg.Err != nil {
			m.err = msg.Err
			return m, tea.Quit
		}
		// Mark current step as done, advance to next
		m.status[m.current] = common.StepDone
		next := m.nextPendingStep()
		if next == -1 {
			// All done
			m.done = true
			return m, tea.Quit
		}
		m.current = next
		m.status[m.current] = common.StepActive
		form := m.steps[m.current].Form
		if form != nil {
			// Resize the new form
			formWidth := m.formWidth()
			m.steps[m.current].Form = form.
				WithWidth(formWidth).
				WithHeight(m.height - 4)
			return m, form.Init()
		}
		// No form → immediate done
		return m, stepDoneCmd(m.steps[m.current].OnDone)
	}

	// Forward messages to the current form
	if m.current < len(m.steps) && m.steps[m.current].Form != nil {
		form := m.steps[m.current].Form
		model, cmd := form.Update(msg)
		m.steps[m.current].Form = model.(*huh.Form)

		// Check if form is completed or aborted
		switch m.steps[m.current].Form.State {
		case huh.StateCompleted:
			return m, stepDoneCmd(m.steps[m.current].OnDone)
		case huh.StateAborted:
			// Mark as skipped, move to next
			m.status[m.current] = common.StepSkipped
			next := m.nextPendingStep()
			if next == -1 {
				m.done = true
				return m, tea.Quit
			}
			m.current = next
			m.status[m.current] = common.StepActive
			if m.steps[m.current].Form != nil {
				formWidth := m.formWidth()
				m.steps[m.current].Form = m.steps[m.current].Form.
					WithWidth(formWidth).
					WithHeight(m.height - 4)
				return m, m.steps[m.current].Form.Init()
			}
			return m, stepDoneCmd(m.steps[m.current].OnDone)
		}

		return m, cmd
	}

	return m, nil
}

// View renders the wizard with side-by-side layout.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initialisation..."
	}

	sidebarWidth := m.sidebarWidth()
	formWidth := m.formWidth()

	// ── Left panel: sidebar ──
	sidebarCfg := common.SidebarConfig{
		Title:   m.title,
		Prereqs: m.prereqs,
		Steps:   m.buildStepList(),
		Width:   sidebarWidth - 2,
	}
	sidebarContent := common.RenderSidebar(sidebarCfg)
	sidebarStyled := lipgloss.NewStyle().
		Width(sidebarWidth).
		Height(m.height - 4).
		PaddingLeft(1).
		PaddingRight(1).
		Render(sidebarContent)

	// ── Separator ──
	sepHeight := m.height - 4
	sepLines := make([]string, sepHeight)
	for i := range sepLines {
		sepLines[i] = "│"
	}
	sep := lipgloss.NewStyle().
		Foreground(common.Subtle).
		Render(strings.Join(sepLines, "\n"))

	// ── Right panel: form ──
	formView := ""
	if m.current < len(m.steps) && m.steps[m.current].Form != nil {
		formView = m.steps[m.current].Form.View()
	} else if m.done {
		formView = lipgloss.NewStyle().
			Foreground(common.Success).
			Bold(true).
			Render(fmt.Sprintf("\n  %s Configuration terminée !", common.IconSuccess))
	}
	formStyled := lipgloss.NewStyle().
		Width(formWidth).
		Height(m.height - 4).
		PaddingLeft(1).
		Render(formView)

	// ── Compose body ──
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebarStyled, sep, formStyled)

	// ── Footer ──
	footer := lipgloss.NewStyle().
		Foreground(common.Subtle).
		PaddingLeft(2).
		Render("enter: confirmer · esc: passer · ctrl+c: quitter")

	// ── Frame ──
	content := lipgloss.JoinVertical(lipgloss.Left, body, "", footer)

	frame := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(common.Primary).
		Width(m.width - 2).
		Padding(0, 0)

	return frame.Render(content)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// sidebarWidth returns the calculated sidebar width.
func (m Model) sidebarWidth() int {
	w := m.width / 3
	if w < 20 {
		w = 20
	}
	if w > 28 {
		w = 28
	}
	return w
}

// formWidth returns the calculated form panel width.
func (m Model) formWidth() int {
	return m.width - m.sidebarWidth() - 3 // -3 for separator + padding
}

// nextPendingStep returns the index of the next pending step, or -1 if none.
func (m Model) nextPendingStep() int {
	for i := m.current + 1; i < len(m.steps); i++ {
		if m.status[i] == common.StepPending {
			return i
		}
	}
	return -1
}

// buildStepList converts internal state to WizardStep slice for sidebar rendering.
func (m Model) buildStepList() []common.WizardStep {
	steps := make([]common.WizardStep, len(m.steps))
	for i, s := range m.steps {
		steps[i] = common.WizardStep{
			Label:  s.Label,
			Status: m.status[i],
		}
	}
	return steps
}
