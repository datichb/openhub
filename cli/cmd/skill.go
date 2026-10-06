package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/skillregistry"
	"github.com/datichb/openhub/cli/internal/tui/progress"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Gestion des skills communautaires",
	Long: `Installe, liste et supprime des skills communautaires depuis le marketplace.

Exemples:
  oh skill list                      Liste les skills installés
  oh skill add golang-idioms         Installe depuis l'index communautaire
  oh skill add https://github.com/u/oh-skill-example  Depuis une URL Git
  oh skill remove golang-idioms      Désinstalle un skill
  oh skill search go                 Recherche dans l'index communautaire`,
}

func init() {
	rootCmd.AddCommand(skillCmd)
	skillCmd.AddCommand(skillListCmd())
	skillCmd.AddCommand(skillAddCmd())
	skillCmd.AddCommand(skillRemoveCmd())
	skillCmd.AddCommand(skillSearchCmd())
	skillCmd.AddCommand(skillBudgetCmd())
	skillCmd.AddCommand(skillCheckCmd())
}

func skillListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Liste les skills communautaires installés",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			reg := skillregistry.NewRegistry()

			skills, err := reg.ListInstalled()
			if err != nil {
				return fmt.Errorf("listant les skills: %w", err)
			}

			if len(skills) == 0 {
				fmt.Fprintln(a.IO.Out, theme.Subtitle.Render("  Aucun skill communautaire installé."))
				fmt.Fprintln(a.IO.Out, "  Utilisez 'oh skill add <nom>' pour en installer.")
				return nil
			}

			fmt.Fprintln(a.IO.Out, theme.Bold.Render("  Skills communautaires installés"))
			fmt.Fprintln(a.IO.Out)

			w := tabwriter.NewWriter(a.IO.Out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  Nom\tVersion\tDescription")
			for _, s := range skills {
				fmt.Fprintf(w, "  %s\t%s\t%s\n", s.Manifest.Name, s.Manifest.Version, s.Manifest.Description)
			}
			w.Flush()
			fmt.Fprintln(a.IO.Out)
			fmt.Fprintf(a.IO.Out, theme.Subtitle.Render("  Répertoire: %s\n"), skillregistry.NewRegistry())
			return nil
		},
	}
}

func skillAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <source>",
		Short: "Installe un skill depuis l'index ou une URL Git",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			source := args[0]
			reg := skillregistry.NewRegistry()

			var skill *skillregistry.InstalledSkill
			err := progress.Run(
				fmt.Sprintf("Installation de %q...", source),
				func() error {
					var e error
					skill, e = reg.Install(source)
					return e
				},
			)
			if err != nil {
				return fmt.Errorf("installation échouée: %w", err)
			}

			fmt.Fprintf(a.IO.Out, "%s Skill %q v%s installé dans %s\n",
				theme.SuccessStyle.Render(theme.IconSuccess),
				skill.Manifest.Name, skill.Manifest.Version, skill.Path)
			fmt.Fprintln(a.IO.Out, theme.Subtitle.Render("  Relancez 'oh deploy' pour déployer le skill sur vos projets."))
			return nil
		},
	}
}

func skillRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Désinstalle un skill communautaire",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			name := args[0]
			reg := skillregistry.NewRegistry()

			if err := reg.Remove(name); err != nil {
				return err
			}

			fmt.Fprintf(a.IO.Out, "%s Skill %q désinstallé.\n",
				theme.SuccessStyle.Render(theme.IconSuccess), name)
			return nil
		},
	}
}

func skillSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search [query]",
		Short: "Recherche dans l'index communautaire",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()

			var entries []skillregistry.IndexEntry
			err := progress.Run(
				"Récupération de l'index communautaire...",
				func() error {
					var e error
					entries, e = skillregistry.FetchIndex()
					return e
				},
			)
			if err != nil {
				return fmt.Errorf("impossible de récupérer l'index: %w", err)
			}

			query := ""
			if len(args) > 0 {
				query = args[0]
			}

			fmt.Fprintln(a.IO.Out)
			fmt.Fprintln(a.IO.Out, theme.Bold.Render("  Skills disponibles"))
			fmt.Fprintln(a.IO.Out)

			w := tabwriter.NewWriter(a.IO.Out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  Nom\tDescription\tTags")
			count := 0
			for _, e := range entries {
				if query != "" {
					match := false
					for _, tag := range e.Tags {
						if contains(tag, query) {
							match = true
							break
						}
					}
					if !match && !contains(e.Name, query) && !contains(e.Description, query) {
						continue
					}
				}
				tags := ""
				if len(e.Tags) > 0 {
					tags = join(e.Tags, ", ")
				}
				fmt.Fprintf(w, "  %s\t%s\t%s\n", e.Name, e.Description, tags)
				count++
			}
			w.Flush()

			if count == 0 {
				fmt.Fprintf(a.IO.Out, "%s", i18n.Tf("cmd.skill.no_match", query))
			} else {
				fmt.Fprintf(a.IO.Out, "\n  %d skill(s) trouvé(s). Utilisez 'oh skill add <nom>' pour installer.\n", count)
			}
			return nil
		},
	}
}

func contains(s, substr string) bool {
	return s != "" && substr != "" &&
		(s == substr || len(s) >= len(substr) &&
			(s[:len(substr)] == substr ||
				containsSubstr(s, substr)))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func join(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += sep + s
	}
	return result
}

