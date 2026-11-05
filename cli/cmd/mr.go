package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitlabapi"
	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/i18n"
	mcpgitlab "github.com/datichb/openhub/cli/internal/mcp/gitlab"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// `oh mr create` publishes the current branch as a GitLab merge request: the
// review itself is a workflow (`oh run review`), the publication is not.

var mrCmd = &cobra.Command{
	Use:   "mr",
	Short: i18n.T("cmd.mr.short"),
	Long:  i18n.T("cmd.mr.long"),
}

var mrCreateCmd = &cobra.Command{
	Use:   "create",
	Short: i18n.T("cmd.mr.create.short"),
	Long:  i18n.T("cmd.mr.create.long"),
	Args:  cobra.NoArgs,
	RunE:  func(cmd *cobra.Command, _ []string) error { return runReviewPublish(cmd) },
}

// runReviewPublish creates a MR on GitLab for the current branch and optionally assigns a reviewer.
func runReviewPublish(cmd *cobra.Command) error {
	a := MustApp()
	ctx := cmd.Context()

	if !a.Config.MCP.Gitlab.WriteEnabled {
		return errors.New(i18n.Tf("cmd.review.publish.write_disabled", theme.Bold.Render("oh mcp setup gitlab")))
	}

	projectID, _ := cmd.Flags().GetString("project")
	project, err := resolveProject(ctx, a, projectID, projectWrite)
	if err != nil {
		return err
	}

	// Get current branch.
	branch := getPublishBranch(project.Path)
	if branch == "" || isMainBranch(branch) {
		return errors.New(i18n.Tf("cmd.review.publish.not_feature_branch", branch))
	}

	// Detect base/target branch.
	targetBranch := gitutil.DetectBaseBranch(project.Path, a.Config.Worktree.BaseBranch)

	// Extract ticket ref from branch for the title.
	ticketRef := extractTicketFromBranch(branch)
	title := branch
	if ticketRef != "" {
		title = fmt.Sprintf("%s: %s", ticketRef, strings.TrimPrefix(branch, fmt.Sprintf("feat/%s-", ticketRef)))
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n", theme.Subtitle.Render(theme.IconArrow),
		i18n.Tf("cmd.review.publish.creating", theme.Bold.Render(branch), targetBranch))

	// Resolve GitLab credentials (MCP cascade).
	access, err := resolveGitLabAccess(ctx, a, project)
	if err != nil {
		return err
	}

	// Resolve GitLab project path (tracker config).
	glProject := resolveGitLabProject(ctx, a, project)
	if glProject == "" {
		return errors.New(i18n.Tf("cmd.review.publish.no_project", theme.Bold.Render("tracker_project")))
	}

	// Create GitLab API client.
	gl := access.Client()

	// Create or find existing MR.
	mr, err := gl.CreateMR(ctx, glProject, branch, targetBranch, title, "")
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.review.publish.create_failed"), err)
	}

	cliout.Success(a.IO.Out, i18n.Tf("cmd.review.publish.created", theme.Bold.Render(mr.WebURL)))
	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.review.publish.title", mr.Title))

	// Assign reviewer if requested.
	reviewerFlag, _ := cmd.Flags().GetString("reviewer")
	reviewerAssigned := ""
	if reviewerFlag != "" {
		if err := assignReviewer(ctx, a, project, gl, glProject, mr.IID, reviewerFlag); err != nil {
			cliout.Warn(a.IO.Out, i18n.Tf("cmd.review.publish.assign_failed", err))
		} else {
			reviewerAssigned = reviewerFlag
			cliout.Success(a.IO.Out, i18n.Tf("cmd.review.publish.assigned", theme.Bold.Render(reviewerFlag)))
		}
	}

	// Emit team event + transition claim if team is enabled for this project.
	if teamEnabledForProject(a, project) {
		publishReviewToTeamState(ctx, a, project.ID, ticketRef, branch, mr.WebURL, reviewerAssigned)
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "  %s %s\n",
		cliout.WarnIcon(), i18n.T("cmd.review.publish.manual_merge"))

	return nil
}

