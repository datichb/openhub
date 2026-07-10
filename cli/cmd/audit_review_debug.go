package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/notify"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/teamstate"
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
	Long: `Lance une session opencode avec l'agent reviewer.

Modes de review disponibles :
  standard                Review classique (checklist 6 catégories)
  adversarial             Critique approfondie (scepticisme maximal, min. 10 findings)
  edge-case               Chasse aux chemins d'exécution non gérés
  standard+adversarial    Sessions parallèles + rapport unifié
  all                     Standard + Adversarial + Edge-case (couverture maximale)

Sans flag --mode, un menu interactif est affiché.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		publish, _ := cmd.Flags().GetBool("publish")
		if publish {
			return runReviewPublish(cmd)
		}

		mode, _ := cmd.Flags().GetString("mode")

		// If no mode specified, the reviewer-standalone skill will handle the
		// interactive prompt via the question tool inside the opencode session.
		// If a mode IS specified, inject it as a tag in the prompt.
		var prompt string
		switch mode {
		case "":
			prompt = i18n.T("cmd.review.prompt")
		case "standard":
			prompt = "[MODE:standard] " + i18n.T("cmd.review.prompt")
		case "adversarial":
			prompt = "[MODE:adversarial] " + i18n.T("cmd.review.prompt")
		case "edge-case":
			prompt = "[MODE:edge-case] " + i18n.T("cmd.review.prompt")
		case "standard+adversarial":
			prompt = "[MODE:standard+adversarial] " + i18n.T("cmd.review.prompt")
		case "all":
			prompt = "[MODE:all] " + i18n.T("cmd.review.prompt")
		default:
			return fmt.Errorf("mode invalide %q — modes disponibles : standard, adversarial, edge-case, standard+adversarial, all", mode)
		}

		return runAgentSession("reviewer", prompt, "review", cmd)
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

// runReviewPublish creates a MR on GitLab for the current branch and assigns a reviewer.
func runReviewPublish(cmd *cobra.Command) error {
	a := MustApp()
	ctx := cmd.Context()

	if !a.Config.MCP.Gitlab.WriteEnabled {
		return fmt.Errorf("GitLab write non activé. Lance %s et active le mode écriture",
			common.Bold.Render("oh service setup"))
	}

	projectID, _ := cmd.Flags().GetString("project")
	project, err := resolveProject(ctx, a, projectID)
	if err != nil {
		return err
	}

	// Get current branch
	branch := getPublishBranch(project.Path)
	if branch == "" || branch == "main" || branch == "master" || branch == "develop" {
		return fmt.Errorf("branche courante (%s) n'est pas une feature branch", branch)
	}

	fmt.Fprintf(a.IO.Out, "%s Création MR pour la branche %s...\n",
		common.Subtitle.Render(common.IconArrow), common.Bold.Render(branch))

	fmt.Fprintf(a.IO.Out, "%s MR prête à être créée pour %s → main\n",
		common.SuccessStyle.Render(common.IconSuccess), branch)

	// Extract ticket ref from branch for the title
	ticketRef := extractTicketFromBranch(branch)
	title := branch
	if ticketRef != "" {
		title = fmt.Sprintf("%s: %s", ticketRef, strings.TrimPrefix(branch, fmt.Sprintf("feat/%s-", ticketRef)))
	}
	fmt.Fprintf(a.IO.Out, "  Titre : %s\n", title)

	// Emit team event if team is enabled
	if a.Config.Team.Enabled {
		emitReviewReadyEvent(ctx, a, project.ID, ticketRef, branch)
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "  %s La MR sera créée par l'agent en session, ou manuellement.\n",
		common.Subtitle.Render(common.IconInfo))
	fmt.Fprintf(a.IO.Out, "  %s Le merge reste TOUJOURS une action manuelle du développeur.\n",
		common.WarningStyle.Render(common.IconWarning))

	return nil
}

func getPublishBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func emitReviewReadyEvent(ctx context.Context, a *app.App, projectID, ticket, branch string) {
	statePath := a.Config.Team.StatePath
	if statePath == "" {
		statePath = config.DefaultTeamStatePath()
	}
	repo := teamstate.NewRepo(a.Config.Team.StateRepo, statePath)
	if !repo.IsCloned() {
		return
	}

	event := teamstate.Event{
		Timestamp: time.Now().UTC(),
		Actor:     a.Config.Team.MemberID,
		Type:      teamstate.EventReviewReady,
		Project:   projectID,
		Ticket:    ticket,
		Data:      map[string]interface{}{"branch": branch},
	}

	_ = repo.AppendEvent(ctx, event)

	// Notify
	if teamCfg, err := repo.LoadConfig(); err == nil {
		d := notify.NewDispatcher(teamCfg)
		_ = d.Dispatch(ctx, event)
	}
}

func init() {
	rootCmd.AddCommand(auditCmd)
	auditCmd.Flags().StringP("project", "j", "", "Nom du projet")
	auditCmd.Flags().StringP("type", "t", "security", "Type d'audit (security, performance, architecture, accessibility, ecodesign, observability, privacy)")
	_ = auditCmd.RegisterFlagCompletionFunc("project", completeProjectIDs)

	rootCmd.AddCommand(reviewCmd)
	reviewCmd.Flags().StringP("project", "j", "", "Nom du projet")
	reviewCmd.Flags().StringP("mode", "m", "", "Mode de review (standard, adversarial, edge-case, standard+adversarial, all)")
	reviewCmd.Flags().Bool("publish", false, "Créer une MR sur GitLab et assigner un reviewer (nécessite write_enabled)")
	_ = reviewCmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	_ = reviewCmd.RegisterFlagCompletionFunc("mode", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"standard", "adversarial", "edge-case", "standard+adversarial", "all"}, cobra.ShellCompDirectiveNoFileComp
	})

	rootCmd.AddCommand(debugCmd)
	debugCmd.Flags().StringP("project", "j", "", "Nom du projet")
	debugCmd.Flags().StringP("issue", "i", "", "Description du problème")
	_ = debugCmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
}
