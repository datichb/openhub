package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

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
				fmt.Fprintf(a.IO.Out, "  Aucun skill correspondant à %q\n", query)
			} else {
				fmt.Fprintf(a.IO.Out, "\n  %d skill(s) trouvé(s). Utilisez 'oh skill add <nom>' pour installer.\n", count)
			}
			return nil
		},
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
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
