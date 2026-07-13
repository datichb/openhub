package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var policiesCmd = &cobra.Command{
	Use:   "policies",
	Short: "Gestion des policies d'équipe",
	Long: `Affiche, vérifie et gère les règles d'équipe appliquées à tous les projets.
Les policies sont stockées dans le team-state (policies.toml) et peuvent
être overridées par projet (policies-override.toml).`,
}

var policiesListCmd = &cobra.Command{
	Use:   "list",
	Short: "Affiche les policies actives",
	RunE:  runPoliciesList,
}

var policiesCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Vérifie les policies contre l'état courant",
	RunE:  runPoliciesCheck,
}

var policiesAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Ajoute une policy custom",
	RunE:  runPoliciesAdd,
}

func init() {
	rootCmd.AddCommand(policiesCmd)
	policiesCmd.AddCommand(policiesListCmd)
	policiesCmd.AddCommand(policiesCheckCmd)
	policiesCmd.AddCommand(policiesAddCmd)

	policiesListCmd.Flags().StringP("project", "p", "", "Project name for merged view (optional)")
	policiesCheckCmd.Flags().StringP("project", "p", "", "Project name")
	policiesCheckCmd.Flags().String("branch", "", "Branch name to check")
	policiesCheckCmd.Flags().String("commit", "", "Commit message to check")
}

func runPoliciesList(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo, err := ensureTeamRepo(ctx, a)
	if err != nil {
		return err
	}

	project, _ := cmd.Flags().GetString("project")

	policies, err := repo.LoadPolicies(project)
	if err != nil {
		return fmt.Errorf("loading policies: %w", err)
	}

	if len(policies) == 0 {
		fmt.Fprintf(a.IO.Out, "\n%s Aucune policy configurée.\n",
			common.Subtitle.Render(common.IconInfo))
		fmt.Fprintf(a.IO.Out, "  Crée %s dans le repo team-state.\n\n",
			common.Bold.Render("policies.toml"))
		return nil
	}

	fmt.Fprintln(a.IO.Out)
	title := "  Team Policies  "
	if project != "" {
		title = fmt.Sprintf("  Team Policies (%s)  ", project)
	}
	fmt.Fprintln(a.IO.Out, common.Title.Render(title))
	fmt.Fprintln(a.IO.Out)

	for _, p := range policies {
		icon := common.SuccessStyle.Render(common.IconSuccess)
		enfStr := common.Subtitle.Render("warn")
		if p.Enforcement == teamstate.EnforcementRefuse {
			enfStr = common.ErrorStyle.Render("refuse")
		}

		typeStr := string(p.Type)
		fmt.Fprintf(a.IO.Out, "  %s %s  [%s] [%s]\n",
			icon,
			common.Bold.Render(p.Name),
			typeStr,
			enfStr)

		if p.Message != "" {
			fmt.Fprintf(a.IO.Out, "    %s\n", p.Message)
		}

		// Show details based on type
		switch p.Type {
		case teamstate.PolicyTypeRegex:
			if p.Rule != "" {
				fmt.Fprintf(a.IO.Out, "    rule: %s\n", p.Rule)
			}
		case teamstate.PolicyTypeLimit:
			fmt.Fprintf(a.IO.Out, "    max: %d", p.Max)
			if p.Unit != "" {
				fmt.Fprintf(a.IO.Out, " %s", p.Unit)
			}
			fmt.Fprintln(a.IO.Out)
		case teamstate.PolicyTypeForbiddenPattern:
			fmt.Fprintf(a.IO.Out, "    patterns: %s\n", strings.Join(p.Patterns, ", "))
			if p.Scope != "" {
				fmt.Fprintf(a.IO.Out, "    scope: %s\n", p.Scope)
			}
		}
		fmt.Fprintln(a.IO.Out)
	}

	return nil
}

func runPoliciesCheck(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo, err := ensureTeamRepo(ctx, a)
	if err != nil {
		return err
	}

	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		project = detectCurrentProject(ctx, a)
	}

	branch, _ := cmd.Flags().GetString("branch")
	commitMsg, _ := cmd.Flags().GetString("commit")

	// Build context from flags and current state
	memberID := a.Config.Team.MemberID
	activeClaims := 0
	if memberID != "" {
		claims, _ := repo.ListClaims("")
		for _, c := range claims {
			if c.ClaimedBy == memberID {
				activeClaims++
			}
		}
	}

	policyCtx := teamstate.PolicyContext{
		BranchName:    branch,
		CommitMessage: commitMsg,
		MemberID:      memberID,
		ActiveClaims:  activeClaims,
	}

	violations, err := repo.CheckAll(project, policyCtx)
	if err != nil {
		return fmt.Errorf("checking policies: %w", err)
	}

	fmt.Fprintln(a.IO.Out)
	if len(violations) == 0 {
		fmt.Fprintf(a.IO.Out, "%s Toutes les policies sont respectées.\n\n",
			common.SuccessStyle.Render(common.IconSuccess))
		return nil
	}

	fmt.Fprintln(a.IO.Out, common.Title.Render("  Policy Violations  "))
	fmt.Fprintln(a.IO.Out)

	for _, v := range violations {
		icon := common.WarningStyle.Render(common.IconWarning)
		if v.Enforcement == teamstate.EnforcementRefuse {
			icon = common.ErrorStyle.Render(common.IconWarning)
		}
		fmt.Fprintf(a.IO.Out, "  %s %s [%s]\n", icon, common.Bold.Render(v.Name), v.Enforcement)
		if v.Message != "" {
			fmt.Fprintf(a.IO.Out, "    %s\n", v.Message)
		}
		if v.Details != "" {
			fmt.Fprintf(a.IO.Out, "    %s\n", common.Subtitle.Render(v.Details))
		}
		fmt.Fprintln(a.IO.Out)
	}

	if teamstate.HasRefuseViolations(violations) {
		return fmt.Errorf("policy violations with enforcement=refuse detected")
	}
	return nil
}

