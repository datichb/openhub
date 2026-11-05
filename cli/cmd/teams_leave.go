package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/fsutil"
	"github.com/datichb/openhub/cli/internal/gitremote"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// Leaving a team: what `oh teams leave <id>` and the « Quitter l'équipe » action
// of the Teams view do, in this order — release my claims (one transaction;
// my card goes in the same one when asked), empty the queue of claim
// operations of the clone, delete the clone, delete the keys of the keychain,
// then take the team out of hub.toml (its default mark and its projects with
// it). Nothing of what is shared is destroyed: the card of the member, with
// the history it carries, stays in members.toml unless --remove-member says
// otherwise.

var teamsLeaveCmd = &cobra.Command{
	Use:   "leave <team-id>",
	Short: i18n.T("v54l.teams.leave.short"),
	Long:  i18n.T("v54l.teams.leave.long"),
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamsLeave,
}

func init() {
	teamsCmd.AddCommand(teamsLeaveCmd)
	teamsLeaveCmd.Flags().Bool("remove-member", false, i18n.T("v54l.teams.leave.flags.remove_member"))
	teamsLeaveCmd.Flags().Bool("force", false, i18n.T("v54l.teams.leave.flags.force"))
	addYesFlag(teamsLeaveCmd)
}

// leaveEnv is what leaving reads and writes: the application (its projects, its
// keychain), the hub.toml it reads, and the way it writes hub.toml. The TUI
// leaves off its event loop, which owns the Config of the app: it gives a copy
// of it and the write that hands the result back to the loop.
type leaveEnv struct {
	a      *app.App
	cfg    *config.Config
	update func(*app.App, func(*config.Config) error) error
}

// cliLeaveEnv is the environment of the commands.
func cliLeaveEnv(a *app.App) leaveEnv {
	return leaveEnv{a: a, cfg: a.Config, update: updateHubConfig}
}

// tuiLeaveEnv is the environment of the TUI; call it on the event loop.
func tuiLeaveEnv(a *app.App) leaveEnv {
	snapshot := *a.Config
	snapshot.Teams = slices.Clone(a.Config.Teams)
	return leaveEnv{a: a, cfg: &snapshot, update: updateHubConfigOffLoop}
}

// leaveOptions are the choices of the member who leaves.
type leaveOptions struct {
	// RemoveMember also takes the card of the member out of members.toml
	// (otherwise the card stays: it carries the history of the member).
	RemoveMember bool
	// Force goes on when the team-state cannot be reached (the claims then
	// stay until their lease runs out) or when the clone holds work the
	// remote does not have.
	Force bool
}

// leavePlan is what leaving a team will do, read from the local clone before
// anything is asked or changed.
type leavePlan struct {
	TeamID, Name string
	Solo         bool
	Member       string
	// Cloned: the clone exists, so the claims and the card can be reached.
	Cloned     bool
	ClonePath  string
	OpenClaims []teamstate.Claim
	DoneClaims int
	CardExists bool
	QueueOps   int
	Pending    teamstate.PendingWork
	// DeleteClone: the clone is deleted; KeepWhy says why it is not (solo
	// space, a folder that is not the clone of this team, shared by another
	// team).
	DeleteClone bool
	KeepWhy     string
	Keys        []string
	Projects    []string

	repo leaveStore
}

// leaveStore is what leaving asks of the clone of the team-state.
type leaveStore interface {
	teamstate.RepoInfo
	teamstate.MemberReader
	teamstate.MemberClaimMover
	teamstate.ClaimOpQueue
	teamstate.PendingWorkReader
}

// leaveReport is what leaving a team did.
type leaveReport struct {
	ClaimsReleased int
	CardRemoved    bool
	QueueLeft      []teamstate.ClaimOp
	CloneRemoved   string
	CloneKept      string
	KeysRemoved    []string
	Detached       []string
	Note           string
	Warnings       []string
}

