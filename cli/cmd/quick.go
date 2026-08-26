package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/prompt"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/components/floating"
)

var quickCmd = &cobra.Command{
	Use:   "quick",
	Short: "Lancement rapide — sélection projet puis opencode",
	Long:  "Affiche un sélecteur de projet puis lance immédiatement opencode sans configuration supplémentaire.",
	RunE:  runQuick,
}

func init() {
	rootCmd.AddCommand(quickCmd)
}

func runQuick(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	// Ensure opencode is installed before proceeding
	if err := ensureOpencode(a); err != nil {
		return err
	}

	projects, err := a.Projects.List(ctx, domain.ProjectStatusActive)
	if err != nil {
		return err
	}

	if len(projects) == 0 {
		return fmt.Errorf("%s", i18n.T("cmd.quick.no_projects"))
	}

	var selectedID string

	if len(projects) == 1 {
		selectedID = projects[0].ID
	} else {
		options := make([]huh.Option[string], len(projects))
		for i, p := range projects {
			options[i] = huh.NewOption(
				fmt.Sprintf("%s — %s", p.Name, p.Path), p.ID)
		}

		form := theme.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(i18n.T("form.project.choose")).
					Options(options...).
					Value(&selectedID),
			),
		)
		if err := floating.Run(floating.Config{
			Title: i18n.T("cmd.quick.short"),
			Form:  form,
		}); err != nil {
			return err
		}
	}

	// Find selected project
	var project *domain.Project
	for i, p := range projects {
		if p.ID == selectedID {
			project = &projects[i]
			break
		}
	}
	if project == nil {
		return fmt.Errorf("%s", i18n.T("cmd.quick.not_found"))
	}

	// Detect stack
	stack := prompt.DetectStack(project.Path)

	fmt.Fprintf(a.IO.Out, "%s %s",
		theme.SuccessStyle.Render(theme.IconArrow),
		project.Name)
	if stack.Language != "" {
		fmt.Fprintf(a.IO.Out, " (%s)", stack.Language)
	}
	fmt.Fprintln(a.IO.Out)

	// Get token
	var bearerToken string
	if a.Secrets != nil {
		token, _ := a.Secrets.Get(ctx, provider.KeychainKey(provider.Bedrock, project.ID))
		if token == "" {
			token, _ = a.Secrets.Get(ctx, provider.KeychainKey(provider.Bedrock, ""))
		}
		bearerToken = token
	}

	return opencode.Exec(opencode.StartOpts{
		ProjectPath: project.Path,
		ProjectID:   project.ID,
		Provider:    "bedrock",
		BearerToken: bearerToken,
	})
}
