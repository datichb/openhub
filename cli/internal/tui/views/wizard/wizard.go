// Package wizard provides a reusable BubbleTea model for multi-step wizards
// with a horizontal step bar and floating panel layout.
// Design System: Aurum v2 "Floating Panels" — see docs/design/aurum.md
package wizard

import (
	"fmt"

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

// Model is the BubbleTea model for a multi-step wizard with floating panels.
type Model struct {
	title   string
	steps   []StepConfig
	status  []common.StepStatus

	current      int
	width        int
	height       int
	err          error
	done         bool
	aborted      bool
	justAdvanced bool // prevents cascading completion after step transition
}

// New creates a wizard model from the given configuration.
// Steps marked with Skip=true will appear as StepDone in the step bar.
// The prereqs parameter is kept for API compatibility but not rendered in the wizard.
func New(title string, _ []string, steps []StepConfig) Model {
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
	if m.current >= len(m.steps) {
		m.done = true
		return tea.Quit
	}
	form := m.steps[m.current].Form
	if form != nil {
		return form.Init()
	}
	return stepDoneCmd(m.steps[m.current].OnDone)
}

// Update handles messages for the wizard model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.current < len(m.steps) && m.steps[m.current].Form != nil {
			fw := m.formWidth()
			fh := m.formHeight()
			m.steps[m.current].Form = m.steps[m.current].Form.
				WithWidth(fw).
				WithHeight(fh)
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
			m.done = true
			return m, tea.Quit
		}
		m.current = next
		m.status[m.current] = common.StepActive
		m.justAdvanced = true // prevent cascading completion
		form := m.steps[m.current].Form
		if form != nil {
			fw := m.formWidth()
			fh := m.formHeight()
			m.steps[m.current].Form = form.
				WithWidth(fw).
				WithHeight(fh)
			return m, form.Init()
		}
		return m, stepDoneCmd(m.steps[m.current].OnDone)
	}

	// Forward messages to the current form
	if m.current < len(m.steps) && m.steps[m.current].Form != nil {
		form := m.steps[m.current].Form
		model, cmd := form.Update(msg)
		m.steps[m.current].Form = model.(*huh.Form)

		// Skip state check on the first Update after advancing
		// (prevents cascading completion from previous step's residual messages)
		if m.justAdvanced {
			m.justAdvanced = false
			return m, cmd
		}

		// Check if form is completed or aborted
		switch m.steps[m.current].Form.State {
		case huh.StateCompleted:
			return m, stepDoneCmd(m.steps[m.current].OnDone)
		case huh.StateAborted:
			m.status[m.current] = common.StepSkipped
			next := m.nextPendingStep()
			if next == -1 {
				m.done = true
				return m, tea.Quit
			}
			m.current = next
			m.status[m.current] = common.StepActive
			m.justAdvanced = true
			if m.steps[m.current].Form != nil {
				fw := m.formWidth()
				fh := m.formHeight()
				m.steps[m.current].Form = m.steps[m.current].Form.
					WithWidth(fw).
					WithHeight(fh)
				return m, m.steps[m.current].Form.Init()
			}
			return m, stepDoneCmd(m.steps[m.current].OnDone)
		}

		return m, cmd
	}

	return m, nil
}

// View renders the wizard with floating panel layout.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	outerWidth := m.width - 2
	innerWidth := outerWidth - 6 // outer padding + inner border + inner padding

	// ── Title (on panel bg) ──
	titleView := lipgloss.NewStyle().
		Bold(true).
		Foreground(common.TextLight).
		PaddingLeft(1).
		Render(m.title)

	// ── Inner panel (raised element: step bar + form) ──
	// Step bar
	stepBar := common.RenderStepBar(m.buildStepList())

	// Step label
	stepLabel := ""
	if m.current < len(m.steps) {
		labelText := fmt.Sprintf("%d/%d · %s", m.current+1, len(m.steps), m.steps[m.current].Label)
		stepLabel = lipgloss.NewStyle().
			Bold(true).
			Foreground(common.Primary).
			Render(labelText)
	}

	// Form content
	formView := ""
	if m.current < len(m.steps) && m.steps[m.current].Form != nil {
		formView = m.steps[m.current].Form.View()
	} else if m.done {
		formView = lipgloss.NewStyle().
			Foreground(common.Success).
			Bold(true).
			Render(fmt.Sprintf("%s Configuration terminée !", common.IconSuccess))
	}

	// Compose inner content (step bar + label + form)
	innerContent := lipgloss.JoinVertical(lipgloss.Left,
		"",
		stepBar,
		"",
		stepLabel,
		"",
		formView,
		"",
	)

	// Inner panel box (raised element)
	innerBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(common.BorderElem).
		Background(common.SurfaceElem).
		Width(innerWidth).
		Padding(0, 1).
		Render(innerContent)

	// ── Footer (on panel bg) ──
	footer := lipgloss.NewStyle().
		Foreground(common.Subtle).
		PaddingLeft(1).
		Render("enter confirmer · esc passer · ctrl+c quitter")

	// ── Outer panel (floating on terminal) ──
	outerContent := lipgloss.JoinVertical(lipgloss.Left,
		"",
		titleView,
		"",
		innerBox,
		"",
		footer,
		"",
	)

	outerFrame := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(common.Border).
		Background(common.Surface).
		Width(outerWidth).
		Padding(0, 1)

	return outerFrame.Render(outerContent)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// formWidth returns the available width for form content inside the inner panel.
func (m Model) formWidth() int {
	// outer border (2) + outer padding (2) + inner border (2) + inner padding (2)
	return m.width - 10
}

// formHeight returns the available height for form content.
func (m Model) formHeight() int {
	// outer frame (2) + title area (3) + inner frame (2) + step bar (3) +
	// step label (2) + footer (3) + spacing (4)
	h := m.height - 19
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


