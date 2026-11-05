package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Doctor › the configuration of the teams. Two checks:
//
//   - team_config: the config.toml of the clone of each shared team, read as it
//     is written: unknown and former keys, numbers outside their bounds, a
//     board that is not valid, notifications that cannot work, and — with a
//     webhook — a team-state repository that anybody can read (the webhook is in
//     it). oh doctor --fix repairs the former keys and the numbers
//     (teamstate.Repo.RepairConfig), after a confirmation.
//   - hub_teams: the [[teams]] of hub.toml (config.ValidateTeams, which only
//     ran when hub.toml was written): two teams marked default are repaired by
//     --fix (the first one stays).

// Identifiers of the checks.
const (
	doctorIDTeamConfig = "team_config"
	doctorIDHubTeams   = "hub_teams"
)

// teamPublicProbe tells whether a repository can be read by anybody (a
// variable for the tests).
var teamPublicProbe = teamstate.ProbePublic

// teamConfigStore is what the check and its repair ask of a team clone.
type teamConfigStore interface {
	teamstate.RepoInfo
	ConfigIssues(ctx context.Context) ([]teamstate.ConfigIssue, error)
	LoadConfig(ctx context.Context) (*teamstate.TeamConfig, error)
	RepairConfig(ctx context.Context) ([]string, error)
}

// teamConfigTarget is a shared team with a clone, as the checks look at it.
type teamConfigTarget struct {
	team config.TeamConfig
	repo teamConfigStore
}

func teamConfigTargets(cfg *config.Config) []teamConfigTarget {
	var out []teamConfigTarget
	for _, t := range sharedTeams(cfg) {
		repo := teamstate.NewRepo(t.StateRepo, t.StatePath)
		if repo.IsCloned() {
			out = append(out, teamConfigTarget{team: t, repo: repo})
		}
	}
	return out
}

func teamConfigDoctorChecks(ctx context.Context) []doctor.Result {
	a := TryApp()
	if a == nil || a.Config == nil {
		return nil
	}
	var out []doctor.Result
	for _, t := range teamConfigTargets(a.Config) {
		out = append(out, teamConfigResults(ctx, t)...)
	}
	return out
}

func teamConfigResults(ctx context.Context, t teamConfigTarget) []doctor.Result {
	id := firstNonEmpty(t.team.ID, "hub")
	name := i18n.Tf("v54c.doctor.team_config.name", id)
	issues, err := t.repo.ConfigIssues(ctx)
	if err != nil {
		return []doctor.Result{{Name: name, OK: true, Warn: true, Detail: i18n.Tf("v54c.doctor.team_config.unreadable", err)}}
	}
	var out []doctor.Result
	res := doctor.Result{Name: name, OK: true, Detail: i18n.T("v54c.doctor.team_config.ok")}
	if len(issues) > 0 {
		lines := make([]string, 0, len(issues))
		for _, i := range issues {
			lines = append(lines, "  - "+i.Detail)
			if i.Kind == teamstate.IssueUnreadable || i.Kind == teamstate.IssueBoard {
				res.OK = false
			}
		}
		res.Warn = res.OK
		res.Detail = i18n.Tf("v54c.doctor.team_config.found", len(issues), t.repo.Path()) + "\n" + strings.Join(lines, "\n")
		if teamstate.FixableIssues(issues) != nil {
			res.Detail += "\n" + i18n.T("v54c.doctor.team_config.fix_hint")
			res.Fix = teamConfigFix(t, issues)
		}
	}
	out = append(out, res)
	if pub := publicRepoResult(ctx, t); pub != nil {
		out = append(out, *pub)
	}
	return out
}

// publicRepoResult warns when the team keeps a webhook in a repository that
// anybody can read; nil when it has none, or when nothing could be said
// (offline, no answer).
func publicRepoResult(ctx context.Context, t teamConfigTarget) *doctor.Result {
	if doctorOffline() {
		return nil
	}
	cfg, err := t.repo.LoadConfig(ctx)
	if err != nil || cfg == nil || (cfg.Notification.EffectiveURL() == "" && len(cfg.Notification.Destinations) == 0) {
		return nil
	}
	cctx, cancel := context.WithTimeout(ctx, doctorConnectionTimeout)
	defer cancel()
	public, perr := teamPublicProbe(cctx, t.team.StateRepo)
	if perr != nil || !public {
		return nil
	}
	return &doctor.Result{
		ID: doctorIDTeamConfig, Name: i18n.Tf("v54c.doctor.team_public.name", firstNonEmpty(t.team.ID, "hub")),
		OK: true, Warn: true, Detail: i18n.Tf("v54c.doctor.team_public.detail", maskRepoURL(t.team.StateRepo)),
	}
}

// teamConfigFix repairs the former keys and the numbers of one team.
func teamConfigFix(t teamConfigTarget, issues []teamstate.ConfigIssue) *doctor.Fix {
	return &doctor.Fix{
		Plan: func(context.Context) (string, error) { return teamConfigPlan(t, issues), nil },
		Apply: doctorFixRun(func(ctx context.Context, w io.Writer) error {
			return repairTeamConfig(ctx, w, t, alwaysConfirm)
		}),
	}
}

