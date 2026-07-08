package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var claimCmd = &cobra.Command{
	Use:   "claim <ticket-id>",
	Short: "Réserve un ticket pour toi",
	Long: `Réserve un ticket dans le team-state, signalant à l'équipe que tu
travailles dessus. Si le ticket est déjà pris, un warning est affiché
mais l'opération n'est pas bloquante.`,
	Args: cobra.ExactArgs(1),
	RunE: runClaim,
}

var releaseCmd = &cobra.Command{
	Use:   "release <ticket-id>",
	Short: "Libère un ticket réservé",
	Long:  `Supprime la réservation sur un ticket, le rendant disponible pour d'autres membres.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runRelease,
}

var claimTransferCmd = &cobra.Command{
	Use:   "transfer <ticket-id>",
	Short: "Transfère un ticket à un autre membre",
	Long:  `Change le propriétaire d'un claim existant sans le libérer.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runClaimTransfer,
}

func init() {
	rootCmd.AddCommand(claimCmd)
	rootCmd.AddCommand(releaseCmd)
	claimCmd.AddCommand(claimTransferCmd)

	claimCmd.Flags().StringP("project", "p", "", "Project ID (auto-detected from cwd if omitted)")
	claimCmd.Flags().String("worktree", "", "Associated branch/worktree name")

	releaseCmd.Flags().StringP("project", "p", "", "Project ID")

	claimTransferCmd.Flags().String("to", "", "Member ID to transfer to (required)")
	claimTransferCmd.Flags().StringP("project", "p", "", "Project ID")
	_ = claimTransferCmd.MarkFlagRequired("to")
}

func runClaim(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo, err := ensureTeamRepo(ctx, a)
	if err != nil {
		return err
	}

	ticketID := args[0]
	project, _ := cmd.Flags().GetString("project")
	worktree, _ := cmd.Flags().GetString("worktree")

	if project == "" {
		project = detectCurrentProject(ctx, a)
		if project == "" {
			return fmt.Errorf("impossible de détecter le projet courant. Utilise --project")
		}
	}

	memberID := a.Config.Team.MemberID
	if memberID == "" {
		return fmt.Errorf("member_id non configuré dans hub.toml. Lance %s",
			common.Bold.Render("oh team init"))
	}

	claim := teamstate.Claim{
		TicketID:  ticketID,
		Project:   project,
		ClaimedBy: memberID,
		ClaimedAt: time.Now().UTC(),
		Worktree:  worktree,
		Status:    "in_progress",
	}

	existing, err := repo.CreateClaim(ctx, claim)
	if err == teamstate.ErrClaimExists {
		fmt.Fprintf(a.IO.Out, "%s %s/%s est déjà pris par %s (depuis %s)\n",
			common.WarningStyle.Render(common.IconWarning),
			project, ticketID,
			common.Bold.Render(existing.ClaimedBy),
			existing.ClaimedAt.Local().Format("02/01 15:04"))
		fmt.Fprintf(a.IO.Out, "  Utilise %s pour transférer si nécessaire.\n",
			common.Bold.Render("oh claim transfer "+ticketID+" --to "+memberID))
		return nil
	}
	if err != nil {
		return fmt.Errorf("creating claim: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "%s %s/%s réservé pour %s\n",
		common.SuccessStyle.Render(common.IconSuccess),
		project, ticketID, memberID)

	return nil
}

func runRelease(cmd *cobra.Command, args []string) error {
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

	if err := repo.ReleaseClaim(ctx, project, ticketID); err != nil {
		if err == teamstate.ErrClaimNotFound {
			fmt.Fprintf(a.IO.Out, "%s %s/%s n'est pas réservé\n",
				common.WarningStyle.Render(common.IconWarning), project, ticketID)
			return nil
		}
		return fmt.Errorf("releasing claim: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "%s %s/%s libéré\n",
		common.SuccessStyle.Render(common.IconSuccess), project, ticketID)
	return nil
}

func runClaimTransfer(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo, err := ensureTeamRepo(ctx, a)
	if err != nil {
		return err
	}

	ticketID := args[0]
	to, _ := cmd.Flags().GetString("to")
	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		project = detectCurrentProject(ctx, a)
		if project == "" {
			return fmt.Errorf("impossible de détecter le projet courant. Utilise --project")
		}
	}

	if err := repo.TransferClaim(ctx, project, ticketID, to); err != nil {
		if err == teamstate.ErrClaimNotFound {
			fmt.Fprintf(a.IO.Out, "%s %s/%s n'est pas réservé\n",
				common.WarningStyle.Render(common.IconWarning), project, ticketID)
			return nil
		}
		return fmt.Errorf("transferring claim: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "%s %s/%s transféré à %s\n",
		common.SuccessStyle.Render(common.IconSuccess),
		project, ticketID, common.Bold.Render(to))
	return nil
}

// detectCurrentProject tries to find the project ID from the current directory.
func detectCurrentProject(ctx context.Context, a *app.App) string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	p, err := a.Projects.GetByPath(ctx, cwd)
	if err != nil {
		return ""
	}
	return p.ID
}
