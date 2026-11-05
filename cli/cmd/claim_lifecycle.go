package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// Release and hand-over of the claims of the team by the member who acts: one
// claim (oh team release, oh team claim transfer <ticket>) or all the open
// claims of a member (oh team claim transfer --from). The member who acts signs
// the events; a claim that another member holds is named in the confirmation,
// and --force is the answer without a terminal.

var releaseCmd = &cobra.Command{
	Use:   "release <ticket-id>",
	Short: i18n.T("cmd.release.short"),
	Long:  i18n.T("cmd.release.long"),
	Args:  cobra.ExactArgs(1),
	RunE:  runRelease,
}

var claimTransferCmd = &cobra.Command{
	Use:   "transfer [ticket-id]",
	Short: i18n.T("cmd.claim.transfer.short"),
	Long:  i18n.T("cmd.claim.transfer.long"),
	Args:  cobra.MaximumNArgs(1),
	RunE:  runClaimTransfer,
}

func init() {
	releaseCmd.Flags().StringP("project", "p", "", i18n.T("cmd.claim.flags.project"))
	releaseCmd.Flags().Bool("force", false, i18n.T("v54l.claim.flags.force"))

	claimTransferCmd.Flags().String("to", "", i18n.T("cmd.claim.transfer.flags.to"))
	claimTransferCmd.Flags().String("from", "", i18n.T("cmd.claim.transfer.flags.from"))
	claimTransferCmd.Flags().Bool("release", false, i18n.T("cmd.claim.transfer.flags.release"))
	claimTransferCmd.Flags().Bool("force", false, i18n.T("v54l.claim.flags.force"))
	claimTransferCmd.Flags().StringP("project", "p", "", i18n.T("cmd.claim.flags.project"))
}

// confirmAgainstOwner asks before an action on a claim another member holds
// (or on a whole batch of claims). --force answers yes; without a terminal and
// without --force it is a usage error: nothing is guessed.
func confirmAgainstOwner(cmd *cobra.Command, question, description string) (bool, error) {
	if force, _ := cmd.Flags().GetBool("force"); force {
		return true, nil
	}
	if !stdinIsTerminal() {
		return false, withExitCode(ExitUsage, errors.New(i18n.T("v54l.claim.needs_force")))
	}
	return confirmPrompt(question, description, false)
}

// explainHandOver words the refusals of a hand-over that the member can act on.
func explainHandOver(err error, to string) error {
	switch {
	case errors.Is(err, teamstate.ErrMemberNotFound):
		return errors.New(i18n.Tf("v54l.transfer.unknown_member", to, theme.Bold.Render("oh team status")))
	case errors.Is(err, teamstate.ErrClaimClosed):
		return errors.New(i18n.T("v54l.transfer.closed"))
	}
	return err
}

// claimCommandProject is the project a claim command works on: -p, else the project
// of the current folder.
func claimCommandProject(cmd *cobra.Command, a *app.App) (string, error) {
	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		project = detectCurrentProject(cmd.Context(), a)
		if project == "" {
			return "", errors.New(i18n.T("cmd.claim.project_not_detected"))
		}
	}
	return project, nil
}

func runRelease(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()
	repo := teamRepo

	ticketID := args[0]
	project, err := claimCommandProject(cmd, a)
	if err != nil {
		return err
	}

	me := commandMember(ctx, a)
	owner := ""
	if existing, gerr := repo.GetClaim(ctx, project, ticketID); gerr == nil {
		owner = existing.ClaimedBy
	}
	if owner != "" && owner != me {
		ok, err := confirmAgainstOwner(cmd,
			i18n.Tf("v54l.release.confirm", project, ticketID, owner),
			i18n.Tf("v54l.release.confirm_desc", owner))
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(a.IO.Out, i18n.Tf("v54l.release.declined", project, ticketID, owner))
			return nil
		}
	}

	_, err = repo.ReleaseClaimAs(teamstate.WithActor(ctx, me), project, ticketID, teamstate.ReleaseOpts{Owner: owner})
	var notOwned *teamstate.ClaimNotOwnedError
	switch {
	case errors.Is(err, teamstate.ErrClaimNotFound):
		cliout.Warn(a.IO.Out, i18n.Tf("cmd.release.not_found", project, ticketID))
		return nil
	case errors.As(err, &notOwned):
		return errors.New(i18n.Tf("v54l.release.moved", project, ticketID, notOwned.Owner))
	case err != nil:
		return fmt.Errorf(i18n.T("cmd.claim.releasing_claim"), err)
	}
	cliout.Success(a.IO.Out, i18n.Tf("cmd.release.success", project, ticketID))
	return nil
}

