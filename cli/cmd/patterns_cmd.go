package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

var patternsCmd = &cobra.Command{
	Use:   "patterns",
	Short: "Gestion de la bibliothèque de patterns",
	Long: `Affiche, crée et gère les patterns de décomposition réutilisables.
Les patterns sont stockés dans le team-state et partagés entre tous les membres.`,
}

var patternsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Liste les patterns disponibles",
	RunE:  runPatternsList,
}

var patternsShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Affiche le contenu d'un pattern",
	Args:  cobra.ExactArgs(1),
	RunE:  runPatternsShow,
}

var patternsAddCmd = &cobra.Command{
	Use:   "add [file]",
	Short: "Ajoute un pattern (interactif ou depuis un fichier)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPatternsAdd,
}

var patternsValidateCmd = &cobra.Command{
	Use:   "validate <name>",
	Short: "Valide un pattern proposé par un agent",
	Args:  cobra.ExactArgs(1),
	RunE:  runPatternsValidate,
}

var patternsRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Supprime un pattern",
	Args:  cobra.ExactArgs(1),
	RunE:  runPatternsRemove,
}

func init() {
	rootCmd.AddCommand(patternsCmd)
	patternsCmd.AddCommand(patternsListCmd)
	patternsCmd.AddCommand(patternsShowCmd)
	patternsCmd.AddCommand(patternsAddCmd)
	patternsCmd.AddCommand(patternsValidateCmd)
	patternsCmd.AddCommand(patternsRemoveCmd)

	patternsListCmd.Flags().StringSlice("tags", nil, "Filter by tags")
	patternsListCmd.Flags().Bool("all", false, "Show unvalidated patterns too")
}

func runPatternsList(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo, err := ensureTeamRepo(ctx, a)
	if err != nil {
		return err
	}

	tags, _ := cmd.Flags().GetStringSlice("tags")
	showAll, _ := cmd.Flags().GetBool("all")

	minTags := 0
	if len(tags) > 0 {
		minTags = 1
	}

	patterns, err := repo.ListPatterns(tags, minTags)
	if err != nil {
		return fmt.Errorf("listing patterns: %w", err)
	}

	fmt.Fprintln(a.IO.Out)
	if len(patterns) == 0 {
		fmt.Fprintf(a.IO.Out, "%s Aucun pattern dans la bibliothèque.\n",
			theme.Subtitle.Render(theme.IconInfo))
		fmt.Fprintf(a.IO.Out, "  Utilise %s pour en créer un.\n\n",
			theme.Bold.Render("oh patterns add"))
		return nil
	}

	fmt.Fprintln(a.IO.Out, theme.Title.Render("  Patterns Library  "))
	fmt.Fprintln(a.IO.Out)

	for _, p := range patterns {
		if !showAll && !p.Validated {
			continue
		}

		icon := theme.SuccessStyle.Render(theme.IconSuccess)
		if !p.Validated {
			icon = theme.WarningStyle.Render(theme.IconWarning)
		}

		tagsStr := strings.Join(p.Tags, ", ")
		fmt.Fprintf(a.IO.Out, "  %s %s  [%s] [%s]\n",
			icon,
			theme.Bold.Render(p.Name),
			p.Complexity,
			tagsStr)

		meta := []string{}
		if p.Source != "" {
			meta = append(meta, "source:"+p.Source)
		}
		if p.Project != "" {
			meta = append(meta, "project:"+p.Project)
		}
		if !p.Validated {
			meta = append(meta, "awaiting validation")
		}
		if len(meta) > 0 {
			fmt.Fprintf(a.IO.Out, "    %s\n", theme.Subtitle.Render(strings.Join(meta, " · ")))
		}
		fmt.Fprintln(a.IO.Out)
	}

	return nil
}

func runPatternsShow(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo, err := ensureTeamRepo(ctx, a)
	if err != nil {
		return err
	}

	name := args[0]
	content, err := repo.ReadPattern(name)
	if err != nil {
		return fmt.Errorf("reading pattern: %w", err)
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, content)
	return nil
}