func skillBudgetCmd() *cobra.Command {
	var allAgents bool
	var threshold int

	cmd := &cobra.Command{
		Use:   "budget [agent-name]",
		Short: "Affiche le budget context window par agent (Bucket A + body)",
		Long: `Calcule le coût en lignes et tokens du system prompt toujours chargé
pour un agent ou tous les agents. Identifie les skills les plus coûteux.

Exemples:
  oh skill budget orchestrator-dev    Budget d'un agent
  oh skill budget --all               Budget de tous les agents
  oh skill budget --all --threshold 200  Flag les skills > 200 lignes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && newWorkflowService(cmd.Context()).Has(cmd.Context(), workflowsvc.Context{}, args[0]) {
				warnDeprecatedAlias(cmd.ErrOrStderr(), "oh skill budget "+args[0], "oh bundle show "+args[0]+" --budget")
				show := bundleShowCmd()
				show.SetContext(cmd.Context())
				show.SetOut(cmd.OutOrStdout())
				show.SetErr(cmd.ErrOrStderr())
				_ = show.Flags().Set("budget", "true")
				return show.RunE(show, args)
			}
			warnDeprecatedAlias(cmd.ErrOrStderr(), "oh skill budget", "oh bundle show <workflow> --budget")
			hubDir := findHubDir()
			if hubDir == "" {
				return fmt.Errorf("hub directory not found")
			}
			agentsDir := hubDir + "/agents"
			skillsDir := hubDir + "/skills"

			if allAgents || len(args) == 0 {
				return runBudgetAll(agentsDir, skillsDir, threshold)
			}
			return runBudgetSingle(agentsDir, skillsDir, args[0], threshold)
		},
	}

	cmd.Flags().BoolVarP(&allAgents, "all", "a", false, "Afficher le budget de tous les agents")
	cmd.Flags().IntVarP(&threshold, "threshold", "t", 150, "Seuil en lignes pour flaguer un skill (défaut: 150)")

	return cmd
}

func runBudgetAll(agentsDir, skillsDir string, threshold int) error {
	budgets, err := deploy.ComputeAllBudgets(agentsDir, skillsDir)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "\n%s\n\n", theme.Title.Render("  Skill Budget — All Agents  "))
	fmt.Fprintf(w, "AGENT\tMODE\tBODY\tBUCKET A\tTOTAL\t~TOKENS\t%% of 200K\n")
	fmt.Fprintf(w, "─────\t────\t────\t────────\t─────\t───────\t────────\n")

	for _, b := range budgets {
		total := b.BodyLines + b.TotalALines
		totalTokens := b.BodyTokens + b.TotalATokens
		pct := float64(totalTokens) / 200000.0 * 100.0
		fmt.Fprintf(w, "%s\t%s\t%d\t%d (%d skills)\t%d\t~%d\t%.1f%%\n",
			b.AgentID, b.AgentMode, b.BodyLines, b.TotalALines, len(b.BucketA), total, totalTokens, pct)
	}
	w.Flush()
	fmt.Println()
	return nil
}

func runBudgetSingle(agentsDir, skillsDir, agentName string, threshold int) error {
	// Find agent file by ID
	budgets, err := deploy.ComputeAllBudgets(agentsDir, skillsDir)
	if err != nil {
		return err
	}

	var budget *deploy.AgentBudget
	for _, b := range budgets {
		if b.AgentID == agentName {
			budget = b
			break
		}
	}
	if budget == nil {
		return fmt.Errorf("agent %q not found", agentName)
	}

	fmt.Printf("\n%s\n\n", theme.Title.Render(fmt.Sprintf("  Skill Budget — %s  ", budget.AgentID)))
	fmt.Printf("Agent: %s (mode: %s)\n\n", budget.AgentID, budget.AgentMode)

	fmt.Println("Bucket A (always inlined):")
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	for _, e := range budget.BucketA {
		flag := ""
		if e.Lines > threshold {
			flag = fmt.Sprintf("  ⚠️ >%dL", threshold)
		}
		fmt.Fprintf(w, "  %s\t%d lines\t~%d tokens%s\n", e.SkillRef, e.Lines, e.Tokens, flag)
	}
	fmt.Fprintf(w, "  ─────────────────────────────────\t\t\n")
	fmt.Fprintf(w, "  Total Bucket A:\t%d lines\t~%d tokens\n", budget.TotalALines, budget.TotalATokens)
	w.Flush()

	fmt.Printf("\nAgent body:\t\t\t%d lines\t~%d tokens\n", budget.BodyLines, budget.BodyTokens)

	totalLines := budget.BodyLines + budget.TotalALines
	totalTokens := budget.BodyTokens + budget.TotalATokens
	pct := float64(totalTokens) / 200000.0 * 100.0
	fmt.Printf("─────────────────────────────────────────────────────\n")
	fmt.Printf("TOTAL ALWAYS-LOADED:\t\t%d lines\t~%d tokens (%.1f%% of 200K)\n", totalLines, totalTokens, pct)

	if len(budget.BucketB) > 0 {
		fmt.Printf("\nBucket B (on-demand, not counted):\n")
		for _, e := range budget.BucketB {
			fmt.Printf("  %s (%d lines)\n", e.SkillRef, e.Lines)
		}
		fmt.Printf("  %d skills available on-demand\n", len(budget.BucketB))
	}

	fmt.Println()
	return nil
}
