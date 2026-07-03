package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

// runAgentSession is a shared helper for commands that resolve a project
// then launch opencode with a specific agent and prompt.
func runAgentSession(agent, prompt, titleLabel string, cmd *cobra.Command) error {
	a := MustApp()
	ctx := cmd.Context()
	projectID, _ := cmd.Flags().GetString("project")
	project, err := resolveProject(ctx, a, projectID)
	if err != nil {
		return err
	}

	fmt.Fprintf(a.IO.Out, "%s %s sur %s\n",
		common.Title.Render("oh "+titleLabel), common.Bold.Render(agent), project.Name)

	return opencode.Exec(opencode.StartOpts{
		ProjectPath: project.Path,
		ProjectID:   project.ID,
		Agent:       agent,
		Prompt:      prompt,
	})
}

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Lance un audit de code via opencode",
	Long: `Lance une session opencode avec l'agent auditor pour réaliser un audit.

Types d'audit disponibles :
  security       Vulnérabilités, injections, gestion des secrets, dépendances
  performance    Fuites mémoire, N+1, rendering, bundle size, lazy loading
  architecture   Couplage, cohésion, patterns, couches, dette technique
  accessibility  WCAG, ARIA, contraste, navigation clavier, screen readers
  ecodesign      Empreinte carbone, poids ressources, requêtes inutiles, green patterns
  observability  Logs, traces, métriques, alerting, corrélation, SLI/SLO
  privacy        RGPD, données personnelles, consentement, rétention, minimisation`,
	RunE: func(cmd *cobra.Command, args []string) error {
		auditType, _ := cmd.Flags().GetString("type")

		// Validate audit type
		validTypes := map[string]string{
			"security":      "sécurité (vulnérabilités, injections, secrets, dépendances)",
			"performance":   "performance (fuites mémoire, N+1, rendering, bundle size)",
			"architecture":  "architecture (couplage, cohésion, patterns, dette technique)",
			"accessibility": "accessibilité (WCAG, ARIA, contraste, navigation clavier)",
			"ecodesign":     "éco-conception (empreinte carbone, poids, requêtes inutiles)",
			"observability": "observabilité (logs, traces, métriques, alerting, SLI/SLO)",
			"privacy":       "vie privée (RGPD, données personnelles, consentement, rétention)",
		}

		description, ok := validTypes[auditType]
		if !ok {
			return fmt.Errorf("%s", i18n.Tf("cmd.audit.invalid_type", auditType))
		}

		prompt := i18n.Tf("cmd.audit.prompt", auditType, description)
		return runAgentSession("auditor", prompt, "audit", cmd)
	},
}

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Lance une review de code via opencode",
	Long:  "Lance une session opencode avec l'agent reviewer.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAgentSession("reviewer", i18n.T("cmd.review.prompt"), "review", cmd)
	},
}

var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Lance une session de debug via opencode",
	Long:  "Lance une session opencode avec l'agent debugger.",
	RunE: func(cmd *cobra.Command, args []string) error {
		issue, _ := cmd.Flags().GetString("issue")
		prompt := i18n.T("cmd.debug.prompt_default")
		if issue != "" {
			prompt = i18n.Tf("cmd.debug.prompt", issue)
		}
		return runAgentSession("debugger", prompt, "debug", cmd)
	},
}

func init() {
	rootCmd.AddCommand(auditCmd)
	auditCmd.Flags().StringP("project", "j", "", "ID du projet")
	auditCmd.Flags().StringP("type", "t", "security", "Type d'audit (security, performance, architecture, accessibility, ecodesign, observability, privacy)")
	_ = auditCmd.RegisterFlagCompletionFunc("project", completeProjectIDs)

	rootCmd.AddCommand(reviewCmd)
	reviewCmd.Flags().StringP("project", "j", "", "ID du projet")
	_ = reviewCmd.RegisterFlagCompletionFunc("project", completeProjectIDs)

	rootCmd.AddCommand(debugCmd)
	debugCmd.Flags().StringP("project", "j", "", "ID du projet")
	debugCmd.Flags().StringP("issue", "i", "", "Description du problème")
	_ = debugCmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
}