// resolveGitLabProject resolves the GitLab project path/ID from config.
func resolveGitLabProject(ctx context.Context, a *app.App, project *domain.Project) string {
	// 1. Per-project override.
	if project.TrackerConfig != nil && project.TrackerConfig.TrackerProject != "" {
		return project.TrackerConfig.TrackerProject
	}
	// 2. Tracker config of the team of the project (the hub team when it has none).
	teamCfg := teamServiceFor(ctx, a).ResolveTeam(project)
	if teamCfg.StateRepo != "" {
		repo := teamstate.NewRepo(teamCfg.StateRepo, teamCfg.StatePath)
		if repo.IsCloned() {
			if tc, err := repo.LoadConfig(ctx); err == nil && tc.Tracker.TrackerProject != "" {
				return tc.Tracker.TrackerProject
			}
		}
	}
	return ""
}

// assignReviewer resolves a team member to a GitLab user and assigns them as reviewer.
func assignReviewer(ctx context.Context, a *app.App, project *domain.Project, gl *gitlabapi.Client, glProject string, mrIID int, memberID string) error {
	// Find the member in the team state of the project to get their GitLab username.
	teamCfg := teamServiceFor(ctx, a).ResolveTeam(project)
	repo := teamstate.NewRepo(teamCfg.StateRepo, teamCfg.StatePath)
	if !repo.IsCloned() {
		return errors.New(i18n.T("cmd.review.publish.no_team_state"))
	}

	member, err := repo.GetMember(ctx, memberID)
	if err != nil {
		return errors.New(i18n.Tf("cmd.review.publish.unknown_member", memberID))
	}
	if member.GitLabUsername == "" {
		return errors.New(i18n.Tf("cmd.review.publish.no_gitlab_username", memberID))
	}

	// Resolve GitLab user ID.
	userID, err := gl.ResolveUserID(ctx, member.GitLabUsername)
	if err != nil {
		return err
	}

	// Assign as reviewer.
	return gl.AssignReviewers(ctx, glProject, mrIID, []int{userID})
}

func getPublishBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// isMainBranch returns true if the branch is a trunk branch (not a feature branch).
func isMainBranch(name string) bool {
	switch name {
	case "main", "master", "develop", "development", "staging", "HEAD":
		return true
	}
	return false
}

// publishReviewToTeamState uses PublishReviewBatch to atomically transition
// the claim to review, store the MR URL, append the review.ready event, and
// announce it (the event sink) — all in a single git commit+push.
func publishReviewToTeamState(ctx context.Context, a *app.App, projectID, ticket, branch, mrURL, reviewer string) {
	// The review is published in the team-state of the team of the project (the
	// one the MR was checked against), by the member of that team.
	var project *domain.Project
	if a.Projects != nil {
		project, _ = a.Projects.Get(ctx, projectID)
	}
	team, err := teamOfProject(ctx, a, project)
	if err != nil || !team.Enabled {
		return
	}
	repo := teamstate.NewRepo(team.StateRepo, team.StatePath)
	if !repo.IsCloned() {
		return
	}

	if ticket == "" || projectID == "" {
		return
	}

	_, err = repo.PublishReviewBatch(ctx, teamstate.PublishReviewParams{
		Project:  projectID,
		TicketID: ticket,
		Actor:    team.MemberID,
		Branch:   branch,
		MRURL:    mrURL,
		Reviewer: reviewer,
		// Label intentionally empty: agent-reviewed is now set by
		// team_review_verdict when the actual review completes.
	})
	if err != nil {
		slog.Warn("teamstate.publish.batch_failed", "project", projectID, "ticket", ticket, "error", err)
		return
	}
	// The review.ready event reaches the webhook of the team (and mentions the
	// reviewer) through the event sink of teamstate, as any event.
}

func init() {
	rootCmd.AddCommand(mrCmd)
	mrCmd.AddCommand(mrCreateCmd)
	mrCreateCmd.Flags().StringP("project", "p", "", i18n.T("cmd.mr.create.flags.project"))
	mrCreateCmd.Flags().String("reviewer", "", i18n.T("cmd.mr.create.flags.reviewer"))
	_ = mrCreateCmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
}

// announceMRCreated is the host of the gitlab MCP server: a merge request that
// the agent created is published to the team of the project of the session
// (claim to review, review.ready event), as oh mr create does for the one a
// person creates. It needs the oh project of the session (OH_PROJECT_ID) and a
// branch that names a ticket; otherwise there is nothing to announce.
func announceMRCreated(ctx context.Context, mr mcpgitlab.MRCreated) {
	a := TryApp()
	projectID := os.Getenv(mcpgitlab.EnvOhProject)
	if a == nil || projectID == "" {
		return
	}
	publishReviewToTeamState(ctx, a, projectID, extractTicketFromBranch(mr.Branch), mr.Branch, mr.WebURL, "")
}
