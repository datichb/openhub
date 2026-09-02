package cmd

import (
	"context"
	"fmt"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// teamInitParams holds the parameters for the core team initialization logic.
// This struct is shared by runTeamInit (wizard), runTeamInitFromTUI, and runTeamCustomSetup.
type teamInitParams struct {
	StateRepo         string
	StatePath         string // empty = use default
	MemberID          string
	DisplayName       string
	GitLabUsername    string
	MattermostUsername string
	Role              string
}

// teamInitCore performs the core git+member+config operations shared by:
//   - runTeamInit (CLI wizard — post-wizard finalization)
//   - runTeamInitFromTUI (TUI modal)
//   - runTeamCustomSetup (programmatic project setup)
//
// It does NOT display any UI — callers handle presentation.
// Operations: clone/pull repo → init structure → add/update member → write hub.toml.
func teamInitCore(ctx context.Context, a *app.App, p teamInitParams) error {
	statePath := p.StatePath
	if statePath == "" {
		statePath = config.DefaultTeamStatePath()
	}

	repo := teamstate.NewRepo(p.StateRepo, statePath)

	// Ensure repo is ready (clone if needed, pull if already cloned)
	if err := repo.EnsureReady(ctx); err != nil {
		return fmt.Errorf("cloning team-state: %w", err)
	}

	// Initialize directory structure (idempotent)
	if err := repo.InitStructure(ctx); err != nil {
		return fmt.Errorf("init structure: %w", err)
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
			return fmt.Errorf("update member: %w", err)
		}
	} else {
		if err := repo.AddMember(ctx, member); err != nil {
			return fmt.Errorf("add member: %w", err)
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
	if len(a.Config.Teams) > 0 {
		// Update first team entry (primary team)
		a.Config.Teams[0] = newTeam
	} else {
		a.Config.Teams = append(a.Config.Teams, newTeam)
	}
	// Clear legacy field
	a.Config.Team = config.TeamConfig{}

	if err := config.Save(a.Config); err != nil {
		return fmt.Errorf("writing hub.toml: %w", err)
	}

	return nil
}
