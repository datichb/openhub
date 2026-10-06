package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/buildinfo"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	remotesvc "github.com/datichb/openhub/cli/internal/services/remote"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: i18n.T("cmd.remote.short"),
	Long:  i18n.T("cmd.remote.long"),
}

func init() {
	rootCmd.AddCommand(remoteCmd)
}

// newRemoteService wires the remote service on the machine secret store and
// hub.toml.
func newRemoteService(a *app.App) *remotesvc.Service {
	return &remotesvc.Service{
		Secrets:   a.Secrets,
		OhVersion: buildinfo.Version,
		SaveTarget: func(t config.RemoteTarget) error {
			return config.Update(func(c *config.Config) error {
				c.Remote.Upsert(t)
				return nil
			})
		},
	}
}

// remoteStepIcon renders the status of a step.
func remoteStepIcon(s remotesvc.StepStatus) string {
	switch s {
	case remotesvc.StepOK:
		return theme.SuccessStyle.Render(theme.IconSuccess)
	case remotesvc.StepCreated, remotesvc.StepUpdated:
		return theme.SuccessStyle.Render("+")
	case remotesvc.StepWarn:
		return theme.WarningStyle.Render(theme.IconWarning)
	case remotesvc.StepFailed:
		return theme.ErrorStyle.Render(theme.IconError)
	default:
		return theme.IconSkipped
	}
}

// RemoteStepLine is the text of a report step (shared by the CLI, Doctor and
// the TUI): label, status word, values and error.
func RemoteStepLine(s remotesvc.Step) string {
	parts := []string{i18n.T("cmd.remote.step." + s.ID)}
	if s.Status != remotesvc.StepOK {
		parts = append(parts, i18n.T("cmd.remote.state."+string(s.Status)))
	}
	if len(s.Args) > 0 {
		parts = append(parts, strings.Join(s.Args, " · "))
	}
	line := strings.Join(parts, " — ")
	if s.Err != nil {
		line += " (" + s.Err.Error() + ")"
	}
	return line
}

func printRemoteReport(w io.Writer, rep *remotesvc.Report) {
	fmt.Fprintln(w, theme.AccentStyle.Render(i18n.Tf("cmd.remote.report.title", rep.Target.Name, rep.Target.URL, rep.Target.RunnerProjectPath())))
	for _, s := range rep.Steps {
		fmt.Fprintf(w, "  %s %s\n", remoteStepIcon(s.Status), RemoteStepLine(s))
	}
	seen := map[string]bool{}
	var hints []string
	for _, s := range rep.Steps {
		if (s.Status == remotesvc.StepFailed || s.Status == remotesvc.StepWarn) && !seen[s.ID] {
			seen[s.ID] = true
			hints = append(hints, i18n.T("cmd.remote.hint."+s.ID))
		}
	}
	if len(hints) > 0 {
		fmt.Fprintln(w)
		for _, h := range hints {
			fmt.Fprintf(w, "  %s %s\n", theme.IconArrow, h)
		}
	}
}
