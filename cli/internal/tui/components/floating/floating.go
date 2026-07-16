// Package floating provides an inline floating prompt component.
// Design System: Aurum v2 "Floating Panels" — see docs/design/aurum.md
//
// FloatingPrompt renders a huh form inside a styled panel (without alt-screen),
// giving simple prompts a premium feel similar to Raycast's command palette.
// It stays inline in the terminal and does not take over the screen.
//
// Usage:
//
//	err := floating.Run(floating.Config{
//	    Title: "Confirm action",
//	    Form:  common.NewForm(huh.NewGroup(huh.NewConfirm().Title("Continue?").Value(&ok))),
//	})
package floating

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/datichb/openhub/cli/internal/tui/common"
)

// Config defines the appearance and content of a floating prompt.
type Config struct {
	// Title is displayed above the form in Copper bold.
	Title string
	// Subtitle is optional, displayed in Lavender below the title.
	Subtitle string
	// Form is the huh form to run inside the floating panel.
	Form *huh.Form
	// Width overrides the panel width. 0 = auto (min 40, max 72).
	Width int
}

// Run renders the floating prompt and blocks until the user completes or cancels.
// Returns the error from the form (nil on success, non-nil on ctrl+c).
func Run(cfg Config) error {
	if !common.UseRichTUI() {
		// Fallback: run the form directly without decoration.
		return cfg.Form.Run()
	}

	m := newModel(cfg)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	fm := finalModel.(model)
	if fm.aborted {
		return huh.ErrUserAborted
	}
	return fm.err
}

// ─────────────────────────────────────────────────────────────────────────────
// BubbleTea model
// ─────────────────────────────────────────────────────────────────────────────

type model struct {
	cfg     Config
	form    *huh.Form
	width   int
	height  int
	err     error
	done    bool
	aborted bool
}

func newModel(cfg Config) model {
	w := cfg.Width
	if w == 0 {
		w = 60
	}
	// Apply Aurum theme to the form
	cfg.Form = cfg.Form.WithTheme(common.AurumTheme())
	cfg.Form = cfg.Form.WithWidth(w - 8) // account for panel border + padding

	return model{
		cfg:  cfg,
		form: cfg.Form,
	}
}

func (m model) Init() tea.Cmd {
	return m.form.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		w := m.cfg.Width
		if w == 0 {
			w = 60
			if m.width > 0 && m.width < w+4 {
				w = m.width - 4
			}
		}
		m.form = m.form.WithWidth(w - 8)
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.aborted = true
			return m, tea.Quit
		}
	}

	// Forward to form
	model, cmd := m.form.Update(msg)
	m.form = model.(*huh.Form)

	switch m.form.State {
	case huh.StateCompleted:
		m.done = true
		return m, tea.Quit
	case huh.StateAborted:
		m.aborted = true
		return m, tea.Quit
	}

	return m, cmd
}

func (m model) View() string {
	w := m.cfg.Width
	if w == 0 {
		w = 60
	}

	// ── Header ──
	var header string
	if m.cfg.Title != "" {
		titleStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(common.TextLight)
		header = titleStyle.Render(m.cfg.Title)
		if m.cfg.Subtitle != "" {
			subtitleStyle := lipgloss.NewStyle().Foreground(common.Subtle)
			header += "\n" + subtitleStyle.Render(m.cfg.Subtitle)
		}
		header += "\n"
	}

	// ── Form content ──
	formView := m.form.View()

	// ── Footer ──
	footer := lipgloss.NewStyle().
		Foreground(common.Subtle).
		Render("enter " + common.IconDot + " confirm  esc " + common.IconDot + " cancel")

	// ── Compose ──
	content := header + formView + "\n\n" + footer

	// ── Panel box ──
	// No Background() on the panel — lipgloss Background on a container creates
	// uneven fill when inner elements have varying widths. The panel is just a
	// decorative border that floats above the terminal background.
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(common.BorderElem).
		Padding(1, 2).
		Width(w)

	// Add vertical margin above for "floating" feel
	return "\n" + panel.Render(content) + "\n"
}

// ─────────────────────────────────────────────────────────────────────────────
// Convenience helpers
// ─────────────────────────────────────────────────────────────────────────────

// Confirm runs a styled confirmation prompt with the given title.
// Returns the boolean result via the value pointer.
func Confirm(title string, value *bool) error {
	form := common.NewForm(
		huh.NewGroup(
			huh.NewConfirm().Title(title).Value(value),
		),
	)
	return Run(Config{Title: "", Form: form})
}

// Select runs a styled single-select prompt with title and options.
func Select[T comparable](title string, options []huh.Option[T], value *T) error {
	form := common.NewForm(
		huh.NewGroup(
			huh.NewSelect[T]().Title(title).Options(options...).Value(value),
		),
	)
	return Run(Config{Title: "", Form: form})
}

// Input runs a styled text input prompt.
func Input(title, placeholder string, value *string) error {
	form := common.NewForm(
		huh.NewGroup(
			huh.NewInput().Title(title).Placeholder(placeholder).Value(value),
		),
	)
	return Run(Config{Title: "", Form: form})
}

// ─────────────────────────────────────────────────────────────────────────────
// Step header (for multi-step inline flows)
// ─────────────────────────────────────────────────────────────────────────────

// RenderStepHeader prints a styled step header for inline multi-step flows.
// Example: "◔ 2/4 · Provider Configuration"
func RenderStepHeader(current, total int, label string) string {
	icon := lipgloss.NewStyle().Foreground(common.Primary).Render(common.IconStepActive)
	counter := lipgloss.NewStyle().Foreground(common.Muted).Render(fmt.Sprintf("%d/%d", current+1, total))
	title := lipgloss.NewStyle().Bold(true).Foreground(common.Primary).Render(label)
	sep := lipgloss.NewStyle().Foreground(common.Muted).Render(" · ")

	return "\n" + strings.Join([]string{icon, counter, sep, title}, " ") + "\n"
}
