package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var takeoverBriefCmd = &cobra.Command{
	Use:     "takeover-brief",
	Aliases: []string{"tb"},
	Short:   "Gestion des briefs de reprise de ticket",
	Long: `Affiche, liste et enrichit les briefs générés lors de transferts de tickets.
Un brief contient le contexte nécessaire pour reprendre le travail d'un collègue.`,
}

var takeoverBriefShowCmd = &cobra.Command{
	Use:   "show <ticket-id>",
	Short: "Affiche le brief de reprise d'un ticket",
	Args:  cobra.ExactArgs(1),
	RunE:  runTakeoverBriefShow,
}

var takeoverBriefListCmd = &cobra.Command{
	Use:   "list",
	Short: "Liste les briefs de reprise existants",
	RunE:  runTakeoverBriefList,
}

var takeoverBriefEnrichCmd = &cobra.Command{
	Use:   "enrich <ticket-id>",
	Short: "Enrichit un brief avec l'IA (lecture du code, analyse)",
	Long: `Lance un agent IA en mode headless pour enrichir le brief existant
avec une analyse du code source, des questions ouvertes identifiées,
et des recommandations pour la suite du travail.`,
	Args: cobra.ExactArgs(1),
	RunE: runTakeoverBriefEnrich,
}

func init() {
	rootCmd.AddCommand(takeoverBriefCmd)
	takeoverBriefCmd.AddCommand(takeoverBriefShowCmd)
	takeoverBriefCmd.AddCommand(takeoverBriefListCmd)
	takeoverBriefCmd.AddCommand(takeoverBriefEnrichCmd)

	takeoverBriefShowCmd.Flags().StringP("project", "p", "", "Project name")
	takeoverBriefListCmd.Flags().StringP("project", "p", "", "Project name")
	takeoverBriefEnrichCmd.Flags().StringP("project", "p", "", "Project name")
}

func runTakeoverBriefShow(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo, err := ensureTeamRepo(ctx, a)
	if err != nil {
		return err
	}

	ticketID := args[0]
	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		project = detectCurrentProject(ctx, a)
		if project == "" {
			return fmt.Errorf("impossible de détecter le projet courant. Utilise --project")
		}
	}

	content, err := repo.ReadBrief(project, ticketID)
	if err != nil {
		if err == teamstate.ErrBriefNotFound {
			fmt.Fprintf(a.IO.Out, "\n%s Aucun brief trouvé pour %s/%s\n\n",
				common.Subtitle.Render(common.IconInfo), project, ticketID)
			return nil
		}
		return fmt.Errorf("reading brief: %w", err)
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, content)
	return nil
}

func runTakeoverBriefList(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo, err := ensureTeamRepo(ctx, a)
	if err != nil {
		return err
	}

	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		project = detectCurrentProject(ctx, a)
		if project == "" {
			return fmt.Errorf("impossible de détecter le projet courant. Utilise --project")
		}
	}

	metas, err := repo.ListBriefs(project)
	if err != nil {
		return fmt.Errorf("listing briefs: %w", err)
	}

	fmt.Fprintln(a.IO.Out)
	if len(metas) == 0 {
		fmt.Fprintf(a.IO.Out, "%s Aucun brief de reprise pour %s\n\n",
			common.Subtitle.Render(common.IconInfo), project)
		return nil
	}

	fmt.Fprintln(a.IO.Out, common.Title.Render(fmt.Sprintf("  Takeover Briefs — %s  ", project)))
	fmt.Fprintln(a.IO.Out)

	for _, m := range metas {
		icon := common.Subtitle.Render(common.IconArrow)
		fmt.Fprintf(a.IO.Out, "  %s %s  %s → %s  (%s, %s)\n",
			icon,
			common.Bold.Render(m.TicketID),
			m.TransferredFrom,
			m.TransferredTo,
			m.Reason,
			m.TransferDate.Format("02 Jan 2006"))
	}
	fmt.Fprintln(a.IO.Out)
	return nil
}

