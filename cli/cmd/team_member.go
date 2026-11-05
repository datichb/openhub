package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var teamMemberCmd = &cobra.Command{
	Use:   "member",
	Short: i18n.T("v54l.team.member.short"),
	Long:  i18n.T("v54l.team.member.long"),
}

var teamMemberRemoveCmd = &cobra.Command{
	Use:   "remove <member-id>",
	Short: i18n.T("v54l.team.member.remove.short"),
	Long:  i18n.T("v54l.team.member.remove.long"),
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamMemberRemove,
}

func init() {
	teamCmd.AddCommand(teamMemberCmd)
	teamMemberCmd.AddCommand(teamMemberRemoveCmd)
	teamMemberRemoveCmd.Flags().String("to", "", i18n.T("v54l.team.member.remove.flags.to"))
	teamMemberRemoveCmd.Flags().Bool("release", false, i18n.T("v54l.team.member.remove.flags.release"))
	addYesFlag(teamMemberRemoveCmd)
}

// runTeamMemberRemove is oh team member remove <id>: another member's card goes
// out of members.toml. Only a lead may (a team with no lead is not locked), and
// the open claims of the member are handed to another member (--to) or released
// (--release) in the same commit; a member who still holds claims is not
// removed without an answer. Leaving oneself is oh teams leave.
func runTeamMemberRemove(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()
	repo := teamRepo
	id := args[0]
	to, _ := cmd.Flags().GetString("to")
	release, _ := cmd.Flags().GetBool("release")
	if to != "" && release {
		return withExitCode(ExitUsage, errors.New(i18n.T("v54l.team.member.remove.to_or_release")))
	}

	me := commandMember(ctx, a)
	if me == "" {
		return errors.New(i18n.Tf("cmd.claim.member_not_configured", theme.Bold.Render("oh team init")))
	}
	if id == me {
		team, _ := ResolveTeam(ctx, a)
		return errors.New(i18n.Tf("v54l.team.member.remove.self", theme.Bold.Render("oh teams leave "+team.TeamID+" --remove-member")))
	}
	if err := repo.Pull(ctx); err != nil {
		cliout.Warn(a.IO.ErrOut, i18n.T("cmd.claim.sync_failed"))
	}
	card, err := repo.GetMember(ctx, id)
	if err != nil {
		if errors.Is(err, teamstate.ErrMemberNotFound) {
			return errors.New(i18n.Tf("v54l.team.member.remove.unknown", id, theme.Bold.Render("oh team status")))
		}
		return err
	}
	// The refusal comes before any question; the transaction checks it again
	// on the latest state.
	if repo.HasLead(ctx) && !repo.IsLead(ctx, me) {
		return errors.New(i18n.Tf("v54l.team.member.remove.needs_lead", me))
	}

	open, done, err := repo.OpenClaimsOf(ctx, id)
	if err != nil {
		return err
	}
	if len(open) > 0 && to == "" && !release {
		return withExitCode(ExitUsage, errors.New(i18n.Tf("v54l.team.member.remove.has_claims",
			id, len(open), theme.Bold.Render("--to <member>"), theme.Bold.Render("--release"))))
	}
	if to == id {
		return withExitCode(ExitUsage, errors.New(i18n.T("v54l.team.member.remove.to_self")))
	}
	if to == me {
		if err := checkBulkRoom(ctx, repo, a, me, open); err != nil {
			return err
		}
	}

	question := i18n.Tf("v54l.team.member.remove.confirm", card.DisplayName, id)
	switch {
	case len(open) == 0:
	case to != "":
		question += " " + i18n.Tf("v54l.team.member.remove.confirm_to", len(open), to)
	default:
		question += " " + i18n.Tf("v54l.team.member.remove.confirm_release", len(open))
	}
	if len(done) > 0 {
		question += " " + i18n.Tf("v54l.team.member.remove.confirm_done", len(done))
	}
	ok, err := confirmAction(cmd, question, false)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	res, err := repo.ReassignClaims(teamstate.WithActor(ctx, me), teamstate.ReassignOpts{
		From: id, To: to, Release: release, Brief: to != "", RemoveMember: true,
		ReleaseReason: teamstate.ReleaseMemberRemoved,
	})
	switch {
	case errors.Is(err, teamstate.ErrRemoveNeedsLead):
		return errors.New(i18n.Tf("v54l.team.member.remove.needs_lead", me))
	case err != nil:
		return explainHandOver(err, to)
	}
	cliout.Success(a.IO.Out, i18n.Tf("v54l.team.member.remove.done", theme.Bold.Render(id)))
	if n := len(res.Transferred); n > 0 {
		fmt.Fprintln(a.IO.Out, i18n.Tf("v54l.team.member.remove.done_to", n, theme.Bold.Render(to)))
	}
	if n := len(res.Released); n > 0 {
		fmt.Fprintln(a.IO.Out, i18n.Tf("v54l.team.member.remove.done_released", n))
	}
	if n := len(res.Kept); n > 0 {
		fmt.Fprintln(a.IO.Out, i18n.Tf("v54l.team.member.remove.done_kept", n))
	}
	return nil
}
