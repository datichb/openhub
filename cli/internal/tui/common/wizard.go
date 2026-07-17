// Package common provides shared TUI styles and constants for the oh CLI.
// Design System: Aurum v2 — see docs/design/aurum.md
package common

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// StepStatus represents the state of a wizard step.
type StepStatus int

const (
	// StepPending means the step has not been started yet.
	StepPending StepStatus = iota
	// StepActive means the step is currently being executed.
	StepActive
	// StepDone means the step completed successfully.
	StepDone
	// StepSkipped means the step was skipped (already configured or declined).
	StepSkipped
)

// WizardStep holds label and status for a single wizard step.
type WizardStep struct {
	Label  string
	Status StepStatus
}

// SidebarConfig defines what the sidebar should render.
type SidebarConfig struct {
	Title   string       // Main title (e.g. "Team Setup")
	Prereqs []string     // Prerequisite items (always shown as checked)
	Steps   []WizardStep // Wizard steps with their statuses
	Width   int          // Available width for rendering
}

// Styles for the sidebar (Aurum v2).
var (
	sidebarTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Primary)

	sidebarPrereq = lipgloss.NewStyle().
			Foreground(theme.LipSuccess)

	sidebarSeparator = lipgloss.NewStyle().
				Foreground(theme.Muted)

	sidebarStepActive = lipgloss.NewStyle().
				Bold(true).
				Foreground(theme.Primary)

	sidebarStepDone = lipgloss.NewStyle().
			Foreground(theme.LipSuccess)

	sidebarStepPending = lipgloss.NewStyle().
				Foreground(theme.Muted)

	sidebarStepSkipped = lipgloss.NewStyle().
				Foreground(theme.Muted).
				Strikethrough(true)

	sidebarHeader = lipgloss.NewStyle().
			Foreground(theme.Subtle)
)

// RenderSidebar returns a formatted sidebar string for use in inline wizard layouts.
// It displays prerequisites (always checked) followed by a separator and the step list.
func RenderSidebar(cfg SidebarConfig) string {
	var b strings.Builder

	// Title
	b.WriteString(sidebarTitle.Render(cfg.Title))
	b.WriteByte('\n')
	b.WriteByte('\n')

	// Prerequisites section
	if len(cfg.Prereqs) > 0 {
		b.WriteString(sidebarHeader.Render("Prérequis"))
		b.WriteByte('\n')

		for _, p := range cfg.Prereqs {
			line := fmt.Sprintf("%s %s", sidebarPrereq.Render(theme.IconSuccess), p)
			b.WriteString(line)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')

		// Separator
		sepWidth := cfg.Width - 2
		if sepWidth < 10 {
			sepWidth = 10
		}
		b.WriteString(sidebarSeparator.Render(strings.Repeat(theme.IconSepNormal, sepWidth)))
		b.WriteByte('\n')
		b.WriteByte('\n')
	}

	// Steps section
	b.WriteString(sidebarHeader.Render("Étapes"))
	b.WriteByte('\n')

	for _, step := range cfg.Steps {
		var line string
		switch step.Status {
		case StepActive:
			line = sidebarStepActive.Render(fmt.Sprintf("%s %s", theme.IconStepActive, step.Label))
		case StepDone:
			line = sidebarStepDone.Render(fmt.Sprintf("%s %s", theme.IconDone, step.Label))
		case StepSkipped:
			line = sidebarStepSkipped.Render(fmt.Sprintf("%s %s", theme.IconPending, step.Label))
		default: // StepPending
			line = sidebarStepPending.Render(fmt.Sprintf("%s %s", theme.IconPending, step.Label))
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}

	return b.String()
}

// RenderStepBar renders a horizontal step bar for full-width wizard layouts.
// Example: ● Config ─── ◔ Identité ─── ○ Notifications ─── ○ Policies
func RenderStepBar(steps []WizardStep) string {
	var parts []string

	for _, step := range steps {
		var icon string
		var style lipgloss.Style

		switch step.Status {
		case StepDone:
			icon = theme.IconDone
			style = sidebarStepDone
		case StepActive:
			icon = theme.IconStepActive
			style = sidebarStepActive
		case StepSkipped:
			icon = theme.IconPending
			style = sidebarStepSkipped
		default:
			icon = theme.IconPending
			style = sidebarStepPending
		}

		parts = append(parts, style.Render(fmt.Sprintf("%s %s", icon, step.Label)))
	}

	connector := sidebarSeparator.Render(" ─── ")
	return strings.Join(parts, connector)
}
