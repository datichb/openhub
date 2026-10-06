package cmd

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// `oh team init --solo`: a local team-state without remote for projects
// without a team (v5 phase 2, P2-T09). The flags are added to the existing
// `team init` command; without --solo the interactive wizard runs as before.
func init() {
	f := teamInitCmd.Flags()
	f.Bool("solo", false, i18n.T("cmd.team.solo.flags.solo"))
	f.String("id", "", i18n.T("cmd.team.solo.flags.id"))
	f.String("name", "", i18n.T("cmd.team.solo.flags.name"))
	f.String("member-id", "", i18n.T("cmd.team.solo.flags.member_id"))
	f.String("project", "", i18n.T("cmd.team.solo.flags.project"))

	wizard := teamInitCmd.RunE
	teamInitCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if solo, _ := cmd.Flags().GetBool("solo"); !solo {
			return wizard(cmd, args)
		}
		return runTeamInitSolo(cmd)
	}
}

func runTeamInitSolo(cmd *cobra.Command) error {
	a := MustApp()
	if _, err := os.Stat(config.ConfigPath()); os.IsNotExist(err) {
		return fmt.Errorf("%s", i18n.Tf("cmd.team.init.hub_not_configured", theme.Bold.Render("oh init")))
	}
	id, _ := cmd.Flags().GetString("id")
	name, _ := cmd.Flags().GetString("name")
	memberID, _ := cmd.Flags().GetString("member-id")
	projectRef, _ := cmd.Flags().GetString("project")

	res, err := initSoloTeam(cmd.Context(), a, soloTeamParams{ID: id, Name: name, MemberID: memberID, ProjectRef: projectRef})
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.team.solo.created", res.Team.ID, res.Team.StatePath))
	fmt.Fprintf(out, "  %s\n", i18n.Tf("cmd.team.solo.member", res.Team.MemberID))
	if res.Project != nil {
		fmt.Fprintf(out, "  %s\n", i18n.Tf("cmd.team.solo.attached", res.Project.Name))
	}
	fmt.Fprintf(out, "  %s\n", theme.Subtitle.Render(i18n.Tf("cmd.team.solo.next", "oh team promote --remote <url>")))
	return nil
}

// soloTeamParams are the inputs of initSoloTeam.
type soloTeamParams struct {
	ID         string // default "solo"
	Name       string
	MemberID   string // default: member id of an existing team, else the OS user
	ProjectRef string // project to attach (id or name), optional
}

type soloTeamResult struct {
	Team    config.TeamConfig
	Repo    *teamstate.Repo
	Project *domain.Project
}

// soloTeamDir is where solo team-states live: ~/.oh/teams/<id>/.
func soloTeamDir(id string) string {
	return filepath.Join(config.HubDir(), "teams", id)
}

// initSoloTeam creates the solo team-state, registers it in hub.toml
// (`solo = true`, appended: existing teams are kept) and attaches the
// project when asked.
func initSoloTeam(ctx context.Context, a *app.App, p soloTeamParams) (*soloTeamResult, error) {
	if p.ID == "" {
		p.ID = "solo"
	}
	team := config.TeamConfig{ID: p.ID, Name: p.Name, Enabled: true, Solo: true, StatePath: soloTeamDir(p.ID), MemberID: p.MemberID}
	if team.MemberID == "" {
		team.MemberID = defaultSoloMemberID(a.Config)
	}
	if err := team.Validate(); err != nil {
		return nil, err
	}
	if a.Config.FindTeam(team.ID) != nil {
		return nil, fmt.Errorf("%s", i18n.Tf("cmd.team.solo.exists", team.ID))
	}

	var project *domain.Project
	if p.ProjectRef != "" {
		var err error
		if project, err = resolveProject(ctx, a, p.ProjectRef); err != nil {
			return nil, err
		}
		if project.TeamID != nil && *project.TeamID != "" {
			return nil, fmt.Errorf("%s", i18n.Tf("cmd.team.solo.project_has_team", project.Name, *project.TeamID))
		}
	}

	repo, err := teamstate.InitSolo(ctx, team.StatePath, teamstate.Member{ID: team.MemberID, DisplayName: team.MemberID})
	if err != nil {
		return nil, err
	}
	a.Config.Teams = append(a.Config.Teams, team)
	if err := config.Save(a.Config); err != nil {
		return nil, fmt.Errorf("writing hub.toml: %w", err)
	}
	if project != nil {
		if err := attachProjectToSolo(ctx, a, repo, project, team.ID); err != nil {
			return nil, err
		}
	}
	return &soloTeamResult{Team: team, Repo: repo, Project: project}, nil
}

// soloSpace is the solo space a project was attached to.
type soloSpace struct {
	Team    config.TeamConfig
	Repo    *teamstate.Repo
	Created bool
}

// attachProjectSolo attaches project to a solo space: the existing one (the
// first cloned), else a new one (solo, solo-2…). Used by the migration and
// the « solo space » choice of the project wizards (P2-T16).
func attachProjectSolo(ctx context.Context, a *app.App, project *domain.Project) (*soloSpace, error) {
	for _, t := range a.Config.Teams {
		if !t.Solo || !t.Enabled {
			continue
		}
		repo := teamstate.NewRepo("", t.StatePath)
		if !repo.IsCloned() {
			continue
		}
		if err := attachProjectToSolo(ctx, a, repo, project, t.ID); err != nil {
			return nil, err
		}
		return &soloSpace{Team: t, Repo: repo}, nil
	}
	res, err := initSoloTeam(ctx, a, soloTeamParams{ID: nextSoloID(a.Config), ProjectRef: project.ID})
	if err != nil {
		return nil, err
	}
	return &soloSpace{Team: res.Team, Repo: res.Repo, Created: true}, nil
}

// nextSoloID is the first free solo space id (solo, solo-2…).
func nextSoloID(cfg *config.Config) string {
	id := "solo"
	for i := 2; cfg.FindTeam(id) != nil; i++ {
		id = fmt.Sprintf("solo-%d", i)
	}
	return id
}

// attachProjectToSolo sets the project's team and creates its workflow
// folders in the team-state.
func attachProjectToSolo(ctx context.Context, a *app.App, repo *teamstate.Repo, project *domain.Project, teamID string) error {
	if err := repo.EnsureWorkflowLayout(teamstate.ProjectScope(project.ID)); err != nil {
		return err
	}
	if err := repo.CommitAndPush(ctx, "team: add project "+project.ID, "."); err != nil {
		return err
	}
	tid := teamID
	project.TeamID = &tid
	return a.Projects.Update(ctx, project)
}

var reMemberID = regexp.MustCompile(`[^a-z0-9._-]+`)

// defaultSoloMemberID reuses the member id of another team, else derives one
// from the OS user name.
func defaultSoloMemberID(cfg *config.Config) string {
	for _, t := range cfg.Teams {
		if t.MemberID != "" {
			return t.MemberID
		}
	}
	name := os.Getenv("USER")
	if u, err := user.Current(); err == nil && u.Username != "" {
		name = u.Username
	}
	name = strings.Trim(reMemberID.ReplaceAllString(strings.ToLower(name), "-"), "-.")
	if name == "" {
		return "me"
	}
	return name
}