func runClaimTransfer(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()
	repo := teamRepo

	to, _ := cmd.Flags().GetString("to")
	from, _ := cmd.Flags().GetString("from")
	release, _ := cmd.Flags().GetBool("release")

	if len(args) == 0 {
		switch {
		case from == "":
			return withExitCode(ExitUsage, errors.New(i18n.T("v54l.transfer.usage")))
		case to == "" && !release, to != "" && release:
			return withExitCode(ExitUsage, errors.New(i18n.T("v54l.transfer.to_or_release")))
		}
		return transferAllClaims(cmd, a, repo, from, to, release)
	}
	switch {
	case from != "":
		return withExitCode(ExitUsage, errors.New(i18n.T("v54l.transfer.from_is_bulk")))
	case release:
		return withExitCode(ExitUsage, errors.New(i18n.Tf("v54l.transfer.release_is_release", theme.Bold.Render("oh team release "+args[0]))))
	case to == "":
		return withExitCode(ExitUsage, errors.New(i18n.T("v54l.transfer.to_required")))
	}

	ticketID := args[0]
	project, err := claimCommandProject(cmd, a)
	if err != nil {
		return err
	}
	me := commandMember(ctx, a)

	existingClaim, _ := repo.GetClaim(ctx, project, ticketID)
	if existingClaim != nil && existingClaim.ClaimedBy != "" && existingClaim.ClaimedBy != me && existingClaim.ClaimedBy != to {
		ok, err := confirmAgainstOwner(cmd,
			i18n.Tf("v54l.transfer.confirm", project, ticketID, existingClaim.ClaimedBy, to),
			i18n.Tf("v54l.transfer.confirm_desc", existingClaim.ClaimedBy))
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(a.IO.Out, i18n.Tf("v54l.transfer.declined", project, ticketID, existingClaim.ClaimedBy))
			return nil
		}
	}

	// Handing a claim in progress to oneself is one more ticket in progress.
	if to == me {
		if err := checkTransferToSelfRoom(ctx, repo, trackerLocalOf(ctx, a), me, existingClaim); err != nil {
			return err
		}
	}

	// The claim, the takeover brief and the claim.transferred event (signed by
	// the member who hands the claim over) are one commit.
	previous, err := repo.TransferClaimWith(teamstate.WithActor(ctx, me), project, ticketID, to,
		teamstate.TransferOpts{BriefReason: "transfer"})
	if err != nil {
		if errors.Is(err, teamstate.ErrClaimNotFound) {
			cliout.Warn(a.IO.Out, i18n.Tf("cmd.claim.transfer.not_found", project, ticketID))
			return nil
		}
		return explainHandOver(fmt.Errorf(i18n.T("cmd.claim.transferring_claim"), err), to)
	}

	cliout.Success(a.IO.Out, i18n.Tf("cmd.claim.transfer.success", project, ticketID, theme.Bold.Render(to)))
	if previous != "" && previous != to {
		cliout.Success(a.IO.Out, i18n.Tf("cmd.claim.transfer.brief_done",
			theme.Subtitle.Render("oh takeover-brief show "+ticketID)))
	}
	return nil
}

// bulkClaimStore is what moving the claims of a member asks of the team-state.
type bulkClaimStore interface {
	teamstate.RepoSync
	teamstate.MemberClaimMover
	wipStore
}

// transferAllClaims is oh team claim transfer --from <member>: the open claims
// of the member go to another member, or are released, in one commit.
func transferAllClaims(cmd *cobra.Command, a *app.App, repo bulkClaimStore, from, to string, release bool) error {
	ctx := cmd.Context()
	me := commandMember(ctx, a)
	if err := repo.Pull(ctx); err != nil {
		cliout.Warn(a.IO.ErrOut, i18n.T("cmd.claim.sync_failed"))
	}
	open, done, err := repo.OpenClaimsOf(ctx, from)
	if err != nil {
		return err
	}
	if len(open) == 0 {
		fmt.Fprintln(a.IO.Out, i18n.Tf("v54l.transfer.nothing", theme.Bold.Render(from), len(done)))
		return nil
	}

	fmt.Fprintln(a.IO.Out, i18n.Tf("v54l.transfer.listing", theme.Bold.Render(from), len(open)))
	for _, c := range open {
		fmt.Fprintf(a.IO.Out, "  - %s/%s (%s)\n", c.Project, c.TicketID, c.Status)
	}
	if len(done) > 0 {
		fmt.Fprintln(a.IO.Out, i18n.Tf("v54l.transfer.done_stay", len(done)))
	}

	if to == me && to != "" {
		if err := checkBulkRoom(ctx, repo, a, me, open); err != nil {
			return err
		}
	}

	question := i18n.Tf("v54l.transfer.bulk_release", len(open), from)
	if to != "" {
		question = i18n.Tf("v54l.transfer.bulk_transfer", len(open), from, to)
	}
	ok, err := confirmAgainstOwner(cmd, question, i18n.Tf("v54l.transfer.bulk_desc", from))
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(a.IO.Out, i18n.T("v54l.transfer.bulk_declined"))
		return nil
	}

	res, err := repo.ReassignClaims(teamstate.WithActor(ctx, me), teamstate.ReassignOpts{
		From: from, To: to, Release: release, Brief: to != "", ReleaseReason: teamstate.ReleaseBulk,
	})
	if err != nil {
		return explainHandOver(fmt.Errorf(i18n.T("cmd.claim.transferring_claim"), err), to)
	}
	if release {
		cliout.Success(a.IO.Out, i18n.Tf("v54l.transfer.bulk_released", len(res.Released), theme.Bold.Render(from)))
		return nil
	}
	cliout.Success(a.IO.Out, i18n.Tf("v54l.transfer.bulk_transferred", len(res.Transferred), theme.Bold.Render(from), theme.Bold.Render(to)))
	return nil
}

// checkBulkRoom checks that handing the claims to the member keeps them within
// their limit of tickets in progress.
func checkBulkRoom(ctx context.Context, repo wipStore, a *app.App, member string, claims []teamstate.Claim) error {
	cfg, err := repo.LoadConfig(ctx)
	if err != nil || cfg == nil {
		return nil //nolint:nilerr // an unreadable team state never blocks the work
	}
	var becoming []teamstate.ClaimRef
	for i := range claims {
		if claims[i].CountsInProgress(cfg.Board) {
			becoming = append(becoming, teamstate.ClaimRef{Project: claims[i].Project, TicketID: claims[i].TicketID})
		}
	}
	return checkInProgressRoom(ctx, repo, trackerLocalOf(ctx, a), member, becoming...)
}
