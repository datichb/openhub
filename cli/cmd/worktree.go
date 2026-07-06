package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/common"
	"github.com/datichb/openhub/cli/internal/worktree"
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
	worktreeCmd.AddCommand(worktreeCleanupCmd())
}

func worktreeListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Liste les worktrees du projet",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			cwd, _ := os.Getwd()

			entries, err := worktree.List(cwd)
			if err != nil {
				return err
			}

			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				type wtJSON struct {
					Branch string `json:"branch"`
					Path   string `json:"path"`
					Head   string `json:"head"`
					IsBare bool   `json:"is_bare"`
				}
				out := make([]wtJSON, len(entries))
				for i, e := range entries {
					out[i] = wtJSON{Branch: e.Branch, Path: e.Path, Head: e.Head, IsBare: e.IsBare}
				}
				return json.NewEncoder(os.Stdout).Encode(out)
			}

			if len(entries) == 0 {
				fmt.Fprintln(a.IO.Out, common.Subtitle.Render(i18n.T("cmd.worktree.none")))
				return nil
			}

			// Detect base branch for merged status
			baseBranch := a.Config.Worktree.BaseBranch
			if baseBranch == "" {
				baseBranch = worktree.DetectBaseBranch(cwd)
			}

			w := tabwriter.NewWriter(a.IO.Out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, i18n.T("cmd.worktree.list.header"))
			for _, e := range entries {
				head := e.Head
				if len(head) > 8 {
					head = head[:8]
				}
				status := ""
				if e.Branch != "" && e.Branch != baseBranch {
					if merged, _ := worktree.IsMerged(cwd, e.Branch, baseBranch); merged {
						status = "[merged]"
					}
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.Branch, e.Path, head, status)
			}
			w.Flush()
			fmt.Fprintf(a.IO.Out, "\n%s\n", common.Subtitle.Render(i18n.Tf("cmd.worktree.list.count", len(entries))))
			return nil
		},
	}

	cmd.Flags().Bool("json", false, "Output in JSON format")
	return cmd
}

func worktreeAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add [branch]",
		Short: "Crée un nouveau worktree",
		Long:  "Crée un worktree pour une branche. Lance un wizard si aucune branche n'est fournie.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			cwd, _ := os.Getwd()

			var branch string
			if len(args) > 0 {
				branch = args[0]
			} else {
				form := huh.NewForm(
					huh.NewGroup(
						huh.NewInput().
							Title(i18n.T("cmd.worktree.branch_name")).
							Description(i18n.T("cmd.worktree.branch_desc")).
							Value(&branch),
					),
				)
				if err := form.Run(); err != nil {
					return err
				}
			}

			if branch == "" {
				return fmt.Errorf("%s", i18n.T("cmd.worktree.branch_required"))
			}

			wtPath, err := worktree.ResolveOrCreate(cwd, branch)
			if err != nil {
				return err
			}

			fmt.Fprintf(a.IO.Out, "%s %s\n",
				common.SuccessStyle.Render(common.IconSuccess),
				i18n.Tf("cmd.worktree.created", common.Bold.Render(branch), wtPath))
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
			cwd, _ := os.Getwd()

			var wtPath string
			if len(args) > 0 {
				wtPath = args[0]
			} else {
				entries, err := worktree.List(cwd)
				if err != nil {
					return err
				}

				// Filter out the main worktree (current directory)
				var selectable []worktree.Entry
				for _, e := range entries {
					if e.Path != cwd && !e.IsBare {
						selectable = append(selectable, e)
					}
				}

				if len(selectable) == 0 {
					fmt.Fprintln(a.IO.Out, common.Subtitle.Render(i18n.T("cmd.worktree.none_to_remove")))
					return nil
				}

				options := make([]huh.Option[string], len(selectable))
				for i, e := range selectable {
					options[i] = huh.NewOption(
						fmt.Sprintf("%s (%s)", e.Branch, e.Path), e.Path)
				}

				form := huh.NewForm(
					huh.NewGroup(
						huh.NewSelect[string]().
							Title(i18n.T("cmd.worktree.select_remove")).
							Options(options...).
							Value(&wtPath),
					),
				)
				if err := form.Run(); err != nil {
					return err
				}
			}

			if err := worktree.Remove(cwd, wtPath, force); err != nil {
				return err
			}

			fmt.Fprintf(a.IO.Out, "%s %s\n",
				common.SuccessStyle.Render(common.IconSuccess),
				i18n.Tf("cmd.worktree.removed", wtPath))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Forcer la suppression")
	return cmd
}

func worktreeCleanupCmd() *cobra.Command {
	var baseBranch string
	var force bool

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Supprime les worktrees dont la branche est mergée",
		Long: `Supprime automatiquement tous les worktrees dont la branche est entièrement
mergée dans la branche de base (main par défaut). Utilise "git branch --merged"
pour la détection.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			cwd, _ := os.Getwd()

			if !worktree.IsGitRepo(cwd) {
				return fmt.Errorf("%s", i18n.T("cmd.worktree.not_git"))
			}

			// Determine base branch
			base := baseBranch
			if base == "" {
				base = a.Config.Worktree.BaseBranch
			}
			if base == "" {
				base = worktree.DetectBaseBranch(cwd)
			}

			fmt.Fprintf(a.IO.Out, "%s %s\n",
				common.SuccessStyle.Render(common.IconArrow),
				i18n.Tf("cmd.worktree.cleanup_searching", common.Bold.Render(base)))

			// Pre-check: list merged branches for confirmation
			if !force {
				entries, err := worktree.List(cwd)
				if err == nil {
					var mergedBranches []string
					for _, e := range entries {
						if e.IsBare || e.Branch == "" || e.Branch == base {
							continue
						}
						if merged, _ := worktree.IsMerged(cwd, e.Branch, base); merged {
							mergedBranches = append(mergedBranches, e.Branch)
						}
					}
					if len(mergedBranches) == 0 {
						fmt.Fprintf(a.IO.Out, "  %s\n", i18n.T("cmd.worktree.cleanup_none"))
						return nil
					}
					for _, branch := range mergedBranches {
						fmt.Fprintf(a.IO.Out, "    %s %s\n", common.Subtitle.Render("·"), branch)
					}
					var confirm bool
					huh.NewConfirm().
						Title(i18n.Tf("cmd.worktree.cleanup.confirm", len(mergedBranches))).
						Value(&confirm).
						Run()
					if !confirm {
						return nil
					}
				}
			}

			removed, err := worktree.CleanupMerged(cwd, base)
			if err != nil {
				return err
			}

			if len(removed) == 0 {
				fmt.Fprintf(a.IO.Out, "  %s\n", i18n.T("cmd.worktree.cleanup_none"))
				return nil
			}

			fmt.Fprintf(a.IO.Out, "%s %s\n",
				common.SuccessStyle.Render(common.IconSuccess),
				i18n.Tf("cmd.worktree.cleanup_done", len(removed)))
			for _, branch := range removed {
				fmt.Fprintf(a.IO.Out, "    %s %s\n", common.Subtitle.Render("·"), branch)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&baseBranch, "base", "b", "", "Branche de base pour la détection (défaut: auto-detect)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	return cmd
}
