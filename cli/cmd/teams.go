package cmd

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/components/floating"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var teamsCmd = &cobra.Command{
	Use:   "teams",
	Short: i18n.T("cmd.teams.short"),
	Long:  i18n.T("cmd.teams.long"),
}

var teamsListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T("cmd.teams.list.short"),
	RunE:  runTeamsList,
}

var teamsAddCmd = &cobra.Command{
	Use:   "add",
	Short: i18n.T("cmd.teams.add.short"),
	Long:  i18n.T("cmd.teams.add.long"),
	RunE:  runTeamsAdd,
}

var teamsRemoveCmd = &cobra.Command{
	Use:   "remove [team-id]",
	Short: i18n.T("cmd.teams.remove.short"),
	Long:  i18n.T("cmd.teams.remove.long"),
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamsRemove,
}

var teamsDetachCmd = &cobra.Command{
	Use:   "detach [project-name]",
	Short: "Détacher un projet de son équipe",
	Long:  "Détache un projet de son équipe actuelle sans supprimer la team. Le projet devient solo.",
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamsDetach,
}

var teamsArchiveCmd = &cobra.Command{
	Use:   "archive [team-id]",
	Short: "Archiver une équipe (désactiver sans supprimer)",
	Long:  "Désactive une équipe en préservant sa configuration. Le team-state local reste intact.\nUtilisez 'oh teams restore' pour réactiver.",
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamsArchive,
}

var teamsRestoreCmd = &cobra.Command{
	Use:   "restore [team-id]",
	Short: "Restaurer une équipe archivée",
	Long:  "Réactive une équipe précédemment archivée et synchronise son team-state.",
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamsRestore,
}

func init() {
	rootCmd.AddCommand(teamsCmd)
	teamsCmd.AddCommand(teamsListCmd)
	teamsCmd.AddCommand(teamsAddCmd)
	teamsCmd.AddCommand(teamsRemoveCmd)
	teamsCmd.AddCommand(teamsDetachCmd)
	teamsCmd.AddCommand(teamsArchiveCmd)
	teamsCmd.AddCommand(teamsRestoreCmd)

	teamsAddCmd.Flags().String("repo", "", "URL du repo team-state (git@... ou https://...)")
	teamsAddCmd.Flags().String("member-id", "", "Votre identifiant dans cette équipe")
	teamsAddCmd.Flags().String("id", "", "Identifiant local pour cette équipe (auto-dérivé du repo si omis)")
	teamsAddCmd.Flags().String("name", "", "Nom d'affichage de l'équipe (optionnel)")

	teamsRemoveCmd.Flags().BoolP("force", "f", false, "Supprimer sans demander de confirmation")
}

func runTeamsList(cmd *cobra.Command, _ []string) error {
	a := MustApp()
	_ = cmd.Context()

	teams := a.Config.Teams

	if len(teams) == 0 {
		fmt.Fprintf(a.IO.Out, "%s Aucune équipe configurée. Utilisez %s pour en ajouter une.\n",
			theme.Subtitle.Render(theme.IconInfo),
			theme.Bold.Render("oh teams add --repo <url> --member-id <id>"))
		return nil
	}

	w := tabwriter.NewWriter(a.IO.Out, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
		theme.Bold.Render("ID"),
		theme.Bold.Render("NOM"),
		theme.Bold.Render("MEMBRE"),
		theme.Bold.Render("REPO"),
		theme.Bold.Render("STATUT"))

	for _, t := range teams {
		status := theme.SuccessStyle.Render("actif")
		if !t.Enabled {
			status = theme.Subtitle.Render("inactif")
		}
		name := t.DisplayName()
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			t.ID, name, t.MemberID, truncateStr(t.StateRepo, 50), status)
	}
	w.Flush()

	// Show project count per team
	fmt.Fprintln(a.IO.Out)
	projects, err := a.Projects.List(context.Background(), "")
	if err == nil {
		teamProjects := make(map[string]int)
		soloCount := 0
		for _, p := range projects {
			if p.TeamID != nil && *p.TeamID != "" {
				teamProjects[*p.TeamID]++
			} else {
				soloCount++
			}
		}
		for _, t := range teams {
			count := teamProjects[t.ID]
			if count > 0 {
				fmt.Fprintf(a.IO.Out, "  %s: %d projet(s)\n", t.DisplayName(), count)
			}
		}
		if soloCount > 0 {
			fmt.Fprintf(a.IO.Out, "  %s: %d projet(s)\n",
				theme.Subtitle.Render("solo (pas d'équipe)"), soloCount)
		}
	}

	return nil
}

