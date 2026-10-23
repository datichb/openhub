package team

import (
	"context"
	"fmt"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// InitStep names the step of Init that failed.
type InitStep string

// The steps of Init.
const (
	StepClone        InitStep = "clone"
	StepStructure    InitStep = "structure"
	StepUpdateMember InitStep = "update_member"
	StepAddMember    InitStep = "add_member"
	StepWriteConfig  InitStep = "write_config"
)

// InitError is a failure of Init, with the step that failed.
type InitError struct {
	Step InitStep
	Err  error
}

func (e *InitError) Error() string { return fmt.Sprintf("team init (%s): %v", e.Step, e.Err) }
func (e *InitError) Unwrap() error { return e.Err }

// InitParams are the inputs of Init (they come from the `oh init` wizard).
type InitParams struct {
	StateRepo          string
	StatePath          string // empty = default path
	MemberID           string
	DisplayName        string
	GitLabUsername     string
	MattermostUsername string
	// TrackerUsername is the name of the member on the tracker (empty = keep
	// the one of the member, if any).
	TrackerUsername string
	Role            string
	// GitLabURL is the GitLab that hosts StateRepo when the remote goes
	// through an SSH alias (empty = the host of the remote, or the one already
	// declared for the same repository).
	GitLabURL string
}

// Init joins a team: clones (or pulls) the team-state, creates its structure
// (idempotent), adds the member or updates it, then writes the team in
// hub.toml (added, or replacing the entry of the same team; the other teams,
// solo spaces included, are left as they are). It displays nothing; failures
// are *InitError.
func (s *Service) Init(ctx context.Context, p InitParams) error {
	statePath := p.StatePath
	if statePath == "" {
		statePath = config.DefaultTeamStatePath()
	}
	repo := teamstate.NewRepo(p.StateRepo, statePath)

	if err := repo.EnsureReady(ctx); err != nil {
		return &InitError{Step: StepClone, Err: err}
	}
	if err := repo.EnsureStructure(ctx); err != nil {
		return &InitError{Step: StepStructure, Err: err}
	}

	member := teamstate.Member{
		ID:                 p.MemberID,
		DisplayName:        p.DisplayName,
		GitLabUsername:     p.GitLabUsername,
		MattermostUsername: p.MattermostUsername,
		TrackerUsername:    CleanUsername(p.TrackerUsername),
		Role:               p.Role,
		DefaultMode:        "semi-auto",
	}
	if repo.HasMember(p.MemberID) {
		if prev, err := repo.GetMember(p.MemberID); err == nil && member.TrackerUsername == "" {
			member.TrackerUsername = prev.TrackerUsername
		}
		if err := repo.UpdateMember(ctx, member); err != nil {
			return &InitError{Step: StepUpdateMember, Err: err}
		}
	} else if err := repo.AddMember(ctx, member); err != nil {
		return &InitError{Step: StepAddMember, Err: err}
	}

	if err := s.SaveTeam(p.StateRepo, statePath, p.MemberID, p.GitLabURL); err != nil {
		return &InitError{Step: StepWriteConfig, Err: err}
	}
	return nil
}

// SaveTeam writes the team of stateRepo in hub.toml (enabled, its ID is the
// name of the repository) and clears the legacy [team] section. gitlabURL is
// the GitLab of the repository when its remote is an SSH alias; empty keeps
// the one already declared for the same repository.
func (s *Service) SaveTeam(stateRepo, statePath, memberID, gitlabURL string) error {
	team := config.TeamConfig{
		ID:        config.RepoNameFromRemote(stateRepo),
		Enabled:   true,
		StateRepo: stateRepo,
		StatePath: statePath,
		MemberID:  memberID,
		GitLabURL: gitlabURL,
	}
	fresh, err := config.UpdateAndGet(func(c *config.Config) error {
		c.UpsertTeam(team)
		c.Team = config.TeamConfig{}
		return nil
	})
	if err != nil {
		return err
	}
	*s.Config = *fresh
	return nil
}
