package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Lance un audit de code via opencode",
	Long:  "Lance une session opencode avec l'agent auditor pour réaliser un audit.",
	RunE: func(cmd *cobra.Command, args []string) error {
		a := GetApp()
		projectID, _ := cmd.Flags().GetString("project")
		project, err := resolveProject(a, projectID)
		if err != nil {
			return err
		}

		auditType, _ := cmd.Flags().GetString("type")
		prompt := fmt.Sprintf("Réalise un audit %s du projet.", auditType)

		fmt.Fprintf(a.IO.Out, "%s Audit %s sur %s\n",
			common.Title.Render("oh audit"), auditType, project.Name)

		return opencode.Exec(opencode.StartOpts{
			ProjectPath: project.Path,
			ProjectID:   project.ID,
			Agent:       "auditor",
			Prompt:      prompt,
		})
	},
}

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Lance une review de code via opencode",
	Long:  "Lance une session opencode avec l'agent reviewer.",
	RunE: func(cmd *cobra.Command, args []string) error {
		a := GetApp()
		projectID, _ := cmd.Flags().GetString("project")
		project, err := resolveProject(a, projectID)
		if err != nil {
			return err
		}

		fmt.Fprintf(a.IO.Out, "%s Review sur %s\n",
			common.Title.Render("oh review"), project.Name)

		return opencode.Exec(opencode.StartOpts{
			ProjectPath: project.Path,
			ProjectID:   project.ID,
			Agent:       "reviewer",
			Prompt:      "Review le code modifié récemment.",
		})
	},
}

var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Lance une session de debug via opencode",
	Long:  "Lance une session opencode avec l'agent debugger.",
	RunE: func(cmd *cobra.Command, args []string) error {
		a := GetApp()
		projectID, _ := cmd.Flags().GetString("project")
		project, err := resolveProject(a, projectID)
		if err != nil {
			return err
		}

		issue, _ := cmd.Flags().GetString("issue")
		prompt := "Debug le problème signalé."
		if issue != "" {
			prompt = fmt.Sprintf("Debug le problème: %s", issue)
		}

		fmt.Fprintf(a.IO.Out, "%s Debug sur %s\n",
			common.Title.Render("oh debug"), project.Name)

		return opencode.Exec(opencode.StartOpts{
			ProjectPath: project.Path,
			ProjectID:   project.ID,
			Agent:       "debugger",
			Prompt:      prompt,
		})
	},
}

func init() {
	rootCmd.AddCommand(auditCmd)
	auditCmd.Flags().StringP("project", "j", "", "ID du projet")
	auditCmd.Flags().StringP("type", "t", "security", "Type d'audit (security, performance, architecture)")

	rootCmd.AddCommand(reviewCmd)
	reviewCmd.Flags().StringP("project", "j", "", "ID du projet")

	rootCmd.AddCommand(debugCmd)
	debugCmd.Flags().StringP("project", "j", "", "ID du projet")
	debugCmd.Flags().StringP("issue", "i", "", "Description du problème")
}
