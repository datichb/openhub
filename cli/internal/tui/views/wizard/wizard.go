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
	justAdvanced bool
}

// New creates a wizard model from the given configuration.
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
		current = len(steps)
	}

	return Model{
		title:   title,
		steps:   steps,
		status:  statuses,
		current: current,
	}
}

// Done returns true when the wizard finished.
func (m Model) Done() bool { return m.done }

// Aborted returns true if the user cancelled.
func (m Model) Aborted() bool { return m.aborted }

// Err returns any error that occurred.
func (m Model) Err() error { return m.err }

// ─────────────────────────────────────────────────────────────────────────────
// BubbleTea interface
// ─────────────────────────────────────────────────────────────────────────────

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

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.current < len(m.steps) && m.steps[m.current].Form != nil {
			m.steps[m.current].Form = m.steps[m.current].Form.
				WithWidth(m.formWidth()).
				WithHeight(m.formHeight())
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.aborted = true
			return m, tea.Quit
		}

	case StepDoneMsg:
		if msg.Err != nil {
			m.err = msg.Err
			return m, tea.Quit
		}
		m.status[m.current] = common.StepDone
		next := m.nextPendingStep()
		if next == -1 {
			m.done = true
			return m, tea.Quit
		}
		m.current = next
		m.status[m.current] = common.StepActive
		m.justAdvanced = true
		form := m.steps[m.current].Form
		if form != nil {
			m.steps[m.current].Form = form.
				WithWidth(m.formWidth()).
				WithHeight(m.formHeight())
			return m, form.Init()
		}
		return m, stepDoneCmd(m.steps[m.current].OnDone)
	}

	// Forward to current form
	if m.current < len(m.steps) && m.steps[m.current].Form != nil {
		form := m.steps[m.current].Form
		model, cmd := form.Update(msg)
		m.steps[m.current].Form = model.(*huh.Form)

		if m.justAdvanced {
			m.justAdvanced = false
			return m, cmd
		}

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
				m.steps[m.current].Form = m.steps[m.current].Form.
					WithWidth(m.formWidth()).
					WithHeight(m.formHeight())
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

	// ── Dimensions ──
	outerW := m.width - 2
	innerW := outerW - 6 // outer border(2) + outer hpad(2) + gap(2)
	innerH := m.height - 10 // outer border(2) + title area(3) + footer area(3) + inner border(2)
	if innerH < 7 {
		innerH = 7
	}

	// ── Step bar ──
	stepBar := common.RenderStepBar(m.buildStepList())

	// ── Step label ──
	stepLabel := ""
	if m.current < len(m.steps) {
		stepLabel = lipgloss.NewStyle().
			Bold(true).
			Foreground(common.Primary).
			Render(fmt.Sprintf("%d/%d · %s", m.current+1, len(m.steps), m.steps[m.current].Label))
	}

	// ── Form ──
	formView := ""
	if m.current < len(m.steps) && m.steps[m.current].Form != nil {
		formView = m.steps[m.current].Form.View()
	} else if m.done {
		formView = lipgloss.NewStyle().
			Foreground(common.Success).
			Bold(true).
			Render(fmt.Sprintf("%s Configuration terminée !", common.IconSuccess))
	}

	// ── Inner panel content ──
	// Use lipgloss.Place to fill the entire inner rectangle with SurfaceElem bg.
	// This ensures no black bands — every cell gets the background color.
	innerText := lipgloss.JoinVertical(lipgloss.Left,
		stepBar,
		"",
		stepLabel,
		"",
		formView,
	)

	// Place the text content inside a fixed-size box filled with SurfaceElem
	innerFilled := lipgloss.Place(
		innerW-2, // width (minus border)
		innerH-2, // height (minus border)
		lipgloss.Left,
		lipgloss.Top,
		innerText,
		lipgloss.WithWhitespaceBackground(common.SurfaceElem),
	)

	// Wrap with rounded border
	innerBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(common.BorderElem).
		BorderBackground(common.Surface).
		Width(innerW - 2).
		Render(innerFilled)

	// ── Title ──
	titleView := lipgloss.NewStyle().
		Bold(true).
		Foreground(common.TextLight).
		Render(m.title)

	// ── Footer ──
	footerView := lipgloss.NewStyle().
		Foreground(common.Subtle).
		Render("enter confirmer · esc passer · ctrl+c quitter")

	// ── Outer panel content ──
	// Compose: title + inner + footer, placed in a filled outer rectangle
	outerText := lipgloss.JoinVertical(lipgloss.Left,
		"",
		"  "+titleView,
		"",
		"  "+innerBox,
		"",
		"  "+footerView,
	)

	// Calculate outer content height (fills remaining space)
	outerH := m.height - 2 // minus outer border

	// Place outer content in a filled box with Surface bg
	outerFilled := lipgloss.Place(
		outerW-2, // width (minus border)
		outerH-2, // height (minus border + padding)
		lipgloss.Left,
		lipgloss.Top,
		outerText,
		lipgloss.WithWhitespaceBackground(common.Surface),
	)

	// Wrap with rounded border
	outerFrame := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(common.Border).
		Width(outerW - 2).
		Render(outerFilled)

	return outerFrame
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func (m Model) formWidth() int {
	return m.width - 16
}

func (m Model) formHeight() int {
	h := m.height - 20
	if h < 5 {
		h = 5
	}
	return h
}

func (m Model) nextPendingStep() int {
	for i := m.current + 1; i < len(m.steps); i++ {
		if m.status[i] == common.StepPending {
			return i
		}
	}
	return -1
}

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
