package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/datichb/openhub/cli/internal/gitlabapi"
	"github.com/datichb/openhub/cli/internal/i18n"
)

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
	fmt.Fprint(&sb, i18n.Tf("cmd.review_feedback.unresolved_review_comments", total))
	for i, d := range discussions[:shown] {
		if len(d.Notes) == 0 {
			continue
		}
		n := d.Notes[0] // first note = the finding
		fmt.Fprintf(&sb, "--- Discussion %d ---\n", i+1)
		fmt.Fprint(&sb, i18n.Tf("cmd.review_feedback.note_author", n.Author.Username))
		if n.Position != nil && n.Position.NewPath != "" {
			fmt.Fprint(&sb, i18n.Tf("cmd.review_feedback.note_file", n.Position.NewPath, n.Position.NewLine))
		}
		fmt.Fprint(&sb, i18n.Tf("cmd.review_feedback.note_comment", clipNote(n.Body)))
		for _, reply := range d.Notes[1:] { // replies, for context
			fmt.Fprintf(&sb, "  ↳ @%s: %s\n", reply.Author.Username, clipNote(reply.Body))
		}
		sb.WriteString("\n")
	}
	if total > shown {
		fmt.Fprint(&sb, i18n.Tf("cmd.review_feedback.and_more_discussions_not_shown", total-shown))
	}
	return sb.String()
}

func clipNote(body string) string {
	if len(body) > maxNoteBodyChars {
		return body[:maxNoteBodyChars] + "... [truncated]"
	}
	return body
}