// planLeave reads what leaving team id would touch. The claims, the card and
// the queue are read from the clone on this machine (no network).
func planLeave(ctx context.Context, env leaveEnv, id string) (*leavePlan, error) {
	a := env.a
	team := env.cfg.FindTeam(id)
	if team == nil {
		return nil, &config.UnknownTeamError{ID: id}
	}
	resolved := team.Resolved()
	p := &leavePlan{
		TeamID:    id,
		Name:      team.DisplayName(),
		Solo:      team.Solo,
		Member:    team.MemberID,
		ClonePath: resolved.StatePath,
		Projects:  teamProjectNames(ctx, a, id),
		Keys:      leaveSecretKeys(ctx, env, team),
	}
	if team.Solo {
		p.KeepWhy = i18n.T("v54l.leave.keep_solo")
		return p, nil
	}
	p.repo = teamstate.NewRepo(team.StateRepo, resolved.StatePath)
	if !p.repo.IsCloned() {
		p.KeepWhy = i18n.T("v54l.leave.keep_not_cloned")
		return p, nil
	}
	p.Cloned = true
	if p.Member != "" {
		if open, done, err := p.repo.OpenClaimsOf(ctx, p.Member); err == nil {
			p.OpenClaims, p.DoneClaims = open, len(done)
		}
		if m, err := p.repo.GetMember(ctx, p.Member); err == nil && m != nil {
			p.CardExists = true
		}
	}
	if ops, err := p.repo.ClaimQueue(); err == nil {
		p.QueueOps = len(ops)
	}
	if pending, err := p.repo.PendingWork(ctx); err == nil {
		p.Pending = pending
	}
	p.DeleteClone, p.KeepWhy = cloneDeletable(env.cfg, team, resolved.StatePath)
	return p, nil
}

// cloneDeletable says whether the folder at path may be deleted with the team:
// it must be the clone of the repository of the team, not a folder another team
// of hub.toml uses, and not the hub folder, the home folder or a root.
func cloneDeletable(cfg *config.Config, team *config.TeamConfig, path string) (ok bool, why string) {
	path = strings.TrimSpace(path)
	if path == "" || !filepath.IsAbs(path) {
		return false, i18n.T("v54l.leave.keep_no_path")
	}
	clean := filepath.Clean(path)
	if home, err := os.UserHomeDir(); err == nil && clean == filepath.Clean(home) {
		return false, i18n.Tf("v54l.leave.keep_unsafe", path)
	}
	if clean == filepath.Clean(config.HubDir()) || clean == filepath.VolumeName(clean)+string(filepath.Separator) {
		return false, i18n.Tf("v54l.leave.keep_unsafe", path)
	}
	if !gitremote.IsCloneOf(clean, team.StateRepo) {
		return false, i18n.Tf("v54l.leave.keep_other_repo", path)
	}
	for i := range cfg.Teams {
		other := &cfg.Teams[i]
		if other.ID != team.ID && filepath.Clean(other.Resolved().StatePath) == clean {
			return false, i18n.Tf("v54l.leave.keep_shared", path, other.ID)
		}
	}
	return true, ""
}

