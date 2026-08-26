package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/progress"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// handleWorktreeMode manages the worktree workflow.
func handleWorktreeMode(a *app.App, project *domain.Project, branch string) (string, error) {
	if !worktree.IsGitRepo(project.Path) {
		return "", fmt.Errorf("%s", i18n.Tf("cmd.start.worktree_not_git", project.Name))
	}

	if branch == "" {
		form := theme.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(i18n.T("cmd.worktree.branch_name")).
					Description(i18n.T("cmd.worktree.branch_desc")).
					Value(&branch),
			),
		)
		if err := form.Run(); err != nil {
			return "", err
		}
		if branch == "" {
			return "", fmt.Errorf("%s", i18n.T("cmd.start.worktree_branch_required"))
		}
	}

	if a.Config.Worktree.AutoCleanup {
		baseBranch := a.Config.Worktree.BaseBranch
		if baseBranch == "" {
			baseBranch = worktree.DetectBaseBranch(project.Path)
		}
		var cleanupResult worktree.CleanupResult
		_ = progress.Run(
			i18n.T("cmd.start.worktree_autocleanup"),
			func() error {
				cleanupResult, _ = worktree.CleanupMerged(project.Path, baseBranch, false)
				return nil
			},
		)
		if len(cleanupResult.Removed) > 0 {
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.start.worktree_cleanup", len(cleanupResult.Removed)))
			for _, b := range cleanupResult.Removed {
				fmt.Fprintf(a.IO.Out, "    %s %s\n", theme.Subtitle.Render("·"), b)
			}
		}
		if len(cleanupResult.Skipped) > 0 {
			fmt.Fprintf(a.IO.Out, "  %s %s\n",
				theme.Subtitle.Render(theme.IconWarning), i18n.Tf("cmd.start.worktree_cleanup_skipped", len(cleanupResult.Skipped)))
		}
	}

	var wtPath string
	if err := progress.Run(
		i18n.Tf("cmd.start.worktree_prep", theme.Bold.Render(branch)),
		func() error {
			var e error
			wtPath, e = worktree.ResolveOrCreate(project.Path, branch)
			return e
		},
	); err != nil {
		return "", fmt.Errorf("worktree: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.start.worktree_path", wtPath))

	// Link worktree to the main project's deployed config via relative symlinks.
	// The main project is guaranteed to be deployed by autoDeployIfNeeded()
	// which runs before handleWorktreeMode in the start flow.
	if err := worktree.EnsureWorktreeConfig(wtPath, project.Path); err != nil {
		return "", fmt.Errorf("worktree config: %w", err)
	}
	fmt.Fprintf(a.IO.Out, "  %s %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		i18n.T("cmd.start.worktree_linked"))

	return wtPath, nil
}
