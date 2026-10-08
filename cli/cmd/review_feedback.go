package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/gitlabapi"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var reviewFeedbackCmd = &cobra.Command{
	Use:   "feedback <ticket-or-branch>",
	Short: i18n.T("cmd.review.feedback.short"),
	Long:  i18n.T("cmd.review.feedback.long"),
	Args:  cobra.ExactArgs(1),
	RunE:  runReviewFeedback,
}

func init() {
	reviewFeedbackCmd.Flags().StringP("project", "p", "", i18n.T("cmd.review.feedback.flag_project"))
	reviewFeedbackCmd.Flags().Bool("yes", false, i18n.T("cmd.review.feedback.flag_yes"))
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
		return fmt.Errorf("%s", i18n.Tf("cmd.review.feedback.no_token", theme.Bold.Render("oh mcp setup gitlab")))
	}
	glURL := resolveGitLabURL(a)
	glProject := resolveGitLabProject(a, project)
	if glProject == "" {
		return fmt.Errorf("%s", i18n.Tf("cmd.review.feedback.no_project", theme.Bold.Render("tracker_project")))
	}

	gl := gitlabapi.NewClient(glURL, glToken)

	// ── 3. Resolve MR from ref ──
	mr, branch, err := resolveMRFromRef(ctx, gl, glProject, project.Path, ref)
	if err != nil {
		return err
	}
	if mr == nil {
		return fmt.Errorf("%s", i18n.Tf("cmd.review.feedback.no_mr", ref))
	}

	// ── 4. Fetch MR discussions ──
	discussions, err := gl.ListMRDiscussions(ctx, glProject, mr.IID, true)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.review.feedback.discussions_error"), err)
	}

	// ── 5. Preview ──
	displayFeedbackPreview(a, mr, branch, discussions)

	if len(discussions) == 0 {
		fmt.Fprintf(a.IO.Out, "\n  %s %s\n",
			theme.SuccessStyle.Render(theme.IconSuccess), i18n.T("cmd.review.feedback.no_discussions"))
		return nil
	}

	// ── 6. Confirm ──
	noConfirm, _ := cmd.Flags().GetBool("yes")
	if !noConfirm {
		fmt.Fprintf(a.IO.Out, "\n  %s", i18n.T("cmd.review.feedback.confirm"))
		var resp string
		_, _ = fmt.Scanln(&resp)
		resp = strings.TrimSpace(strings.ToLower(resp))
		if resp == "n" || resp == "no" || resp == "non" {
			return nil
		}
	}

	// ── 7. Launch the review-feedback workflow ──
	if err := requireV2(ctx); err != nil {
		return err
	}
	// The workflow computes its branch, target branch and feedback from the
	// merge request (`from:`), as for `oh run review-feedback -i mr=<url>`.
	return runAlias(cmd, workflowAlias{Old: "oh review feedback", Workflow: "review-feedback",
		Opts: runOptions{Project: project, LooseInputs: map[string]string{"mr": mr.WebURL}}})
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
	fmt.Fprintf(a.IO.Out, "  %s %s\n", theme.Subtitle.Render(i18n.T("cmd.review.feedback.label_title")), mr.Title)
	fmt.Fprintf(a.IO.Out, "  %s %s\n", theme.Subtitle.Render(i18n.T("cmd.review.feedback.label_url")), mr.WebURL)

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

	fmt.Fprintf(a.IO.Out, "  %s %d\n", theme.Subtitle.Render(i18n.T("cmd.review.feedback.label_unresolved")), len(discussions))

	// Authors.
	var authorParts []string
	for name, count := range authors {
		authorParts = append(authorParts, fmt.Sprintf("@%s (%d)", name, count))
	}
	fmt.Fprintf(a.IO.Out, "  %s %s\n", theme.Subtitle.Render(i18n.T("cmd.review.feedback.label_authors")), strings.Join(authorParts, ", "))

	// Files.
	if len(files) > 0 {
		var fileParts []string
		for path, count := range files {
			fileParts = append(fileParts, fmt.Sprintf("%s (%d)", path, count))
		}
		fmt.Fprintf(a.IO.Out, "  %s %s\n", theme.Subtitle.Render(i18n.T("cmd.review.feedback.label_files")), strings.Join(fileParts, ", "))
	}
}

// Prompt size caps to prevent token overflow with large MRs.
const (
	maxFeedbackDiscussions = 30
	maxNoteBodyChars       = 2000
)

// formatFeedbackDiscussions renders the unresolved discussions of a merge
// request: the `feedback` input of the review-feedback workflow (its prompt
// template gives the branch, the MR and the steps).
func formatFeedbackDiscussions(discussions []gitlabapi.Discussion) string {
	var sb strings.Builder
	total := len(discussions)
	shown := min(total, maxFeedbackDiscussions)
	fmt.Fprintf(&sb, "Commentaires de review non résolus (%d):\n\n", total)
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
		fmt.Fprintf(&sb, "Commentaire:\n%s\n", clipNote(n.Body))
		for _, reply := range d.Notes[1:] { // replies, for context
			fmt.Fprintf(&sb, "  ↳ @%s: %s\n", reply.Author.Username, clipNote(reply.Body))
		}
		sb.WriteString("\n")
	}
	if total > shown {
		fmt.Fprintf(&sb, "... et %d discussions supplémentaires non affichées.\n", total-shown)
	}
	return sb.String()
}

func clipNote(body string) string {
	if len(body) > maxNoteBodyChars {
		return body[:maxNoteBodyChars] + "... [truncated]"
	}
	return body
}
