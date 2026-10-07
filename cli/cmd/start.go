package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Lance une session (alias de oh run, déprécié)",
	Long:  i18n.T("cmd.start.long"),
	RunE:  runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
	addStartFlags(startCmd)
	_ = startCmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
}

// addStartFlags registers the flags of oh start.
func addStartFlags(startCmd *cobra.Command) {
	startCmd.Flags().StringP("agent", "a", "", "Agent à utiliser")
	startCmd.Flags().StringP("prompt", "m", "", "Prompt initial")
	startCmd.Flags().StringP("provider", "P", "", "Provider LLM (bedrock, anthropic, openai)")
	startCmd.Flags().StringP("project", "p", "", "Nom du projet (détection auto sinon)")
	startCmd.Flags().StringP("resume", "r", "", "Reprendre une session existante (ID)")
	startCmd.Flags().StringP("worktree", "w", "", "Branche pour lancer dans un git worktree")
	startCmd.Flags().Bool("dev", false, "Mode développement (orchestrator-dev + tickets)")
	startCmd.Flags().StringP("ticket", "t", "", "Ticket ID à travailler directement (skip le picker, requiert --dev)")
	startCmd.Flags().StringP("label", "l", "", "Filtrer tickets par label (requiert --dev)")
	startCmd.Flags().StringP("assignee", "A", "", "Filtrer tickets par assignee (requiert --dev)")
	startCmd.Flags().Bool("onboard", false, "Mode onboarding — crée/enrichit le wiki projet")
	startCmd.Flags().Bool("refresh", false, "Force la re-découverte du wiki (requiert --onboard)")
	startCmd.Flags().Bool("recap", false, "Afficher le récap et demander confirmation avant le lancement")
	startCmd.Flags().BoolP("yes", "y", false, "Deprecated: le lancement rapide est le défaut. Utilisez --recap pour forcer le récap.")
	startCmd.Flags().Bool("parallel", false, "Lance N sessions en parallèle sur des tickets différents")
	startCmd.Flags().StringSlice("tickets", nil, "Liste des tickets à traiter en parallèle (séparés par des virgules)")
	startCmd.Flags().Int("max-sessions", 0, "Nombre max de sessions parallèles (0 = valeur config, default: 3)")
	startCmd.Flags().String("priority", "", "Ticket prioritaire (merge en premier)")

	// --- Sweep mode flags ---
	startCmd.Flags().String("sweep", "", "Objectif sweep haut niveau (active le mode sweep)")
	startCmd.Flags().String("sweep-strategy", "", "Stratégie de décomposition: manual, by-file, by-package, llm (obligatoire)")
	startCmd.Flags().StringSlice("sweep-tasks", nil, "Liste manuelle de tâches (requiert --sweep-strategy=manual)")
	startCmd.Flags().StringSlice("sweep-include", nil, "Glob patterns à inclure")
	startCmd.Flags().StringSlice("sweep-exclude", nil, "Glob patterns à exclure")
	startCmd.Flags().String("sweep-verify", "none", "Vérification post-sweep: none, tests, lint, build, all, custom")
	startCmd.Flags().String("sweep-verify-cmd", "", "Commande de vérification custom (requiert --sweep-verify=custom)")
	startCmd.Flags().Bool("sweep-dry-run", false, "Afficher le plan décomposé sans exécuter")
	startCmd.Flags().String("sweep-branch-prefix", "sweep/", "Préfixe des branches sweep")

	// Mark --yes as deprecated (no-op with warning)
	_ = startCmd.Flags().MarkDeprecated("yes", "le lancement rapide est le défaut. Utilisez --recap pour forcer le récap.")
	// Options of the former parallel and sweep launches, without equivalent
	// in the workflows (one server group, the sweep conductor plans itself).
	for _, name := range []string{"max-sessions", "priority", "sweep-branch-prefix"} {
		_ = startCmd.Flags().MarkDeprecated(name, "sans effet depuis oh v5 (oh run ticket / oh run sweep).")
	}
}

func runStart(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()
	if err := requireV2(ctx); err != nil {
		return err
	}

	// --- Resume: attach to an existing session (resumed when asleep) ---
	if resumeID, _ := cmd.Flags().GetString("resume"); resumeID != "" {
		warnDeprecatedAlias(cmd.ErrOrStderr(), "oh start --resume", "oh session attach "+resumeID+" --how here")
		id, err := resolveSessionRef(ctx, resumeID)
		if err != nil {
			return err
		}
		svc, err := newRunService(ctx, a)
		if err != nil {
			return err
		}
		return runAttachChild(ctx, a, svc, id)
	}

	// --- Validate flag combinations ---
	f := cmd.Flags()
	devMode, _ := f.GetBool("dev")
	onboardMode, _ := f.GetBool("onboard")
	parallelMode, _ := f.GetBool("parallel")
	sweepMode := f.Changed("sweep")
	labelFlag, _ := f.GetString("label")
	assigneeFlag, _ := f.GetString("assignee")
	refreshFlag, _ := f.GetBool("refresh")
	agent, _ := f.GetString("agent")

	modeCount := 0
	for _, on := range []bool{parallelMode, sweepMode, devMode, onboardMode} {
		if on {
			modeCount++
		}
	}
	if modeCount > 1 {
		return fmt.Errorf("les modes --parallel, --sweep, --dev et --onboard sont mutuellement exclusifs")
	}
	if agent != "" && modeCount > 0 {
		return errors.New(i18n.T("cmd.v1.unsupported.agent_with_mode"))
	}
	if tickets, _ := f.GetStringSlice("tickets"); parallelMode && len(tickets) == 0 {
		return errors.New(i18n.T("cmd.v1.unsupported.parallel_tickets"))
	}
	if labelFlag != "" && !devMode {
		return fmt.Errorf("%s", i18n.Tf("cmd.start.flag_requires_dev", "label"))
	}
	if assigneeFlag != "" && !devMode {
		return fmt.Errorf("%s", i18n.Tf("cmd.start.flag_requires_dev", "assignee"))
	}
	if labelFlag != "" && assigneeFlag != "" {
		return fmt.Errorf("%s", i18n.T("cmd.start.label_assignee_exclusive"))
	}
	if refreshFlag && !onboardMode {
		return fmt.Errorf("%s", i18n.T("cmd.start.refresh_requires_onboard"))
	}

	// --- Workflow aliases (v5): oh start → oh run <workflow> ---
	return startAlias(cmd, a)
}
