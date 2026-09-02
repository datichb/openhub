package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var claimCmd = &cobra.Command{
	Use:   "claim <ticket-id>",
	Short: i18n.T("cmd.claim.short"),
	Long:  i18n.T("cmd.claim.long"),
	Args:  cobra.ExactArgs(1),
	RunE:  runClaim,
}

var releaseCmd = &cobra.Command{
	Use:   "release <ticket-id>",
	Short: i18n.T("cmd.release.short"),
	Long:  i18n.T("cmd.release.long"),
	Args:  cobra.ExactArgs(1),
	RunE:  runRelease,
}

var claimTransferCmd = &cobra.Command{
	Use:   "transfer <ticket-id>",
	Short: i18n.T("cmd.claim.transfer.short"),
	Long:  i18n.T("cmd.claim.transfer.long"),
	Args:  cobra.ExactArgs(1),
	RunE:  runClaimTransfer,
}

func init() {
	teamCmd.AddCommand(claimCmd)
	teamCmd.AddCommand(releaseCmd)
	claimCmd.AddCommand(claimTransferCmd)

	claimCmd.Flags().StringP("project", "p", "", i18n.T("cmd.claim.flags.project"))
	claimCmd.Flags().String("worktree", "", i18n.T("cmd.claim.flags.worktree"))
	claimCmd.Flags().Bool("planned", false, i18n.T("cmd.claim.flags.planned"))

	releaseCmd.Flags().StringP("project", "p", "", i18n.T("cmd.claim.flags.project"))

	claimTransferCmd.Flags().String("to", "", i18n.T("cmd.claim.transfer.flags.to"))
	claimTransferCmd.Flags().StringP("project", "p", "", i18n.T("cmd.claim.flags.project"))
	_ = claimTransferCmd.MarkFlagRequired("to")
}

