package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// oh team rejoin: the text version of « Rejoindre une équipe ». In this order:
//
//  1. the access to the repository (probe, then the token of an HTTPS one,
//     checked and kept in the keychain of oh), before anything is cloned;
//  2. the clone, and the members of the team;
//  3. who the person is: a card of the list (« C'est moi »), or their own
//     card, created here (the identity is verified before it is published);
//  4. the name on the tracker, the GitLab API token, then hub.toml.

func addTeamRejoinFlags(c *cobra.Command) {
	c.Flags().String("repo", "", i18n.T("cmd.team.rejoin.flags.repo"))
	c.Flags().String("member-id", "", i18n.T("cmd.team.rejoin.flags.member_id"))
	c.Flags().Bool("no-retro-tag", false, i18n.T("cmd.team.rejoin.flags.no_retro_tag"))
	c.Flags().String("gitlab-url", "", i18n.T("cmd.team.rejoin.flags.gitlab_url"))
	c.Flags().String("tracker-username", "", i18n.T("cmd.team.rejoin.flags.tracker_username"))
	c.Flags().Bool("new-member", false, i18n.T("v54a.rejoin.flags.new_member"))
	c.Flags().String("display-name", "", i18n.T("v54a.rejoin.flags.display_name"))
	c.Flags().String("gitlab-username", "", i18n.T("v54a.rejoin.flags.gitlab_username"))
	c.Flags().String("role", "", i18n.T("v54a.rejoin.flags.role"))
}

// rejoinFlags are the options of oh team rejoin (and of oh teams add, which is
// the same command).
type rejoinFlags struct {
	repo, memberID, gitlabURL string
	noRetroTag, newMember     bool
	displayName, handle, role string
	trackerSet                bool
	tracker                   string
	// teamID and teamName are the ID and the name asked for the team in
	// hub.toml (oh teams add --id, --name).
	teamID, teamName string
}

func readRejoinFlags(cmd *cobra.Command) rejoinFlags {
	f := rejoinFlags{}
	f.repo, _ = cmd.Flags().GetString("repo")
	f.memberID, _ = cmd.Flags().GetString("member-id")
	f.gitlabURL, _ = cmd.Flags().GetString("gitlab-url")
	f.noRetroTag, _ = cmd.Flags().GetBool("no-retro-tag")
	f.newMember, _ = cmd.Flags().GetBool("new-member")
	f.displayName, _ = cmd.Flags().GetString("display-name")
	f.handle, _ = cmd.Flags().GetString("gitlab-username")
	f.role, _ = cmd.Flags().GetString("role")
	if cmd.Flags().Changed("tracker-username") {
		f.trackerSet = true
		f.tracker, _ = cmd.Flags().GetString("tracker-username")
	}
	return f
}

// cliRejoin is one run of oh team rejoin.
type cliRejoin struct {
	ctx   context.Context
	a     *app.App
	p     *cliPrompter
	flags rejoinFlags

	teamRepo rejoinRepo
	members  []teamstate.Member
	member   *teamstate.Member
	// tokenAsked: the GitLab API token was asked (before the card was
	// published, when the person created it).
	tokenAsked bool
}

func runTeamRejoin(cmd *cobra.Command, _ []string) error {
	a := MustApp()
	r := &cliRejoin{ctx: cmd.Context(), a: a, p: newCLIPrompter(a), flags: readRejoinFlags(cmd)}
	return r.run()
}

func (r *cliRejoin) run() error {
	a, f := r.a, &r.flags
	out := a.IO.Out
	if _, err := os.Stat(config.ConfigPath()); os.IsNotExist(err) {
		return fmt.Errorf("%s", i18n.Tf("cmd.team.rejoin.hub_not_configured", theme.Bold.Render("oh init")))
	}
	f.gitlabURL = strings.TrimSpace(f.gitlabURL)
	if msg := teamGitLabURLProblem(f.gitlabURL); msg != "" {
		return errors.New(msg)
	}
	if f.repo == "" {
		return errors.New(i18n.T("cmd.team.repo_is_required_url_of_the"))
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, theme.Title.Render("  Team Rejoin  "))
	fmt.Fprintln(out)

	// 1. the access, before the clone
	if err := r.p.ensureAccess(r.ctx, a, f.repo); err != nil {
		return err
	}
	// 2. the clone and the members
	if err := r.clone(); err != nil {
		return err
	}
	// 3. who the person is
	if err := r.pickMember(); err != nil {
		return err
	}
	// 4. tracker, GitLab token, hub.toml
	trackerName := r.askTracker()
	if !r.tokenAsked {
		r.askGitLabToken(r.member.GitLabUsername)
	}
	return r.finish(trackerName)
}