func teamConfigPlan(t teamConfigTarget, issues []teamstate.ConfigIssue) string {
	var lines []string
	for _, i := range teamstate.FixableIssues(issues) {
		lines = append(lines, "  - "+i.Detail)
	}
	return i18n.Tf("v54c.doctor.team_config.fix_plan", t.repo.Path()) + "\n" + strings.Join(lines, "\n")
}

// fixTeamConfig is oh doctor --fix for the config.toml of the teams.
func fixTeamConfig(cmd *cobra.Command) error {
	a := MustApp()
	for _, t := range teamConfigTargets(a.Config) {
		if err := repairTeamConfig(cmd.Context(), cmd.OutOrStdout(), t, cmdConfirm(cmd)); err != nil {
			return err
		}
	}
	return nil
}

func repairTeamConfig(ctx context.Context, w io.Writer, t teamConfigTarget, confirm confirmFunc) error {
	issues, err := t.repo.ConfigIssues(ctx)
	if err != nil || len(teamstate.FixableIssues(issues)) == 0 {
		return nil //nolint:nilerr // an unreadable config is the check's to report, not the repair's
	}
	fmt.Fprintln(w, teamConfigPlan(t, issues))
	ok, err := confirm(i18n.T("v54c.doctor.team_config.fix_confirm"))
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(w, i18n.T("cmd.doctor.fix.cancelled"))
		return nil
	}
	// the repair is a publication: it is signed by the member of the team
	repaired, err := t.repo.RepairConfig(teamstate.WithActor(ctx, t.team.MemberID))
	if err != nil {
		if nl, isNotLead := teamstate.IsNotLead(err); isNotLead {
			return nl
		}
		return fmt.Errorf("%s", i18n.Tf("v54c.doctor.team_config.fix_failed", t.repo.Path(), err))
	}
	cliout.Success(w, i18n.Tf("v54c.doctor.team_config.fix_done", len(repaired), t.repo.Path()))
	return nil
}

// ─── hub.toml: the [[teams]] ────────────────────────────────────────────────

func hubTeamsDoctorChecks(context.Context) []doctor.Result {
	a := TryApp()
	if a == nil || a.Config == nil {
		return nil
	}
	name := i18n.T("v54c.doctor.hub_teams.name")
	if marks := rawDefaultTeams(); len(marks) > 1 {
		return []doctor.Result{{
			Name: name, OK: true, Warn: true,
			Detail: i18n.Tf("v54c.doctor.hub_teams.two_defaults", strings.Join(marks, ", "), marks[0]),
			Fix:    hubTeamsFix(),
		}}
	}
	if err := config.ValidateTeams(a.Config.Teams); err != nil {
		return []doctor.Result{{Name: name, Detail: i18n.Tf("v54c.doctor.hub_teams.invalid", err)}}
	}
	return nil
}

// rawDefaultTeams are the teams marked default in hub.toml as it is written
// (the loaded Config keeps the first one only).
func rawDefaultTeams() []string {
	data, err := os.ReadFile(config.ConfigPath())
	if err != nil {
		return nil
	}
	var raw struct {
		Teams []config.TeamConfig `toml:"teams"`
	}
	if toml.Unmarshal(data, &raw) != nil {
		return nil
	}
	return config.DefaultTeamMarks(raw.Teams)
}

func hubTeamsFix() *doctor.Fix {
	return &doctor.Fix{
		Plan: func(context.Context) (string, error) {
			marks := rawDefaultTeams()
			if len(marks) < 2 {
				return i18n.T("cmd.doctor.fix.nothing"), nil
			}
			return i18n.Tf("v54c.doctor.hub_teams.fix_plan", marks[0], strings.Join(marks[1:], ", ")), nil
		},
		Apply: doctorFixRun(func(_ context.Context, w io.Writer) error { return repairHubTeams(MustApp(), w) }),
	}
}

// repairHubTeams writes hub.toml with a single default team (the first).
func repairHubTeams(a *app.App, w io.Writer) error {
	if len(rawDefaultTeams()) < 2 {
		return nil
	}
	if err := updateHubConfig(a, func(c *config.Config) error {
		c.KeepFirstDefaultTeam()
		return nil
	}); err != nil {
		return err
	}
	cliout.Success(w, i18n.T("v54c.doctor.hub_teams.fix_done"))
	return nil
}

// fixHubTeams is oh doctor --fix for the [[teams]] of hub.toml.
func fixHubTeams(cmd *cobra.Command) error {
	if len(rawDefaultTeams()) < 2 {
		return nil
	}
	ok, err := cmdConfirm(cmd)(i18n.Tf("v54c.doctor.hub_teams.fix_confirm", rawDefaultTeams()[0]))
	if err != nil || !ok {
		return err
	}
	return repairHubTeams(MustApp(), cmd.OutOrStdout())
}