func runPatternsAdd(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo, err := ensureTeamRepo(ctx, a)
	if err != nil {
		return err
	}

	var (
		name       string
		tagsStr    string
		complexity string
		content    string
	)

	// If a file is provided, read content from it
	if len(args) == 1 {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("reading file %s: %w", args[0], err)
		}
		content = string(data)
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, theme.Title.Render("  Ajouter un pattern  "))
	fmt.Fprintln(a.IO.Out)

	steps := []views.WizardStep{
		{
			Label: "Pattern Info",
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddInputField("Nom du pattern (slug)", name, 0, nil,
					func(text string) { name = text })
				form.AddInputField("Tags (séparés par des virgules)", tagsStr, 0, nil,
					func(text string) { tagsStr = text })
				complexities := []string{"low", "medium", "high"}
				form.AddDropDown("Complexité", complexities, 0,
					func(_ string, idx int) { complexity = complexities[idx] })
				form.AddButton("Create", func() { onDone() })
				return form
			},
			OnDone: func() error {
				if name == "" {
					return fmt.Errorf("le nom est requis")
				}
				if strings.Contains(name, " ") {
					return fmt.Errorf("utilise des tirets, pas d'espaces")
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{
					{Label: "Name", Value: name},
					{Label: "Tags", Value: tagsStr},
					{Label: "Complexity", Value: complexity},
				}
			},
		},
	}

	wizResult := views.RunWizard(views.WizardConfig{
		Layout: layout.Config{
			ProjectName: a.Config.Name,
			Command:     "patterns add",
			StatusHints: i18n.T("wizard.hints.cancel"),
		},
		Steps: steps,
	})

	if wizResult.Aborted {
		return nil
	}
	if wizResult.Err != nil {
		return wizResult.Err
	}

	// Parse tags
	var tags []string
	for _, t := range strings.Split(tagsStr, ",") {
		if trimmed := strings.TrimSpace(t); trimmed != "" {
			tags = append(tags, trimmed)
		}
	}

	// If no content from file, open editor or ask
	if content == "" {
		content = fmt.Sprintf("# Pattern : %s\n\n## Contexte d'usage\n\n## Décomposition type\n\n## Dépendances typiques\n\n## Variantes connues\n", name)
		fmt.Fprintf(a.IO.Out, "\n%s Contenu par défaut créé. Édite le fichier après création pour le compléter.\n",
			theme.Subtitle.Render(theme.IconInfo))
	}

	project := detectCurrentProject(ctx, a)

	p := teamstate.Pattern{
		Name:       name,
		Tags:       tags,
		Complexity: complexity,
		Source:     "manual",
		Project:    project,
		Validated:  true, // manual additions are validated immediately
	}

	if err := repo.CreatePattern(ctx, p, content); err != nil {
		return fmt.Errorf("creating pattern: %w", err)
	}

	// Commit and push
	relIndex := "patterns/index.toml"
	relMd := "patterns/" + name + ".md"
	if err := repo.CommitAndPush(ctx, fmt.Sprintf("patterns: add %s", name), relIndex, relMd); err != nil {
		return fmt.Errorf("committing pattern: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "\n%s Pattern %q ajouté avec succès.\n",
		theme.SuccessStyle.Render(theme.IconSuccess), name)
	fmt.Fprintf(a.IO.Out, "  %s pour voir le contenu.\n\n",
		theme.Bold.Render("oh patterns show "+name))
	return nil
}

func runPatternsValidate(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo, err := ensureTeamRepo(ctx, a)
	if err != nil {
		return err
	}

	name := args[0]
	if err := repo.ValidatePattern(ctx, name); err != nil {
		return fmt.Errorf("validating pattern: %w", err)
	}

	// Commit and push
	if err := repo.CommitAndPush(ctx, fmt.Sprintf("patterns: validate %s", name), "patterns/index.toml"); err != nil {
		return fmt.Errorf("committing validation: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "%s Pattern %q validé.\n",
		theme.SuccessStyle.Render(theme.IconSuccess), name)
	return nil
}

func runPatternsRemove(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo, err := ensureTeamRepo(ctx, a)
	if err != nil {
		return err
	}

	name := args[0]
	if err := repo.RemovePattern(ctx, name); err != nil {
		return fmt.Errorf("removing pattern: %w", err)
	}

	// Commit and push
	if err := repo.CommitAndPush(ctx, fmt.Sprintf("patterns: remove %s", name), "patterns/index.toml"); err != nil {
		return fmt.Errorf("committing removal: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "%s Pattern %q supprimé.\n",
		theme.SuccessStyle.Render(theme.IconSuccess), name)
	return nil
}
