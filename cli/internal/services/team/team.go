// Package team is the team service: the rules that the command line, the
// terminal UI and the automatic tracker engine all follow, written once.
//
//   - ResolveTeam: which team a project belongs to (a project without team
//     falls back on the hub team);
//   - GitLabToken: which GitLab token is used (one cascade for tracker sync,
//     merge requests, identity check and MCP);
//   - TrackerEngine and SyncTracker: the tracker engine of the team, with
//     typed errors that each caller translates (ErrTeamNotConfigured,
//     ErrNotCloned, ErrTrackerNotConfigured, ErrTrackerDisabled,
//     ErrNoProjects, *CredentialsError);
//   - Init: join a team (clone the team-state, register the member, write
//     hub.toml).
//
// The package builds no message for the user: errors are typed and in
// English, the callers translate them. It knows neither cmd nor the screens.
package team

import (
	"context"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// ProjectLister is what the service needs from the project store.
// domain.ProjectStore implements it.
type ProjectLister interface {
	List(ctx context.Context, status domain.ProjectStatus) ([]domain.Project, error)
}

// Service holds what the team rules read: the hub configuration, the
// keychain and the project store. Build one per operation: it keeps no state.
type Service struct {
	// Config is the hub configuration (hub.toml).
	Config *config.Config
	// Secrets is the keychain; nil leaves only environment variables.
	Secrets tracker.SecretGetter
	// Projects lists the hub projects the tracker is synchronized for; nil
	// means none.
	Projects ProjectLister
	// HubDir is the directory where the tracker engine keeps its state;
	// empty means config.HubDir().
	HubDir string
}

// New returns a service on the hub configuration, the keychain and the
// project store (either may be nil).
func New(cfg *config.Config, secrets tracker.SecretGetter, projects ProjectLister) *Service {
	return &Service{Config: cfg, Secrets: secrets, Projects: projects}
}

// ResolveTeam is the one rule that says which team applies to a project: the
// team the project belongs to when team features are active for it, else the
// hub team (the default team of hub.toml). A nil project is the hub team.
// Enabled is false when neither applies. A team without ID gets the name of
// its repository, which is the name its keychain keys use.
func (s *Service) ResolveTeam(project *domain.Project) config.ResolvedTeamConfig {
	tc := s.resolveTeam(project)
	if tc.TeamID == "" && tc.StateRepo != "" {
		tc.TeamID = config.RepoNameFromRemote(tc.StateRepo)
	}
	if tc.StatePath == "" && tc.StateRepo != "" {
		tc.StatePath = config.DefaultTeamStatePath()
	}
	return tc
}

func (s *Service) resolveTeam(project *domain.Project) config.ResolvedTeamConfig {
	if project != nil {
		if tc := config.ResolveTeamForProject(s.Config, project); tc.Enabled {
			return tc
		}
	}
	hub := s.Config.ActiveTeam()
	return config.ResolvedTeamConfig{
		Enabled:   hub.Enabled,
		TeamID:    hub.ID,
		StateRepo: hub.StateRepo,
		StatePath: hub.StatePath,
		MemberID:  hub.MemberID,
	}
}

// TokenSource names where a GitLab token was found.
type TokenSource = tracker.TokenSource

// TokenOption narrows the GitLab token cascade.
type TokenOption func(*tracker.GitLabTokenQuery)

// WithProject adds the MCP token private to a hub project to the cascade.
func WithProject(projectID string) TokenOption {
	return func(q *tracker.GitLabTokenQuery) { q.ProjectID = projectID }
}

// WithTrackerKey adds the key set by tracker_token_key to the cascade.
func WithTrackerKey(key string) TokenOption {
	return func(q *tracker.GitLabTokenQuery) { q.TrackerKey = key }
}

// GitLabToken returns the GitLab token to use and where it comes from, empty
// when there is none. It is the only cascade (tracker.ResolveGitLabToken):
//
//  1. GITLAB_TOKEN
//  2. tracker_token_key (WithTrackerKey)
//  3. the token of the project (WithProject)
//  4. the token of the team teamID
//  5. the hub MCP token ([mcp.gitlab] token key, then openhub.mcp.gitlab.token)
//  6. the former tracker key openhub.tracker.gitlab.token
func (s *Service) GitLabToken(ctx context.Context, teamID string, opts ...TokenOption) (string, TokenSource) {
	q := s.tokenQuery(teamID)
	for _, o := range opts {
		o(&q)
	}
	return tracker.ResolveGitLabToken(ctx, q)
}

func (s *Service) tokenQuery(teamID string) tracker.GitLabTokenQuery {
	q := tracker.GitLabTokenQuery{TeamID: teamID, Secrets: s.Secrets}
	if s.Config != nil {
		q.MCPKey = s.Config.MCP.Gitlab.Token
	}
	return q
}