// clone brings the team-state down and lists its members.
func (r *cliRejoin) clone() error {
	out := r.a.IO.Out
	fmt.Fprint(out, i18n.Tf("cmd.team.cloning_team_state_2", theme.WarningStyle.Render(theme.IconDot)))
	repo, members, stale, err := listTeamMembers(r.ctx, r.flags.repo, teamStatePathFor(r.a, r.flags.repo))
	if err != nil {
		return err
	}
	if stale != nil {
		fmt.Fprintf(out, "\n  %s %s\n", theme.WarningStyle.Render(theme.IconWarning), i18n.Tf("v532m.pick.stale", staleCause(stale)))
	}
	r.teamRepo, r.members = repo, members
	return nil
}

// pickMember settles who the person is: the card given with --member-id, their
// own card to create (--new-member, or the first choice of the list), or the
// card they pick in the list.
func (r *cliRejoin) pickMember() error {
	f := &r.flags
	switch {
	case f.newMember:
		return r.createCard()
	case f.memberID != "":
		m, ok := memberByID(r.members, f.memberID)
		if !ok {
			return fmt.Errorf(i18n.T("cmd.team.member_not_found_in_the_team"), f.memberID)
		}
		r.member = &m
		return nil
	}
	out := r.a.IO.Out
	fmt.Fprintln(out)
	fmt.Fprintln(out, i18n.T("cmd.team.rejoin.select_member"))
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  0. %s\n", i18n.T("v532m.pick.create"))
	for i, m := range r.members {
		gitlab := ""
		if m.GitLabUsername != "" {
			gitlab = fmt.Sprintf(" — gitlab: %s", m.GitLabUsername)
		}
		fmt.Fprintf(out, "  %d. %s (%s)%s\n", i+1, m.DisplayName, m.ID, gitlab)
	}
	fmt.Fprintln(out)
	choice, err := strconv.Atoi(r.p.line(i18n.Tf("cmd.team.rejoin.choose", len(r.members))))
	if err != nil || choice < 0 || choice > len(r.members) {
		return errors.New(i18n.T("cmd.team.invalid_selection"))
	}
	if choice == 0 {
		return r.createCard()
	}
	r.member = &r.members[choice-1]
	return nil
}

// ask is the value of a flag, else the answer to a question (the default on
// Enter).
func (r *cliRejoin) ask(flag, labelKey, def string) string {
	if flag != "" {
		return flag
	}
	label := i18n.T(labelKey)
	if def != "" {
		label += " [" + def + "]"
	}
	return firstNonEmpty(r.p.line("  "+label+": "), def)
}