func runPoliciesAdd(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo, err := ensureTeamRepo(ctx, a)
	if err != nil {
		return err
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, common.Title.Render("  Ajouter une policy custom  "))
	fmt.Fprintln(a.IO.Out)

	var (
		name        string
		policyType  string
		enforcement string
		message     string
		rule        string
		patterns    string
		scope       string
		maxVal      int
	)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Nom de la policy").
				Description("Préfixer par 'custom_' pour les règles personnalisées").
				Placeholder("custom_no_debugger").
				Value(&name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("le nom est requis")
					}
					return nil
				}),
			huh.NewSelect[string]().
				Title("Type de règle").
				Options(
					huh.NewOption("Regex (valide un pattern)", "regex"),
					huh.NewOption("Forbidden Pattern (interdit des patterns dans le diff)", "forbidden_pattern"),
					huh.NewOption("Limit (valeur maximale)", "limit"),
					huh.NewOption("Boolean (activé/désactivé)", "boolean"),
				).
				Value(&policyType),
			huh.NewSelect[string]().
				Title("Enforcement").
				Options(
					huh.NewOption("Warn (avertissement)", "warn"),
					huh.NewOption("Refuse (bloquant)", "refuse"),
				).
				Value(&enforcement),
			huh.NewInput().
				Title("Message (affiché en cas de violation)").
				Value(&message),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	// Type-specific form
	switch policyType {
	case "regex":
		regexForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Pattern regex").
					Placeholder(`^(feat|fix)/.+`).
					Value(&rule),
			),
		)
		if err := regexForm.Run(); err != nil {
			return err
		}
	case "forbidden_pattern":
		patternForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Patterns interdits (séparés par des virgules)").
					Placeholder("console.log, debugger, binding.pry").
					Value(&patterns),
				huh.NewSelect[string]().
					Title("Scope").
					Options(
						huh.NewOption("Diff seulement", "diff_only"),
						huh.NewOption("Fichiers modifiés", "modified_files"),
						huh.NewOption("Tous les fichiers", "all_files"),
					).
					Value(&scope),
			),
		)
		if err := patternForm.Run(); err != nil {
			return err
		}
	case "limit":
		limitForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Valeur maximale").
					Value(&rule), // reuse rule field temporarily
			),
		)
		if err := limitForm.Run(); err != nil {
			return err
		}
		fmt.Sscanf(rule, "%d", &maxVal)
	}

	// Build the TOML block to append
	var block strings.Builder
	block.WriteString(fmt.Sprintf("\n[policies.%s]\n", name))
	block.WriteString(fmt.Sprintf("type = %q\n", policyType))

	switch policyType {
	case "regex":
		block.WriteString(fmt.Sprintf("rule = %q\n", rule))
	case "forbidden_pattern":
		parts := strings.Split(patterns, ",")
		trimmed := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				trimmed = append(trimmed, fmt.Sprintf("%q", t))
			}
		}
		block.WriteString(fmt.Sprintf("patterns = [%s]\n", strings.Join(trimmed, ", ")))
		if scope != "" {
			block.WriteString(fmt.Sprintf("scope = %q\n", scope))
		}
	case "limit":
		block.WriteString(fmt.Sprintf("max = %d\n", maxVal))
	case "boolean":
		block.WriteString("enabled = true\n")
	}

	block.WriteString(fmt.Sprintf("enforcement = %q\n", enforcement))
	if message != "" {
		block.WriteString(fmt.Sprintf("message = %q\n", message))
	}

	// Append to policies.toml
	policiesPath := repo.Path() + "/policies.toml"
	f, err := openOrCreatePoliciesFile(policiesPath)
	if err != nil {
		return fmt.Errorf("opening policies.toml: %w", err)
	}
	if _, err := f.WriteString(block.String()); err != nil {
		f.Close()
		return fmt.Errorf("writing policy: %w", err)
	}
	f.Close()

	// Commit and push
	if err := repo.CommitAndPush(ctx, fmt.Sprintf("policies: add %s", name), "policies.toml"); err != nil {
		return fmt.Errorf("committing policy: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "\n%s Policy %q ajoutée avec succès.\n\n",
		common.SuccessStyle.Render(common.IconSuccess),
		name)
	return nil
}

func openOrCreatePoliciesFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
}
