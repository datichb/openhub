package team

import (
	"context"
	"errors"
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
	// StepJoinMember: the card of the member was refused (ID or GitLab handle
	// taken by a colleague); the error is a *teamstate.MemberIDTakenError or
	// a *teamstate.HandleTakenError.
	StepJoinMember  InitStep = "join_member"
	StepWriteConfig InitStep = "write_config"
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
	// Claim is « C'est moi »: the ID is the one of a card that exists in the
	// team-state and the person says it is theirs. The card is attached as it
	// is (its empty fields are filled, nothing else changes). Without it a
	// taken ID is refused (*teamstate.MemberIDTakenError), whoever types it,
	// except for the member_id hub.toml already has for this team (a
	// reinstallation).
	Claim bool
	// GitLabURL is the GitLab that hosts StateRepo when the remote goes
	// through an SSH alias (empty = the host of the remote, or the one already
	// declared for the same repository).
	GitLabURL string
}

// InitResult is what Init reports besides success.
type InitResult struct {
	// StructureWarning is a *teamstate.StructureWarning when the folders of
	// the team-state exist but their commit ("team: init structure") failed
	// (offline, remote refusal): the join went on, the next one commits them.
	StructureWarning error
	// Join tells what happened to the card of the member: created, or an
	// existing one attached (and the fields it filled).
	Join teamstate.JoinResult
}

// Init joins a team: clones (or pulls) the team-state, creates its structure
// (idempotent), registers the card of the member (JoinMember: a colleague's card is never
// replaced), then writes the team in
// hub.toml (added, or replacing the entry of the same team; the other teams,
// solo spaces included, are left as they are). It displays nothing; failures
// are *InitError. What does not fail the join but deserves a word is in the
// result.
func (s *Service) Init(ctx context.Context, p InitParams) (InitResult, error) {
	var res InitResult
	statePath := p.StatePath
	if statePath == "" {
		statePath = s.StatePathFor(p.StateRepo)
	}
	repo := teamstate.NewRepo(p.StateRepo, statePath)

	if err := repo.EnsureReady(ctx); err != nil {
		return res, &InitError{Step: StepClone, Err: err}
	}
	if err := repo.EnsureStructure(ctx); err != nil {
		if !teamstate.IsStructureWarning(err) {
			return res, &InitError{Step: StepStructure, Err: err}
		}
		res.StructureWarning = err // the folders exist; the commit is retried at the next join
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
	join, err := repo.JoinMember(ctx, member, s.joinMode(p))
	if err != nil {
		step := StepAddMember
		if errors.Is(err, teamstate.ErrMemberExists) || errors.Is(err, teamstate.ErrHandleTaken) {
			step = StepJoinMember
		}
		return res, &InitError{Step: step, Err: err}
	}
	res.Join = join

	if err := s.SaveTeam(p.StateRepo, statePath, p.MemberID, p.GitLabURL); err != nil {
		return res, &InitError{Step: StepWriteConfig, Err: err}
	}
	return res, nil
}

// joinMode is what joining means for the card the member names (see
// teamstate.JoinModeFor).
func (s *Service) joinMode(p InitParams) teamstate.JoinMode {
	return teamstate.JoinModeFor(s.MemberIDOf(p.StateRepo), p.MemberID, p.Claim)
}

// MemberIDOf is the member_id hub.toml has for the team whose team-state is
// stateRepo ("" when the team is not configured).
func (s *Service) MemberIDOf(stateRepo string) string {
	if s.Config == nil {
		return ""
	}
	teams := append(append([]config.TeamConfig{}, s.Config.Teams...), s.Config.Team)
	for _, t := range teams {
		if t.MemberID != "" && teamstate.SameRemote(t.StateRepo, stateRepo) {
			return t.MemberID
		}
	}
	return ""
}

// StatePathFor is where the clone of the team-state stateRepo lives: the one
// path of every team assistant. A team of hub.toml that already has this
// repository keeps its folder (a team configured before is neither cloned a
// second time nor moved); otherwise config.TeamStatePath(stateRepo),
// ~/.oh/team-states/<host>/<path of the repository> (an existing clone of this
// repository at a former location is kept). A folder that holds the clone of
// another repository is refused by teamstate.Repo.EnsureReady, never reused.
func (s *Service) StatePathFor(stateRepo string) string {
	if s.Config != nil {
		teams := append(append([]config.TeamConfig{}, s.Config.Teams...), s.Config.Team)
		for _, t := range teams {
			if teamstate.SameRemote(t.StateRepo, stateRepo) {
				return config.ResolveStatePath(t.StateRepo, t.StatePath)
			}
		}
	}
	return config.TeamStatePath(stateRepo)
}

// SaveTeam writes the team of stateRepo in hub.toml (enabled) and clears the
// legacy [team] section. The team already configured for this repository keeps
// its ID; a new one gets the name of the repository, or another ID when the
// team of a different repository already has it (config.Config.TeamIDFor). gitlabURL is
// the GitLab of the repository when its remote is an SSH alias; empty keeps
// the one already declared for the same repository.
func (s *Service) SaveTeam(stateRepo, statePath, memberID, gitlabURL string) error {
	team := config.TeamConfig{
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