// createCard makes the card of the person in the team-state. The identity is
// verified (not blocking) before the card is published: a token that is
// somebody else's is told, and the person decides. A taken ID is refused, with
// « C'est moi » to attach the card that exists.
func (r *cliRejoin) createCard() error {
	a, out, f := r.a, r.a.IO.Out, &r.flags
	defID, defName, defHandle := identityDefaults(a)()
	id := r.ask(f.memberID, "v532m.identity.id", defID)
	name := r.ask(f.displayName, "v532m.identity.display_name", firstNonEmpty(defName, id))
	handle := teamstate.NormalizeHandle(r.ask(f.handle, "v532m.identity.gitlab", defHandle))
	role := firstNonEmpty(f.role, joinerRole)
	if !slices.Contains(teamRoles, role) {
		return fmt.Errorf("%s", i18n.Tf("v54a.rejoin.bad_role", role, strings.Join(teamRoles, ", ")))
	}
	own := teamService(a).MemberIDOf(f.repo)

	claim := false
	for attempt := 0; ; attempt++ {
		m := teamstate.Member{ID: strings.TrimSpace(id), DisplayName: name, GitLabUsername: handle, Role: role}
		if problem := (identityFields{ID: &m.ID, DisplayName: &m.DisplayName, GitLab: &m.GitLabUsername, Role: &m.Role}).problem(nil, "", ""); problem != "" {
			return errors.New(problem)
		}
		if attempt == 0 && m.GitLabUsername != "" {
			r.askGitLabToken(m.GitLabUsername)
			if !r.confirmIdentity(m) {
				return errors.New(i18n.T("v54a.rejoin.identity_refused"))
			}
		}
		res, err := r.teamRepo.JoinMember(r.ctx, m, teamstate.JoinModeFor(own, m.ID, claim))
		if err == nil {
			r.member = &res.Member
			r.members = replaceMember(r.members, res.Member)
			if res.Created {
				fmt.Fprintf(out, "  %s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("v54a.rejoin.card_created", res.Member.ID))
			}
			return nil
		}
		if !isJoinRefusal(err) || attempt >= 2 {
			return joinRefusalError(err, "v532m.join.id_taken_hint")
		}
		fmt.Fprintf(out, "  %s %s\n", theme.WarningStyle.Render(theme.IconWarning), joinRefusalText(err, "v532m.join.id_taken_hint"))
		if errors.Is(err, teamstate.ErrMemberExists) && !claim && r.p.yes("  "+i18n.T("v54a.rejoin.claim_question"), false) {
			claim = true
			continue
		}
		claim = false
		if errors.Is(err, teamstate.ErrMemberExists) {
			id = r.p.line("  " + i18n.T("v532m.identity.id") + ": ")
		} else {
			handle = teamstate.NormalizeHandle(r.p.line("  " + i18n.T("v532m.identity.gitlab") + ": "))
		}
	}
}

// confirmIdentity verifies the GitLab identity of the card before it is
// published. A token that cannot tell (none, refused, GitLab unreachable) does
// not stop anything; a token that is somebody else's is told, and the person
// decides.
func (r *cliRejoin) confirmIdentity(m teamstate.Member) bool {
	out := r.a.IO.Out
	err := precheckIdentity(r.ctx, r.a, r.flags.repo, r.flags.gitlabURL, m)
	var mismatch *identityMismatchError
	var unverified *identityUnverifiedError
	switch {
	case err == nil:
		return true
	case errors.As(err, &mismatch):
		fmt.Fprintf(out, "  %s %s\n", theme.WarningStyle.Render(theme.IconWarning), mismatch.Error())
		return r.p.yes("  "+i18n.T("v54a.rejoin.publish_anyway"), false)
	case errors.As(err, &unverified):
		fmt.Fprintf(out, "  %s %s\n", theme.WarningStyle.Render(theme.IconWarning), unverified.Error())
	default:
		slog.Warn("identity check before publishing the card", "error", err)
	}
	return true
}

// askTracker is the name of the person on the tracker: given with
// --tracker-username, else asked when the tracker is on another server than the
// team repository and their card has none yet (nil: not touched).
func (r *cliRejoin) askTracker() *string {
	f := &r.flags
	if f.trackerSet {
		v := f.tracker
		return &v
	}
	if r.member.TrackerUsername != "" || !trackerUsernameWanted(r.ctx, r.teamRepo, f.repo, f.gitlabURL) {
		return nil
	}
	out := r.a.IO.Out
	fmt.Fprintf(out, "\n  %s %s\n", theme.InfoStyle.Render(theme.IconInfo), i18n.T("cmd.team.rejoin.tracker_hint"))
	if name := r.p.line("  " + i18n.T("cmd.team.rejoin.tracker_prompt")); name != "" {
		return &name
	}
	return nil
}

// askGitLabToken asks what to do with the GitLab API token (identity check,
// tracker) when the card has a GitLab handle: reuse the one that is known, give
// another one, or leave it.
func (r *cliRejoin) askGitLabToken(handle string) {
	if handle == "" || r.tokenAsked {
		return
	}
	r.tokenAsked = true
	a, out, f := r.a, r.a.IO.Out, &r.flags
	teamID := teamIDFor(a, f.repo)
	source := gitlabTokenSourceAt(r.ctx, a, teamID, f.repo, f.gitlabURL)

	fmt.Fprintln(out)
	askNew := false
	if source != "" {
		fmt.Fprintf(out, "  %s %s\n", theme.InfoStyle.Render(theme.IconInfo), i18n.Tf("cmd.init.wizard_rejoin_token_reuse", source))
		fmt.Fprintf(out, "  [1] %s\n", i18n.T("cmd.init.wizard_rejoin_token_reuse_short"))
		fmt.Fprintf(out, "  [2] %s\n", i18n.T("cmd.init.wizard_rejoin_token_new"))
		fmt.Fprintf(out, "  [3] %s\n", i18n.T("cmd.init.wizard_rejoin_token_skip"))
		askNew = r.p.line(i18n.Tf("cmd.init.wizard_choice_prompt", 1)) == "2"
	} else {
		fmt.Fprintf(out, "  %s %s\n", theme.WarningStyle.Render(theme.IconDot), i18n.T("cmd.init.wizard_rejoin_gitlab_hint"))
		fmt.Fprintf(out, "  [1] %s\n", i18n.T("cmd.init.wizard_rejoin_token_new"))
		fmt.Fprintf(out, "  [2] %s\n", i18n.T("cmd.init.wizard_rejoin_token_skip"))
		askNew = r.p.line(i18n.Tf("cmd.init.wizard_choice_prompt", 1)) != "2"
	}
	if askNew {
		r.storeGitLabToken(teamID)
	}
}

// storeGitLabToken asks the token and keeps it in the keychain of oh, under the
// key of the team.
func (r *cliRejoin) storeGitLabToken(teamID string) {
	token := r.p.secret("  " + i18n.T("cmd.init.wizard_rejoin_gitlab_token_label") + ": ")
	if token == "" || r.a.Secrets == nil {
		return
	}
	if err := r.a.Secrets.Set(r.ctx, config.TeamGitLabTokenKey(teamID), token); err != nil {
		slog.Error("failed to save GitLab token to keychain", "team", teamID, "error", err)
	}
}

// finish rejoins the team as the member: verifies the identity, writes hub.toml,
// tags the sessions of this machine.
func (r *cliRejoin) finish(trackerName *string) error {
	a, out, f := r.a, r.a.IO.Out, &r.flags
	fmt.Fprintf(out, "\n  %s %s\n", theme.WarningStyle.Render(theme.IconDot), i18n.T("cmd.team.rejoin.validating"))
	result, err := teamRejoinCore(r.ctx, a, teamRejoinParams{
		StateRepo:       f.repo,
		MemberID:        r.member.ID,
		GitLabURL:       f.gitlabURL,
		TrackerUsername: trackerName,
		TeamID:          f.teamID,
		TeamName:        f.teamName,
	})
	if err != nil {
		return err
	}
	switch {
	case result.ProfileErr != nil:
		fmt.Fprintf(out, "  %s %s\n", theme.WarningStyle.Render(theme.IconWarning),
			i18n.Tf("cmd.team.rejoin.tracker_failed", result.ProfileErr))
	case trackerName != nil && result.Member.TrackerUsername != "":
		fmt.Fprintf(out, "  %s %s\n", theme.SuccessStyle.Render(theme.IconSuccess),
			i18n.Tf("cmd.team.rejoin.tracker_saved", result.Member.TrackerUsername))
	}
	if result.IdentityUnverified != nil {
		fmt.Fprintf(out, "  %s %s\n", theme.WarningStyle.Render(theme.IconWarning), result.IdentityUnverified.Error())
	}
	if result.IdentityMismatch != nil {
		r.recoverIdentity(result.IdentityMismatch)
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "  %s %s\n", theme.SuccessStyle.Render(theme.IconSuccess),
		i18n.Tf("cmd.team.rejoin.success", result.Member.DisplayName, result.Member.ID, result.TeamName))
	if result.EventCount > 0 {
		fmt.Fprintf(out, "  %s %s\n", theme.InfoStyle.Render(theme.IconInfo), i18n.Tf("cmd.team.rejoin.events_found", result.EventCount))
	}
	if !f.noRetroTag {
		if count, err := retroTagSessions(r.ctx, r.member.ID); err != nil {
			slog.Warn("retro-tag failed", "error", err)
		} else if count > 0 {
			fmt.Fprintf(out, "  %s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.team.rejoin.retro_tagged", count))
		}
	}
	fmt.Fprintf(out, "\n  %s %s\n\n", theme.InfoStyle.Render(theme.IconInfo), i18n.T("cmd.team.tracker_hint"))
	return nil
}

