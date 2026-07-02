package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/tui/common"
)

var worktreeCmd = &cobra.Command{
	Use:     "worktree",
	Aliases: []string{"wt"},
	Short:   "Gestion des git worktrees",
}

func init() {
	rootCmd.AddCommand(worktreeCmd)
	worktreeCmd.AddCommand(worktreeListCmd())
	worktreeCmd.AddCommand(worktreeAddCmd())
	worktreeCmd.AddCommand(worktreeRemoveCmd())
}

func worktreeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Liste les worktrees du projet",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			worktrees, err := listWorktrees()
			if err != nil {
				return err
			}

			if len(worktrees) == 0 {
				fmt.Fprintln(a.IO.Out, common.Subtitle.Render("Aucun worktree."))
				return nil
			}

			w := tabwriter.NewWriter(a.IO.Out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "BRANCHE\tCHEMIN\tHEAD")
			for _, wt := range worktrees {
				fmt.Fprintf(w, "%s\t%s\t%s\n", wt.branch, wt.path, wt.head[:8])
			}
			w.Flush()
			fmt.Fprintf(a.IO.Out, "\n%s\n", common.Subtitle.Render(fmt.Sprintf("%d worktree(s)", len(worktrees))))
			return nil
		},
	}
}

func worktreeAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add [branch]",
		Short: "Crée un nouveau worktree",
		Long:  "Crée un worktree pour une branche. Lance un wizard si aucune branche n'est fournie.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()

			var branch string
			if len(args) > 0 {
				branch = args[0]
			} else {
				form := huh.NewForm(
					huh.NewGroup(
						huh.NewInput().
							Title("Nom de la branche").
							Description("Sera créée si elle n'existe pas").
							Value(&branch),
					),
				)
				if err := form.Run(); err != nil {
					return err
				}
			}

			if branch == "" {
				return fmt.Errorf("le nom de branche est requis")
			}

			// Determine worktree path (sibling directory)
			cwd, _ := os.Getwd()
			parentDir := filepath.Dir(cwd)
			repoName := filepath.Base(cwd)
			wtPath := filepath.Join(parentDir, repoName+"-"+branch)

			// Create worktree
			_, err := exec.Command("git", "worktree", "add", "-b", branch, wtPath).CombinedOutput()
			if err != nil {
				// Try without -b (branch already exists)
				out, err := exec.Command("git", "worktree", "add", wtPath, branch).CombinedOutput()
				if err != nil {
					return fmt.Errorf("git worktree add: %s", strings.TrimSpace(string(out)))
				}
			}

			fmt.Fprintf(a.IO.Out, "%s Worktree créé: %s → %s\n",
				common.SuccessStyle.Render(common.IconSuccess),
				common.Bold.Render(branch), wtPath)
			return nil
		},
	}
}

func worktreeRemoveCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "remove [path]",
		Aliases: []string{"rm"},
		Short:   "Supprime un worktree",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()

			var wtPath string
			if len(args) > 0 {
				wtPath = args[0]
			} else {
				worktrees, err := listWorktrees()
				if err != nil {
					return err
				}
				if len(worktrees) == 0 {
					fmt.Fprintln(a.IO.Out, common.Subtitle.Render("Aucun worktree à supprimer."))
					return nil
				}

				options := make([]huh.Option[string], len(worktrees))
				for i, wt := range worktrees {
					options[i] = huh.NewOption(
						fmt.Sprintf("%s (%s)", wt.branch, wt.path), wt.path)
				}

				form := huh.NewForm(
					huh.NewGroup(
						huh.NewSelect[string]().
							Title("Worktree à supprimer").
							Options(options...).
							Value(&wtPath),
					),
				)
				if err := form.Run(); err != nil {
					return err
				}
			}

			gitArgs := []string{"worktree", "remove", wtPath}
			if force {
				gitArgs = append(gitArgs, "--force")
			}

			out, err := exec.Command("git", gitArgs...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("git worktree remove: %s", strings.TrimSpace(string(out)))
			}

			fmt.Fprintf(a.IO.Out, "%s Worktree supprimé: %s\n",
				common.SuccessStyle.Render(common.IconSuccess), wtPath)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Forcer la suppression")
	return cmd
}

type worktreeInfo struct {
	path   string
	head   string
	branch string
}

func listWorktrees() ([]worktreeInfo, error) {
	out, err := exec.Command("git", "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	var worktrees []worktreeInfo
	var current worktreeInfo

	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			if current.path != "" {
				worktrees = append(worktrees, current)
			}
			current = worktreeInfo{path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "HEAD "):
			current.head = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			// refs/heads/main → main
			current.branch = strings.TrimPrefix(ref, "refs/heads/")
		}
	}
	if current.path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}
