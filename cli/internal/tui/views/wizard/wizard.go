// Package wizard provides a reusable BubbleTea model for multi-step wizards
// with a horizontal step bar and full-width form layout.
// Design System: Aurum — see docs/design/aurum.md
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
	Label  string     // Display name in the step bar
	Form   *huh.Form  // The huh form for this step (nil if skip)
	Skip   bool       // If true, step is pre-skipped (already satisfied)
	OnDone func() error // Callback executed when form completes (commit, write, etc.)
}

// ─────────────────────────────────────────────────────────────────────────────
// Model
// ─────────────────────────────────────────────────────────────────────────────

// Model is the BubbleTea model for a multi-step wizard with step bar.
type Model struct {
	title   string
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
// Steps marked with Skip=true will appear as StepDone in the step bar.
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
		steps:   steps,
		status:  statuses,
		current: current,
	}
}

// Done returns true when the wizard finished (all steps done/skipped).
func (m Model) Done() bool {
	return m.done
}

// Aborted returns true if the user cancelled (ctrl+c).
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
		// Resize current form to fit available space
		if m.current < len(m.steps) && m.steps[m.current].Form != nil {
			formWidth := m.contentWidth()
			formHeight := m.contentHeight()
			m.steps[m.current].Form = m.steps[m.current].Form.
				WithWidth(formWidth).
				WithHeight(formHeight)
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
			formWidth := m.contentWidth()
			formHeight := m.contentHeight()
			m.steps[m.current].Form = form.
				WithWidth(formWidth).
				WithHeight(formHeight)
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
				formWidth := m.contentWidth()
				formHeight := m.contentHeight()
				m.steps[m.current].Form = m.steps[m.current].Form.
					WithWidth(formWidth).
					WithHeight(formHeight)
				return m, m.steps[m.current].Form.Init()
			}
			return m, stepDoneCmd(m.steps[m.current].OnDone)
		}

		return m, cmd
	}

	return m, nil
}

// View renders the wizard with step bar + full width layout.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	innerWidth := m.width - 4 // -4 for border padding

	// ── Header: Title ──
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(common.Primary).
		PaddingLeft(2)
	header := titleStyle.Render(m.title)

	// ── Step Bar ──
	stepBar := lipgloss.NewStyle().
		PaddingLeft(2).
		Render(common.RenderStepBar(m.buildStepList()))

	// ── Separator (Gold) ──
	sepGold := lipgloss.NewStyle().
		Foreground(common.Primary).
		Render(strings.Repeat(common.IconSepBold, innerWidth))

	// ── Step Label ──
	stepLabel := ""
	if m.current < len(m.steps) {
		labelText := fmt.Sprintf("%d/%d · %s", m.current+1, len(m.steps), m.steps[m.current].Label)
		stepLabel = lipgloss.NewStyle().
			Bold(true).
			Foreground(common.Primary).
			PaddingLeft(2).
			Render(labelText)
	}

	// ── Form Content ──
	formView := ""
	if m.current < len(m.steps) && m.steps[m.current].Form != nil {
		formView = m.steps[m.current].Form.View()
	} else if m.done {
		formView = lipgloss.NewStyle().
			Foreground(common.Success).
			Bold(true).
			PaddingLeft(2).
			Render(fmt.Sprintf("%s Configuration terminée !", common.IconSuccess))
	}
	formStyled := lipgloss.NewStyle().
		PaddingLeft(1).
		Render(formView)

	// ── Separator (Graphite) ──
	sepGraphite := lipgloss.NewStyle().
		Foreground(common.Border).
		Render(strings.Repeat(common.IconSepBold, innerWidth))

	// ── Footer ──
	footer := lipgloss.NewStyle().
		Foreground(common.Subtle).
		PaddingLeft(2).
		Render("enter confirmer · esc passer · ctrl+c quitter")

	// ── Compose ──
	content := lipgloss.JoinVertical(lipgloss.Left,
		"",
		header,
		"",
		stepBar,
		"",
		sepGold,
		"",
		stepLabel,
		"",
		formStyled,
		"",
		sepGraphite,
		footer,
	)

	// ── Frame ──
	frame := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(common.Border).
		Width(m.width - 2).
		Height(m.height - 2)

	return frame.Render(content)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// contentWidth returns the available width for form content.
func (m Model) contentWidth() int {
	return m.width - 8 // borders + padding
}

// contentHeight returns the available height for form content.
func (m Model) contentHeight() int {
	// Total height minus: frame borders (2) + header (3) + step bar (3) +
	// sep (1) + step label (2) + sep (1) + footer (1) + spacing (4)
	h := m.height - 17
	if h < 5 {
		h = 5
	}
	return h
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

// buildStepList converts internal state to WizardStep slice for step bar rendering.
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
