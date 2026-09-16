package launcher

import (
	"fmt"
	"io"

	"github.com/charmbracelet/huh"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// CLILaunchUI implements LaunchUI for direct terminal usage (oh start, oh audit, etc.).
type CLILaunchUI struct {
	Out io.Writer
}

// NewCLIUI creates a CLI launch UI that writes to the given output writer.
func NewCLIUI(out io.Writer) *CLILaunchUI {
	return &CLILaunchUI{Out: out}
}

func (c *CLILaunchUI) Confirm(title string) (bool, error) {
	var confirm bool
	err := theme.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(title).
				Affirmative("Launch").
				Negative("Cancel").
				Value(&confirm),
		),
	).Run()
	if err != nil || !confirm {
		fmt.Fprintf(c.Out, "%s %s\n", theme.Subtitle.Render(theme.IconArrow), i18n.T("cmd.start.cancelled"))
		return false, err
	}
	return true, nil
}

func (c *CLILaunchUI) Notify(msg string, level Level) {
	var icon string
	switch level {
	case LevelWarning:
		icon = theme.WarningStyle.Render(theme.IconWarning)
	case LevelError:
		icon = theme.ErrorStyle.Render(theme.IconError)
	case LevelSuccess:
		icon = theme.SuccessStyle.Render(theme.IconSuccess)
	default:
		icon = theme.SuccessStyle.Render(theme.IconArrow)
	}
	fmt.Fprintf(c.Out, "%s %s\n", icon, msg)
}

// SuspendAndExec returns nil for CLI mode — no TUI to suspend.
func (c *CLILaunchUI) SuspendAndExec() func(func() error) error {
	return nil
}
