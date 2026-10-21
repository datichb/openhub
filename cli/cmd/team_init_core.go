package cmd

import (
	"context"
	"fmt"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// teamInitParams holds the parameters for the core team initialization logic
// (see teamInitCore).
type teamInitParams struct {
	StateRepo          string
	StatePath          string // empty = use default
	MemberID           string
	DisplayName        string
	GitLabUsername     string
	MattermostUsername string
	Role               string
}

// teamInitCore performs the git+member+config operations of the team step of
// the `oh init` wizard (teamInitParams come from its state). It does NOT
// display any UI — the caller handles presentation.
// Operations: clone/pull repo → init structure → add/update member → write
// hub.toml (the team is added, or replaces the entry of the same team; the
// other teams, solo spaces included, are left as they are).
func teamInitCore(ctx context.Context, a *app.App, p teamInitParams) error {
	statePath := p.StatePath
	if statePath == "" {
		statePath = config.DefaultTeamStatePath()
	}

	repo := teamstate.NewRepo(p.StateRepo, statePath)

	// Ensure repo is ready (clone if needed, pull if already cloned)
	if err := repo.EnsureReady(ctx); err != nil {
		return fmt.Errorf(i18n.T("cmd.team_init_core.cloning_team_state"), err)
	}

	// Initialize directory structure (idempotent)
	if err := repo.InitStructure(ctx); err != nil {
		return fmt.Errorf(i18n.T("cmd.team_init_core.init_structure"), err)
	}

	// Register or update member
	member := teamstate.Member{
		ID:                 p.MemberID,
		DisplayName:        p.DisplayName,
		GitLabUsername:     p.GitLabUsername,
		MattermostUsername: p.MattermostUsername,
		Role:               p.Role,
		DefaultMode:        "semi-auto",
	}
	if repo.HasMember(p.MemberID) {
		if err := repo.UpdateMember(ctx, member); err != nil {
			return fmt.Errorf(i18n.T("cmd.team_init_core.update_member"), err)
		}
	} else {
		if err := repo.AddMember(ctx, member); err != nil {
			return fmt.Errorf(i18n.T("cmd.team_init_core.add_member"), err)
		}
	}

	// Persist team config to hub.toml
	newTeam := config.TeamConfig{
		ID:        config.RepoNameFromRemote(p.StateRepo),
		Enabled:   true,
		StateRepo: p.StateRepo,
		StatePath: statePath,
		MemberID:  p.MemberID,
	}
	a.Config.UpsertTeam(newTeam)
	// Clear legacy field
	a.Config.Team = config.TeamConfig{}

	if err := config.Save(a.Config); err != nil {
		return fmt.Errorf(i18n.T("cmd.team_init_core.writing_hub_toml"), err)
	}

	return nil
}