func runTakeoverBriefEnrich(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo, err := ensureTeamRepo(ctx, a)
	if err != nil {
		return err
	}

	ticketID := args[0]
	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		project = detectCurrentProject(ctx, a)
		if project == "" {
			return fmt.Errorf("impossible de détecter le projet courant. Utilise --project")
		}
	}

	// Read the existing brief
	content, err := repo.ReadBrief(project, ticketID)
	if err != nil {
		if err == teamstate.ErrBriefNotFound {
			return fmt.Errorf("aucun brief trouvé pour %s/%s. Effectue d'abord un transfert", project, ticketID)
		}
		return err
	}

	// Find the project path for the headless run
	p, err := a.Projects.GetByName(ctx, project)
	if err != nil {
		return fmt.Errorf("projet %s introuvable dans le hub: %w", project, err)
	}

	fmt.Fprintf(a.IO.Out, "\n%s Enrichissement du brief via IA...\n",
		common.Subtitle.Render(common.IconArrow))

	prompt := fmt.Sprintf(`Voici un brief de reprise de ticket. Enrichis-le en :
1. Lisant les fichiers mentionnés pour comprendre l'état du code
2. Identifiant les questions ouvertes (TODO, FIXME, patterns incomplets)
3. Identifiant les risques (tests manquants, edge cases non couverts)
4. Proposant les prochaines étapes concrètes

Brief existant :
---
%s
---

Produis un Markdown structuré complet avec les sections :
## Contexte et décisions architecturales
## Questions ouvertes
## Risques identifiés
## Prochaines étapes recommandées`, content)

	output, err := opencode.RunHeadless(opencode.HeadlessOpts{
		ProjectPath: p.Path,
		Agent:       "brief-enricher",
		Prompt:      prompt,
	})
	if err != nil {
		fmt.Fprintf(a.IO.Out, "%s L'enrichissement a échoué: %v\n",
			common.WarningStyle.Render(common.IconWarning), err)
		return nil
	}

	// Save the enriched version
	enrichedContent := fmt.Sprintf("# Takeover Brief (enrichi): %s\n\n%s", ticketID, output)
	enrichedPath := repo.Path() + "/projects/" + project + "/takeover-briefs/"

	// Find the latest brief file to derive the enriched filename
	entries, _ := readDirSafe(enrichedPath)
	var latestBase string
	for _, e := range entries {
		name := e.Name()
		if len(name) > len(ticketID)+1 && name[:len(ticketID)] == ticketID && hasSuffix(name, ".md") && !hasSuffix(name, ".enriched.md") {
			latestBase = name[:len(name)-3] // strip .md
		}
	}

	if latestBase == "" {
		return fmt.Errorf("impossible de trouver le fichier brief de base")
	}

	enrichedFile := enrichedPath + latestBase + ".enriched.md"
	if err := writeFile(enrichedFile, []byte(enrichedContent)); err != nil {
		return fmt.Errorf("écriture du brief enrichi: %w", err)
	}

	// Commit and push
	relPath := "projects/" + project + "/takeover-briefs/" + latestBase + ".enriched.md"
	_ = repo.CommitAndPush(ctx, fmt.Sprintf("takeover: enriched brief for %s/%s", project, ticketID), relPath)

	fmt.Fprintf(a.IO.Out, "%s Brief enrichi sauvegardé. %s\n\n",
		common.SuccessStyle.Render(common.IconSuccess),
		common.Subtitle.Render("oh takeover-brief show "+ticketID))
	return nil
}

// helpers for the enrich command
func readDirSafe(path string) ([]dirEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	result := make([]dirEntry, len(entries))
	for i, e := range entries {
		result[i] = dirEntry{name: e.Name()}
	}
	return result, nil
}

type dirEntry struct {
	name string
}

func (d dirEntry) Name() string { return d.name }

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
