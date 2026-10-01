package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/gitlabapi"
	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var reviewFeedbackCmd = &cobra.Command{
	Use:   "feedback <ticket-or-branch>",
	Short: i18n.T("cmd.review.feedback.short"),
	Long: `Récupère les commentaires non résolus d'une MR GitLab et lance une session
de correction pour traiter le feedback du reviewer humain.

L'argument peut être un identifiant de ticket (e.g. SRU-142) ou un nom de branche
(e.g. feat/SRU-142-auth-refactor). La MR est résolue automatiquement.`,
	Args: cobra.ExactArgs(1),
	RunE: runReviewFeedback,
}

func init() {
	reviewFeedbackCmd.Flags().StringP("project", "p", "", "Nom du projet")
	reviewFeedbackCmd.Flags().Bool("yes", false, "Passer la confirmation")
	_ = reviewFeedbackCmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
}

// runReviewFeedback fetches MR discussions and launches a feedback correction session.
func runReviewFeedback(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()
	ref := args[0]

	// ── 1. Resolve project ──
	projectID, _ := cmd.Flags().GetString("project")
	project, err := resolveProject(ctx, a, projectID)
	if err != nil {
		return err
	}

	// ── 2. Resolve GitLab credentials ──
	glToken := resolveGitLabToken(ctx, a)
	if glToken == "" {
		return fmt.Errorf("aucun token GitLab trouvé — configure via %s ou GITLAB_TOKEN",
			theme.Bold.Render("oh service setup"))
	}
	glURL := resolveGitLabURL(a)
	glProject := resolveGitLabProject(a, project)
	if glProject == "" {
		return fmt.Errorf("projet GitLab non configuré — ajoute %s dans la config tracker",
			theme.Bold.Render("tracker_project"))
	}

	gl := gitlabapi.NewClient(glURL, glToken)

	// ── 3. Resolve MR from ref ──
	mr, branch, err := resolveMRFromRef(ctx, gl, glProject, project.Path, ref)
	if err != nil {
		return err
	}
	if mr == nil {
		return fmt.Errorf("aucune MR ouverte trouvée pour %q", ref)
	}

	// ── 4. Fetch MR discussions ──
	discussions, err := gl.ListMRDiscussions(ctx, glProject, mr.IID, true)
	if err != nil {
		return fmt.Errorf("impossible de récupérer les discussions : %w", err)
	}

	// ── 5. Preview ──
	displayFeedbackPreview(a, mr, branch, discussions)

	if len(discussions) == 0 {
		fmt.Fprintf(a.IO.Out, "\n  %s Aucune discussion non résolue. Rien à corriger.\n",
			theme.SuccessStyle.Render(theme.IconSuccess))
		return nil
	}

	// ── 6. Confirm ──
	noConfirm, _ := cmd.Flags().GetBool("yes")
	if !noConfirm {
		fmt.Fprintf(a.IO.Out, "\n  Lancer la session de correction ? [Y/n] ")
		var resp string
		fmt.Scanln(&resp)
		resp = strings.TrimSpace(strings.ToLower(resp))
		if resp == "n" || resp == "no" || resp == "non" {
			return nil
		}
	}

	// ── 7. Build prompt and launch ──
	if err := ensureOpencode(a); err != nil {
		return err
	}

	prompt := buildFeedbackPrompt(mr, branch, discussions)
	fmt.Fprintf(a.IO.Out, "\n%s Lancement session feedback sur %s\n",
		theme.Title.Render("oh review feedback"), theme.Bold.Render(project.Name))

	l := launcher.New(a, launcher.NewCLIUI(a.IO.Out))
	return l.Launch(ctx, launcher.LaunchOpts{
		ProjectID:   project.ID,
		ProjectPath: project.Path,
		Agent:       "orchestrator-dev",
		Prompt:      prompt,
		SkipSummary: true,
		SkipConfirm: true,
		SkipDeploy:  false,
		DeployFunc: func(a *app.App, prov string) {
			autoDeployIfNeeded(a, project, findHubDir(), prov, "", true)
		},
	})
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// resolveGitLabURL extracts the GitLab URL from config, with env fallback.
func resolveGitLabURL(a *app.App) string {
	if u := a.Config.MCP.Gitlab.URL; u != "" {
		return u
	}
	if u := os.Getenv("GITLAB_URL"); u != "" {
		return u
	}
	return "https://gitlab.com"
}

// resolveMRFromRef resolves a merge request from a ticket ref or branch name.
// Tries: ref as branch directly → ticket pattern → current branch fallback.
func resolveMRFromRef(ctx context.Context, gl *gitlabapi.Client, glProject, projectPath, ref string) (*gitlabapi.MRInfo, string, error) {
	// Try ref as a branch name directly.
	mr, err := gl.FindMRByBranch(ctx, glProject, ref)
	if err != nil {
		return nil, "", err
	}
	if mr != nil {
		return mr, ref, nil
	}

	// Try to find a branch containing the ticket ref.
	branch := findBranchForTicket(projectPath, ref)
	if branch != "" {
		mr, err = gl.FindMRByBranch(ctx, glProject, branch)
		if err != nil {
			return nil, "", err
		}
		if mr != nil {
			return mr, branch, nil
		}
	}

	// Try claim MR URL is a V2 feature — for now the branch lookup is sufficient.
	return nil, "", nil
}

// findBranchForTicket looks for a local branch containing the ticket ref.
func findBranchForTicket(projectPath, ticketRef string) string {
	if projectPath == "" {
		return ""
	}
	branch := getCurrentBranch(projectPath)
	if branch != "" && strings.Contains(strings.ToUpper(branch), strings.ToUpper(ticketRef)) {
		return branch
	}
	return ""
}

// displayFeedbackPreview shows a summary of the MR and discussions.
func displayFeedbackPreview(a *app.App, mr *gitlabapi.MRInfo, branch string, discussions []gitlabapi.Discussion) {
	baseBranch := mr.TargetBranch
	if baseBranch == "" {
		baseBranch = "main"
	}
	fmt.Fprintf(a.IO.Out, "\n  %s MR !%d — %s → %s\n",
		theme.Subtitle.Render(theme.IconArrow), mr.IID, theme.Bold.Render(branch), baseBranch)
	fmt.Fprintf(a.IO.Out, "  %s %s\n", theme.Subtitle.Render("Titre:"), mr.Title)
	fmt.Fprintf(a.IO.Out, "  %s %s\n", theme.Subtitle.Render("URL:"), mr.WebURL)

	if len(discussions) == 0 {
		return
	}

	// Count unique authors and files.
	authors := make(map[string]int)
	files := make(map[string]int)
	for _, d := range discussions {
		if len(d.Notes) > 0 {
			authors[d.Notes[0].Author.Username]++
			if d.Notes[0].Position != nil && d.Notes[0].Position.NewPath != "" {
				files[d.Notes[0].Position.NewPath]++
			}
		}
	}

	fmt.Fprintf(a.IO.Out, "  %s %d\n", theme.Subtitle.Render("Discussions non résolues:"), len(discussions))

	// Authors.
	var authorParts []string
	for name, count := range authors {
		authorParts = append(authorParts, fmt.Sprintf("@%s (%d)", name, count))
	}
	fmt.Fprintf(a.IO.Out, "  %s %s\n", theme.Subtitle.Render("Auteurs:"), strings.Join(authorParts, ", "))

	// Files.
	if len(files) > 0 {
		var fileParts []string
		for path, count := range files {
			fileParts = append(fileParts, fmt.Sprintf("%s (%d)", path, count))
		}
		fmt.Fprintf(a.IO.Out, "  %s %s\n", theme.Subtitle.Render("Fichiers:"), strings.Join(fileParts, ", "))
	}
}

// Prompt size caps to prevent token overflow with large MRs.
const (
	maxFeedbackDiscussions = 30
	maxNoteBodyChars       = 2000
)

// buildFeedbackPrompt constructs the prompt for the feedback correction session.
func buildFeedbackPrompt(mr *gitlabapi.MRInfo, branch string, discussions []gitlabapi.Discussion) string {
	baseBranch := mr.TargetBranch
	if baseBranch == "" {
		baseBranch = gitutil.DetectBaseBranch(".", "")
	}

	var sb strings.Builder
	sb.WriteString("[MODE:feedback] ")
	sb.WriteString("[SKILL:orchestrator/orchestrator-dev-feedback-mode] ")
	sb.WriteString(fmt.Sprintf("[BRANCH:%s] [BASE:%s] ", branch, baseBranch))
	sb.WriteString("\n\n")

	sb.WriteString("Tu dois traiter le feedback de review reçu sur cette MR.\n\n")
	sb.WriteString(fmt.Sprintf("MR: %s\n", mr.WebURL))
	sb.WriteString(fmt.Sprintf("Titre: %s\n", mr.Title))
	sb.WriteString(fmt.Sprintf("Branche: %s → %s\n\n", branch, baseBranch))

	total := len(discussions)
	shown := total
	if shown > maxFeedbackDiscussions {
		shown = maxFeedbackDiscussions
	}
	sb.WriteString(fmt.Sprintf("Commentaires de review non résolus (%d):\n\n", total))

	for i, d := range discussions[:shown] {
		if len(d.Notes) == 0 {
			continue
		}
		n := d.Notes[0] // first note = the finding
		fmt.Fprintf(&sb, "--- Discussion %d ---\n", i+1)
		fmt.Fprintf(&sb, "Auteur: @%s\n", n.Author.Username)
		if n.Position != nil && n.Position.NewPath != "" {
			fmt.Fprintf(&sb, "Fichier: %s:%d\n", n.Position.NewPath, n.Position.NewLine)
		}
		body := n.Body
		if len(body) > maxNoteBodyChars {
			body = body[:maxNoteBodyChars] + "... [truncated]"
		}
		fmt.Fprintf(&sb, "Commentaire:\n%s\n", body)

		// Include replies for context.
		if len(d.Notes) > 1 {
			for _, reply := range d.Notes[1:] {
				replyBody := reply.Body
				if len(replyBody) > maxNoteBodyChars {
					replyBody = replyBody[:maxNoteBodyChars] + "... [truncated]"
				}
				fmt.Fprintf(&sb, "  ↳ @%s: %s\n", reply.Author.Username, replyBody)
			}
		}
		sb.WriteString("\n")
	}

	if total > shown {
		fmt.Fprintf(&sb, "... et %d discussions supplémentaires non affichées.\n\n", total-shown)
	}

	sb.WriteString("Workflow:\n")
	sb.WriteString("1. Lis chaque commentaire de review\n")
	sb.WriteString("2. Pour chaque commentaire, applique la correction demandée\n")
	sb.WriteString("3. Vérifie que les tests passent après les corrections\n")
	sb.WriteString("4. Fais un commit groupé (message: fix(review): address reviewer feedback)\n")
	sb.WriteString("5. Si l'outil gitlab_reply_to_mr_discussion est disponible, poste une réponse sur chaque thread résolu\n")

	return sb.String()
}
