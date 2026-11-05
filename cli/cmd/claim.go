package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/textfmt"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var claimCmd = &cobra.Command{
	Use:   "claim <ticket-id>",
	Short: i18n.T("cmd.claim.short"),
	Long:  i18n.T("cmd.claim.long"),
	Args:  cobra.ExactArgs(1),
	RunE:  runClaim,
}

func init() {
	teamCmd.AddCommand(claimCmd)
	teamCmd.AddCommand(releaseCmd)
	claimCmd.AddCommand(claimTransferCmd)

	claimCmd.Flags().StringP("project", "p", "", i18n.T("cmd.claim.flags.project"))
	claimCmd.Flags().String("worktree", "", i18n.T("cmd.claim.flags.worktree"))
	claimCmd.Flags().Bool("planned", false, i18n.T("cmd.claim.flags.planned"))
	addYesFlag(claimCmd)
}

func runClaim(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo := teamRepo

	// Pull latest state to avoid conflicts and stale WIP checks
	if pullErr := repo.Pull(ctx); pullErr != nil {
		cliout.Warn(a.IO.ErrOut, i18n.T("cmd.claim.sync_failed"))
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

	memberID := commandMember(ctx, a)
	if memberID == "" {
		return errors.New(i18n.Tf("cmd.claim.member_not_configured",
			theme.Bold.Render("oh team init")))
	}

	// Check max_ticket_wip policy before claiming
	activeClaims := 0
	allClaims, _ := repo.ListClaims(ctx, "")
	for _, c := range allClaims {
		if c.ClaimedBy == memberID {
			activeClaims++
		}
	}

	policyCtx := teamstate.PolicyContext{
		MemberID:     memberID,
		ActiveClaims: activeClaims,
	}
	violations, _ := repo.CheckAll(ctx, project, policyCtx)
	for _, v := range violations {
		if v.Name == "max_ticket_wip" && !v.Passed {
			if v.Enforcement == teamstate.EnforcementRefuse {
				fmt.Fprintf(a.IO.Out, "%s %s\n",
					theme.ErrorStyle.Render(theme.IconWarning), v.Message)
				fmt.Fprintf(a.IO.Out, "  %s\n", theme.Subtitle.Render(v.Details))
				return fmt.Errorf(i18n.T("cmd.claim.policy_violation"), v.Name)
			}
			// Warn only
			cliout.Warn(a.IO.Out, v.Message)
			fmt.Fprintf(a.IO.Out, "  %s\n\n", theme.Subtitle.Render(v.Details))
		}
	}

	initialStatus := teamstate.ClaimStatusInProgress
	if planned {
		initialStatus = teamstate.ClaimStatusPlanned
	}

	// Use board config for dynamic status resolution when available.
	if cfg, err := repo.LoadConfig(ctx); err == nil && cfg != nil && cfg.Board.HasCustomColumns() {
		if planned {
			initialStatus = cfg.Board.InitialStatus()
		} else {
			initialStatus = cfg.Board.DefaultWorkStatus()
		}
	}

	// One more ticket in progress for the member: refused past
	// max_in_progress_per_member. A ticket that is already claimed is not
	// new work (claimExists answers it).
	if !planned {
		if _, gerr := repo.GetClaim(ctx, project, ticketID); errors.Is(gerr, teamstate.ErrClaimNotFound) {
			if err := checkInProgressRoom(ctx, repo, trackerLocalOf(ctx, a), memberID, teamstate.ClaimRef{Project: project, TicketID: ticketID}); err != nil {
				return err
			}
		}
	}

	// Taken by hand: no session works on the ticket, so the claim carries
	// none (the session that starts on it takes it, AdoptClaim). Its lease
	// lasts lease_hours_manual (72 h by default) and nothing renews it by
	// itself; claiming the ticket again does.
	claim := teamstate.Claim{
		TicketID:  ticketID,
		Project:   project,
		ClaimedBy: memberID,
		ClaimedAt: time.Now().UTC(),
		Worktree:  worktree,
		Status:    initialStatus,
	}

	// The claim.taken event goes in the commit of the claim.
	existing, err := repo.CreateClaim(teamstate.WithClaimEvent(ctx, memberID), claim)
	if errors.Is(err, teamstate.ErrClaimExists) {
		return claimExists(cmd, a, repo, existing, project, ticketID, memberID)
	}
	if err != nil {
		return fmt.Errorf(i18n.T("cmd.claim.creating_claim"), err)
	}

	cliout.Success(a.IO.Out, i18n.Tf("cmd.claim.success", project, ticketID, memberID))
	return nil
}

// claimTakeover is what moving the claim of another member to oneself asks
// of the team-state: the takeover brief and the transfer.
type claimTakeover interface {
	teamstate.ClaimLifecycle
}

// claimConflictStore is what answering a ticket that is already claimed asks
// of the team-state.
type claimConflictStore interface {
	teamstate.ClaimReader
	teamstate.ConfigReader
	teamstate.ClaimRenewer
	teamstate.EventWriter
	claimTakeover
}

// claimExists answers a ticket that is already claimed. A claim of another
// member that has expired (its lease is past, or, without lease, it has been
// inactive longer than stale_days; never one in review: it stays reserved
// until the merge or its release) can be taken over after the common
// confirmation (--yes without a terminal): a takeover brief is generated, the
// claim comes with a new lease. The member's own claim is renewed (lease
// only), expired or not, without confirmation: for a claim taken by hand, it
// is the only renewal. Any other claim is reported and left as it is.
//
// The claim is taken by hand, no session works on it: it carries none (the
// first session that starts on the ticket takes it, AdoptClaim).
func claimExists(cmd *cobra.Command, a *app.App, repo claimConflictStore, existing *teamstate.Claim, project, ticketID, memberID string) error {
	ctx := cmd.Context()
	if existing == nil {
		return nil
	}
	teamCfg, _ := repo.LoadConfig(ctx)
	now := time.Now()
	expired := existing.IsExpired(now, teamCfg)

	if existing.ClaimedBy == memberID {
		// One's own claim is renewed, expired or not, without confirmation.
		end, err := repo.RenewClaim(ctx, project, ticketID, memberID, now)
		switch {
		case err == nil:
			key := "cmd.claims.own_renewed_live"
			if expired {
				key = "cmd.claims.own_renewed"
			}
			cliout.Success(a.IO.Out, i18n.Tf(key, project, ticketID, textfmt.DateTime(end)))
			return nil
		case errors.Is(err, teamstate.ErrNoLease):
			cliout.Warn(a.IO.Out, i18n.Tf("cmd.claims.own_no_lease", project, ticketID))
			return nil
		default:
			return fmt.Errorf(i18n.T("cmd.claim.creating_claim"), err)
		}
	}

	if expired && existing.ClaimedBy != "" && existing.ClaimedBy != memberID {
		// The claim comes to the member as it is: in progress, it is one more.
		if err := checkTransferToSelfRoom(ctx, repo, trackerLocalOf(ctx, a), memberID, existing); err != nil {
			return err
		}
		return takeOverExpiredClaim(cmd, a, repo, existing, project, ticketID, memberID)
	}

	// Not expired — standard warning
	cliout.Warn(a.IO.Out, i18n.Tf("cmd.claim.already_taken",
		project, ticketID,
		theme.Bold.Render(existing.ClaimedBy),
		textfmt.ShortDateTime(existing.ClaimedAt)))
	var board teamstate.BoardConfig
	if teamCfg != nil {
		board = teamCfg.Board
	}
	if existing.InReviewOn(board) {
		fmt.Fprintln(a.IO.Out, i18n.T("cmd.claims.in_review"))
	}
	fmt.Fprintf(a.IO.Out, "%s\n",
		i18n.Tf("cmd.claim.transfer_hint",
			theme.Bold.Render("oh team claim transfer "+ticketID+" --to "+memberID)))

	// Emit conflict event (best-effort).
	_ = repo.AppendEvent(ctx, teamstate.Event{
		Actor:   memberID,
		Type:    teamstate.EventClaimConflict,
		Project: project,
		Ticket:  ticketID,
		Data:    map[string]interface{}{"current_owner": existing.ClaimedBy},
	})
	return nil
}

// takeOverExpiredClaim says why the claim of another member can be taken over,
// asks for the confirmation, then generates the takeover brief and moves the
// claim to the member.
func takeOverExpiredClaim(cmd *cobra.Command, a *app.App, repo claimTakeover, existing *teamstate.Claim, project, ticketID, memberID string) error {
	ctx := cmd.Context()
	previousOwner := existing.ClaimedBy
	reason := "stale"
	if !existing.LeaseUntil.IsZero() {
		reason = "expired"
		cliout.Warn(a.IO.Out, i18n.Tf("cmd.claims.expired.lease", project, ticketID, theme.Bold.Render(previousOwner), textfmt.DateTime(existing.LeaseUntil.Time())))
	} else {
		since := existing.ClaimedAt
		if !existing.LastActivity.IsZero() {
			since = existing.LastActivity.Time()
		}
		cliout.Warn(a.IO.Out, i18n.Tf("cmd.claim.stale_warning", project, ticketID, theme.Bold.Render(previousOwner), int(time.Since(since).Hours()/24)))
	}
	ok, err := confirmActionWith(cmd,
		i18n.Tf("cmd.claims.expired.question", project, ticketID, previousOwner),
		i18n.T("cmd.claims.expired.description"), true)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintf(a.IO.Out, "%s\n", i18n.Tf("cmd.claims.expired.declined", previousOwner))
		return nil
	}

	// The takeover brief, the claim and the claim.transferred event (signed by
	// the member who takes over) are one commit.
	if _, err := repo.TransferClaimWith(teamstate.WithActor(ctx, memberID), project, ticketID, memberID,
		teamstate.TransferOpts{Reason: reason, BriefReason: reason}); err != nil {
		return explainHandOver(fmt.Errorf(i18n.T("cmd.claim.transferring_claim"), err), memberID)
	}
	cliout.Success(a.IO.Out, i18n.T("cmd.claim.brief_generated"))
	cliout.Success(a.IO.Out, i18n.Tf("cmd.claim.transferred_from_to", project, ticketID, previousOwner, memberID))
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
