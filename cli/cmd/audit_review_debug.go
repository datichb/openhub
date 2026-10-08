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
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitlabapi"
	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/notify"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// agentCommandAlias runs `oh audit|review|debug` as their workflow (O15):
// flags become inputs when the workflow declares them, text goes to its
// first free text input.
func agentCommandAlias(cmd *cobra.Command, old, workflowID string, inputs map[string]string, text string) error {
	ctx := cmd.Context()
	if err := requireV2(ctx); err != nil {
		return err
	}
	a := MustApp()
	projectID, _ := cmd.Flags().GetString("project")
	project, err := resolveProject(ctx, a, projectID)
	if err != nil {
		return err
	}
	return runAlias(cmd, workflowAlias{Old: old, Workflow: workflowID,
		Opts: runOptions{Project: project, LooseInputs: inputs, Text: text}})
}

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: i18n.T("cmd.audit.short"),
	Long:  i18n.T("cmd.audit.long"),
	RunE: func(cmd *cobra.Command, args []string) error {
		auditType, _ := cmd.Flags().GetString("type")

		// Validate audit type
		validTypes := map[string]string{
			"security":      "sécurité (vulnérabilités, injections, secrets, dépendances)",
			"performance":   "performance (fuites mémoire, N+1, rendering, bundle size)",
			"architecture":  "architecture (couplage, cohésion, patterns, dette technique)",
			"accessibility": "accessibilité (WCAG, ARIA, contraste, navigation clavier)",
			"ecodesign":     "éco-conception (empreinte carbone, poids, requêtes inutiles)",
			"observability": "observabilité (logs, traces, métriques, alerting, SLI/SLO)",
			"privacy":       "vie privée (RGPD, données personnelles, consentement, rétention)",
		}

		if _, ok := validTypes[auditType]; !ok {
			return fmt.Errorf("%s", i18n.Tf("cmd.audit.invalid_type", auditType))
		}

		return agentCommandAlias(cmd, "oh audit", "audit", map[string]string{"type": auditType}, "")
	},
}

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: i18n.T("cmd.review.short"),
	Long:  i18n.T("cmd.review.long"),
	RunE: func(cmd *cobra.Command, args []string) error {
		publish, _ := cmd.Flags().GetBool("publish")
		if publish {
			return runReviewPublish(cmd)
		}

		mode, _ := cmd.Flags().GetString("mode")
		reviewBranch, _ := cmd.Flags().GetString("branch")
		switch mode {
		case "", "standard", "adversarial", "edge-case", "standard+adversarial", "all":
		default:
			return errors.New(i18n.Tf("cmd.review.invalid_mode", mode))
		}
		return agentCommandAlias(cmd, "oh review", "review", map[string]string{"review_mode": mode, "branch": reviewBranch}, "")
	},
}

var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: i18n.T("cmd.debug.short"),
	Long:  i18n.T("cmd.debug.long"),
	RunE: func(cmd *cobra.Command, args []string) error {
		issue, _ := cmd.Flags().GetString("issue")
		return agentCommandAlias(cmd, "oh debug", "debug", map[string]string{"issue": issue}, issue)
	},
}

// runReviewPublish creates a MR on GitLab for the current branch and optionally assigns a reviewer.
func runReviewPublish(cmd *cobra.Command) error {
	a := MustApp()
	ctx := cmd.Context()

	if !a.Config.MCP.Gitlab.WriteEnabled {
		return errors.New(i18n.Tf("cmd.review.publish.write_disabled", theme.Bold.Render("oh mcp setup gitlab")))
	}

	projectID, _ := cmd.Flags().GetString("project")
	project, err := resolveProject(ctx, a, projectID)
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
	glToken := resolveGitLabToken(ctx, a)
	if glToken == "" {
		return errors.New(i18n.Tf("cmd.review.feedback.no_token", theme.Bold.Render("oh mcp setup gitlab")))
	}
	glURL := resolveGitLabURL(a)

	// Resolve GitLab project path (tracker config).
	glProject := resolveGitLabProject(a, project)
	if glProject == "" {
		return errors.New(i18n.Tf("cmd.review.publish.no_project", theme.Bold.Render("tracker_project")))
	}

	// Create GitLab API client.
	gl := gitlabapi.NewClient(glURL, glToken)

	// Create or find existing MR.
	mr, err := gl.CreateMR(ctx, glProject, branch, targetBranch, title, "")
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.review.publish.create_failed"), err)
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.review.publish.created", theme.Bold.Render(mr.WebURL)))
	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.review.publish.title", mr.Title))

	// Assign reviewer if requested.
	reviewerFlag, _ := cmd.Flags().GetString("reviewer")
	if reviewerFlag != "" {
		if err := assignReviewer(ctx, a, gl, glProject, mr.IID, reviewerFlag); err != nil {
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.WarningStyle.Render(theme.IconWarning), i18n.Tf("cmd.review.publish.assign_failed", err))
		} else {
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.review.publish.assigned", theme.Bold.Render(reviewerFlag)))
		}
	}

	// Emit team event + transition claim if team is enabled for this project.
	if teamEnabledForProject(a, project) {
		publishReviewToTeamState(ctx, a, project.ID, ticketRef, branch, mr.WebURL)
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "  %s %s\n",
		theme.WarningStyle.Render(theme.IconWarning), i18n.T("cmd.review.publish.manual_merge"))

	return nil
}