func runTeamsAdd(cmd *cobra.Command, _ []string) error {
	a := MustApp()

	repo, _ := cmd.Flags().GetString("repo")
	memberID, _ := cmd.Flags().GetString("member-id")
	id, _ := cmd.Flags().GetString("id")
	name, _ := cmd.Flags().GetString("name")

	if repo == "" {
		return fmt.Errorf("le flag --repo est requis (URL du repo team-state)")
	}
	if memberID == "" {
		return fmt.Errorf("le flag --member-id est requis (votre identifiant dans cette équipe)")
	}

	// Auto-derive ID from repo if not provided
	if id == "" {
		id = config.RepoNameFromRemote(repo)
	}

	// Check for duplicates
	if existing := a.Config.FindTeam(id); existing != nil {
		return fmt.Errorf("une équipe avec l'ID %q existe déjà (repo: %s)", id, existing.StateRepo)
	}
	if existing := a.Config.FindTeamByRepo(repo); existing != nil {
		return fmt.Errorf("ce repo est déjà configuré sous l'ID %q", existing.ID)
	}

	// Build the new team entry
	newTeam := config.TeamConfig{
		ID:        id,
		Name:      name,
		Enabled:   true,
		StateRepo: repo,
		StatePath: config.TeamStatePath(repo),
		MemberID:  memberID,
	}

	a.Config.Teams = append(a.Config.Teams, newTeam)

	if err := config.Save(a.Config); err != nil {
		return fmt.Errorf("sauvegarde config: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "%s Équipe %s ajoutée avec succès.\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		theme.Bold.Render(newTeam.DisplayName()))
	fmt.Fprintf(a.IO.Out, "  ID:       %s\n", id)
	fmt.Fprintf(a.IO.Out, "  Repo:     %s\n", repo)
	fmt.Fprintf(a.IO.Out, "  Membre:   %s\n", memberID)
	fmt.Fprintf(a.IO.Out, "\n  Utilisez %s pour synchroniser le team-state.\n",
		theme.Bold.Render("oh team init"))

	return nil
}

func runTeamsRemove(cmd *cobra.Command, args []string) error {
	a := MustApp()
	teamID := args[0]

	// Find the team
	team := a.Config.FindTeam(teamID)
	if team == nil {
		return fmt.Errorf("équipe %q non trouvée. Utilisez %s pour voir les équipes configurées.",
			teamID, theme.Bold.Render("oh teams list"))
	}

	// Check for projects still attached
	projects, err := a.Projects.List(context.Background(), "")
	if err == nil {
		var attached []string
		for _, p := range projects {
			if p.TeamID != nil && *p.TeamID == teamID {
				attached = append(attached, p.Name)
			}
		}
		if len(attached) > 0 {
			fmt.Fprintf(a.IO.Out, "%s %d projet(s) sont rattachés à cette équipe:\n",
				theme.WarningStyle.Render(theme.IconWarning), len(attached))
			for _, name := range attached {
				fmt.Fprintf(a.IO.Out, "  - %s\n", name)
			}
			fmt.Fprintf(a.IO.Out, "  Ces projets deviendront des projets solo.\n\n")
		}
	}

	// Confirmation prompt unless --force
	force, _ := cmd.Flags().GetBool("force")
	if !force {
		var confirm bool
		_ = floating.Run(floating.Config{
			Title: i18n.T("cmd.teams.remove.short"),
			Form: theme.NewForm(huh.NewGroup(huh.NewConfirm().
				Title(fmt.Sprintf("Supprimer l'équipe %q ?", teamID)).
				Value(&confirm))),
		})
		if !confirm {
			return nil
		}
	}

	// Remove from config
	newTeams := make([]config.TeamConfig, 0, len(a.Config.Teams)-1)
	for _, t := range a.Config.Teams {
		if t.ID != teamID {
			newTeams = append(newTeams, t)
		}
	}
	a.Config.Teams = newTeams

	if err := config.Save(a.Config); err != nil {
		return fmt.Errorf("sauvegarde config: %w", err)
	}

	// Detach projects from the removed team
	if projects != nil {
		ctx := context.Background()
		for i := range projects {
			if projects[i].TeamID != nil && *projects[i].TeamID == teamID {
				projects[i].TeamID = nil
				_ = a.Projects.Update(ctx, &projects[i])
			}
		}
	}

	fmt.Fprintf(a.IO.Out, "%s Équipe %s retirée.\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		theme.Bold.Render(teamID))

	return nil
}

// truncateStr truncates a string to maxLen, adding "..." if truncated.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// runTeamsDetach detaches a single project from its team.
func runTeamsDetach(_ *cobra.Command, args []string) error {
	a := MustApp()
	projectName := args[0]

	ctx := context.Background()
	project, err := a.Projects.GetByName(ctx, projectName)
	if err != nil {
		return fmt.Errorf("projet %q non trouvé: %w", projectName, err)
	}

	if project.TeamID == nil {
		fmt.Fprintf(a.IO.Out, "%s Le projet %s n'est rattaché à aucune équipe.\n",
			theme.Subtitle.Render(theme.IconWarning),
			theme.Bold.Render(projectName))
		return nil
	}

	teamID := *project.TeamID
	project.TeamID = nil
	if err := a.Projects.Update(ctx, project); err != nil {
		return fmt.Errorf("détachement du projet: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "%s Projet %s détaché de l'équipe %s.\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		theme.Bold.Render(projectName),
		theme.Bold.Render(teamID))
	return nil
}

// runTeamsArchive disables a team without removing it from config.
func runTeamsArchive(_ *cobra.Command, args []string) error {
	a := MustApp()
	teamID := args[0]

	team := a.Config.FindTeam(teamID)
	if team == nil {
		return fmt.Errorf("équipe %q non trouvée. Utilisez %s pour voir les équipes configurées.",
			teamID, theme.Bold.Render("oh teams list"))
	}

	if !team.Enabled {
		fmt.Fprintf(a.IO.Out, "%s L'équipe %s est déjà archivée.\n",
			theme.Subtitle.Render(theme.IconWarning),
			theme.Bold.Render(teamID))
		return nil
	}

	// Set enabled = false
	for i := range a.Config.Teams {
		if a.Config.Teams[i].ID == teamID {
			a.Config.Teams[i].Enabled = false
			break
		}
	}

	if err := config.Save(a.Config); err != nil {
		return fmt.Errorf("sauvegarde config: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "%s Équipe %s archivée. Utilisez %s pour la réactiver.\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		theme.Bold.Render(teamID),
		theme.Bold.Render("oh teams restore "+teamID))
	return nil
}

// runTeamsRestore re-enables an archived team.
func runTeamsRestore(_ *cobra.Command, args []string) error {
	a := MustApp()
	teamID := args[0]

	team := a.Config.FindTeam(teamID)
	if team == nil {
		return fmt.Errorf("équipe %q non trouvée. Utilisez %s pour voir les équipes configurées.",
			teamID, theme.Bold.Render("oh teams list"))
	}

	if team.Enabled {
		fmt.Fprintf(a.IO.Out, "%s L'équipe %s est déjà active.\n",
			theme.Subtitle.Render(theme.IconWarning),
			theme.Bold.Render(teamID))
		return nil
	}

	// Set enabled = true
	for i := range a.Config.Teams {
		if a.Config.Teams[i].ID == teamID {
			a.Config.Teams[i].Enabled = true
			break
		}
	}

	if err := config.Save(a.Config); err != nil {
		return fmt.Errorf("sauvegarde config: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "%s Équipe %s restaurée et active.\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		theme.Bold.Render(teamID))
	return nil
}
