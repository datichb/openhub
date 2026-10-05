package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

func skillCheckCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: i18n.T("cmd.skill.check.short"),
		Long:  i18n.T("cmd.skill.check.long"),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			hubDir := findHubDir()
			if hubDir == "" {
				return fmt.Errorf("%s", i18n.T("cmd.skill.check.no_hub"))
			}
			problems, err := bundle.CheckSkills(hubDir)
			if err != nil {
				return err
			}
			errs := 0
			for _, p := range problems {
				if p.Error {
					errs++
				}
			}
			if asJSON {
				if problems == nil {
					problems = []bundle.SkillProblem{}
				}
				if err := json.NewEncoder(a.IO.Out).Encode(problems); err != nil {
					return err
				}
			} else {
				printSkillProblems(a.IO.Out, problems, errs)
			}
			if errs > 0 {
				cmd.SilenceUsage = true
				return fmt.Errorf("%s", i18n.Tf("cmd.skill.check.failed", errs))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, i18n.T("cmd.skill.check.flags.json"))
	return cmd
}

func printSkillProblems(out interface{ Write([]byte) (int, error) }, problems []bundle.SkillProblem, errs int) {
	if len(problems) == 0 {
		fmt.Fprintf(out, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.T("cmd.skill.check.ok"))
		return
	}
	for _, p := range problems {
		icon := theme.WarningStyle.Render(theme.IconWarning)
		if p.Error {
			icon = theme.ErrorStyle.Render(theme.IconError)
		}
		fmt.Fprintf(out, "  %s %s\n", icon, i18n.Tf("cmd.skill.check.kind."+string(p.Kind), p.Ref, p.Detail))
	}
	fmt.Fprintf(out, "\n  %s\n", i18n.Tf("cmd.skill.check.summary", errs, len(problems)-errs))
}