// resolveGitLabToken resolves a GitLab token from available sources (MCP cascade).
func resolveGitLabToken(ctx context.Context, a *app.App) string {
	// 1. Env var
	if tok := os.Getenv("GITLAB_TOKEN"); tok != "" {
		return tok
	}
	if a.Secrets == nil {
		return ""
	}
	// 2. MCP configured key
	tokenKey := a.Config.MCP.Gitlab.Token
	if tokenKey == "" {
		tokenKey = config.DefaultGitLabTokenKey
	}
	if tok, err := a.Secrets.Get(ctx, tokenKey); err == nil && tok != "" {
		return tok
	}
	// 3. Default key
	if tokenKey != config.DefaultGitLabTokenKey {
		if tok, err := a.Secrets.Get(ctx, config.DefaultGitLabTokenKey); err == nil && tok != "" {
			return tok
		}
	}
	return ""
}

// resolveGitLabProject resolves the GitLab project path/ID from config.
func resolveGitLabProject(a *app.App, project *domain.Project) string {
	// 1. Per-project override.
	if project.TrackerConfig != nil && project.TrackerConfig.TrackerProject != "" {
		return project.TrackerConfig.TrackerProject
	}
	// 2. Team tracker config.
	teamCfg := a.Config.ActiveTeam()
	if teamCfg.StateRepo != "" {
		statePath := teamCfg.StatePath
		if statePath == "" {
			statePath = config.DefaultTeamStatePath()
		}
		repo := teamstate.NewRepo(teamCfg.StateRepo, statePath)
		if repo.IsCloned() {
			if tc, err := repo.LoadConfig(); err == nil && tc.Tracker.TrackerProject != "" {
				return tc.Tracker.TrackerProject
			}
		}
	}
	return ""
}

// assignReviewer resolves a team member to a GitLab user and assigns them as reviewer.
func assignReviewer(ctx context.Context, a *app.App, gl *gitlabapi.Client, glProject string, mrIID int, memberID string) error {
	// Find the member in team state to get their GitLab username.
	teamCfg := a.Config.ActiveTeam()
	statePath := teamCfg.StatePath
	if statePath == "" {
		statePath = config.DefaultTeamStatePath()
	}
	repo := teamstate.NewRepo(teamCfg.StateRepo, statePath)
	if !repo.IsCloned() {
		return errors.New(i18n.T("cmd.review.publish.no_team_state"))
	}

	member, err := repo.GetMember(memberID)
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
// dispatch a notification — all in a single git commit+push.
func publishReviewToTeamState(ctx context.Context, a *app.App, projectID, ticket, branch, mrURL string) {
	statePath := a.Config.ActiveTeam().StatePath
	if statePath == "" {
		statePath = config.DefaultTeamStatePath()
	}
	repo := teamstate.NewRepo(a.Config.ActiveTeam().StateRepo, statePath)
	if !repo.IsCloned() {
		return
	}

	if ticket == "" || projectID == "" {
		return
	}

	event, err := repo.PublishReviewBatch(ctx, teamstate.PublishReviewParams{
		Project:  projectID,
		TicketID: ticket,
		Actor:    a.Config.ActiveTeam().MemberID,
		Branch:   branch,
		MRURL:    mrURL,
		// Label intentionally empty: agent-reviewed is now set by
		// team_review_verdict when the actual review completes (R3 fix).
	})
	if err != nil {
		slog.Warn("teamstate.publish.batch_failed", "project", projectID, "ticket", ticket, "error", err)
		return
	}

	// Best-effort notification dispatch.
	if teamCfg, cfgErr := repo.LoadConfig(); cfgErr == nil {
		d := notify.NewDispatcher(teamCfg)
		_ = d.Dispatch(ctx, event)
	}
}

func init() {
	rootCmd.AddCommand(auditCmd)
	auditCmd.Flags().StringP("project", "p", "", "Nom du projet")
	auditCmd.Flags().StringP("type", "t", "security", "Type d'audit (security, performance, architecture, accessibility, ecodesign, observability, privacy)")
	_ = auditCmd.RegisterFlagCompletionFunc("project", completeProjectIDs)

	rootCmd.AddCommand(reviewCmd)
	reviewCmd.Flags().StringP("project", "p", "", "Nom du projet")
	reviewCmd.Flags().StringP("mode", "m", "", "Mode de review (standard, adversarial, edge-case, standard+adversarial, all)")
	reviewCmd.Flags().StringP("branch", "b", "", "Branche à reviewer (diff vs main). Par défaut : branche courante si feature branch")
	reviewCmd.Flags().Bool("publish", false, "Créer une MR sur GitLab et optionnellement assigner un reviewer (nécessite write_enabled)")
	reviewCmd.Flags().String("reviewer", "", "Member ID du reviewer à assigner sur la MR (utilisé avec --publish)")
	_ = reviewCmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	reviewCmd.AddCommand(reviewFeedbackCmd)
	_ = reviewCmd.RegisterFlagCompletionFunc("mode", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"standard", "adversarial", "edge-case", "standard+adversarial", "all"}, cobra.ShellCompDirectiveNoFileComp
	})

	rootCmd.AddCommand(debugCmd)
	debugCmd.Flags().StringP("project", "p", "", "Nom du projet")
	debugCmd.Flags().StringP("issue", "i", "", "Description du problème")
	_ = debugCmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
}