func runClaim(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo := teamRepo

	// Pull latest state to avoid conflicts and stale WIP checks
	if pullErr := repo.Pull(ctx); pullErr != nil {
		fmt.Fprintf(a.IO.ErrOut, "%s %s\n",
			theme.WarningStyle.Render(theme.IconWarning),
			i18n.T("cmd.claim.sync_failed"))
	}

	ticketID := args[0]
	project, _ := cmd.Flags().GetString("project")
	worktree, _ := cmd.Flags().GetString("worktree")
	planned, _ := cmd.Flags().GetBool("planned")

	if project == "" {
		project = detectCurrentProject(ctx, a)
		if project == "" {
			return errors.New(i18n.T("cmd.claim.project_not_detected"))
		}
	}

	memberID := a.Config.ActiveTeam().MemberID
	if memberID == "" {
		return errors.New(i18n.Tf("cmd.claim.member_not_configured",
			theme.Bold.Render("oh team init")))
	}

	// Check max_ticket_wip policy before claiming
	activeClaims := 0
	allClaims, _ := repo.ListClaims("")
	for _, c := range allClaims {
		if c.ClaimedBy == memberID {
			activeClaims++
		}
	}

	policyCtx := teamstate.PolicyContext{
		MemberID:     memberID,
		ActiveClaims: activeClaims,
	}
	violations, _ := repo.CheckAll(project, policyCtx)
	for _, v := range violations {
		if v.Name == "max_ticket_wip" && !v.Passed {
			if v.Enforcement == teamstate.EnforcementRefuse {
				fmt.Fprintf(a.IO.Out, "%s %s\n",
					theme.ErrorStyle.Render(theme.IconWarning), v.Message)
				fmt.Fprintf(a.IO.Out, "  %s\n", theme.Subtitle.Render(v.Details))
				return fmt.Errorf("policy violation: %s", v.Name)
			}
			// Warn only
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.WarningStyle.Render(theme.IconWarning), v.Message)
			fmt.Fprintf(a.IO.Out, "  %s\n\n", theme.Subtitle.Render(v.Details))
		}
	}

	initialStatus := teamstate.ClaimStatusInProgress
	if planned {
		initialStatus = teamstate.ClaimStatusPlanned
	}

	claim := teamstate.Claim{
		TicketID:  ticketID,
		Project:   project,
		ClaimedBy: memberID,
		ClaimedAt: time.Now().UTC(),
		Worktree:  worktree,
		Status:    initialStatus,
	}

	existing, err := repo.CreateClaim(ctx, claim)
	if err == teamstate.ErrClaimExists {
		// Check if stale
		teamCfg, _ := repo.LoadConfig()
		staleDays := 3
		if teamCfg != nil && teamCfg.Takeover.StaleDays > 0 {
			staleDays = teamCfg.Takeover.StaleDays
		}

		if existing != nil && repo.IsStale(existing, staleDays) {
			daysSince := int(time.Since(existing.ClaimedAt).Hours() / 24)
			if !existing.LastActivity.IsZero() {
				daysSince = int(time.Since(existing.LastActivity).Hours() / 24)
			}

			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.WarningStyle.Render(theme.IconWarning),
				i18n.Tf("cmd.claim.stale_warning",
					project, ticketID,
					theme.Bold.Render(existing.ClaimedBy),
					daysSince))

			// Propose generating a takeover brief
			fmt.Fprintf(a.IO.Out, "%s", i18n.T("cmd.claim.stale_confirm"))
			var response string
			fmt.Scanln(&response)
			if response == "" || response == "y" || response == "Y" {
				previousOwner := existing.ClaimedBy
				// Generate brief
				brief, briefErr := repo.GenerateRawBrief(ctx, project, ticketID, previousOwner, memberID, "stale")
				if briefErr == nil {
					_ = repo.SaveBrief(ctx, brief)
					fmt.Fprintf(a.IO.Out, "%s %s\n",
						theme.SuccessStyle.Render(theme.IconSuccess),
						i18n.T("cmd.claim.brief_generated"))
				}
				// Transfer the claim
				_ = repo.TransferClaim(ctx, project, ticketID, memberID)
				fmt.Fprintf(a.IO.Out, "%s %s\n",
					theme.SuccessStyle.Render(theme.IconSuccess),
					i18n.Tf("cmd.claim.transferred_from_to",
						project, ticketID, previousOwner, memberID))
				return nil
			}
			// User declined brief but still wants to claim — do transfer
			_ = repo.TransferClaim(ctx, project, ticketID, memberID)
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.SuccessStyle.Render(theme.IconSuccess),
				i18n.Tf("cmd.claim.transferred_no_brief", project, ticketID))
			return nil
		}

		// Not stale — standard warning
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			theme.WarningStyle.Render(theme.IconWarning),
			i18n.Tf("cmd.claim.already_taken",
				project, ticketID,
				theme.Bold.Render(existing.ClaimedBy),
				existing.ClaimedAt.Local().Format("02/01 15:04")))
		fmt.Fprintf(a.IO.Out, "%s\n",
			i18n.Tf("cmd.claim.transfer_hint",
				theme.Bold.Render("oh claim transfer "+ticketID+" --to "+memberID)))
		return nil
	}
	if err != nil {
		return fmt.Errorf("creating claim: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		i18n.Tf("cmd.claim.success", project, ticketID, memberID))

	// Emit team event (best-effort, do not block on error).
	_ = repo.AppendEvent(ctx, teamstate.Event{
		Actor:   memberID,
		Type:    teamstate.EventClaimTaken,
		Project: project,
		Ticket:  ticketID,
	})

	return nil
}

func runRelease(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo := teamRepo

	ticketID := args[0]
	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		project = detectCurrentProject(ctx, a)
		if project == "" {
			return errors.New(i18n.T("cmd.claim.project_not_detected"))
		}
	}

	if err := repo.ReleaseClaim(ctx, project, ticketID); err != nil {
		if err == teamstate.ErrClaimNotFound {
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.WarningStyle.Render(theme.IconWarning),
				i18n.Tf("cmd.release.not_found", project, ticketID))
			return nil
		}
		return fmt.Errorf("releasing claim: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		i18n.Tf("cmd.release.success", project, ticketID))

	_ = repo.AppendEvent(ctx, teamstate.Event{
		Actor:   a.Config.ActiveTeam().MemberID,
		Type:    teamstate.EventClaimReleased,
		Project: project,
		Ticket:  ticketID,
	})

	return nil
}

func runClaimTransfer(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo := teamRepo

	ticketID := args[0]
	to, _ := cmd.Flags().GetString("to")
	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		project = detectCurrentProject(ctx, a)
		if project == "" {
			return errors.New(i18n.T("cmd.claim.project_not_detected"))
		}
	}

	// Get existing claim info before transfer (for brief generation)
	existingClaim, _ := repo.GetClaim(project, ticketID)
	var previousOwner string
	if existingClaim != nil {
		previousOwner = existingClaim.ClaimedBy
	}

	if err := repo.TransferClaim(ctx, project, ticketID, to); err != nil {
		if err == teamstate.ErrClaimNotFound {
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.WarningStyle.Render(theme.IconWarning),
				i18n.Tf("cmd.claim.transfer.not_found", project, ticketID))
			return nil
		}
		return fmt.Errorf("transferring claim: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		i18n.Tf("cmd.claim.transfer.success", project, ticketID, theme.Bold.Render(to)))

	// Generate takeover brief automatically
	if previousOwner != "" {
		fmt.Fprintf(a.IO.Out, "\n%s %s\n",
			theme.Subtitle.Render(theme.IconArrow),
			i18n.T("cmd.claim.transfer.generating_brief"))

		brief, err := repo.GenerateRawBrief(ctx, project, ticketID, previousOwner, to, "transfer")
		if err != nil {
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.WarningStyle.Render(theme.IconWarning),
				i18n.Tf("cmd.claim.transfer.brief_error", err))
		} else {
			if err := repo.SaveBrief(ctx, brief); err != nil {
				fmt.Fprintf(a.IO.Out, "%s %s\n",
					theme.WarningStyle.Render(theme.IconWarning),
					i18n.Tf("cmd.claim.transfer.brief_save_error", err))
			} else {
				fmt.Fprintf(a.IO.Out, "%s %s\n",
					theme.SuccessStyle.Render(theme.IconSuccess),
					i18n.Tf("cmd.claim.transfer.brief_done",
						theme.Subtitle.Render("oh takeover-brief show "+ticketID)))
			}
		}
	}

	_ = repo.AppendEvent(ctx, teamstate.Event{
		Actor:   previousOwner,
		Type:    teamstate.EventClaimTransferred,
		Project: project,
		Ticket:  ticketID,
		Data:    map[string]interface{}{"to": to},
	})

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