// recoverIdentity: the token is a bot's or somebody else's; the person can give
// a personal one, or go on without the verification.
func (r *cliRejoin) recoverIdentity(mismatch *identityMismatchError) {
	a, out, f := r.a, r.a.IO.Out, &r.flags
	msg := i18n.Tf("cmd.init.wizard_identity_user_mismatch", mismatch.AuthenticatedAs, mismatch.MemberID, mismatch.ExpectedUser)
	if mismatch.IsBot {
		msg = i18n.Tf("cmd.init.wizard_identity_bot_detected", mismatch.AuthenticatedAs)
	}
	fmt.Fprintf(out, "\n  %s %s\n", theme.WarningStyle.Render(theme.IconDot), msg)
	fmt.Fprintf(out, "  [1] %s\n", i18n.T("cmd.init.wizard_identity_verify"))
	fmt.Fprintf(out, "  [2] %s\n", i18n.T("cmd.init.wizard_identity_skip"))
	if r.p.line(i18n.Tf("cmd.init.wizard_choice_prompt", 2)) != "1" {
		return
	}
	r.storeGitLabToken(teamIDFor(a, f.repo))
	repo := teamstate.NewRepo(f.repo, teamStatePathFor(a, f.repo))
	member, _ := repo.GetMember(r.ctx, r.member.ID)
	if member == nil {
		return
	}
	if err := validateGitLabIdentity(r.ctx, a, repo, member); err != nil {
		fmt.Fprintf(out, "  %s %s\n", theme.WarningStyle.Render(theme.IconDot), err.Error())
		return
	}
	fmt.Fprint(out, i18n.Tf("cmd.team.identity_verified", theme.SuccessStyle.Render(theme.IconSuccess)))
}