// leaveSecretKeys are the keychain keys that belong to the team alone: the
// ones named openhub.team.<id>.…, and the token and user of the git access to
// its repository (unless another team uses the same repository). Only keys
// that hold a value are listed.
func leaveSecretKeys(ctx context.Context, env leaveEnv, team *config.TeamConfig) []string {
	a := env.a
	if a.Secrets == nil {
		return nil
	}
	candidates := map[string]bool{config.TeamGitLabTokenKey(team.ID): true}
	for _, name := range provider.AllProviders() {
		if k := provider.TeamKeychainKey(name, team.ID); k != "" {
			candidates[k] = true
		}
	}
	prefix := "openhub.team." + team.ID + "."
	if listed, err := a.Secrets.List(ctx); err == nil {
		for _, k := range listed {
			if strings.HasPrefix(k, prefix) {
				candidates[k] = true
			}
		}
	}
	if base := config.GitRepoKeyBase(team.StateRepo); base != "" {
		shared := false
		for i := range env.cfg.Teams {
			if o := &env.cfg.Teams[i]; o.ID != team.ID && config.GitRepoKeyBase(o.StateRepo) == base {
				shared = true
			}
		}
		if !shared {
			candidates[config.TeamGitTokenKey(team.StateRepo)] = true
			candidates[config.TeamGitUserKey(team.StateRepo)] = true
		}
	}
	var keys []string
	for k := range candidates {
		if v, err := a.Secrets.Get(ctx, k); err == nil && v != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// Lines is the recap of what leaving does, for the confirmation of the CLI and
// of the TUI.
func (p *leavePlan) Lines(opts leaveOptions) []string {
	var out []string
	switch {
	case p.Solo:
	case !p.Cloned:
		out = append(out, i18n.T("v54l.leave.plan.no_clone"))
	case p.Member == "":
		out = append(out, i18n.T("v54l.leave.plan.no_member"))
	default:
		if len(p.OpenClaims) > 0 {
			out = append(out, i18n.Tf("v54l.leave.plan.claims", len(p.OpenClaims), p.Member))
		} else {
			out = append(out, i18n.T("v54l.leave.plan.no_claims"))
		}
		switch {
		case opts.RemoveMember && p.CardExists:
			out = append(out, i18n.Tf("v54l.leave.plan.card_removed", p.Member))
		case opts.RemoveMember:
			out = append(out, i18n.Tf("v54l.leave.plan.card_absent", p.Member))
		default:
			out = append(out, i18n.Tf("v54l.leave.plan.card_kept", p.Member, "--remove-member"))
		}
	}
	if p.QueueOps > 0 {
		out = append(out, i18n.Tf("v54l.leave.plan.queue", p.QueueOps))
	}
	if p.DeleteClone {
		out = append(out, i18n.Tf("v54l.leave.plan.clone", p.ClonePath))
	} else if p.KeepWhy != "" {
		out = append(out, i18n.Tf("v54l.leave.plan.clone_kept", p.KeepWhy))
	}
	if !p.Pending.None() {
		out = append(out, i18n.Tf("v54l.leave.plan.pending", p.Pending.Unpushed, len(p.Pending.Uncommitted)))
	}
	if len(p.Keys) > 0 {
		out = append(out, i18n.Tf("v54l.leave.plan.keys", len(p.Keys)))
	}
	if len(p.Projects) > 0 {
		out = append(out, i18n.Tf("v54l.leave.plan.projects", len(p.Projects), strings.Join(p.Projects, ", ")))
	}
	return append(out, i18n.T("v54l.leave.plan.hub"))
}

// leaveTeam leaves team id. A failure of the team-state (it cannot be reached,
// the access is refused) stops before anything local is deleted, unless
// opts.Force goes on. The report says what was done even when an error comes
// back.
func leaveTeam(ctx context.Context, env leaveEnv, id string, opts leaveOptions) (*leaveReport, error) {
	a := env.a
	plan, err := planLeave(ctx, env, id)
	if err != nil {
		return nil, err
	}
	rep := &leaveReport{}

	if plan.Cloned && plan.Member != "" {
		if err := leaveTeamState(ctx, plan, opts, rep); err != nil {
			return rep, err
		}
	}
	if plan.Cloned && plan.QueueOps > 0 {
		flushLeaveQueue(ctx, plan, rep)
	}

	if plan.DeleteClone {
		if pending, err := plan.repo.PendingWork(ctx); err == nil && !pending.None() && !opts.Force {
			return rep, errors.New(i18n.Tf("v54l.leave.pending", plan.ClonePath, pending.Unpushed, len(pending.Uncommitted), "--force"))
		}
		if err := fsutil.RemoveAllWritable(plan.ClonePath); err != nil {
			return rep, fmt.Errorf("%s: %w", i18n.Tf("v54l.leave.clone_failed", plan.ClonePath), err)
		}
		rep.CloneRemoved = plan.ClonePath
	} else {
		rep.CloneKept = plan.KeepWhy
	}

	for _, k := range plan.Keys {
		if err := a.Secrets.Delete(ctx, k); err != nil {
			rep.Warnings = append(rep.Warnings, i18n.Tf("v54l.leave.key_failed", k, err))
			continue
		}
		rep.KeysRemoved = append(rep.KeysRemoved, k)
	}

	rep.Detached, rep.Note, err = removeTeamVia(ctx, a, id, env.update)
	return rep, err
}

// leaveTeamState frees the claims of the member in the team-state (and removes
// the card on request), in one transaction.
func leaveTeamState(ctx context.Context, plan *leavePlan, opts leaveOptions, rep *leaveReport) error {
	removeCard := opts.RemoveMember && plan.CardExists
	if len(plan.OpenClaims) == 0 && !removeCard {
		return nil
	}
	res, err := plan.repo.ReassignClaims(teamstate.WithActor(ctx, plan.Member), teamstate.ReassignOpts{
		From: plan.Member, Release: true, ReleaseReason: teamstate.ReleaseLeftTeam, RemoveMember: removeCard,
	})
	if err != nil {
		if !opts.Force {
			return errors.New(i18n.Tf("v54l.leave.remote_failed", err, "--force"))
		}
		rep.Warnings = append(rep.Warnings, i18n.Tf("v54l.leave.remote_skipped", err))
		return nil
	}
	rep.ClaimsReleased = len(res.Released)
	rep.CardRemoved = res.MemberRemoved
	return nil
}

// flushLeaveQueue applies what the queue of claim operations holds while the
// clone still exists (the operations on the claims just released are dropped)
// and reports what is left, which goes with the clone.
func flushLeaveQueue(ctx context.Context, plan *leavePlan, rep *leaveReport) {
	flushCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if _, _, err := plan.repo.FlushClaimQueue(flushCtx); err != nil {
		rep.Warnings = append(rep.Warnings, i18n.Tf("v54l.leave.queue_flush_failed", err))
	}
	if left, err := plan.repo.ClaimQueue(); err == nil {
		rep.QueueLeft = left
	}
}

// Summary is what leaving did, one line per step, for the CLI and for the
// toast of the TUI.
func (rep *leaveReport) Summary(id string) []string {
	var out []string
	if rep.ClaimsReleased > 0 {
		out = append(out, i18n.Tf("v54l.leave.done.claims", rep.ClaimsReleased))
	}
	if rep.CardRemoved {
		out = append(out, i18n.T("v54l.leave.done.card"))
	}
	if len(rep.QueueLeft) > 0 {
		refs := make([]string, len(rep.QueueLeft))
		for i, op := range rep.QueueLeft {
			refs[i] = fmt.Sprintf("%s %s/%s", op.Kind, op.Project, op.TicketID)
		}
		slices.Sort(refs)
		out = append(out, i18n.Tf("v54l.leave.done.queue_left", len(rep.QueueLeft), strings.Join(refs, ", ")))
	}
	if rep.CloneRemoved != "" {
		out = append(out, i18n.Tf("v54l.leave.done.clone", rep.CloneRemoved))
	} else if rep.CloneKept != "" {
		out = append(out, i18n.Tf("v54l.leave.done.clone_kept", rep.CloneKept))
	}
	if len(rep.KeysRemoved) > 0 {
		out = append(out, i18n.Tf("v54l.leave.done.keys", len(rep.KeysRemoved)))
	}
	if len(rep.Detached) > 0 {
		out = append(out, i18n.Tf("v54l.leave.done.detached", len(rep.Detached), strings.Join(rep.Detached, ", ")))
	}
	out = append(out, rep.Warnings...)
	if rep.Note != "" {
		out = append(out, rep.Note)
	}
	return out
}

func runTeamsLeave(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	id := args[0]
	opts := leaveOptions{}
	opts.RemoveMember, _ = cmd.Flags().GetBool("remove-member")
	opts.Force, _ = cmd.Flags().GetBool("force")

	env := cliLeaveEnv(a)
	plan, err := planLeave(ctx, env, id)
	if err != nil {
		var unknown *config.UnknownTeamError
		if errors.As(err, &unknown) {
			return errors.New(i18n.Tf("cmd.teams.not_found", id, theme.Bold.Render("oh teams list")))
		}
		return err
	}
	fmt.Fprintln(a.IO.Out, i18n.Tf("v54l.leave.plan.title", theme.Bold.Render(plan.Name)))
	for _, line := range plan.Lines(opts) {
		fmt.Fprintf(a.IO.Out, "  - %s\n", line)
	}
	fmt.Fprintln(a.IO.Out)

	ok, err := confirmAction(cmd, i18n.Tf("v54l.leave.confirm", plan.Name), false)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	rep, err := leaveTeam(ctx, env, id, opts)
	if rep != nil {
		for _, line := range rep.Summary(id) {
			fmt.Fprintln(a.IO.Out, line)
		}
	}
	if err != nil {
		return err
	}
	cliout.Success(a.IO.Out, i18n.Tf("v54l.leave.done", theme.Bold.Render(id)))
	return nil
}
