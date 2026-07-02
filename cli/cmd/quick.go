package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/prompt"
	"github.com/datichb/openhub/cli/internal/tui/common"
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

	projects, err := a.Projects.List(domain.ProjectStatusActive)
	if err != nil {
		return err
	}

	if len(projects) == 0 {
		return fmt.Errorf("aucun projet enregistré. Lancez `oh init` ou `oh project add`")
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

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Choisir un projet").
					Options(options...).
					Value(&selectedID),
			),
		)
		if err := form.Run(); err != nil {
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
		return fmt.Errorf("projet non trouvé")
	}

	// Detect stack
	stack := prompt.DetectStack(project.Path)

	fmt.Fprintf(a.IO.Out, "%s %s",
		common.SuccessStyle.Render(common.IconArrow),
		project.Name)
	if stack.Language != "" {
		fmt.Fprintf(a.IO.Out, " (%s)", stack.Language)
	}
	fmt.Fprintln(a.IO.Out)

	// Get token
	var bearerToken string
	if a.Secrets != nil {
		token, _ := a.Secrets.Get("bedrock-token-" + project.ID)
		if token == "" {
			token, _ = a.Secrets.Get("bedrock-token-default")
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
